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

const HEARTBEAT_INTERVAL = time.Second * 2

////////////////////////////////////////////////////////////////////////////////////////////

// Workers

func PeerWorker() {
	for {
		peerLock.Lock()

		// TODO: get number of connected peers, not all peers
		if len(peers) < NetworkSettings.MinimumPeerConnections {
			getMorePeers()
		}

		peerLock.Unlock()

		time.Sleep(10 * time.Second)
	}
}

func getMorePeers() {
	newPeers, err := server.GetNodes(NetworkSettings.UniqueUserID)
	if err != nil {
		log.Println("Error retrieving more peer addresses from server:", err)
		return
	}

	for _, p := range newPeers {
		if peers[p.ClientID] != nil || p.ClientID == NetworkSettings.UniqueUserID {
			// Already have this peer, or it's us
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
		// This side listens for heartbeats from the new peer, so no client clock is set
		LastHeartbeat: time.Now(),
	}, nil
}

func OutgoingWorker() {
	for {
		update := <- OutgoingUpdates

		peerLock.Lock()

		for _, peer := range peers {
			if update.PlayerID == peer.ClientID {
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

// TODO: add workers to monitor heartbeats and send heartbeats
func HeartbeatWorker(clientID uint64, peerConn *clientlib.ClientClockRemote) {
	for {
		// Peer has been removed; stop sending heartbeats
		//if _, ok := peers[clientID]; !ok {
		//	log.Printf("Peer %d has failed; no longer sending heartbeats", clientID)
		//	peerLock.Unlock()
		//	return
		//}

		beat := make(chan error, 1)

		go func() { beat <- peerConn.Heartbeat(NetworkSettings.UniqueUserID) }()

		select {
		case e := <-beat:
			if e != nil {
				// TODO handle disconnection
				log.Printf("Could not send heartbeat. Peer with ID %d is disconnected\n", clientID)
				continue
			}
			// TODO The client has reconnected; reset to true
			log.Printf("Sent heartbeat to peer with ID %d\n", clientID)

		case <-time.After(HEARTBEAT_INTERVAL):
			// TODO handle disconnection
			log.Printf("Could not send heartbeat. Peer with ID %d is disconnected\n", clientID)
		}

		time.Sleep(HEARTBEAT_INTERVAL)
	}
}

func HeartbeatMonitorWorker(clientID uint64) {
	time.Sleep(HEARTBEAT_INTERVAL * 2) // Grace period before monitoring begins
	for {
		peerLock.Lock()
		// Peer has been removed; stop monitoring
		//if _, ok := peers[clientID]; !ok {
		//	peerLock.Unlock()
		//	continue //return
		//}

		// Check time since last heartbeat
		if time.Since(peers[clientID].LastHeartbeat) > (HEARTBEAT_INTERVAL) {
			log.Printf("Did not receive heartbeat on time. Peer with ID %d has timed out\n", clientID)
			// TODO Handle peer failure
			peerLock.Unlock()
			continue //return
		}

		log.Printf("Received heartbeat. Peer with ID %d is alive\n", clientID)
		peerLock.Unlock()
		time.Sleep(HEARTBEAT_INTERVAL)
	}
}


////////////////////////////////////////////////////////////////////////////////////////////

// Player-to-Player API

func (*ClientListener) NotifyUpdate(clientID uint64, update clientlib.Update) error {
	RecordUpdates <- update
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
		// No need to set LastHeartbeat; will be sending heartbeats to this peer, not monitoring them.
	}
	peerLock.Unlock()

	return nil
}