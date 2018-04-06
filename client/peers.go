package main

import (
	"../clientlib"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type PeerRecord struct {
	ClientID      uint64
	Api           *clientlib.ClientAPIRemote
	Rpc           *clientlib.ClientClockRemote
	LastHeartbeat time.Time
}

type ClientListener int

var (
	OutgoingUpdates = make(chan clientlib.Update, 1000)
)

var (
	peerLock = sync.Mutex{}
	peers    = make(map[uint64]*PeerRecord)
)

const (
	HEARTBEAT_INTERVAL       = time.Second * 1
	HEARTBEAT_TIMEOUT        = HEARTBEAT_INTERVAL * 2
	FAILURE_NOTIFICATION_TTL = 3 // TODO: need to decide on number
)

type ExistingPeerError string

func (e ExistingPeerError) Error() string {
	return fmt.Sprintf("Client already knows peer id [%s].", string(e))
}

////////////////////////////////////////////////////////////////////////////////////////////

// Workers

func PeerWorker() {
	for {
		peerLock.Lock()

		if len(peers) < NetworkSettings.MinimumPeerConnections {
			getMorePeers()
		}

		peerLock.Unlock()

		time.Sleep(5 * time.Second)
	}
}

func getMorePeers() {
	newPeers, err := Server.GetNodes(NetworkSettings.UniqueUserID, Logger)
	if err != nil {
		log.Fatal("Error retrieving more peer addresses from server:", err)
	}

	for _, p := range newPeers {
		if peers[p.ClientID] != nil || p.ClientID == NetworkSettings.UniqueUserID {
			continue
		}

		log.Println("Adding peer", p.ClientID, "at address", p.Address)

		peer, err := newPeer(p.ClientID, p.Address, p.RPCAddress)
		if err != nil {
			log.Println("Error adding new peer at address", p.Address, "with error:", err)
			continue
		}

		peers[peer.ClientID] = peer
		go HeartbeatMonitorWorker(peer.ClientID)
	}
}

func newPeer(id uint64, addr string, rpcAddr string) (*PeerRecord, error) {
	// Try to connect
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	api := clientlib.NewClientAPIRemote(conn, PeerLogger, IsLogUpdates)

	client, err := rpc.Dial("tcp", rpcAddr)
	if err != nil {
		return nil, err
	}
	clockClient := clientlib.NewClientClockRemoteAPI(client)

	if err = api.Register(NetworkSettings.UniqueUserID, LocalAddr.String(), RPCAddr.String()); err != nil {
		conn.Close()
		client.Close()
		return nil, err
	}

	return &PeerRecord{
		ClientID:      id,
		Api:           api,
		Rpc:           clockClient,
		LastHeartbeat: Clock.GetCurrentTime(),
	}, nil
}

func OutgoingWorker() {
	for {
		update := <-OutgoingUpdates

		peerLock.Lock()

		for _, peer := range peers {
			if update.PlayerID == peer.ClientID {
				continue
			}

			err := peer.Api.NotifyUpdate(NetworkSettings.UniqueUserID, update)
			if err != nil {
				// Too noisy to log
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
	apiListener := clientlib.NewClientAPIListener(&listener, conn, PeerLogger, IsLogUpdates)

	log.Println("Listening on", LocalAddr)

	for {
		err = apiListener.Accept()
		if err != nil {
			log.Println("Error listening on UDP connection:", err)
		}
	}
}

func HeartbeatWorker(clientID uint64) {
	for {
		beat := make(chan error, 1)

		var conn *clientlib.ClientClockRemote
		peerLock.Lock()
		if _, ok := peers[clientID]; !ok {
			peerLock.Unlock()
			return
		} else {
			conn = peers[clientID].Rpc
		}
		peerLock.Unlock()

		go func() { beat <- conn.Heartbeat(NetworkSettings.UniqueUserID) }()

		select {
		case e := <-beat:
			if e != nil {
				peerLock.Lock()
				if _, ok := peers[clientID]; ok {
					if err := peers[clientID].Rpc.Ping(); err != nil {
						handleDisconnection(clientID)
						peerLock.Unlock()
						return
					}
				}
				peerLock.Unlock()
				break
			}
			log.Printf("Heartbeat() Peer %d is alive\n", clientID)
		case <-time.After(HEARTBEAT_TIMEOUT):
			peerLock.Lock()
			if _, ok := peers[clientID]; ok {
				if err := peers[clientID].Rpc.Ping(); err != nil {
					handleDisconnection(clientID)
					peerLock.Unlock()
					return
				}
			}
			peerLock.Unlock()
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func HeartbeatMonitorWorker(clientID uint64) {
	time.Sleep(HEARTBEAT_INTERVAL) // Grace period before monitoring begins
	for {
		time.Sleep(HEARTBEAT_INTERVAL)
		peerLock.Lock()

		if _, ok := peers[clientID]; !ok {
			peerLock.Unlock()
			return
		}

		if time.Since(peers[clientID].LastHeartbeat) > HEARTBEAT_TIMEOUT {
			if err := peers[clientID].Rpc.Ping(); err != nil {
				handleDisconnection(clientID)
				peerLock.Unlock()
				return
			}
			peers[clientID].LastHeartbeat = Clock.GetCurrentTime()
			peerLock.Unlock()
			continue
		}

		log.Printf("HeartbeatMonitor() Peer %d is alive\n", clientID)
		peerLock.Unlock()
	}
}

// NOTE: must acquire lock before calling
func handleDisconnection(clientID uint64) {
	log.Println("handleDisconnection()", clientID)
	err := Server.NotifyFailure(clientID)
	if err != nil {
		log.Fatalf("handleDisconnection() Error notifying server of disconnected peer %d: %s\n", clientID, err)
	}

	if err := removePeer(clientID); err != nil {
		log.Println("handleDisconnection() error removing peer", clientID)
	}

	for _, peer := range peers {
		err = peer.Api.NotifyFailure(clientID, FAILURE_NOTIFICATION_TTL)
		if err != nil {
			log.Printf("handleDisconnection() Error notifying peer %d of disconnected peer %d", peer.ClientID, clientID)
		}
	}
}

func removePeer(clientID uint64) (err error) {
	if peer, ok := peers[clientID]; ok {
		if err = peer.Api.Conn.Close(); err != nil {
			log.Println("removePeer() error closing connection with peer", clientID)
		}
		if err = peer.Rpc.Conn.Close(); err != nil {
			log.Println("removePeer() error closing connection with peer", clientID)
		}

		RecordUpdates <- clientlib.DeadPlayer(clientID).Timestamp(Clock.GetCurrentTime())
		delete(peers, clientID)
	}

	return err
}

////////////////////////////////////////////////////////////////////////////////////////////

// Player-to-Player API

func (*ClientListener) NotifyUpdate(clientID uint64, update clientlib.Update) error {
	RecordUpdates <- update
	return nil
}

func (*ClientListener) NotifyFailure(clientID uint64, ttl int) error {
	log.Println("NotifyFailure()", clientID)
	peerLock.Lock()

	if err := removePeer(clientID); err != nil {
		log.Println("NotifyFailure() error removing peer", clientID)
	}

	if ttl > 0 {
		ttl -= 1
		for _, peer := range peers {
			err := peer.Api.NotifyFailure(clientID, ttl)
			if err != nil {
				log.Printf("NotifyFailure() Error notifying peer %d of disconnected peer %d: %s\n", peer.ClientID, clientID, err)
			}
		}
	}

	peerLock.Unlock()
	return nil
}

func (*ClientListener) Register(clientID uint64, address string, tcpAddress string) error {
	log.Println("Register()", clientID, "address", address)

	// Don't do anything if you already know this peer
	peerLock.Lock()
	if _, ok := peers[clientID]; ok {
		peerLock.Unlock()
		return ExistingPeerError(clientID)
	}
	peerLock.Unlock()

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

	// Write down this new peer
	peerLock.Lock()
	peers[clientID] = &PeerRecord{
		ClientID:      clientID,
		Api:           clientlib.NewClientAPIRemote(conn, PeerLogger, IsLogUpdates),
		Rpc:           clientlib.NewClientClockRemoteAPI(client),
		LastHeartbeat: Clock.GetCurrentTime(),
	}
	peerLock.Unlock()

	go HeartbeatWorker(clientID)

	return nil
}
