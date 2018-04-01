/*
	Implements a thin server for P2P Battle Tanks game for CPSC 416 Project 2.
	This server is responsible for peer discovery and clock synchronisation

	Usage:
		go run server.go <IP Address : Port>
*/

package main

import (
	"errors"
	"fmt"
	"github.com/DistributedClocks/GoVector/govec"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/clientlib"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/clocklib"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/serverlib"
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
	status      Status
	displayName string
	address     string
	rpcAddress  string
	offset      time.Duration
}

// -----------------------------------------------------------------------------

// KV: Define types.

// A ClientInfo represents information about a client.
type ClientInfo struct {
	ClientId int
}

// -----------------------------------------------------------------------------

// Error definitions

// Contains bad ID
type InvalidClientError string

func (e InvalidClientError) Error() string {
	return fmt.Sprintf("Invalid Client Id [%s]. Please register.", e)
}

// -----------------------------------------------------------------------------

// KV: Key is not available on any online client.
type KeyUnavailableError string

func (e KeyUnavailableError) Error() string {
	return fmt.Sprintf("Key [%s] is unavailable on any online client.", e)
}

// -----------------------------------------------------------------------------

// State Variables

var connections = struct {
	sync.RWMutex
	m map[uint64]Connection
}{m: make(map[uint64]Connection)}

var Logger *govec.GoLog

var Clock *clocklib.ClockManager = &clocklib.ClockManager{0}

// -----------------------------------------------------------------------------

// KV: Global variables and data structures for the server's state.

// A map to store which client IDs have a key-value pair.
type KeyToClients struct {
	Mutex sync.RWMutex
	M     map[int][]uint64
}

var keyToClients KeyToClients = KeyToClients{sync.RWMutex{}, make(map[int][]uint64)}

// The number of clients a key-value pair should be replicated on.
var globalReplicationFactor int = 3

// -----------------------------------------------------------------------------

// KV: Server side functions for a distributed key-value store.

func (s *TankServer) KVGet(arg *crdtlib.GetArg, reply *crdtlib.GetReply) error {

	// Get the latest client that stores this key-value pair and is online.
	keyToClients.Mutex.Lock()
	defer keyToClients.Mutex.Unlock()
	connections.Lock()
	defer connections.Unlock()
	var latestOnline uint64
	found := false
	key := arg.Key
	for _, clientId_ := range keyToClients.M[key] {
		client, ok := connections.m[clientId_]
		if ok && client.status == CONNECTED {
			latestOnline = clientId_
			found = true
		}
	}

	// If there is no client with this key-value pair, return an error.
	if !found {
		reply.Ok = false
		reply.HasAlready = false
		reply.Unavailable = true
		return KeyUnavailableError(key)
	}

	// If this client itself has this key-value pair, indicate this in the
	// reply.
	if latestOnline == arg.ClientId {
		reply.Ok = true
		reply.HasAlready = true
		return nil
	}

	// If this key-value pair is stored by another client, retrieve it using an
	// RPC and send it to the calling client.
	connection := connections.m[latestOnline]
	client, err := rpc.Dial("tcp", connection.rpcAddress)
	if err != nil {
		// TODO: Better failure handling
		log.Fatal(err)
	}
	clockClient := clientlib.NewClientClockRemoteAPI(client)
	value, err := clockClient.KVClientGet(key)
	if err != nil {
		// TODO: Better failure handling
		log.Fatal(err)
	}
	reply.Ok = true
	reply.Value = value

	return nil
}

