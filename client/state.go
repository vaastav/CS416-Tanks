package main

import (
	"github.com/faiface/pixel"
	"log"
	// "../clientlib"
	"proj2_f4u9a_g8z9a_i4x8_s8a9/clientlib"
)

var (
	RecordUpdates = make(chan clientlib.Update, 1000)
)

var (
	records = make(map[uint64]*PlayerRecord)
)

type PlayerRecord struct {
	ID    uint64
	Pos   pixel.Vec
	Angle float64
}

func (r *PlayerRecord) Accept(update clientlib.Update) {
	switch update.Kind {
	case clientlib.POSITION:
		r.Pos = update.Pos
		r.Angle = update.Angle
	}
}

func RecordWorker() {
	for {
		// Get the next incoming update
		update := <-RecordUpdates

		// Add this player to our records if we haven't heard of them before
		if records[update.PlayerID] == nil {
			log.Println("heard of new player", update.PlayerID)
			records[update.PlayerID] = &PlayerRecord{
				ID: update.PlayerID,
			}
		}

		// Accept the update
		records[update.PlayerID].Accept(update)

		// TODO validate incoming updates

		// Display the update
		UpdateChannel <- update

		// Send the update out
		OutgoingUpdates <- update
	}
}
