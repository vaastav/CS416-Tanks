package main

import (
	"io/ioutil"
	"os"
	"log"
	"net"
	"net/rpc"
	"path"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
	"strconv"
	"time"
)

type ClockController int

func (c *ClockController) TimeRequest(request int, t *time.Time) error {
	*t = Clock.GetCurrentTime()
	return nil
}

func (c *ClockController) SetOffset(offset time.Duration, ack *bool) error {
	Clock.Offset = offset
	return nil
}

// -----------------------------------------------------------------------------

// KV: Get and Put functions.

func (c *ClockController) KVClientGet(key int, value *crdtlib.ValueType) error {

	KVMap.Lock()
	defer KVMap.Unlock()
	*value = KVMap.M[key]

	return nil
}

func WriteKVPair(key int, value crdtlib.ValueType) error {

	fname := path.Join(".", KVDir, strconv.FormatInt(int64(key), 10) + ".kv")

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

func (c *ClockController) KVClientPut(arg *crdtlib.PutArg, ok *bool) error {

	KVMap.Lock()
	defer KVMap.Unlock()
	key := arg.Key
	value := arg.Value
	KVMap.M[key] = value
	err := WriteKVPair(key, value)
	if err != nil {
		return err
	}
	*ok = true

	return nil
}

// -----------------------------------------------------------------------------

func ClockWorker() {
	inbound, err := net.ListenTCP("tcp", RPCAddr)
	if err != nil {
		log.Fatal(err)
	}

	server := new(ClockController)
	rpc.Register(server)
	rpc.Accept(inbound)
}
