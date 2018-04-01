/*

This package specifies the applications's interface to a conflict free replicated data store (CRDT) to be used in project 2 of UBC CS 416 2017W2.

*/

package crdtlib

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
)

// ----------------------------------------------------------------------------

// Define types.

// A int represents the type of keys in the key-value store.
// type int int

// A Value represents the type of values in the key-value store.
type ValueType struct {
	NumKills int
	NumDeaths int
}

// A ClientKVService represents a service for the client side of a bidirectional
// RPC.
type ClientKVService int

// A ConnectionStatus represents the status of a connection to the server.
type ConnectionStatus int

const (
	DISCONNECTED ConnectionStatus = iota
	CONNECTED
)

// A ConnectArg represents an argument type passed when a client sends the server an
// RPC to register itself.
type ConnectArg struct {
	ClientId int
	ClientIp string
}

// A FileExistsArg represents an argument type passed when a client sends the
// server an RPC to check if a key exists globally.
type KeyExistsArg struct {
	ClientId int
	ClientIp string
	Key      int
}

// A GetArg represents an argument type passed when a client the server an RPC
// to get the value of a key.
type GetArg struct {
	ClientId uint64
	// ClientIp string
	Key      int
}

// A GetReply represents the reply sent from the server to the client on a Get
// call.
type GetReply struct {
	Ok          bool
	HasAlready  bool
	Unavailable bool
	Value       ValueType
}

// A RetrieveArg represents an argument type passed when a server sends a client
// an RPC to retrieve the value given a key.
type RetrieveArg struct {
	Key int
}

// A RetrieveReply represents the reply sent from the client to the server on a
// Retrieve Call.
type RetrieveReply struct {
	Ok    bool
	Value ValueType
}

// A PutArg represents an argument type passed when a client sends the server an
// RPC to write a key-value pair.
type PutArg struct {
	// ClientId int
	// ClientIp string
	Key      int
	Value    ValueType
}

// A PutReply represents the reply sent from the server to the client on a Put
// call.
type PutReply struct {
	Ok bool
}

// An AddArg represents an argument type passed when a server sends a client an
// RPC to add a key-value pair.
type AddArg struct {
	Key   int
	Value ValueType
}

// An AddReply represents the reply sent from the client to the server on an Add
// call.
type AddReply struct {
	Ok bool
}

type GlobalMap struct {
	Mutex sync.RWMutex
	M     map[int]ValueType
}

// Implementation of the KVStore interface.
type KVS struct {
	ClientId         int
	ClientIp         string
	LocalPath        string
	Fname            string
	M                map[int]ValueType
	ConnectionStatus ConnectionStatus
	Server           *rpc.Client
}

// ----------------------------------------------------------------------------

// Define interfaces.

type KVStore interface {
	LocalKeyExists(key int) (exists bool, err error)

	GlobalKeyExists(key int) (exists bool, err error)

	Get(key int) (value ValueType, err error)

	Put(key int, value ValueType) (err error)

	Disconnect() (err error)
}

// ----------------------------------------------------------------------------

// Global variables and helper functions.

var globalMap GlobalMap = GlobalMap{sync.RWMutex{}, make(map[int]ValueType)}

func (ckvs *ClientKVService) Add(arg *AddArg, reply *AddReply) error {

	globalMap.Mutex.Lock()
	defer globalMap.Mutex.Unlock()
	globalMap.M[arg.Key] = arg.Value
	reply.Ok = true

	return nil

}

func (ckvs *ClientKVService) Retrieve(arg *RetrieveArg, reply *RetrieveReply) error {

	globalMap.Mutex.Lock()
	defer globalMap.Mutex.Unlock()
	reply.Value = globalMap.M[arg.Key]
	reply.Ok = true

	return nil
}

// Reads a map from a file on disk.
func ReadMap(localPath, fname string) (m map[int]ValueType, err error) {

	fname = path.Join(localPath, fname)

	if _, err = os.Stat(fname); os.IsNotExist(err) {
		// If the file does not exist, create one.
		f, err := os.Create(fname)
		if err != nil {
			return nil, err
		}
		f.Close()
	}

	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	d := gob.NewDecoder(bytes.NewBuffer(b))
	err = d.Decode(&m)
	if err != nil {
		return nil, err
	}

	return m, nil

}

// Returns the ID from a past connection to the server. If an ID does not exist,
// returns -1.
func FetchOldId(localPath string) (id int, err error) {

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		os.Mkdir(localPath, os.ModePerm)
	}

	id = -1
	fname := path.Join(localPath, "id.txt")

	if _, err := os.Stat(fname); os.IsNotExist(err) {
		// If the ID file does not exist, create one.
		f, err := os.Create(fname)
		if err != nil {
			return id, err
		}
		defer f.Close()
		return id, nil
	} else {
		// Otherwise, read the id from the file.
		f, err := os.Open(fname)
		if err != nil {
			return id, err
		}
		defer f.Close()
		idBuf := make([]byte, 64)
		n, err := f.Read(idBuf)
		if err != nil {
			return id, err
		}
		id, err = strconv.Atoi(string(idBuf[:n]))
		if err != nil {
			return id, err
		}
	}

	return id, nil

}

