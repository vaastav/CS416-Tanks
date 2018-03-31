package serverlib

import (
	"../clientlib"
	"net/rpc"
	"time"
)

type ServerAPI interface {
	Register(address string, rpcAddress string, clientID uint64, displayName string) (clientlib.PeerNetSettings, error)
	Connect(clientID uint64) (bool, error)
	GetNodes(clientID uint64) ([]PeerInfo, error)
	// TODO: add method to notify server of a peer disconnection
}

type RPCServerAPI struct {
	api *rpc.Client
}

type PeerInfo struct {
	Address string
	RPCAddress string
	ClientID uint64
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

func (r *RPCServerAPI) Register(address string, rpcAddress string, clientID uint64, displayName string) (clientlib.PeerNetSettings, error) {
	request := PeerInfo{address, rpcAddress, clientID, displayName}
	var settings clientlib.PeerNetSettings

	if err :=  r.doApiCall("TankServer.Register", &request, &settings); err != nil {
		return clientlib.PeerNetSettings{}, err
	}

	return settings, nil
}

func (r *RPCServerAPI) Connect(clientID uint64) (bool, error) {
	var ack bool
	
	if err := r.doApiCall("TankServer.Connect", &clientID, &ack); err != nil {
		return false, err
	}

	return ack, nil
}

func (r *RPCServerAPI) GetNodes(clientID uint64) ([]PeerInfo, error) {
	var nodes []PeerInfo

	if err := r.doApiCall("TankServer.GetNodes", &clientID, &nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}