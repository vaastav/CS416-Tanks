package clientlib

import (
	"../crdtlib"
	"github.com/DistributedClocks/GoVector/govec"
	"net/rpc"
	"time"
)

type ClientClockAPI interface {
	// -----------------------------------------------------------------------------

	// KV: Key-value store API calls.
	KVClientGet(key int, logger *govec.GoLog) (crdtlib.ValueType, error)
	KVClientPut(key int, vale crdtlib.ValueType, logger *govec.GoLog) error

	// -----------------------------------------------------------------------------
	TimeRequest() (time.Time, error)
	SetOffset(offset time.Duration) error
	Heartbeat(clientID uint64) error
	Recover() (bool, error)
	Ping() error
}

type ClientClockRemote struct {
	Conn *rpc.Client
}

const (
	TIMEOUT              = 20 * time.Second
	CONNECTIVITY_TIMEOUT = 2 * time.Second
)

type GetTimeRequest struct {
	B []byte
}

type GetTimeResponse struct {
	T time.Time
	B []byte
}

type SetOffsetRequest struct {
	Offset time.Duration
	B      []byte
}

type SetOffsetResponse struct {
	Ack bool
	B   []byte
}

type KVClientGetRequest struct {
	Key uint64
	B   []byte
}

type KVClientGetResponse struct {
	Value crdtlib.ValueType
	B     []byte
}

type KVClientPutRequest struct {
	Arg crdtlib.PutArg
	B   []byte
}

type KVClientPutResponse struct {
	Ack bool
	B   []byte
}

type DisconnectedError string

func (e DisconnectedError) Error() string {
	return "Disconnected from server"
}

func NewClientClockRemoteAPI(api *rpc.Client) *ClientClockRemote {
	return &ClientClockRemote{api}
}

func (c *ClientClockRemote) doApiCall(call string, request interface{}, response interface{}, timeout time.Duration) error {
	channel := c.Conn.Go(call, request, response, nil)
	select {
	case channel := <-channel.Done:
		return channel.Error
	case <-time.After(timeout):
		return DisconnectedError("")
	}
}

// -----------------------------------------------------------------------------

// KV: Get and Put functions.

func (c *ClientClockRemote) KVClientGet(key uint64, logger *govec.GoLog) (crdtlib.ValueType, error) {
	value := crdtlib.ValueType{0, 0}
	var response KVClientGetResponse
	b := logger.PrepareSend("[KVClientGet] requesting from client", key)
	request := KVClientGetRequest{key, b}
	if err := c.doApiCall("ClockController.KVClientGet", &request, &response, TIMEOUT); err != nil {
		logger.UnpackReceive("[KVClientGet] request from client failed", response.B, &value)
		return crdtlib.ValueType{0, 0}, nil
	}

	logger.UnpackReceive("[KVClientGet] request from client succeeded", response.B, &value)
	return response.Value, nil

}

func (c *ClientClockRemote) KVClientPut(key uint64, value crdtlib.ValueType, logger *govec.GoLog) error {
	arg := crdtlib.PutArg{key, value}
	var ok bool
	var response KVClientPutResponse
	b := logger.PrepareSend("[KVClientPut] requesting from client", key)
	request := KVClientPutRequest{arg, b}
	if err := c.doApiCall("ClockController.KVClientPut", &request, &response, TIMEOUT); err != nil {
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
	if err := c.doApiCall("ClockController.TimeRequest", &request, &response, TIMEOUT); err != nil {
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
	if err := c.doApiCall("ClockController.SetOffset", &request, &response, TIMEOUT); err != nil {
		logger.UnpackReceive("[SetOffset] command failed", response.B, &ack)
		return err
	}

	logger.UnpackReceive("[SetOffset] command succeeded", response.B, &ack)
	return nil
}

func (c *ClientClockRemote) Heartbeat(clientID uint64) error {
	request := clientID
	var ack bool

	if err := c.doApiCall("ClockController.Heartbeat", &request, &ack, CONNECTIVITY_TIMEOUT); err != nil {
		return err
	}

	return nil
}

func (c *ClientClockRemote) Recover() (bool, error) {
	request := 0
	var ack bool
	if err := c.doApiCall("ClockController.Recover", &request, &ack, TIMEOUT); err != nil {
		return false, err
	}

	return ack, nil
}

func (c *ClientClockRemote) Ping() error {
	request := 0
	var ack bool
	if err := c.doApiCall("ClockController.Ping", &request, &ack, CONNECTIVITY_TIMEOUT); err != nil {
		return err
	}

	return nil
}