// Writes the ID to a file on disk.
func WriteId(localPath string, id int) error {

	fname := path.Join(localPath, "id.txt")

	f, err := os.OpenFile(fname, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(strconv.FormatInt(int64(id), 10))
	if err != nil {
		return err
	}
	f.Sync()

	return nil
}

// Writes a map to a file on disk.
func WriteMap(localPath, fname string, m map[int]ValueType) error {

	fname = path.Join(localPath, fname+".txt")

	if _, err := os.Stat(fname); os.IsNotExist(err) {
		// If the file does not exist, create one.
		f, err := os.Create(fname)
		if err != nil {
			return err
		}
		f.Close()
	}

	f, err := os.OpenFile(fname, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()

	// Serialize the map and write it to file.
	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)
	err = e.Encode(m)
	if err != nil {
		return err
	}
	_, err = f.Write(b.Bytes())
	if err != nil {
		return err
	}
	f.Sync()

	return nil
}

// ----------------------------------------------------------------------------

// Implementation of KVStore.

func (kvs KVS) LocalKeyExists(key int) (exists bool, err error) {

	if _, ok := kvs.M[key]; ok {
		return true, nil
	} else {
		return false, nil
	}

}

func (kvs KVS) GlobalKeyExists(key int) (exists bool, err error) {

	if kvs.ConnectionStatus == DISCONNECTED {
		return false, errors.New("GlobalKeyExists:: Disconnected!")
	}

	// Ask the server whether any client has this file or not.
	var reply bool
	arg := &KeyExistsArg{kvs.ClientId, kvs.ClientIp, key}
	err = kvs.Server.Call("KVService.KeyExists", arg, &reply)
	if err != nil {
		return false, err
	}

	return reply, nil

}

func (kvs KVS) Get(key int) (value ValueType, err error) {

	if kvs.ConnectionStatus == DISCONNECTED {
		return ValueType{}, errors.New("Get:: Disconnected!")
	}

	var reply GetReply
	// arg := &GetArg{uint64(kvs.ClientId), kvs.ClientIp, key}
	arg := &GetArg{uint64(kvs.ClientId), key}
	err = kvs.Server.Call("KVService.Get", arg, &reply)
	if err != nil {
		return ValueType{}, err
	}
	if reply.HasAlready {
		return kvs.M[key], nil
	} else {
		return reply.Value, nil
	}

}

func (kvs KVS) Put(key int, value ValueType) (err error) {

	if kvs.ConnectionStatus == DISCONNECTED {
		return errors.New("Put:: Disconnected!")
	}

	var reply PutReply
	// arg := &PutArg{kvs.ClientId, kvs.ClientIp, key, value}
	arg := &PutArg{key, value}
	err = kvs.Server.Call("KVService.Put", arg, &reply)
	if err != nil {
		return err
	}

	return nil

}

func (kvs KVS) Disconnect() (err error) {

	if kvs.ConnectionStatus == DISCONNECTED {
		// If the client is disconnect, return an error.
		return errors.New("Disconnecting in disconnected mode.")
	} else {
		// Otherwise, modify the connection status to DISCONNECTED, notify the
		// server and close the connection.
		var reply int
		arg := &ConnectArg{kvs.ClientId, kvs.ClientIp}
		err := kvs.Server.Call("KVService.Disconnect", arg, &reply)
		if err != nil {
			return err
		}
		kvs.Server.Close()
		kvs.ConnectionStatus = DISCONNECTED
	}

	return nil

}

// ----------------------------------------------------------------------------

func Connect(serverAddrArg string, localIp string) (kvstore KVStore, err error) {

	localPath := "stats-dir"

	// Instantiate a KVStore object.
	kvs := new(KVS)
	kvs.LocalPath = localPath
	kvs.Fname = "map_data.txt"
	kvs.M, err = ReadMap(kvs.LocalPath, kvs.Fname)
	globalMap.Mutex.Lock()
	globalMap.M = kvs.M
	globalMap.Mutex.Unlock()
	if err != nil {
		return kvs, err
	}

	// Listen to incoming connections.
	connection, err := net.Listen("tcp", localIp+":0")
	if err != nil {
		return kvs, err
	}

	// Store the client IP address and port.
	kvs.ClientIp = connection.Addr().String()

	// Register a ClientKVService to respond to requests from the server.
	service := new(ClientKVService)
	rpc.Register(service)

	// Service in a new goroutine.
	go rpc.Accept(connection)
	runtime.Gosched()

	// Establish a connection with the server.
	server, err := rpc.Dial("tcp", serverAddrArg)
	if err != nil {
		kvs.ConnectionStatus = DISCONNECTED
	} else {
		kvs.ConnectionStatus = CONNECTED
		kvs.Server = server
	}

	// Fetch an old ID from disk, if any.
	id, err := FetchOldId(localPath)
	if err != nil {
		return kvs, err
	}
	// use id somehow for now.
	fmt.Println(id)

	// Register with the server to provide the client's details, and obtain an ID
	// if requried.
	var reply int
	arg := &ConnectArg{id, connection.Addr().String()}
	err = server.Call("KVService.Connect", arg, &reply)
	if err != nil {
		kvs.ConnectionStatus = DISCONNECTED
		return kvs, err
	}
	kvs.ClientId = reply

	// Save the ID to disk for future use.
	WriteId(localPath, reply)

	// TODO: Read logs as part of a restart procedure.

	return kvs, nil

}
