package clientlib

import (
	"net"
	"fmt"
)

type PeerNetSettings struct {
	MinimumPeerConnections uint8

	UniqueUserID string

	DisplayName string
}

type ClientAPI interface {
	NotifyUpdate(clientID uint64, update Update) error
	Register(clientID uint64, address string) error
}

type ClientAPIRemote struct {
	conn *net.UDPConn
}

type ClientAPIError string

func (e ClientAPIError) Error() string {
	return fmt.Sprintf("ClientAPI Error: %s", e)
}

func NewClientAPIRemote(conn *net.UDPConn) *ClientAPIRemote {
	return &ClientAPIRemote{
		conn: conn,
	}
}

func (a *ClientAPIRemote) doAPICall(msg ClientMessage) error {
	// Send our message
	err := SendMessage(a.conn, nil, &msg)
	if err != nil {
		return err
	}

	// Wait for a reply
	var reply ClientReply
	_, err = ReceiveMessage(a.conn, &reply)
	if err != nil {
		return err
	}

	// Return an error if one occurred
	switch reply.Kind {
	case OKAY:
		return nil
	case ERROR:
		return ClientAPIError(reply.Error)
	}

	panic("Unreachable")
}

func (a *ClientAPIRemote) NotifyUpdate(clientID uint64, update Update) error {
	return a.doAPICall(ClientMessage{
		Kind: UPDATE,
		ClientID: clientID,
		Update: update,
	})
}

func (a *ClientAPIRemote) Register(clientID uint64, address string) error {
	return a.doAPICall(ClientMessage{
		Kind: REGISTER,
		ClientID: clientID,
	})
}

type ClientAPIListener struct {
	table    ClientAPI
	conn *net.UDPConn
}

func NewClientAPIListener(table ClientAPI, conn *net.UDPConn) *ClientAPIListener {
	return &ClientAPIListener{
		table:    table,
		conn: conn,
	}
}

func (l *ClientAPIListener) Accept() error {
	// Receive a message and who it came from
	var msg ClientMessage
	addr, err := ReceiveMessage(l.conn, &msg)
	if err != nil {
		return err
	}

	// Process the message
	switch msg.Kind {
	case UPDATE:
		err = l.table.NotifyUpdate(msg.ClientID, msg.Update)
	case REGISTER:
		err = l.table.Register(msg.ClientID, msg.Address)
	}

	// Send a reply
	reply := ClientReply{
		Kind: OKAY,
	}

	if err != nil {
		// Include the error if one occurred
		reply.Kind = ERROR
		reply.Error = err.Error()
	}

	// Send the reply message
	return SendMessage(l.conn, addr, &reply)
}