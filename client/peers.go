package main

import (
	"../clientlib"
	"net"
	"log"
	"sync"
	"time"
	"net/rpc"
)

type PeerRecord struct {
	ClientID uint64
	Api *clientlib.ClientAPIRemote
	ConnectionStatus Status
	LastHeartbeat time.Time
}

type ClientListener int

var (
	OutgoingUpdates = make(chan clientlib.Update, 1000)
)

var (
	peerLock = sync.Mutex{}
	peers = make(map[uint64]*PeerRecord)
)

const (
	HEARTBEAT_INTERVAL_SEND = time.Second * 2
	HEARTBEAT_INTERVAL_RECV = time.Second * 3
	HEARTBEAT_TIMEOUT = time.Second * 5
	FAILURE_NOTIFICATION_TTL = 3 // TODO: need to decide on number
)

type Status int
const (
	CONNECTED Status = iota
	DISCONNECTED
)

////////////////////////////////////////////////////////////////////////////////////////////

// Workers

func PeerWorker() {
	for {
		peerLock.Lock()

		if countPeers() < NetworkSettings.MinimumPeerConnections {
			getMorePeers()
		}

		peerLock.Unlock()

		time.Sleep(10 * time.Second)
	}
}

func countPeers() int {
	count := 0
	for _, peer := range peers {
		if peer.ConnectionStatus == CONNECTED {
			count += 1
		}
	}
	return count
}

func getMorePeers() {
	newPeers, err := server.GetNodes(NetworkSettings.UniqueUserID)
	if err != nil {
		log.Println("Error retrieving more peer addresses from server:", err)
		return
	}

	for _, p := range newPeers {
		if p.ClientID == NetworkSettings.UniqueUserID {
			continue
		}
		if peers[p.ClientID] != nil {
			// If we're receiving this node from the server, it's (re)connected; mark node connected if otherwise
			if peers[p.ClientID].ConnectionStatus == DISCONNECTED {
				updateConnectionStatus(p.ClientID, CONNECTED)
			}
			continue
		}

		log.Println("Adding peer", p.ClientID, "at address", p.Address)

		peer, err := newPeer(p.ClientID, p.Address)
		if err != nil {
			log.Println("Error adding new peer at address", p.Address, "with error:", err)
			continue
		}

		peers[peer.ClientID] = peer
	}
}

func newPeer(id uint64, addr string) (*PeerRecord, error) {
	// Try to connect
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	api := clientlib.NewClientAPIRemote(conn)
	err = api.Register(NetworkSettings.UniqueUserID, LocalAddr.String(), RPCAddr.String())
	if err != nil {
		return nil, err
	}
	go HeartbeatMonitorWorker(id)

	return &PeerRecord{
		ClientID: id,
		Api: api,
		ConnectionStatus: CONNECTED,
		LastHeartbeat: Clock.GetCurrentTime(),
	}, nil
}

func OutgoingWorker() {
	for {
		update := <- OutgoingUpdates

		peerLock.Lock()

		for _, peer := range peers {
			if update.PlayerID == peer.ClientID { // TODO: what about disconnected peers?
				// Skip notifying clients about their own updates
				continue
			}

			err := peer.Api.NotifyUpdate(NetworkSettings.UniqueUserID, update)
			if err != nil {
				// TODO: until updates get thrown out, this is too noisy to log
			}
		}

		peerLock.Unlock()
	}
}

func ListenerWorker() {
	conn, err := net.ListenUDP("udp", LocalAddr)
	if err != nil {
		// OK to exit here; we can't handle this failure
		log.Fatal(err)
	}

	var listener ClientListener
	apiListener := clientlib.NewClientAPIListener(&listener, conn)

	log.Println("Listening on", LocalAddr)

	for {
		err = apiListener.Accept()
		if err != nil {
			log.Println("Error listening on UDP connection:", err)
		}
	}
}

func HeartbeatWorker(clientID uint64, peerConn *clientlib.ClientClockRemote) {
	for {
		beat := make(chan error, 1)

		go func() { beat <- peerConn.Heartbeat(NetworkSettings.UniqueUserID) }()

		select {
		case e := <-beat:
			// Option 1: If Heartbeat() returns error, handle disconnection
			if e != nil {
				peerLock.Lock()
				handleDisconnection(clientID)
				peerLock.Unlock()
				break
			}
			// Option 2: If peer was previously disconnected, handle reconnection
			log.Printf("[Heartbeat] Peer %d is alive\n", clientID)
			peerLock.Lock()
			handleReconnection(clientID)
			peerLock.Unlock()
		case <-time.After(HEARTBEAT_TIMEOUT):
			// Option 3: If Heartbeat() times out, handle disconnection
			peerLock.Lock()
			handleDisconnection(clientID)
			peerLock.Unlock()
		}

		time.Sleep(HEARTBEAT_INTERVAL_SEND)
	}
}

