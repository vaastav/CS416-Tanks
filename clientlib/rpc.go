package clientlib

import (
	"net"
	"bytes"
	"encoding/gob"
)

type ClientMessage struct {
	Kind ClientMessageKind
	ClientID uint64
	Update Update
	Address string
}

type ClientReply struct {
	Kind ClientReplyKind
	Error string
}

type ClientMessageKind int
type ClientReplyKind int

const (
	OKAY ClientReplyKind = iota
	ERROR
)

const (
	UPDATE ClientMessageKind = iota
	REGISTER
)

// We have to do a dance because UDP is packet based, and gob expects a stream based protocol

// Sends a message using conn, optionally to addr. If addr is null, whatever the remote
// end of conn is receives the message.
func SendMessage(conn *net.UDPConn, addr *net.UDPAddr, msg interface{}) error {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return err
	}

	var err error
	if addr == nil {
		_, err = conn.Write(buf.Bytes())
	} else {
		_, err = conn.WriteTo(buf.Bytes(), addr)
	}

	return err
}

func ReceiveMessage(conn *net.UDPConn, msg interface{}) (*net.UDPAddr, error) {
	buf := make([]byte, 0x400)

	n, addr, err := conn.ReadFromUDP(buf)

	if err != nil {
		return nil, err
	}

	if err := gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(msg); err != nil {
		return nil, err
	}

	return addr, nil
}