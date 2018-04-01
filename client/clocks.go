package main

import (
	"log"
	"net"
	"net/rpc"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
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
