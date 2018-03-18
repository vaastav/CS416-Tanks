package clientlib

import "github.com/faiface/pixel"

type PeerNetSettings struct {
	MinimumPeerConnections uint8

	UniqueUserID string

	DisplayName string
}

type PlayerRecord struct {
	ID uint64
	Pos pixel.Vec
	Angle float64
}

func (r *PlayerRecord) Accept(update Update) {
	switch update.Kind {
	case POSITION:
		r.Pos = update.Pos
		r.Angle = update.Angle
	}
}
