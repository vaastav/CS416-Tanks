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
	client *clientlib.ClientClockRemote
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
			log.Fatal("syncClocks() Failed to dial TCP address:", err)
		}

		before := Clock.GetCurrentTime()
		
		clockClient := clientlib.NewClientClockRemoteAPI(client)
		connection.client = clockClient
		connections.m[key] = connection

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
		//client, err := rpc.Dial("tcp", connection.rpcAddress)
		//if err != nil {
		//	// TODO : Better failure handling
		//	log.Fatal("syncClocks() Failed ", err)
		//}

		offset := offsetAverage - m[key]
		connection.offset = offset
		connections.m[key] = connection

		//clockClient := clientlib.NewClientClockRemoteAPI(client)
		err := connection.client.SetOffset(offset)
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
	log.Printf("NotifyConnection() Node %d reported as %s by node %d\n", connectionInfo.PeerID, getStatusString(connectionInfo.Status), connectionInfo.ReporterID)
	connections.Lock()
	defer connections.Unlock()
	disconnectedNodes.Lock()
	defer disconnectedNodes.Unlock()

	*ack = false
	if _, exists := connections.m[connectionInfo.PeerID]; !exists {
		log.Printf("NotifyConnection() Node with ID %d does not exist\n", connectionInfo.PeerID)
		return nil
	}
	conn := connections.m[connectionInfo.PeerID]

	if connectionInfo.Status == clientlib.CONNECTED {
		// Option #1: node reports that node PeerID has reconnected
		// Remove reporterID from list of 'reporters'
		if disconnected, exists := disconnectedNodes.m[connectionInfo.PeerID]; exists {
			disconnectedNodes.m[connectionInfo.PeerID] = removeId(disconnected, connectionInfo.ReporterID)
		} else {
			return nil
		}

		// If no reporters remaining (i.e. they've all reconnected) mark node connected
		if len(disconnectedNodes.m[connectionInfo.PeerID]) == 0 {
			//
			if err := conn.client.TestConnection(); err != nil {
				log.Printf("NotifyConnection() Error testing connection with node %d\n", connectionInfo.PeerID)
				return nil
			}
			log.Println("Test connection!")

			// A. Update reconnected node with all current disconnections
			if ok := updateConnectionState(connectionInfo.PeerID, conn); !ok {
				log.Printf("NotifyConnection() Error updating connection state of node %d\n", connectionInfo.PeerID)
				return nil
			}

			// B. Remove node from disconnected list and reset connection status
			delete(disconnectedNodes.m, connectionInfo.PeerID)
			conn.status = clientlib.CONNECTED
			connections.m[connectionInfo.PeerID] = conn
			*ack = true // Indicates that server considers node reconnected

			log.Printf("NotifyConnection() Notifying nodes of reconnection of node %d\n", connectionInfo.PeerID)
			for id, conn := range connections.m {
				if id == connectionInfo.ReporterID || id == connectionInfo.PeerID || conn.status == clientlib.DISCONNECTED {
					continue
				}
				err := conn.client.NotifyConnection(connectionInfo.PeerID)
				if err != nil {
					log.Printf("NotifyConnection() Error notifying client %d of reconnection of node %d: %s\n", id, connectionInfo.PeerID, err)
				}
			}
		}
	} else {
		// Option #2: node reports that node PeerID has disconnected
		if _, exists := disconnectedNodes.m[connectionInfo.ReporterID]; exists {
			log.Printf("NotifyConnection() Reporting node %d is disconnected!\n", connectionInfo.PeerID)
			return nil
		}

		if conn.status == clientlib.CONNECTED {
			conn.status = clientlib.DISCONNECTED
			connections.m[connectionInfo.PeerID] = conn
		}
		*ack = true

		// Add reporting peer's ID to list
		if disconnected, exists := disconnectedNodes.m[connectionInfo.PeerID]; !exists {
			disconnectedNodes.m[connectionInfo.PeerID] = []uint64{connectionInfo.ReporterID}
		} else {
			if !containsId(disconnected, connectionInfo.ReporterID) {
				disconnectedNodes.m[connectionInfo.PeerID] = append(disconnected, connectionInfo.ReporterID)
			}
		}
	}

	return nil
}

