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
	"../crdtlib"
	"../serverlib"
	"errors"
	"fmt"
	"github.com/DistributedClocks/GoVector/govec"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type TankServer int

type Connection struct {
	status      Status
	displayName string
	address     string
	rpcAddress  string
	client      *clientlib.ClientClockRemote
	offset      time.Duration
}

type Status int

const (
	DISCONNECTED Status = iota
	CONNECTED
)

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

var StatsLogger *govec.GoLog

var Clock *clocklib.ClockManager = &clocklib.ClockManager{}

// -----------------------------------------------------------------------------

// KV: Global variables and data structures for the server's state.

// A map to store which client IDs have a key-value pair.
type KeyToClients struct {
	Mutex sync.RWMutex
	M     map[uint64][]uint64
}

var keyToClients KeyToClients = KeyToClients{sync.RWMutex{}, make(map[uint64][]uint64)}

// The number of clients a key-value pair should be replicated on.
var globalReplicationFactor int = 3

// -----------------------------------------------------------------------------

// KV: Server side functions for a distributed key-value store.

func (s *TankServer) KVGet(request *serverlib.KVGetRequest, response *serverlib.KVGetResponse) error {

	// Get the latest client that stores this key-value pair and is online.
	keyToClients.Mutex.Lock()
	defer keyToClients.Mutex.Unlock()
	connections.Lock()
	defer connections.Unlock()
	var latestOnline uint64
	var clientID uint64
	StatsLogger.UnpackReceive("[KVGet] Request received from client", request.B, &clientID)
	arg := request.Arg
	var reply crdtlib.GetReply
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
		b := StatsLogger.PrepareSend("[KVGet] Request from client failed", reply)
		*response = serverlib.KVGetResponse{reply, b}
		return KeyUnavailableError(key)
	}

	// If this client itself has this key-value pair, indicate this in the
	// reply.
	if latestOnline == arg.ClientId {
		reply.Ok = true
		reply.HasAlready = true
		b := StatsLogger.PrepareSend("[KVGet] Request from client succeeded", reply)
		*response = serverlib.KVGetResponse{reply, b}
		return nil
	}

	// If this key-value pair is stored by another client, retrieve it using an
	// RPC and send it to the calling client.
	connection := connections.m[latestOnline]
	client, err := rpc.Dial("tcp", connection.rpcAddress)
	if err != nil {
		// TODO: Better failure handling
		b := StatsLogger.PrepareSend("[KVGet] Request from client failed", reply)
		*response = serverlib.KVGetResponse{reply, b}
		log.Fatal(err)
	}
	clockClient := clientlib.NewClientClockRemoteAPI(client)
	value, err := clockClient.KVClientGet(key, StatsLogger)
	if err != nil {
		// TODO: Better failure handling
		b := StatsLogger.PrepareSend("[KVGet] Request from client failed", reply)
		*response = serverlib.KVGetResponse{reply, b}
		log.Fatal(err)
	}
	reply.Ok = true
	reply.Value = value
	b := StatsLogger.PrepareSend("[KVGet] Request from client succeeded", reply)
	*response = serverlib.KVGetResponse{reply, b}

	return nil
}

func (s *TankServer) KVPut(request *serverlib.KVPutRequest, response *serverlib.KVPutResponse) error {

	// If this key-value pair does not exist in any client, add it. Otherwise,
	// update it.
	connections.Lock()
	defer connections.Unlock()
	keyToClients.Mutex.Lock()
	defer keyToClients.Mutex.Unlock()
	var k uint64
	StatsLogger.UnpackReceive("[KVPut] Request received from client", request.B, k)
	arg := request.Arg
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
			var reply crdtlib.PutReply
			reply.Ok = false
			b := StatsLogger.PrepareSend("[KVPut] request from client failed", true)
			*response = serverlib.KVPutResponse{reply, b}
			log.Fatal(err)
		}
		clockClient := clientlib.NewClientClockRemoteAPI(client)
		err = clockClient.KVClientPut(key, value, StatsLogger)
		if err != nil {
			// TODO: Better failure handling
			var reply crdtlib.PutReply
			reply.Ok = false
			b := StatsLogger.PrepareSend("[KVPut] request from client failed", true)
			*response = serverlib.KVPutResponse{reply, b}
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

	var reply crdtlib.PutReply
	reply.Ok = true
	b := StatsLogger.PrepareSend("[KVPut] request from client succeeded", true)
	*response = serverlib.KVPutResponse{reply, b}
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
	var offsetTotal time.Duration = Clock.GetOffset()
	var offsetNum time.Duration = 1

	for key, connection := range connections.m {
		if connection.status == DISCONNECTED {
			continue
		}
		client, err := rpc.Dial("tcp", connection.rpcAddress)
		if err != nil {
			// TODO : Better failure handling
			// Probably wanna mark the client DISCONNECTED
			log.Fatal("syncClocks() Failed to dial TCP address:", err)
		}

		before := Clock.GetCurrentTime()

		clockClient := clientlib.NewClientClockRemoteAPI(client)
		connection.client = clockClient
		connections.m[key] = connection

		t, err := clockClient.TimeRequest(Logger)
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

		offset := offsetAverage - m[key]
		connection.offset = offset
		connections.m[key] = connection

		err := connection.client.SetOffset(offset, Logger)
		if err != nil {
			log.Fatal(err)
		}
	}
	Logger.LogLocalEvent("Setting local clock offset")
	Clock.SetOffset(Clock.GetOffset() + offsetAverage)
	connections.Unlock()
}

