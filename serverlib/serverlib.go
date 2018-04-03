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
	KVGet(key uint64, clientId uint64, logger *govec.GoLog) (crdtlib.GetReply, error)
	KVPut(key uint64, value crdtlib.ValueType, logger *govec.GoLog) error

	// -----------------------------------------------------------------------------
	Register(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog) (clientlib.PeerNetSettings, error)
	Connect(clientID uint64, logger *govec.GoLog) (bool, error)
	GetNodes(clientID uint64, logger *govec.GoLog) ([]PeerInfo, error)
	NotifyConnection(connectionStatus clientlib.Status, peerID uint64, reporterID uint64) (bool, error)
}

type RPCServerAPI struct {
	api *rpc.Client
}

type PeerInfo struct {
	Address     string
	RPCAddress  string
	ClientID    uint64
	DisplayName string
	B           []byte
}

type ClientIDRequest struct {
	ClientID uint64
	B        []byte
}

type PeerSettingsRequest struct {
	Settings clientlib.PeerNetSettings
	B        []byte
}

type GetNodesResponse struct {
	Nodes []PeerInfo
	B     []byte
}

type ConnectResponse struct {
	Ack bool
	B   []byte
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

type ConnectionInfo struct {
	Status     clientlib.Status /* The connection status of the effected node */
	PeerID     uint64           /* The ID of the effected node */
	ReporterID uint64           /* The ID of the node notifying the server */
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

func (r *RPCServerAPI) Register(address string, rpcAddress string, clientID uint64, displayName string, logger *govec.GoLog) (clientlib.PeerNetSettings, error) {
	b := logger.PrepareSend("[Resgiter] request sent to server", address)
	request := PeerInfo{address, rpcAddress, clientID, displayName, b}
	var settings PeerSettingsRequest
	var id uint64

	if err := r.doApiCall("TankServer.Register", &request, &settings); err != nil {
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

	logger.UnpackReceive("[GetNodes] request accepted by server", response.B, &id)
	return response.Nodes, nil
}


func (r *RPCServerAPI) NotifyConnection(connectionStatus clientlib.Status, peerID uint64, reporterID uint64) (bool, error) {
	request := ConnectionInfo{Status:connectionStatus, PeerID:peerID, ReporterID: reporterID}
	var ack bool

	if err := r.api.Call("TankServer.NotifyConnection", &request, &ack); err != nil {
		return false, err
	}

	return ack, nil
}
