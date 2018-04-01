package serverlib

import (
	"github.com/DistributedClocks/GoVector/govec"
	"net/rpc"
	"../clientlib"
	"../crdtlib"
	"time"
)

type ServerAPI interface {
	// -----------------------------------------------------------------------------

	// KV: Key-value store API calls.
	KVGet(key int, clientId uint64) (crdtlib.GetReply, error)
	KVPut(key int, value crdtlib.ValueType) error

	// -----------------------------------------------------------------------------
	Register(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog) (clientlib.PeerNetSettings, error)
	Connect(clientID uint64, logger *govec.GoLog) (bool, error)
	GetNodes(clientID uint64, logger *govec.GoLog) ([]PeerInfo, error)
}

type RPCServerAPI struct {
	api *rpc.Client
}

type PeerInfo struct {
	Address     string
	RPCAddress  string
	ClientID    uint64
	DisplayName string
	B []byte
}

type ClientIDRequest struct {
	ClientID uint64
	B []byte
}

type PeerSettingsRequest struct {
	Settings clientlib.PeerNetSettings
	B []byte
}

type GetNodesResponse struct {
	Nodes []PeerInfo
	B []byte
}

type ConnectResponse struct {
	Ack bool
	B []byte
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

func (r *RPCServerAPI) Register(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog) (clientlib.PeerNetSettings, error) {
	b := logger.PrepareSend("[Resgiter] request sent to server", address)
	request := PeerInfo{address, rpcAddress, clientID, displayName, b}
	var settings PeerSettingsRequest
	var id uint64

	if err :=  r.doApiCall("TankServer.Register", &request, &settings); err != nil {
		logger.UnpackReceive("[Register] request rejected by server", settings.B, id)
		return clientlib.PeerNetSettings{}, err
	}

	logger.UnpackReceive("[Register] request accepted by server", settings.B, &id)
	return settings.Settings, nil
}

func (r *RPCServerAPI) Connect(clientID uint64, logger *govec.GoLog) (bool, error) {
	var response ConnectResponse
	var ack bool
	b := logger.PrepareSend("[Connect] request sent to server", clientID)
	request := ClientIDRequest{clientID, b}
	if err := r.doApiCall("TankServer.Connect", &request, &response); err != nil {
		logger.UnpackReceive("[Connect] request rejected by server", response.B, &ack)
		return false, err
	}

	logger.UnpackReceive("[Connect] request accepted by server", response.B, &ack)
	return response.Ack, nil
}

func (r *RPCServerAPI) GetNodes(clientID uint64, logger *govec.GoLog) ([]PeerInfo, error) {

	b := logger.PrepareSend("[GetNodes] request sent to server", clientID)
	request := ClientIDRequest{clientID, b}
	var response GetNodesResponse
	var id uint64
	if err := r.doApiCall("TankServer.GetNodes", &request, &response); err != nil {
		logger.UnpackReceive("[GetNodes] request rejected by server", response.B, &id)
		return nil, err
	}

	logger.UnpackReceive("[GetNodes] request rejected by server", response.B, &id)
	return response.Nodes, nil
}