func (s *TankServer) Register(peerInfo serverlib.PeerInfo, settings *serverlib.PeerSettingsRequest) error {
	log.Println("Register()", peerInfo.ClientID)
	var incomingMessage string
	Logger.UnpackReceive("[Register] request received from client", peerInfo.B, &incomingMessage)
	fmt.Println(incomingMessage)
	newSettings := clientlib.PeerNetSettings{
		MinimumPeerConnections: 2,
		UniqueUserID:           peerInfo.ClientID,
		DisplayName:            peerInfo.DisplayName,
	}

	b := Logger.PrepareSend("[Register] request accepted from client", peerInfo.ClientID)
	*settings = serverlib.PeerSettingsRequest{newSettings, b}

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

func (s *TankServer) Connect(clientReq serverlib.ClientIDRequest, response *serverlib.ConnectResponse) error {
	clientID := clientReq.ClientID
	log.Println("Connect()", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[Connect] received from client", clientReq.B, &incomingMessage)
	connections.Lock()
	c, ok := connections.m[clientID]
	if !ok {
		connections.Unlock()
		ack := false
		b := Logger.PrepareSend("[Connect] Request rejected from client", ack)
		*response = serverlib.ConnectResponse{ack, b}
		return InvalidClientError(clientID)
	}
	if c.status == CONNECTED {
		connections.Unlock()
		ack := false
		b := Logger.PrepareSend("[Connect] Request rejected from client", ack)
		*response = serverlib.ConnectResponse{ack, b}
		return errors.New("client already connected")
	}
	c.status = CONNECTED
	connections.m[clientID] = c
	connections.Unlock()
	ack := true
	b := Logger.PrepareSend("[Connect] Request accepted from client", ack)
	*response = serverlib.ConnectResponse{ack, b}
	// Sync clock with the new client
	go s.syncClocks()
	return nil
}

func (s *TankServer) GetNodes(clientReq serverlib.ClientIDRequest, addrSet *serverlib.GetNodesResponse) error {
	clientID := clientReq.ClientID
	log.Println("GetNodes()", clientID)
	var incomingMessage int
	Logger.UnpackReceive("[GetNodes] received from client", clientReq.B, &incomingMessage)
	connections.RLock()
	defer connections.RUnlock()

	if _, ok := connections.m[clientID]; !ok {
		b := Logger.PrepareSend("[GetNodes] rejected from client", clientID)
		*addrSet = serverlib.GetNodesResponse{nil, b}
		return InvalidClientError(clientID)
	}

	// TODO: Maybe don't return ALL peers...
	peerAddresses := make([]serverlib.PeerInfo, 0, len(connections.m)-1)

	for key, connection := range connections.m {
		if key == clientID || connection.status == DISCONNECTED {
			continue
		}

		peerAddresses = append(peerAddresses, serverlib.PeerInfo{
			Address:     connection.address,
			RPCAddress:  connection.rpcAddress,
			ClientID:    key,
			DisplayName: connection.displayName,
		})
	}

	// TODO : Filter the addresses better for network topology
	b := Logger.PrepareSend("[GetNodes] accepted from client", clientID)
	*addrSet = serverlib.GetNodesResponse{peerAddresses, b}
	return nil
}

func (s *TankServer) NotifyFailure(clientID uint64, ack *bool) error {
	log.Println("NotifyFailure()", clientID)
	connections.Lock()
	defer connections.Unlock()

	if conn, ok := connections.m[clientID]; ok && conn.status == CONNECTED {
		conn.status = DISCONNECTED
		connections.m[clientID] = conn
	}

	*ack = true
	return nil
}

func monitorConnections() {
	for {
		time.Sleep(time.Second * 2)
		connections.RLock()
		for id, connection := range connections.m {
			if connection.status == DISCONNECTED {
				if err := connection.client.Ping(); err != nil {
					continue
				}

				if success, _ := connection.client.Recover(); success {
					connections.RUnlock() // Do a little dance with locks, because we didn't want to
					connections.Lock()    // hog the write lock while waiting for timeouts on RPC calls

					connection.status = CONNECTED
					connections.m[id] = connection

					connections.Unlock()
					connections.RLock()
				}
			}
		}
		connections.RUnlock()
	}
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
	StatsLogger = govec.InitGoVector("server", "serverstatslogfile")
	server := new(TankServer)
	rpc.Register(server)
	log.Println("Listening now")
	Logger.LogLocalEvent("Listening Now")
	rpc.Accept(inbound)
}