func (s *TankServer) KVPut(arg *crdtlib.PutArg, reply *crdtlib.PutReply) error {

	// If this key-value pair does not exist in any client, add it. Otherwise,
	// update it.
	connections.Lock()
	defer connections.Unlock()
	keyToClients.Mutex.Lock()
	defer keyToClients.Mutex.Unlock()
	key := arg.Key
	value := arg.Value
	clientsTmp := keyToClients.M[key]

	var clients []uint64
	for _, clientId := range clientsTmp {
		connection := connections.m[clientId]
		if connection.status == DISCONNECTED {
			continue
		}
		clients = append(clients, clientId)
	}

	// Replication factor is 3.
	curReplicas := len(clients)
	if curReplicas < 3 {
		// If we do not have enough client replicas, randomly choose additional
		// clients to store this key-value pair on.
		var candidates []uint64
		for clientId, connection := range connections.m {
			if connection.status == DISCONNECTED {
				continue
			}
			// Do not choose a client if it already exists in our list.
			alreadyThere := false
			for _, clientId_ := range clients {
				if clientId_ == clientId {
					alreadyThere = true
					break
				}
			}
			if !alreadyThere {
				candidates = append(candidates, clientId)
			}
		}
		// Randomly permute the candidate array and get the required number of
		// client IDs from the beginning of the array.
		numRequired := 3 - curReplicas
		perm := rand.Perm(len(candidates))
		for i := 0; i < numRequired; i++ {
			clients = append(clients, candidates[perm[i]])
		}
	}

	// Send an RPC to each client to store it.
	for _, clientId := range clients {
		connection := connections.m[clientId]
		client, err := rpc.Dial("tcp", connection.rpcAddress)
		if err != nil {
			// TODO: Better failure handling
			log.Fatal(err)
		}
		clockClient := clientlib.NewClientClockRemoteAPI(client)
		err = clockClient.KVClientPut(key, value)
		if err != nil {
			// TODO: Better failure handling
			log.Fatal(err)
		}
		// If this client did not already have this pair, update keysToClients to
		// reflect that it now has this pair.
		found := false
		for _, id := range keyToClients.M[key] {
			if id == clientId {
				found = true
				break
			}
		}
		if !found {
			keyToClients.M[key] = append(keyToClients.M[key], clientId)
		}
	}

	reply.Ok = true

	return nil
}

// -----------------------------------------------------------------------------

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
		if connection.status == DISCONNECTED {
			continue
		}
		s := fmt.Sprintf("Requesting client %d for time", key)
		// Update to preapre send
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
		if connection.status == DISCONNECTED {
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

func (s *TankServer) Register(peerInfo serverlib.PeerInfo, settings *clientlib.PeerNetSettings) error {
	log.Println("register", peerInfo.ClientID)
	var incomingMessage int
	Logger.UnpackReceive("[Register] received from client", encodeToByte(peerInfo), &incomingMessage)
	newSettings := clientlib.PeerNetSettings{
		MinimumPeerConnections: 1,
		UniqueUserID:           peerInfo.ClientID,
		DisplayName:            peerInfo.DisplayName,
	}

	*settings = newSettings

	connections.Lock()
	connections.m[peerInfo.ClientID] = Connection{
		status:      DISCONNECTED,
		displayName: peerInfo.DisplayName,
		address:     peerInfo.Address,
		rpcAddress:  peerInfo.RPCAddress,
		offset:      0,
	}

	connections.Unlock()
	return nil
}

func (s *TankServer) Connect(clientID uint64, ack *bool) error {
	log.Println("connect", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[Connect] received from client", encodeToByte(clientID), &incomingMessage)
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
	// Sync clock with the new client
	go s.syncClocks()
	return nil
}

func (s *TankServer) GetNodes(clientID uint64, addrSet *[]serverlib.PeerInfo) error {
	log.Println("getnodes", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[GetNodes] received from client", encodeToByte(clientID), &incomingMessage)
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
			Address:     connection.address,
			ClientID:    key,
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
		log.Fatal("Usage: go run server.go <IP Address : Port>")
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

	Logger = govec.InitGoVector("server", "serverlogfile")
	server := new(TankServer)
	rpc.Register(server)
	fmt.Println("Listening now")
	Logger.LogLocalEvent("Listening Now")
	rpc.Accept(inbound)
}
