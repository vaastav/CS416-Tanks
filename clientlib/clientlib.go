package clientlib

import (
	"fmt"
	"github.com/DistributedClocks/GoVector/govec"
	"net"
)

type PeerNetSettings struct {
	MinimumPeerConnections int

	UniqueUserID uint64

	DisplayName string
}

type ClientAPI interface {
	NotifyUpdate(clientID uint64, update Update) error
	NotifyFailure(clientID uint64, ttl int) error
	Register(clientID uint64, address string, tcpAddress string) error
}

type ClientAPIRemote struct {
	Conn         *net.UDPConn
	Logger       *govec.GoLog
	IsLogUpdates bool
}

type ClientAPIError string

func (e ClientAPIError) Error() string {
	return fmt.Sprintf("ClientAPI Error: %s", string(e))
}

func NewClientAPIRemote(conn *net.UDPConn, logger *govec.GoLog, logUpdates bool) *ClientAPIRemote {
	return &ClientAPIRemote{
		Conn:         conn,
		Logger:       logger,
		IsLogUpdates: logUpdates,
	}
}

func (a *ClientAPIRemote) doAPICall(msg ClientMessage) error {
	// Send our message
	err := SendMessage(a.Conn, nil, &msg, a.Logger, a.IsLogUpdates)
	if err != nil {
		return err
	}

	// Wait for a reply
	var reply ClientReply
	_, err = ReceiveMessage(a.Conn, &reply, a.Logger, a.IsLogUpdates)
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

func (a *ClientAPIRemote) doAPICallAsync(msg ClientMessage) error {
	// Send our message
	err := SendMessage(a.Conn, nil, &msg, a.Logger, a.IsLogUpdates)
	if err != nil {
		return err
	}

	// don't wait for a reply
	return nil
}

func (a *ClientAPIRemote) NotifyUpdate(clientID uint64, update Update) error {
	return a.doAPICallAsync(ClientMessage{
		Kind:     UPDATE,
		ClientID: clientID,
		Update:   update,
	})
}

func (a *ClientAPIRemote) NotifyFailure(clientID uint64, ttl int) error {
	return a.doAPICallAsync(ClientMessage{
		Kind:     FAILURE,
		ClientID: clientID,
		Ttl:      ttl,
	})
}

func (a *ClientAPIRemote) Register(clientID uint64, address string, tcpAddress string) error {
	return a.doAPICall(ClientMessage{
		Kind:       REGISTER,
		ClientID:   clientID,
		Address:    address,
		TcpAddress: tcpAddress,
	})
}

type ClientAPIListener struct {
	table        ClientAPI
	conn         *net.UDPConn
	Logger       *govec.GoLog
	IsLogUpdates bool
}

func NewClientAPIListener(table ClientAPI, conn *net.UDPConn, logger *govec.GoLog, logUpdates bool) *ClientAPIListener {
	return &ClientAPIListener{
		table:        table,
		conn:         conn,
		Logger:       logger,
		IsLogUpdates: logUpdates,
	}
}

func (l *ClientAPIListener) Accept() error {
	// Receive a message and who it came from
	var msg ClientMessage
	addr, err := ReceiveMessage(l.conn, &msg, l.Logger, l.IsLogUpdates)
	if err != nil {
		return err
	}

	// Process the message
	switch msg.Kind {
	case UPDATE:
		// NotifyUpdate doesn't need a response
		return l.table.NotifyUpdate(msg.ClientID, msg.Update)
	case FAILURE:
		return l.table.NotifyFailure(msg.ClientID, msg.Ttl)
	case REGISTER:
		err = l.table.Register(msg.ClientID, msg.Address, msg.TcpAddress)
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
	return SendMessage(l.conn, addr, &reply, l.Logger, l.IsLogUpdates)
}
