package main

import (
	"../clientlib"
	"log"
	"net"
	"net/rpc"
	"time"
)

type ClockController int

func (c *ClockController) TimeRequest(request int, t * time.Time) error {
	*t = Clock.GetCurrentTime()
	return nil
}

func (c *ClockController) SetOffset(offset time.Duration, ack * bool) error {
	Clock.Offset = offset
	return nil
}

func (c *ClockController) Heartbeat(clientID uint64, ack *bool) error {
	peerLock.Lock()
	defer peerLock.Unlock()

	log.Println("ba-bump")
	if _, ok := peers[clientID]; ok {
		peers[clientID].LastHeartbeat = Clock.GetCurrentTime()
	}

	return nil
}

func (c *ClockController) NotifyConnection(clientID uint64, ack *bool) error {
	peerLock.Lock()
	defer peerLock.Unlock()

	log.Println("TOLD SOMETHING IS CONNECTED")
	updateConnectionStatus(clientID, clientlib.CONNECTED)
	log.Printf("%s\n", peers)
	// TODO: update associated sprite

	return nil
}

func (c *ClockController) UpdateConnectionState(connectionInfo map[uint64]clientlib.Status, ack *bool) error {
	peerLock.Lock()
	defer peerLock.Unlock()

	log.Println("Updating connection state")
	for id, status := range connectionInfo {
		updateConnectionStatus(id, status)
	}
	log.Printf("PEERS %s\n", peers)

	return nil
}

//func (c *ClockController) NotifyDisconnection(clientID uint64, ack *bool) error {
//	peerLock.Lock()
//	defer peerLock.Unlock()
//
//	log.Println("TOLD SOMETHING IS DISCONNECTED")
//	updateConnectionStatus(clientID, clientlib.DISCONNECTED)
//	log.Printf("%s\n", peers)
//	// TODO: update associated sprite
//
//	return nil
//}

func (c *ClockController) TestConnection(request int, ack *bool) error {
	*ack = true
	log.Println("PING!")
	return nil
}

func ClockWorker() {
	inbound, err := net.ListenTCP("tcp", RPCAddr)
	if err != nil {
		// OK to exit here; we can't handle this failure
		log.Fatal(err)
	}

	server := new(ClockController)
	rpc.Register(server)
	//rpc.Accept(inbound)
	for {
		conn, _ := inbound.Accept()
		go rpc.ServeConn(conn)
	}
}