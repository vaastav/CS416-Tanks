package serverlib

import (
	"../peerclientlib"
	"net"
	"net/rpc"	
	"time"
)

type ServerAPI interface {
	Register(address net.Addr, display_name string) (peerclientlib.PeerNetSettings, error)
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

func (r *RPCServerAPI) Register(address net.Addr, display_name string) (peerclientlib.PeerNetSettings, error) {
	request := PeerInfo{address, display_name}
	var settings peerclientlib.PeerNetSettings

	if err :=  r.doApiCall("TankServer.Register", &request, &settings); err != nil {
		return peerclientlib.PeerNetSettings{}, err
	}

	return settings, nil
}