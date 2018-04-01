package serverlib

import (
	"net/rpc"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/clientlib"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
	"time"
)

type ServerAPI interface {
	Register(address string, rpcAddress string, clientID uint64, displayName string) (clientlib.PeerNetSettings, error)
	Connect(clientID uint64) (bool, error)
	GetNodes(clientID uint64) ([]PeerInfo, error)

	// -----------------------------------------------------------------------------

	// KV: Key-value store API calls.
	KVGet(key int, clientId uint64) (crdtlib.GetReply, error)
	KVPut(key int, value crdtlib.ValueType) error

	// -----------------------------------------------------------------------------
}

type RPCServerAPI struct {
	api *rpc.Client
}

type PeerInfo struct {
	Address     string
	RPCAddress  string
	ClientID    uint64
	DisplayName string
}

// Error definitions

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

// -----------------------------------------------------------------------------

// KV: Key-value store API call implementations.

func (r *RPCServerAPI) KVGet(key int, clientId uint64) (crdtlib.GetReply, error) {

	arg := crdtlib.GetArg{clientId, key}
	var reply crdtlib.GetReply

	if err := r.doApiCall("TankServer.KVGet", &arg, &reply); err != nil {
		return crdtlib.GetReply{false, false, false, crdtlib.ValueType{0, 0}}, err
	}

	return reply, nil
}

func (r *RPCServerAPI) KVPut(key int, value crdtlib.ValueType) error {

	arg := crdtlib.PutArg{key, value}
	var reply crdtlib.PutReply

	if err := r.doApiCall("TankServer.KVPut", &arg, &reply); err != nil {
		return err
	}

	return nil
}

// -----------------------------------------------------------------------------

func (r *RPCServerAPI) Register(address string, rpcAddress string, clientID uint64, displayName string) (clientlib.PeerNetSettings, error) {
	request := PeerInfo{address, rpcAddress, clientID, displayName}
	var settings clientlib.PeerNetSettings

	if err := r.doApiCall("TankServer.Register", &request, &settings); err != nil {
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
