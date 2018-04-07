package serverlib

import (
	"../clientlib"
	"../crdtlib"
	"bitbucket.org/bestchai/dinv/dinvRT"
	"github.com/DistributedClocks/GoVector/govec"
	"net/rpc"
	"time"
)

type ServerAPI interface {
	// -----------------------------------------------------------------------------

	// KV: Key-value store API calls.
	KVGet(key uint64, clientId uint64, logger *govec.GoLog) (crdtlib.GetReply, error)
	KVPut(key uint64, value crdtlib.ValueType, logger *govec.GoLog) error

	// -----------------------------------------------------------------------------
	Connect(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog, useDinv bool) (int, error)
	Register(displayName string, clientID uint64, logger *govec.GoLog, useDinv bool) (clientlib.PeerNetSettings, error)
	GetNodes(clientID uint64, logger *govec.GoLog, useDinv bool) ([]PeerInfo, error)
	Disconnect(clientID uint64, logger *govec.GoLog, useDinv bool) (bool, error)
	NotifyFailure(clientID uint64) error
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

type ConnectRequest struct {
	Pi    PeerInfo
	B     []byte
	DinvB []byte
}

type RegisterRequest struct {
	DisplayName string
	ClientID    uint64
	B           []byte
	DinvB       []byte
}

type ClientIDRequest struct {
	ClientID uint64
	B        []byte
	DinvB    []byte
}

type RegisterResponse struct {
	Settings clientlib.PeerNetSettings
	B        []byte
	DinvB    []byte
}

type GetNodesResponse struct {
	Nodes []PeerInfo
	B     []byte
	DinvB []byte
}

type ConnectResponse struct {
	MinConnections int
	B              []byte
	DinvB          []byte
}

type DisconnectResponse struct {
	Ack   bool
	B     []byte
	DinvB []byte
}

type KVGetRequest struct {
	Arg crdtlib.GetArg
	B   []byte
}

type KVGetResponse struct {
	Reply crdtlib.GetReply
	B     []byte
}

type KVPutRequest struct {
	Arg crdtlib.PutArg
	B   []byte
}

type KVPutResponse struct {
	Reply crdtlib.PutReply
	B     []byte
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

func (r *RPCServerAPI) KVGet(key uint64, clientId uint64, logger *govec.GoLog) (crdtlib.GetReply, error) {
	arg := crdtlib.GetArg{clientId, key}
	var reply crdtlib.GetReply
	var response KVGetResponse
	b := logger.PrepareSend("[KVGet] Sending get key request to server", clientId)
	request := KVGetRequest{arg, b}
	if err := r.doApiCall("TankServer.KVGet", &request, &response); err != nil {
		logger.UnpackReceive("[KVGet] Get request to server errored out", response.B, &reply)
		return crdtlib.GetReply{false, false, false, crdtlib.ValueType{0, 0}}, err
	}

	logger.UnpackReceive("[KVGet] Get request to server succesful", response.B, &reply)
	return response.Reply, nil
}

func (r *RPCServerAPI) KVPut(key uint64, value crdtlib.ValueType, logger *govec.GoLog) error {
	arg := crdtlib.PutArg{key, value}
	var reply crdtlib.PutReply
	var response KVPutResponse
	b := logger.PrepareSend("[KVPut] Sending request to server", key)
	request := KVPutRequest{arg, b}
	if err := r.doApiCall("TankServer.KVPut", &request, &response); err != nil {
		logger.UnpackReceive("[KVPut] Put request to server errored out", response.B, &reply)
		return err
	}

	logger.UnpackReceive("[KVPut] Put request to server successful", response.B, &reply)
	return nil
}

// -----------------------------------------------------------------------------

func (r *RPCServerAPI) Register(displayName string, clientID uint64, logger *govec.GoLog, useDinv bool) (clientlib.PeerNetSettings, error) {
	var request RegisterRequest
	b := logger.PrepareSend("[Resgiter] request sent to server", displayName)
	if useDinv {
		dinvb := dinvRT.Pack(displayName)
		request = RegisterRequest{displayName, clientID, b, dinvb}
	} else {
		request = RegisterRequest{displayName, clientID, b, b}
	}
	var settings RegisterResponse
	var id uint64
	var clientID2 uint64

	if err := r.doApiCall("TankServer.Register", &request, &settings); err != nil {
		logger.UnpackReceive("[Register] request rejected by server", settings.B, id)
		if useDinv {
			dinvRT.Unpack(settings.DinvB, &clientID2)
		}
		return clientlib.PeerNetSettings{}, err
	}

	logger.UnpackReceive("[Register] request accepted by server", settings.B, &id)
	if useDinv {
		dinvRT.Unpack(settings.DinvB, &clientID2)
	}
	return settings.Settings, nil
}

func (r *RPCServerAPI) Connect(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog, useDinv bool) (int, error) {
	var response ConnectResponse
	var minConnections int
	var id uint64
	var request ConnectRequest
	pi := PeerInfo{address, rpcAddress, clientID, displayName}
	b := logger.PrepareSend("[Connect] request sent to server", clientID)
	if useDinv {
		dinvb := dinvRT.Pack(clientID)
		request = ConnectRequest{pi, b, dinvb}
	} else {
		request = ConnectRequest{pi, b, b}
	}
	if err := r.doApiCall("TankServer.Connect", &request, &response); err != nil {
		logger.UnpackReceive("[Connect] request rejected by server", response.B, &minConnections)
		if useDinv {
			dinvRT.Unpack(response.DinvB, &id)
		}
		return 0, err
	}

	logger.UnpackReceive("[Connect] request accepted by server", response.B, &minConnections)
	if useDinv {
		dinvRT.Unpack(response.DinvB, &id)
	}
	return response.MinConnections, nil
}

func (r *RPCServerAPI) Disconnect(clientID uint64, logger *govec.GoLog, useDinv bool) (bool, error) {
	var response DisconnectResponse
	var id uint64
	var request ClientIDRequest
	b := logger.PrepareSend("[Disconnect] request sent to server", clientID)
	if useDinv {
		dinvb := dinvRT.Pack(clientID)
		request = ClientIDRequest{clientID, b, dinvb}
	} else {
		request = ClientIDRequest{clientID, b, b}
	}
	if err := r.doApiCall("TankServer.Disconnect", &request, &response); err != nil {
		logger.UnpackReceive("[Disconnect] request rejected by server", response.B, &id)
		if useDinv {
			dinvRT.Unpack(response.DinvB, &id)
		}
		return false, err
	}

	logger.UnpackReceive("[Disconnect] request accepted by server", response.B, &id)
	if useDinv {
		dinvRT.Unpack(response.DinvB, &id)
	}
	return response.Ack, nil
}

func (r *RPCServerAPI) GetNodes(clientID uint64, logger *govec.GoLog, useDinv bool) ([]PeerInfo, error) {
	var request ClientIDRequest
	b := logger.PrepareSend("[GetNodes] request sent to server", clientID)
	if useDinv {
		dinvb := dinvRT.Pack(clientID)
		request = ClientIDRequest{clientID, b, dinvb}
	} else {
		request = ClientIDRequest{clientID, b, b}
	}
	var response GetNodesResponse
	var id uint64
	var id2 uint64
	if err := r.doApiCall("TankServer.GetNodes", &request, &response); err != nil {
		logger.UnpackReceive("[GetNodes] request rejected by server", response.B, &id)
		if useDinv {
			dinvRT.Unpack(response.DinvB, &id2)
		}
		return nil, err
	}

	logger.UnpackReceive("[GetNodes] request accepted by server", response.B, &id)
	if useDinv {
		dinvRT.Unpack(response.DinvB, &id2)
	}
	return response.Nodes, nil
}

func (r *RPCServerAPI) NotifyFailure(clientID uint64) error {
	request := clientID
	var ack bool

	if err := r.api.Call("TankServer.NotifyFailure", &request, &ack); err != nil {
		return err
	}

	return nil
}
