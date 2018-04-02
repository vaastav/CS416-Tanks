package clientlib

import (
	"github.com/DistributedClocks/GoVector/govec"
	"net/rpc"
	"../crdtlib"
	"time"
)

type ClientClockAPI interface {
	TimeRequest() (time.Time, error)
	SetOffset(offset time.Duration) error
	KVClientGet(key int, logger *govec.GoLog) (crdtlib.ValueType, error)
	KVClientPut(key int, vale crdtlib.ValueType, logger *govec.GoLog) error
}

type ClientClockRemote struct {
	api *rpc.Client
}

type GetTimeRequest struct {
	B []byte
}

type GetTimeResponse struct {
	T time.Time
	B []byte
}

type SetOffsetRequest struct {
	Offset time.Duration
	B []byte
}

type SetOffsetResponse struct {
	Ack bool
	B []byte
}

type KVClientGetRequest struct {
	Key int
	B []byte
}

type KVClientGetResponse struct {
	Value crdtlib.ValueType
	B []byte
}

type KVClientPutRequest struct {
	Arg crdtlib.PutArg
	B []byte
}

type KVClientPutResponse struct {
	Ack bool
	B []byte
}

type DisconnectedError string

func (e DisconnectedError) Error() string {
	return "Disconnected from server"
}

func NewClientClockRemoteAPI(api *rpc.Client) *ClientClockRemote {
	return &ClientClockRemote{api}
}

func (c *ClientClockRemote) doApiCall(call string, request interface{}, response interface{}) error {
	channel := c.api.Go(call, request, response, nil)
	select {
	case channel := <-channel.Done:
		return channel.Error
	case <-time.After(20 * time.Second):
		return DisconnectedError("")
	}
}

// -----------------------------------------------------------------------------

// KV: Get and Put functions.

func (c *ClientClockRemote) KVClientGet(key int, logger *govec.GoLog) (crdtlib.ValueType, error) {
	value := crdtlib.ValueType{0, 0}
	var response KVClientGetResponse
	b := logger.PrepareSend("[KVClientGet] requesting from client", key)
	request := KVClientGetRequest{key, b}
	if err := c.doApiCall("ClockController.KVClientGet", &request, &response); err != nil {
		logger.UnpackReceive("[KVClientGet] request from client failed", response.B, &value)
		return crdtlib.ValueType{0, 0}, nil
	}

	logger.UnpackReceive("[KVClientGet] request from client succeeded", response.B, &value)
	return response.Value, nil

}

func (c *ClientClockRemote) KVClientPut(key int, value crdtlib.ValueType, logger *govec.GoLog) error {

	arg := crdtlib.PutArg{key, value}
	var ok bool
	var response KVClientPutResponse
	b := logger.PrepareSend("[KVClientPut] requesting from client", key)
	request := KVClientPutRequest{arg, b}
	if err := c.doApiCall("ClockController.KVClientPut", &request, &response); err != nil {
		logger.UnpackReceive("[KVClientPut] request from client failed", response.B, &ok)
		return err
	}

	logger.UnpackReceive("[KVClientPut] request from client succeeded", response.B, &ok)
	return nil
}

// -----------------------------------------------------------------------------

func (c *ClientClockRemote) TimeRequest(logger *govec.GoLog) (time.Time, error) {
	var t time.Time
	var response GetTimeResponse
	b := logger.PrepareSend("[TimeRequest] sending command to client", 0)
	request := GetTimeRequest{b}
	if err := c.doApiCall("ClockController.TimeRequest", &request, &response); err != nil {
		logger.UnpackReceive("[TimeRequest] command failed", response.B, &t)
		return time.Time{}, err
	}

	logger.UnpackReceive("[TimeRequest] command succeeded", response.B, &t)
	return response.T, nil
}

func (c *ClientClockRemote) SetOffset(offset time.Duration, logger *govec.GoLog) error {
	var ack bool
	var response SetOffsetResponse
	b := logger.PrepareSend("[SetOffset] sending command to client", offset)
	request := SetOffsetRequest{offset, b}
	if err := c.doApiCall("ClockController.SetOffset", &request, &response); err != nil {
		logger.UnpackReceive("[SetOffset] command failed", response.B, &ack)
		return err
	}

	logger.UnpackReceive("[SetOffset] command succeeded", response.B, &ack)
	return nil
}
