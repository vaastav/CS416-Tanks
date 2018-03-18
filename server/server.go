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
	"crypto/rand"
	"errors"
	"log"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
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
	m map[string]Connection
}{m : make(map[string]Connection)}

// Server Implementation

func getUniqueUserID() (string, error) {
	b := make([]byte, 16)
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }

    uuid := fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

    return uuid, nil
}

func (s *TankServer) Register (peerInfo serverlib.PeerInfo, settings *clientlib.PeerNetSettings) error {

	// TODO Refactor this magic number
	uuid, err := getUniqueUserID()
	if err != nil {
		return err
	}
	newSettings := clientlib.PeerNetSettings{1, uuid, peerInfo.DisplayName}
	*settings = newSettings

	connections.Lock()
	connections.m[uuid] = Connection{status : DISCONNECTED, displayName : peerInfo.DisplayName, address : peerInfo.Address.String()}
	connections.Unlock()
	return nil
}

func (s *TankServer) Connect (settings clientlib.PeerNetSettings, ack *bool) error {
	connections.Lock()
	c, ok := connections.m[settings.UniqueUserID]
	if !ok {
		connections.Unlock()
		return InvalidClientError(settings.UniqueUserID)
	}
	if c.status == CONNECTED {
		connections.Unlock()
		return errors.New("Client already connected")
	}
	c.status = CONNECTED
	connections.m[settings.UniqueUserID] = c
	connections.Unlock()
	*ack = true
	return nil
}

func (s *TankServer) GetNodes (settings clientlib.PeerNetSettings, addrSet *[]string) error {
	connections.RLock()
	defer connections.RUnlock()

	if _, ok := connections.m[settings.UniqueUserID]; !ok {
		return InvalidClientError(settings.UniqueUserID)
	}

	peerAddresses := make([]string, 0, len(connections.m)-1)

	for key, connection := range connections.m {
		if key == settings.UniqueUserID {
			continue
		}
		peerAddresses = append(peerAddresses, connection.address)
	}

	// TODO : Filter the addresses better for network topology
	*addrSet = peerAddresses
	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run server.go <IP Address : Port")
	}
	ip_addr := os.Args[1]
	// TODO : Does the server need to have its connection as UDP as well?
	// I imagine we can let it be as TCP
	server_addr, err := net.ResolveTCPAddr("tcp", ip_addr)
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", server_addr)
	if err != nil {
		log.Fatal(err)
	}

	server := new(TankServer)
	rpc.Register(server)
	fmt.Println("Listening now")
	rpc.Accept(inbound)
}
