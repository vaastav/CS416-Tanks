package main

import (
	"log"
	"net"
	"../clientlib"
	"sync"
	"time"
)

type PeerRecord struct {
	ClientID uint64
	Api      *clientlib.ClientAPIRemote
}

type ClientListener int

var (
	OutgoingUpdates = make(chan clientlib.Update, 1000)
)

var (
	peerLock = sync.Mutex{}
	peers    = make(map[uint64]*PeerRecord)
)

func PeerWorker() {
	for {
		peerLock.Lock()

		if len(peers) < NetworkSettings.MinimumPeerConnections {
			getMorePeers()
		}

		peerLock.Unlock()

		time.Sleep(10 * time.Second)
	}
}

func getMorePeers() {
	newPeers, err := Server.GetNodes(NetworkSettings.UniqueUserID, Logger)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range newPeers {
		if peers[p.ClientID] != nil || p.ClientID == NetworkSettings.UniqueUserID {
			// already have this peer, or it's us
			continue
		}

		log.Println("Adding peer", p.ClientID, "address", p.Address)

		peer, err := newPeer(p.ClientID, p.Address)
		if err != nil {
			log.Fatal(err)
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
	err = api.Register(NetworkSettings.UniqueUserID, LocalAddr.String())
	if err != nil {
		log.Fatal(err)
	}

	return &PeerRecord{
		ClientID: id,
		Api:      api,
	}, nil
}

func OutgoingWorker() {
	for {
		update := <-OutgoingUpdates

		peerLock.Lock()

		for _, peer := range peers {
			if update.PlayerID == peer.ClientID {
				// Skip notifying clients about their own updates
				continue
			}

			err := peer.Api.NotifyUpdate(NetworkSettings.UniqueUserID, update)
			if err != nil {
				log.Fatal(err)
			}
		}

		peerLock.Unlock()
	}
}

func ListenerWorker() {
	conn, err := net.ListenUDP("udp", LocalAddr)
	if err != nil {
		log.Fatal(err)
	}

	var listener ClientListener
	apiListener := clientlib.NewClientAPIListener(&listener, conn)

	log.Println("Listening on", LocalAddr)

	for {
		err = apiListener.Accept()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (*ClientListener) NotifyUpdate(clientID uint64, update clientlib.Update) error {
	// Notify about this new update
	RecordUpdates <- update
	return nil
}

func (*ClientListener) Register(clientID uint64, address string) error {
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

	// Write down this new peer
	peerLock.Lock()
	peers[clientID] = &PeerRecord{
		ClientID: clientID,
		Api:      clientlib.NewClientAPIRemote(conn),
	}
	peerLock.Unlock()

	return nil
}
