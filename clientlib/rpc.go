package clientlib

import (
	"bytes"
	"encoding/gob"
	"github.com/DistributedClocks/GoVector/govec"
	"net"
)

type ClientMessage struct {
	Kind       ClientMessageKind
	ClientID   uint64
	Update     Update
	Address    string
	TcpAddress string
	Ttl        int
}

type ClientReply struct {
	Kind  ClientReplyKind
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
	FAILURE
	REGISTER
)

// We have to do a dance because UDP is packet based, and gob expects a stream based protocol

// Sends a message using conn, optionally to addr. If addr is null, whatever the remote
// end of conn is receives the message.
func SendMessage(conn *net.UDPConn, addr *net.UDPAddr, msg interface{}, logger *govec.GoLog, logUpdates bool) error {
	buf := make([]byte, 0x400)
	if logUpdates {
		buf = logger.PrepareSend("[SendMessage] sending message to peer", msg)
	} else {
		var bufBytes bytes.Buffer
		if err := gob.NewEncoder(&bufBytes).Encode(msg); err != nil {
			return err
		}

		buf = bufBytes.Bytes()
	}

	var err error
	if addr == nil {
		_, err = conn.Write(buf)
	} else {
		_, err = conn.WriteTo(buf, addr)
	}

	return err
}

func ReceiveMessage(conn *net.UDPConn, msg interface{}, logger *govec.GoLog, logUpdates bool) (*net.UDPAddr, error) {
	buf := make([]byte, 0x400)

	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}

	if logUpdates {
		logger.UnpackReceive("[ReceiveMessage] received message from peer", buf[:n], msg)
	} else {
		if err := gob.NewDecoder(bytes.NewReader(buf[:n])).Decode(msg); err != nil {
			return nil, err
		}
	}

	return addr, nil
}
