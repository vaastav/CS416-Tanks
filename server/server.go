/*
	Implements a thin server for P2P Battle Tanks game for CPSC 416 Project 2.
	This server is responsible for peer discovery and clock synchronisation

	Usage:
		go run server.go <IP Address : Port>
*/

package main

import (
	"../clientlib"
	"../serverlib"
	"math/rand"
	"errors"
	"log"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type TankServer int

type Status int

const (
	// Connected mode.
	DISCONNECTED Status = iota
	// Disconnected mode.
	CONNECTED
)

type Connection struct {
	status Status
	displayName string
	address string
}

// Error definitions

// Contains bad ID
type InvalidClientError string

func (e InvalidClientError) Error() string {
	return fmt.Sprintf("Invalid Client Id [%s]. Please register.", e)
}

// State Variables

var connections = struct {
	sync.RWMutex
	m map[uint64]Connection
}{m : make(map[uint64]Connection)}

// Server Implementation

func (s *TankServer) Register (peerInfo serverlib.PeerInfo, settings *clientlib.PeerNetSettings) error {
	log.Println("register", peerInfo.ClientID)
	newSettings := clientlib.PeerNetSettings{
		MinimumPeerConnections: 1,
		UniqueUserID: peerInfo.ClientID,
		DisplayName: peerInfo.DisplayName,
	}

	*settings = newSettings

	connections.Lock()
	connections.m[peerInfo.ClientID] = Connection{
		status: DISCONNECTED,
		displayName: peerInfo.DisplayName,
		address: peerInfo.Address,
	}

	connections.Unlock()
	return nil
}

func (s *TankServer) Connect (clientID uint64, ack *bool) error {
	log.Println("connect", clientID)
	connections.Lock()
	c, ok := connections.m[clientID]
	if !ok {
		connections.Unlock()
		return InvalidClientError(clientID)
	}
	if c.status == CONNECTED {
		connections.Unlock()
		return errors.New("client already connected")
	}
	c.status = CONNECTED
	connections.m[clientID] = c
	connections.Unlock()
	*ack = true
	return nil
}

func (s *TankServer) GetNodes (clientID uint64, addrSet *[]serverlib.PeerInfo) error {
	log.Println("getnodes", clientID)
	connections.RLock()
	defer connections.RUnlock()

	if _, ok := connections.m[clientID]; !ok {
		return InvalidClientError(clientID)
	}

	peerAddresses := make([]serverlib.PeerInfo, 0, len(connections.m)-1)

	for key, connection := range connections.m {
		if key == clientID {
			continue
		}
		peerAddresses = append(peerAddresses, serverlib.PeerInfo{
			Address: connection.address,
			ClientID: key,
			DisplayName: connection.displayName,
		})
	}

	// TODO : Filter the addresses better for network topology
	*addrSet = peerAddresses
	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) != 2 {
		log.Fatal("Usage: go run server.go <IP Address : Port")
	}
	ipAddr := os.Args[1]
	// TODO : Does the server need to have its connection as UDP as well?
	// I imagine we can let it be as TCP
	serverAddr, err := net.ResolveTCPAddr("tcp", ipAddr)
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	server := new(TankServer)
	rpc.Register(server)
	fmt.Println("Listening now")
	rpc.Accept(inbound)
}
