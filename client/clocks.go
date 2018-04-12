package main

import (
	"../clientlib"
	"../crdtlib"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"path"
	"strconv"
	"time"
	"encoding/binary"
)

type ClockController int

// -----------------------------------------------------------------------------

// KV: Get and Put functions.

func WriteKVPair(key uint64, value crdtlib.ValueType) error {

	fname := path.Join(".", KVDir, strconv.FormatUint(key, 10)+".kv")

	if _, err := os.Stat(fname); os.IsNotExist(err) {
		f, err := os.Create(fname)
		if err != nil {
			return err
		}
		f.Close()
	}

	v0 := strconv.FormatInt(int64(value.NumKills), 10)
	v1 := strconv.FormatInt(int64(value.NumDeaths), 10)
	valueStr := v0 + "\n" + v1 + "\n"
	valueBytes := []byte(valueStr)
	err := ioutil.WriteFile(fname, valueBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClockController) KVClientGet(request clientlib.KVClientGetRequest, response *clientlib.KVClientGetResponse) error {

	key := request.Key
	var k uint64
	KVLogger.UnpackReceive("KVClientGet() Request received from server", request.B, &k)
	KVMap.Lock()
	defer KVMap.Unlock()
	value := KVMap.M[key]
	b := KVLogger.PrepareSend("KVClientGet() Request executed", value)
	*response = clientlib.KVClientGetResponse{value, b}

	return nil
}

func (c *ClockController) KVClientPut(request clientlib.KVClientPutRequest, response *clientlib.KVClientPutResponse) error {

	arg := request.Arg
	var k uint64
	var ok bool
	KVLogger.UnpackReceive("KVClientGet() Request received from server", request.B, &k)
	KVMap.Lock()
	defer KVMap.Unlock()
	key := arg.Key
	value := arg.Value
	KVMap.M[key] = value
	err := WriteKVPair(key, value)
	if err != nil {
		ok = false
		b := KVLogger.PrepareSend("KVClientPut() Request failed", ok)
		*response = clientlib.KVClientPutResponse{ok, b}
		return err
	}
	b := KVLogger.PrepareSend("KVClientPut() Request succeeded", ok)
	*response = clientlib.KVClientPutResponse{ok, b}
	ok = true

	return nil
}

// -----------------------------------------------------------------------------

func (c *ClockController) TimeRequest(request clientlib.GetTimeRequest, t *clientlib.GetTimeResponse) error {
	var i int
	Logger.UnpackReceive("TimeRequest() command received from server", request.B, &i)
	b := Logger.PrepareSend("TimeRequest() command executed", Clock.GetCurrentTime())
	*t = clientlib.GetTimeResponse{Clock.GetCurrentTime(), b}
	return nil
}

func (c *ClockController) SetOffset(request clientlib.SetOffsetRequest, response *clientlib.SetOffsetResponse) error {
	var offset time.Duration
	Logger.UnpackReceive("SetOffset() command received from server", request.B, &offset)
	Clock.SetOffset(request.Offset)
	b := Logger.PrepareSend("SetOffset() command executed", true)
	*response = clientlib.SetOffsetResponse{true, b}
	return nil
}

func (c *ClockController) Heartbeat(clientID uint64, ack *bool) error {
	peerLock.Lock()
	defer peerLock.Unlock()

	if _, ok := peers[clientID]; ok {
		peers[clientID].LastHeartbeat = Clock.GetCurrentTime()
	}

	return nil
}

func (c *ClockController) Recover(request int, ack *bool) error {
	peerLock.Lock()
	log.Println("Recover()")

	for id := range peers {
		if err := removePeer(id); err != nil {
			log.Println("Recover() error removing peer", id)
		}
	}

	peerLock.Unlock()
	*ack = true
	return nil
}

func (c *ClockController) Ping(request int, ack *bool) error {
	*ack = true
	return nil
}

// -----------------------------------------------------------------------------

func ClockWorker(serverAddr string) {
	awaitAddr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	awaitAddr.Port += 10

	conn, err := net.DialTCP("tcp", nil, awaitAddr)
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", RPCAddr)
	if err != nil {
		log.Fatal(err)
	}

	server := new(ClockController)
	rpc.Register(server)

	// start our inbound listener for other clients
	go rpc.Accept(inbound)

	// dial server and send our ID
	var idBytes [8]byte
	binary.BigEndian.PutUint64(idBytes[:], NetworkSettings.UniqueUserID)
	conn.Write(idBytes[:])

	log.Println("Clock worker connected")

	// then start serving RPC on it
	rpc.ServeConn(conn)

	log.Println("Clock worker died")
}
