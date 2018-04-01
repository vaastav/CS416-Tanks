package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"path"
	"path/filepath"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
	"strconv"
	"strings"
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

func ReadMapFromFile() (map[int]crdtlib.ValueType, error) {

	M := make(map[int]crdtlib.ValueType)

	files, err := filepath.Glob(path.Join(".", KVDir, "*.kv"))
	if err != nil {
		return M, err
	}

	for _, file := range files {
		_, fname := path.Split(file)
		fname = strings.Split(fname, ".")[0]
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return M, err
		}
		dataStr := strings.Split(string(data), "\n")
		v0, _ := strconv.Atoi(dataStr[0])
		v1, _ := strconv.Atoi(dataStr[1])
		k, _ := strconv.Atoi(fname)
		val := crdtlib.ValueType{v0, v1}
		M[k] = val
	}

	return M, nil
}

func (c *ClockController) KVClientGet(key int, value *crdtlib.ValueType) error {

	KVMap.Lock()
	defer KVMap.Unlock()
	*value = KVMap.M[key]

	return nil
}

// TODO: Implement this.
func (c *ClockController) KVClientPut(arg *crdtlib.PutArg, ok *bool) error {
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
