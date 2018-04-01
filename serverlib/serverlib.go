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
	NotifyConnection(connectionStatus clientlib.Status, peerID uint64, reporterID uint64) (bool, error)
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

func (r *RPCServerAPI) NotifyConnection(connectionStatus clientlib.Status, peerID uint64, reporterID uint64) (bool, error) {
	request := ConnectionInfo{Status:connectionStatus, PeerID:peerID, ReporterID: reporterID}
	var ack bool

	if err := r.doApiCall("TankServer.NotifyConnection", &request, &ack); err != nil {
		return false, err
	}

	return ack, nil
}
