package clientlib

import (
	"net/rpc"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/crdtlib"
	"time"
)

type ClientClockAPI interface {
	TimeRequest() (time.Time, error)
	SetOffset(offset time.Duration) error
	KVClientGet(key int) (crdtlib.ValueType, error)
	KVClientPut(key int, vale crdtlib.ValueType) error
}

type ClientClockRemote struct {
	api *rpc.Client
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

func (c *ClientClockRemote) KVClientGet(key int) (crdtlib.ValueType, error) {

	request := key
	value := crdtlib.ValueType{0, 0}

	if err := c.doApiCall("ClockController.KVClientGet", &request, &value); err != nil {
		return crdtlib.ValueType{0, 0}, nil
	}

	return value, nil

}

func (c *ClientClockRemote) KVClientPut(key int, value crdtlib.ValueType) error {

	arg := crdtlib.PutArg{key, value}
	var ok bool

	if err := c.doApiCall("ClockController.KVClientPut", &arg, &ok); err != nil {
		return err
	}

	return nil
}

// -----------------------------------------------------------------------------

func (c *ClientClockRemote) TimeRequest() (time.Time, error) {
	request := 0
	var t time.Time

	if err := c.doApiCall("ClockController.TimeRequest", &request, &t); err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func (c *ClientClockRemote) SetOffset(offset time.Duration) error {
	request := offset
	var ack bool

	if err := c.doApiCall("ClockController.SetOffset", &request, &ack); err != nil {
		return err
	}

	return nil
}
