package main

import (
	"log"
	"net"
	"net/rpc"
	"time"
)

type ClockController int

func (c *ClockController) TimeRequest(request int, t * time.Time) error {
	*t = Clock.GetCurrentTime()
	return nil
}

func (c *ClockController) SetOffset(offset time.Duration, ack * bool) error {
	Clock.Offset = offset
	return nil
}

func ClockWorker() {
	// TODO: this listens to TCP connection
	inbound, err := net.ListenTCP("tcp", RPCAddr)
	if err != nil {
		// OK to exit here; we can't handle this failure
		log.Fatal(err)
	}

	server := new(ClockController)
	rpc.Register(server)
	rpc.Accept(inbound)
}