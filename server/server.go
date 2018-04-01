/*
	Implements a thin server for P2P Battle Tanks game for CPSC 416 Project 2.
	This server is responsible for peer discovery and clock synchronization

	Usage:
		go run server.go <IP Address : Port>
*/

package main

import (
	"../clientlib"
	"../clocklib"
	"../serverlib"
	"math/rand"
	"errors"
	"log"
	"fmt"
	"github.com/DistributedClocks/GoVector/govec"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type TankServer int

type Connection struct {
	status clientlib.Status
	displayName string
	address string
	rpcAddress string
	offset time.Duration
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

var disconnectedNodes = struct {
	sync.RWMutex
	m map[uint64][]uint64 /* key = disconnected node ID; value = list of node IDs who've reported node as disconnected */
}{m : make(map[uint64][]uint64)}

var Logger *govec.GoLog

var Clock *clocklib.ClockManager = &clocklib.ClockManager{0}

// Server Implementation

func encodeToByte(v interface{}) (data []byte) {
	s := fmt.Sprintf("%v", v)
	return []byte(s)
}

func (s *TankServer) syncClocks() {
	Logger.LogLocalEvent("Syncing Clocks")
	connections.Lock()
	if len(connections.m) == 0 {
		connections.Unlock()
		return
	}

	m := make(map[uint64]time.Duration)
	var offsetTotal time.Duration = Clock.Offset
	var offsetNum time.Duration = 1

	for key, connection := range connections.m {
		if connection.status == clientlib.DISCONNECTED {
			continue
		}
		s := fmt.Sprintf("Requesting client %d for time", key)
		// Update to prepare send
		Logger.LogLocalEvent(s)
		client, err := rpc.Dial("tcp", connection.rpcAddress)
		if err != nil {
			// TODO : Better failure handling
			log.Fatal(err)
		}

		before := Clock.GetCurrentTime()
		
		clockClient := clientlib.NewClientClockRemoteAPI(client)
		t, err := clockClient.TimeRequest()
		if err != nil {
			log.Fatal(err)
		}
		after := Clock.GetCurrentTime()

		clientTime := t.Add(after.Sub(before) / 2)
		clientOffset := clientTime.Sub(after)
		m[key] = clientOffset
		offsetTotal += clientOffset
		offsetNum++
	}

	offsetAverage := offsetTotal / offsetNum

	for key, connection := range connections.m {
		if connection.status == clientlib.DISCONNECTED {
			continue
		}

		// Update to prepare send
		s := fmt.Sprintf("Telling client %d to set offset", key)
		Logger.LogLocalEvent(s)
		client, err := rpc.Dial("tcp", connection.rpcAddress)
		if err != nil {
			// TODO : Better failure handling
			log.Fatal(err)
		}

		offset := offsetAverage - m[key]
		connection.offset = offset
		connections.m[key] = connection

		clockClient := clientlib.NewClientClockRemoteAPI(client)
		err = clockClient.SetOffset(offset)
		if err != nil {
			log.Fatal(err)
		}
	}
	Logger.LogLocalEvent("Setting local clock offset")
	Clock.Offset += offsetAverage
	connections.Unlock()
}

func (s *TankServer) Register (peerInfo serverlib.PeerInfo, settings *clientlib.PeerNetSettings) error {
	log.Println("Register()", peerInfo.ClientID)
	var incomingMessage int
	Logger.UnpackReceive("[Register] received from client", encodeToByte(peerInfo), &incomingMessage)
	newSettings := clientlib.PeerNetSettings{
		MinimumPeerConnections: 1,
		UniqueUserID: peerInfo.ClientID,
		DisplayName: peerInfo.DisplayName,
	}

	*settings = newSettings

	connections.Lock()
	connections.m[peerInfo.ClientID] = Connection{
		status: clientlib.DISCONNECTED,
		displayName: peerInfo.DisplayName,
		address: peerInfo.Address,
		rpcAddress: peerInfo.RPCAddress,
		offset: 0,
	}

	connections.Unlock()
	return nil
}

func (s *TankServer) Connect (clientID uint64, ack *bool) error {
	log.Println("Connect()", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[Connect] received from client", encodeToByte(clientID), &incomingMessage)
	connections.Lock()
	c, ok := connections.m[clientID]
	if !ok {
		connections.Unlock()
		return InvalidClientError(clientID)
	}
	if c.status == clientlib.CONNECTED {
		connections.Unlock()
		return errors.New("[Connect] client already connected")
	}
	c.status = clientlib.CONNECTED
	connections.m[clientID] = c
	connections.Unlock()
	*ack = true
	// Sync clock with the new client
	go s.syncClocks()
	return nil
}

func (s *TankServer) GetNodes (clientID uint64, addrSet *[]serverlib.PeerInfo) error {
	log.Println("GetNodes()", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[GetNodes] received from client", encodeToByte(clientID), &incomingMessage)
	connections.RLock()
	defer connections.RUnlock()

	if _, ok := connections.m[clientID]; !ok {
		return InvalidClientError(clientID)
	}

	// TODO: Maybe don't return ALL peers...
	peerAddresses := make([]serverlib.PeerInfo, 0, len(connections.m)-1)

	for key, connection := range connections.m {
		if key == clientID || connection.status == clientlib.DISCONNECTED {
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

// Notify server that node PeerID has either disconnected or reconnected.
func (s *TankServer) NotifyConnection(connectionInfo serverlib.ConnectionInfo, ack *bool) error {
	var status string
	if connectionInfo.Status == clientlib.CONNECTED {
		status = "connected"
	} else {
		status = "disconnected"
	}
	log.Printf("[NotifyConnection] Node %d reported as %s by node %d\n", connectionInfo.PeerID, status, connectionInfo.ReporterID)

	connections.Lock()
	defer connections.Unlock()
	disconnectedNodes.Lock()
	defer disconnectedNodes.Unlock()

	if _, exists := connections.m[connectionInfo.PeerID]; !exists {
		log.Printf("[NotifyConnection] Node with ID %d does not exist\n", connectionInfo.PeerID)
		*ack = false
		return nil
	}
	conn := connections.m[connectionInfo.PeerID]

	if connectionInfo.Status == clientlib.DISCONNECTED {
		// Option #1: node reports that node PeerID has disconnected
		if conn.status == clientlib.CONNECTED {
			conn.status = clientlib.DISCONNECTED
			connections.m[connectionInfo.PeerID] = conn
		}

		// Add reporting peer's ID to list
		if disconnected, exists := disconnectedNodes.m[connectionInfo.PeerID]; !exists {
			disconnectedNodes.m[connectionInfo.PeerID] = []uint64{connectionInfo.ReporterID}
			// Notify disconnected node of their state? (our assumption is server is always reachable) TODO
		} else {
			disconnectedNodes.m[connectionInfo.PeerID] = append(disconnected, connectionInfo.ReporterID)
		}
	} else {
		// Option #2: node reports that node PeerID has reconnected
		// Remove reporterID from list of 'reporters'
		if disconnected, exists := disconnectedNodes.m[connectionInfo.PeerID]; exists {
			disconnectedNodes.m[connectionInfo.PeerID] = removeId(disconnected, connectionInfo.ReporterID)
		} else {
			log.Printf("[NotifyConnection] Node with ID %d is already marked as connected\n", connectionInfo.PeerID)
			*ack = false
			return nil
		}

		// If no reporters remaining (i.e. they've all reconnected) mark node connected
		if len(disconnectedNodes.m[connectionInfo.PeerID]) == 0 {
			delete(disconnectedNodes.m, connectionInfo.PeerID)
			conn.status = clientlib.CONNECTED
			connections.m[connectionInfo.PeerID] = conn
			// Notify reconnected node of their state TODO
		}
	}

	*ack = true
	return nil
}

func monitorConnections() {
	for {
		disconnectedNodes.Lock()
		for nodeID, reporters := range disconnectedNodes.m {
			// 1. Ensure node is in fact disconnected
			connections.RLock()
			if conn, _ := connections.m[nodeID]; conn.status == clientlib.CONNECTED {
				delete(disconnectedNodes.m, nodeID)
				connections.RUnlock()
				break
			}
			connections.RUnlock()

			// 2. Check for disconnected reporters; remove them from node's list
			for _, reporterID := range reporters {
				if _, exists := disconnectedNodes.m[reporterID]; exists {
					disconnectedNodes.m[nodeID] = removeId(disconnectedNodes.m[nodeID], reporterID)
				}
			}

			// 3. If no reporters remaining, remove node from map and mark as reconnected
			if len(reporters) == 0 {
				delete(disconnectedNodes.m, nodeID)
				connections.Lock()
				conn := connections.m[nodeID]
				conn.status = clientlib.CONNECTED
				connections.m[nodeID] = conn
				connections.Unlock()
			}
		}
		disconnectedNodes.Unlock()
	}
}

func removeId(ids []uint64, id uint64) []uint64 {
	for i, v := range ids {
		if v == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) != 2 {
		log.Fatal("Usage: go run server.go <IP Address : Port>")
	}
	ipAddr := os.Args[1]

	serverAddr, err := net.ResolveTCPAddr("tcp", ipAddr)
	if err != nil {
		log.Fatal(err)
	}
	inbound, err := net.ListenTCP("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	go monitorConnections()

	Logger = govec.InitGoVector("server", "serverlogfile")
	server := new(TankServer)
	rpc.Register(server)
	log.Println("Listening now")
	Logger.LogLocalEvent("Listening Now")
	rpc.Accept(inbound)
}
