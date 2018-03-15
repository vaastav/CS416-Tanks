/*
	Implements a thin server for P2P Battle Tanks game for CPSC 416 Project 2.
	This server is responsible for peer discovery and clock synchronisation

	Usage:
		go run server.go <IP Address : Port>
*/

package main

import (
	"log"
	"fmt"
	"net"
	"net/rpc"
	"os"
)

type TankServer int

func (s *TankServer) Register () error {
	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run server.go <IP Address : Port")
	}
	ip_addr := os.Args[1]
	// TODO : Does the server need to have its connection as UDP as well?
	// I imagine we can let it be as TCP
	server_addr, err := net.ResolveTCPAddr("tcp", ip_addr)
	if err != nil {
		log.Fatal(err)
	}

	inbound, err := net.ListenTCP("tcp", server_addr)
	if err != nil {
		log.Fatal(err)
	}

	server := new(TankServer)
	rpc.Register(server)
	fmt.Println("Listening now")
	rpc.Accept(inbound)
}
