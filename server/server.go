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
	"bitbucket.org/bestchai/dinv/dinvRT"
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
	"encoding/binary"
)

type TankServer int

type Connection struct {
	status      Status
	displayName string
	address     string
	rpcClient   *rpc.Client
	rpcAddress  string
	client      *clientlib.ClientClockRemote
	offset      time.Duration
	partner     serverlib.PeerInfo
}

type Status int

const (
	DISCONNECTED Status = iota
	CONNECTED
	RECONNECTED
	NOTINGAME
)

// Error definitions

// Contains bad ID
type InvalidClientError string

func (e InvalidClientError) Error() string {
	return fmt.Sprintf("Invalid Client Id [%s]. Please register.", string(e))
}

// Contains bad display name
type DisplayNameInUseError string

func (e DisplayNameInUseError) Error() string {
	return fmt.Sprintf("Display Name [%s] is already in use.", string(e))
}

// -----------------------------------------------------------------------------

// KV: Key is not available on any online client.
type KeyUnavailableError string

func (e KeyUnavailableError) Error() string {
	return fmt.Sprintf("Key [%s] is unavailable on any online client.", string(e))
}

// -----------------------------------------------------------------------------

// State Variables

const (
	MinPeerConnections = 2
)

var connections = struct {
	sync.RWMutex
	m map[uint64]*Connection
}{m: make(map[uint64]*Connection)}

var displayNames = struct {
	sync.RWMutex
	M map[string]bool
}{M: make(map[string]bool)}

