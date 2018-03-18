package main

import (
	"../clientlib"
	"../serverlib"
	"net"
	"log"
	"sync"
)

type PeerRecord struct {
	ClientID uint64
	Api *clientlib.ClientAPIRemote
}

type ClientListener int

var (
	OutgoingUpdates = make(chan clientlib.Update, 100)
)

var (
	peerLock = sync.Mutex{}
	peers = make(map[uint64]*PeerRecord)
	server serverlib.ServerAPI
)

// TODO talk to the server and wire up other clients completely

func OutgoingWorker() {
	for {
		update := <- OutgoingUpdates

		peerLock.Lock()

		for _, peer := range peers {
			err := peer.Api.NotifyUpdate(ClientID, update)
			if err != nil {
				log.Fatal(err)
			}
		}

		peerLock.Unlock()
	}
}

func ListenerWorker(localAddr string) {
	localUDPAddr, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, localUDPAddr)
	if err != nil {
		log.Fatal(err)
	}

	var listener ClientListener
	apiListener := clientlib.NewClientAPIListener(&listener, conn)

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
		Api: clientlib.NewClientAPIRemote(conn),
	}
	peerLock.Unlock()

	return nil
}