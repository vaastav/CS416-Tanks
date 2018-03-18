package main

import (
	"../clientlib"
	"github.com/faiface/pixel"
)

var (
	RecordUpdates = make(chan clientlib.Update, 100)
)

var (
	records = make(map[uint64]*PlayerRecord)
)

type PlayerRecord struct {
	ID uint64
	Pos pixel.Vec
	Angle float64
}

func (r *PlayerRecord) Accept(update clientlib.Update) {
	switch update.Kind {
	case clientlib.POSITION:
		r.Pos = update.Pos
		r.Angle = update.Angle
	}
}

func PeerWorker() {
	for {
		// Get the next incoming update
		update := <- RecordUpdates

		// Add this player to our records if we haven't heard of them before
		if records[update.PlayerID] == nil {
			records[update.PlayerID] = &PlayerRecord{
				ID: update.PlayerID,
			}
		}

		// Accept the update
		records[update.PlayerID].Accept(update)

		// TODO validate incoming updates
		UpdateChannel <- update
	}
}