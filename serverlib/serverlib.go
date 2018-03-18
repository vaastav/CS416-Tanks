package serverlib

import (
	"../clientlib"
	"net"
	"net/rpc"	
	"time"
)

type ServerAPI interface {
	Register(address net.Addr, display_name string) (clientlib.PeerNetSettings, error)
	GetNodes(uuid string) ([]net.Addr, error)
}

type RPCServerAPI struct {
	api *rpc.Client
}

type PeerInfo struct {
	Address net.Addr
	DisplayName string
}

// Error defintions

type DisconnectedError string

func (e DisconnectedError) Error() string {
	return "Disconnected from server"
}

func NewRPCServerAPI(api *rpc.Client) *RPCServerAPI {
	return &RPCServerAPI{api}
}

func (r *RPCServerAPI) doApiCall(call string, request interface{}, response interface{}) error {
	c := r.api.Go(call, request, response, nil)

	select {
	case c := <-c.Done:
		return c.Error
	case <-time.After(20 * time.Second):
		return DisconnectedError("")
	}
}

func (r *RPCServerAPI) Register(address net.Addr, display_name string) (clientlib.PeerNetSettings, error) {
	request := PeerInfo{address, display_name}
	var settings clientlib.PeerNetSettings

	if err :=  r.doApiCall("TankServer.Register", &request, &settings); err != nil {
		return clientlib.PeerNetSettings{}, err
	}

	return settings, nil
}

func (r *RPCServerAPI) Connect(settings clientlib.PeerNetSettings) (bool, error) {
	var ack bool
	
	if err := r.doApiCall("TankServer.Connect", &settings, &ack); err != nil {
		return false, err
	}

	return ack, nil
}

func (r *RPCServerAPI) GetNodes(settings clientlib.PeerNetSettings) ([]string, error) {
	var nodes []string

	if err := r.doApiCall("TankServer.GetNodes", &settings, &nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}