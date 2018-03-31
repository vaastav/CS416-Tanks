package clientlib

import (
	"net/rpc"
	"time"
)

type ClientClockAPI interface {
	TimeRequest() (time.Time, error)
	SetOffset(offset time.Duration) error
	Heartbeat(clientID uint64) error
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

func (c *ClientClockRemote) Heartbeat(clientID uint64) error {
	request := clientID
	var ack bool

	if err := c.doApiCall("ClockController.Heartbeat", &request, &ack); err != nil {
		return err
	}

	return nil
}