func monitorConnections() {
	for {
		disconnectedNodes.Lock()
	loop:
		for nodeID, reporters := range disconnectedNodes.m {
			// 1. Check for disconnected reporters; remove them from node's list
			for _, reporterID := range reporters {
				if _, exists := disconnectedNodes.m[reporterID]; exists {
					disconnectedNodes.m[nodeID] = removeId(disconnectedNodes.m[nodeID], reporterID)
				}
			}

			// 2. If no reporters remaining, test connection; if successful, remove node from map and mark as reconnected
			if len(disconnectedNodes.m[nodeID]) == 0 {
				connections.Lock()
				conn := connections.m[nodeID]
				if err := conn.client.TestConnection(); err != nil {
					log.Printf("NotifyConnection() Error testing connection with node %d\n", nodeID)
					continue loop
				}
				log.Println("Test connection!")

				// A. Update reconnected node with all current disconnections
				if ok := updateConnectionState(nodeID, conn); !ok {
					log.Printf("NotifyConnection() Error updating connection state of node %d\n", nodeID)
					continue loop
				}

				// B. Remove node from disconnected list and reset connection status
				delete(disconnectedNodes.m, nodeID)
				conn.status = clientlib.CONNECTED
				connections.m[nodeID] = conn

				// C. Notify all nodes of reconnection
				log.Printf("broadcastReconnection() Notifying nodes of reconnection of node %d\n", nodeID)
				for id, peerConn := range connections.m {
					if id == nodeID || peerConn.status == clientlib.DISCONNECTED {
						continue
					}
					err := peerConn.client.NotifyConnection(nodeID)
					if err != nil {
						log.Printf("broadcastReconnection() Error notifying client %d of reconnection of node %d: %s\n", id, nodeID, err)
					}
				}
				connections.Unlock()
			}
		}
		disconnectedNodes.Unlock()
	}
}

// TODO: need to fix this so all info gets there
// When a node comes back online, its connection state for each node might be stale
func updateConnectionState(clientID uint64, conn Connection) bool {
	//for id := range disconnectedNodes.m {
	//	if id == clientID {
	//		continue
	//	}
	//	err := conn.client.NotifyDisconnection(id)
	//	if err != nil {
	//		log.Printf("updateConnectionState() Error updating connection state of node %d: %s\n", clientID, err)
	//		return false
	//	}
	//}
	log.Printf("Updating connection state of node %d\n", clientID)
	connectionState := make(map[uint64]clientlib.Status)
	for id, connInfo := range connections.m {
		if id == clientID {
			continue
		}
		connectionState[id] = connInfo.status
		//
		//var err error
		//if connInfo.status == clientlib.CONNECTED {
		//	log.Printf("Notify connection state of node %d\n", id)
		//	err = conn.client.NotifyConnection(id)
		//} else {
		//	log.Printf("Notify disconnection state of node %d\n", id)
		//	err = conn.client.NotifyDisconnection(id)
		//}
		//if err != nil {
		//	log.Printf("updateConnectionState() Error updating connection state of node %d: %s\n", clientID, err)
		//	return false
		//}
	}
	if err := conn.client.UpdateConnectionState(connectionState); err != nil {
		log.Printf("updateConnectionState() Error updating connection state of node %d: %s\n", clientID, err)
		return false
	}

	return true
}

func getStatusString(status clientlib.Status) string {
	if status == clientlib.CONNECTED {
		return "reconnected"
	} else {
		return "disconnected"
	}
}

func removeId(ids []uint64, id uint64) []uint64 {
	for i, val := range ids {
		if val == id {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

func containsId(ids []uint64, id uint64) bool {
	for _, val := range ids {
		if val == id {
			return true
		}
	}
	return false
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) != 2 {
		log.Fatal("Usage: go run server.go <IP Address : Port>")
	}
	ipAddr := os.Args[1]

	serverAddr, err := net.ResolveTCPAddr("tcp", ipAddr)
	if err != nil {
		log.Fatal("main() Failed to resolve TCP address:", err)
	}
	inbound, err := net.ListenTCP("tcp", serverAddr)
	if err != nil {
		log.Fatal("main() Failed to listen on TCP address:", err)
	}

	go monitorConnections()

	Logger = govec.InitGoVector("server", "serverlogfile")
	server := new(TankServer)
	rpc.Register(server)
	log.Println("Listening now")
	Logger.LogLocalEvent("Listening Now")
	//rpc.Accept(inbound)

	for {
		conn, _ := inbound.Accept()
		go rpc.ServeConn(conn)
	}
}
