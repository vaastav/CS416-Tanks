package main

import (
	"../clientlib"
	"log"
	"net"
	"net/rpc"
	"../crdtlib"
	"time"
)

type ClockController int

func (c *ClockController) TimeRequest(request clientlib.GetTimeRequest, t * clientlib.GetTimeResponse) error {
	var i int
	Logger.UnpackReceive("[TimeRequest] command received from server", request.B, &i)
	b := Logger.PrepareSend("[TimeRequest] command executed", Clock.GetCurrentTime())
	*t = clientlib.GetTimeResponse{Clock.GetCurrentTime(), b}
	return nil
}

func (c *ClockController) SetOffset(request clientlib.SetOffsetRequest, response *clientlib.SetOffsetResponse) error {
	var offset time.Duration
	Logger.UnpackReceive("[SetOffset] command received from server", request.B, &offset)
	Clock.Offset = request.Offset
	b := Logger.PrepareSend("[SetOffset] command executed", true)
	*response = clientlib.SetOffsetResponse{true, b}
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