func HeartbeatMonitorWorker(clientID uint64) {
	time.Sleep(HEARTBEAT_INTERVAL_RECV) // Grace period before monitoring begins
	for {
		time.Sleep(HEARTBEAT_INTERVAL_RECV)
		peerLock.Lock()
		// 1. Check time since last heartbeat
		if time.Since(peers[clientID].LastHeartbeat) > (HEARTBEAT_INTERVAL_RECV) {
			handleDisconnection(clientID)
			peerLock.Unlock()
			continue
		}

		// 2. If peer was previously marked as disconnected, notify server of reconnection
		handleReconnection(clientID)
		log.Printf("[HeartbeatMonitor] Peer %d is alive\n", clientID)

		peerLock.Unlock()
	}
}

// NOTE: must acquire lock before calling
func handleReconnection(clientID uint64) {
	if peers[clientID].ConnectionStatus == DISCONNECTED {
		ack, err := server.Connect(clientID)
		if err != nil || !ack {
			log.Println("[HandleReconnection] Error notifying server of reconnected peer", clientID)
			return
		}
		// What if we wait for server to decide to update connection status? TODO
		// updateConnectionStatus(clientID, CONNECTED)
	}
}

// NOTE: must acquire lock before calling
func handleDisconnection(clientID uint64) {
	if peers[clientID].ConnectionStatus == CONNECTED {
		log.Printf("[HandleDisconnection] Peer %d has timed out\n", clientID)
		// 1. Mark peer disconnected
		updateConnectionStatus(clientID, DISCONNECTED)

		// 2. Notify server of disconnection
		ack, err := server.NotifyDisconnection(clientID)
		if err != nil || !ack {
			log.Println("[HandleDisconnection] Error notifying server of disconnected peer", clientID)
			return
		}

		// 3. Fade out associated sprite TODO

		// 4. Notify peers of disconnection
		for _, peer := range peers {
			if peer.ConnectionStatus == DISCONNECTED {
				continue
			}
			err = peer.Api.NotifyFailure(clientID, FAILURE_NOTIFICATION_TTL)
			if err != nil {
				log.Printf("[HandleDisconnection] Error notifying peer %d of disconnected peer %d", peer.ClientID, clientID)
			}
		}
	}
}

// NOTE: must acquire lock before calling
func updateConnectionStatus(clientID uint64, status Status) {
	if _, ok := peers[clientID]; ok {
		peers[clientID].ConnectionStatus = status
	}
}

////////////////////////////////////////////////////////////////////////////////////////////

// Player-to-Player API

func (*ClientListener) NotifyUpdate(clientID uint64, update clientlib.Update) error {
	RecordUpdates <- update
	return nil
}

func (*ClientListener) NotifyFailure(clientID uint64, ttl int) error {
	log.Printf("[NotifyFailure] Received notification of failed peer %d, with TTL %d\n", clientID, ttl)
	peerLock.Lock()

	// Mark peer disconnected TODO: use updateConnectionStatus()
	if _, exists := peers[clientID]; exists {
		if peers[clientID].ConnectionStatus == CONNECTED {
			log.Printf("[NotifyFailure ~ STRANGE STATE] Received notification of failed peer %d, but peer is marked as connected\n", clientID)
		}
		peers[clientID].ConnectionStatus = DISCONNECTED
	}

	// Flood failure message to other peers
	if ttl > 0 {
		ttl -= 1
		for _, peer := range peers {
			if peer.ConnectionStatus == DISCONNECTED {
				continue
			}
			err := peer.Api.NotifyFailure(clientID, ttl)
			if err != nil {
				log.Printf("[NotifyFailure] Error notifying peer %d of disconnected peer %d", peer.ClientID, clientID)
			}
		}
	}

	peerLock.Unlock()

	return nil
}

func (*ClientListener) Register(clientID uint64, address string, tcpAddress string) error {
	log.Println("Register", clientID, "address", address)

	// Try to connect
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return err
	}

	client, err := rpc.Dial("tcp", tcpAddress)
	if err != nil {
		return err
	}
	clockClient := clientlib.NewClientClockRemoteAPI(client)
	go HeartbeatWorker(clientID, clockClient)

	// Write down this new peer
	peerLock.Lock()
	peers[clientID] = &PeerRecord{
		ClientID: clientID,
		Api: clientlib.NewClientAPIRemote(conn),
		ConnectionStatus: CONNECTED,
		// No need to set LastHeartbeat; will be sending heartbeats to this peer, not monitoring them
	}
	peerLock.Unlock()

	return nil
}