var UseDinv bool

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

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

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
	if connection.rpcClient == nil {
		// TODO: Better failure handling
		b := StatsLogger.PrepareSend("[KVGet] Request from client failed", reply)
		*response = serverlib.KVGetResponse{reply, b}
		log.Fatal("Client not connected")
	}

	clockClient := clientlib.NewClientClockRemoteAPI(connection.rpcClient)
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
		if connection.status != CONNECTED {
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
			if connection.status != CONNECTED {
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
		numRequired := min(3-curReplicas, len(candidates))
		perm := rand.Perm(len(candidates))
		for i := 0; i < numRequired; i++ {
			clients = append(clients, candidates[perm[i]])
		}
	}

	// Send an RPC to each client to store it.
	for _, clientId := range clients {
		connection := connections.m[clientId]
		if connection.rpcClient == nil {
			// TODO: Better failure handling
			var reply crdtlib.PutReply
			reply.Ok = false
			b := StatsLogger.PrepareSend("[KVPut] request from client failed", true)
			*response = serverlib.KVPutResponse{reply, b}
			log.Fatal("client not connected")
		}
		clockClient := clientlib.NewClientClockRemoteAPI(connection.rpcClient)
		err := clockClient.KVClientPut(key, value, StatsLogger)
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

func encodeToString(v interface{}) (s string) {
	s = fmt.Sprintf("%v", v)
	return s
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
		if connection.status != CONNECTED {
			continue
		}

		if connection.rpcClient == nil {
			// TODO : Better failure handling
			// Probably wanna mark the client DISCONNECTED
			log.Fatal("syncClocks() Client", key, "not connected")
		}

		before := Clock.GetCurrentTime()

		clockClient := clientlib.NewClientClockRemoteAPI(connection.rpcClient)
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
		if connection.status != CONNECTED {
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

func (s *TankServer) Register(request serverlib.RegisterRequest, settings *serverlib.RegisterResponse) error {
	log.Println("Register()", request.DisplayName)
	var incomingMessage string
	Logger.UnpackReceive("[Register] request received from client", request.B, &incomingMessage)
	fmt.Println(incomingMessage)
	var addressString string
	if UseDinv {
		dinvRT.Unpack(request.DinvB, &addressString)
	}
	displayNames.Lock()
	_, ok := displayNames.M[request.DisplayName]
	if ok {
		displayNames.Unlock()
		b := Logger.PrepareSend("[Register] request rejected from client", request.ClientID)
		if UseDinv {
			dinvb := dinvRT.Pack(request.ClientID)
			*settings = serverlib.RegisterResponse{clientlib.PeerNetSettings{}, b, dinvb}
		} else {
			*settings = serverlib.RegisterResponse{clientlib.PeerNetSettings{}, b, b}
		}
		return DisplayNameInUseError(request.DisplayName)
	}
	displayNames.M[request.DisplayName] = true
	newSettings := clientlib.PeerNetSettings{
		UniqueUserID: request.ClientID,
		DisplayName:  request.DisplayName,
	}

	if UseDinv {
		dinvRT.Track("server.Register", "displayNames", encodeToString(displayNames.M))
	}
	displayNames.Unlock()
	b := Logger.PrepareSend("[Register] request accepted from client", request.ClientID)
	if UseDinv {
		dinvb := dinvRT.Pack(request.ClientID)
		*settings = serverlib.RegisterResponse{newSettings, b, dinvb}
	} else {
		*settings = serverlib.RegisterResponse{newSettings, b, b}
	}

	connections.Lock()
	connections.m[request.ClientID] = &Connection{
		status:      NOTINGAME,
		displayName: request.DisplayName,
	}

	connections.Unlock()
	return nil
}

func (s *TankServer) Connect(clientReq serverlib.ConnectRequest, response *serverlib.ConnectResponse) error {
	peerInfo := clientReq.Pi
	clientID := peerInfo.ClientID
	log.Println("Connect()", clientID)
	var incomingMessage int
	var dinvMessage int
	Logger.UnpackReceive("[Connect] received from client", clientReq.B, &incomingMessage)
	if UseDinv {
		dinvRT.Unpack(clientReq.DinvB, &dinvMessage)
	}
	connections.Lock()

	c, ok := connections.m[clientID]
	if !ok {
		connections.Unlock()
		b := Logger.PrepareSend("[Connect] Request rejected from client", 0)
		if UseDinv {
			dinvb := dinvRT.Pack(dinvMessage)
			*response = serverlib.ConnectResponse{0, b, dinvb}
		} else {
			*response = serverlib.ConnectResponse{0, b, b}
		}

		connections.Unlock()

		return InvalidClientError(clientID)
	}

	if c.status == CONNECTED {
		connections.Unlock()
		b := Logger.PrepareSend("[Connect] Request rejected from client", 0)
		if UseDinv {
			dinvb := dinvRT.Pack(dinvMessage)
			*response = serverlib.ConnectResponse{0, b, dinvb}
		} else {
			*response = serverlib.ConnectResponse{0, b, b}
		}

		connections.Unlock()

		return errors.New("client already connected")
	}

	// find a partner node
	var partner serverlib.PeerInfo

	for id, otherC := range connections.m {
		if id != clientID && otherC.status == CONNECTED {
			partner.Address = otherC.address
			partner.RPCAddress = otherC.rpcAddress
			partner.ClientID = id
			partner.DisplayName = otherC.displayName

			break
		}
	}

	connections.m[peerInfo.ClientID] = &Connection{
		status:      CONNECTED,
		displayName: peerInfo.DisplayName,
		address:     peerInfo.Address,
		rpcAddress:  peerInfo.RPCAddress,
		rpcClient:   c.rpcClient,
		offset:      0,
		partner:     partner,
	}

	connections.Unlock()

	b := Logger.PrepareSend("[Connect] Request accepted from client", MinPeerConnections)
	if UseDinv {
		dinvb := dinvRT.Pack(dinvMessage)
		*response = serverlib.ConnectResponse{MinPeerConnections, b, dinvb}
	} else {
		*response = serverlib.ConnectResponse{MinPeerConnections, b, b}
	}

	// Sync clock with the new client
	go s.syncClocks()

	return nil
}

func (s *TankServer) Disconnect(request serverlib.ClientIDRequest, response *serverlib.DisconnectResponse) error {
	clientID := request.ClientID
	log.Println("Disconnect()", clientID)
	var incomingMessage int
	var dinvMessage int
	Logger.UnpackReceive("[Disconnect] received from client", request.B, &incomingMessage)
	if UseDinv {
		dinvRT.Unpack(request.DinvB, &dinvMessage)
	}
	connections.Lock()
	c, ok := connections.m[clientID]
	if !ok {
		connections.Unlock()
		b := Logger.PrepareSend("[Disconnect] Request rejected from client", 0)
		if UseDinv {
			dinvb := dinvRT.Pack(dinvMessage)
			*response = serverlib.DisconnectResponse{false, b, dinvb}
		} else {
			*response = serverlib.DisconnectResponse{false, b, b}
		}
		return InvalidClientError(clientID)
	}
	c.status = NOTINGAME
	connections.m[clientID] = c
	connections.Unlock()
	b := Logger.PrepareSend("[DisConnect] Request accepted from client", MinPeerConnections)
	if UseDinv {
		dinvb := dinvRT.Pack(dinvMessage)
		*response = serverlib.DisconnectResponse{true, b, dinvb}
	} else {
		*response = serverlib.DisconnectResponse{true, b, b}
	}
	return nil
}

func (s *TankServer) GetNodes(clientReq serverlib.ClientIDRequest, addrSet *serverlib.GetNodesResponse) error {
	clientID := clientReq.ClientID
	log.Println("GetNodes()", clientID)
	var incomingMessage int
	var dinvMessage int
	Logger.UnpackReceive("[GetNodes] received from client", clientReq.B, &incomingMessage)
	if UseDinv {
		dinvRT.Unpack(clientReq.DinvB, &dinvMessage)
	}
	connections.Lock()

	if _, ok := connections.m[clientID]; !ok {
		b := Logger.PrepareSend("[GetNodes] rejected from client", clientID)
		if UseDinv {
			dinvb := dinvRT.Pack(dinvMessage)
			*addrSet = serverlib.GetNodesResponse{nil, b, dinvb}
		} else {
			*addrSet = serverlib.GetNodesResponse{nil, b, b}
		}

		connections.Unlock()

		return InvalidClientError(clientID)
	}

	var peerAddresses []serverlib.PeerInfo

	// DISCONNECTED is the zero value, so we can use it to test for non-existent clients
	if connections.m[connections.m[clientID].partner.ClientID] == nil ||
		connections.m[connections.m[clientID].partner.ClientID].status == DISCONNECTED {
		// try to find a partner now
		for id, c := range connections.m {
			if id != clientID && c.status == CONNECTED {
				// found a partner
				connections.m[clientID].partner = serverlib.PeerInfo{
					Address: c.address,
					RPCAddress: c.rpcAddress,
					ClientID: id,
					DisplayName: c.displayName,
				}

				break
			}
		}
	}

	// If our partner is connected, then produce it as a peer
	if connections.m[connections.m[clientID].partner.ClientID] != nil &&
		connections.m[connections.m[clientID].partner.ClientID].status == CONNECTED {
		peerAddresses = append(peerAddresses, connections.m[clientID].partner)
	}

	connections.Unlock()

	if connections.m[clientID].status == RECONNECTED {
		connections.Lock()
		conn := connections.m[clientID]
		conn.status = CONNECTED
		connections.m[clientID] = conn
		connections.Unlock()
	}

	b := Logger.PrepareSend("[GetNodes] accepted from client", clientID)
	if UseDinv {
		dinvb := dinvRT.Pack(dinvMessage)
		*addrSet = serverlib.GetNodesResponse{peerAddresses, b, dinvb}
	} else {
		*addrSet = serverlib.GetNodesResponse{peerAddresses, b, b}
	}
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
		connections.Lock()
		for id, connection := range connections.m {
			if connection.status == DISCONNECTED {
				if success, _ := connection.client.Recover(); success {
					connection.status = RECONNECTED
					connections.m[id] = connection
				}
			}
		}
		connections.Unlock()
	}
}

func awaitClientConnections(addr *net.TCPAddr) {
	inbound, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := inbound.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// read clientID
		var idBytes [8]byte
		n, err := conn.Read(idBytes[:])
		if err != nil {
			log.Fatal(err)
		}

		if n != 8 {
			log.Fatal("didn't read enough bytes")
		}

		clientId := binary.BigEndian.Uint64(idBytes[:])

		connections.Lock()

		connection := connections.m[clientId]
		if connection == nil {
			log.Fatal("client not connected")
		}

		log.Println("Connected to new client", clientId)

		connection.rpcClient = rpc.NewClient(conn)

		connections.Unlock()
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

	awaitAddr := serverAddr
	awaitAddr.Port += 10

	go awaitClientConnections(awaitAddr)

	v2 := os.Getenv("USE_DINV")
	UseDinv = true
	if v2 == "" {
		UseDinv = false
	}

	// Needed to make dinv work on windows as this is a function I have added
	// Comment after generating relevant logs
	if UseDinv {
		//dinvRT.DoFast()
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
