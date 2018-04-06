package main

import (
	"../clientlib"
	"github.com/faiface/pixel"
	"log"
	"time"
)

const (
	TimeDelta = -100 * time.Millisecond
)

var (
	RecordUpdates = make(chan clientlib.Update, 1000)
)

var (
	records = make(map[uint64]*PlayerRecord)
	history []clientlib.Update
	historyMap = make(map[uint64]interface{})
)

type PlayerRecord struct {
	ID    uint64
	Time  time.Time
	Pos   pixel.Vec
	Angle float64
}

func (r *PlayerRecord) Accept(update clientlib.Update) {
	r.Time = update.Time

	switch update.Kind {
	case clientlib.POSITION:
		r.Pos = update.Pos
		r.Angle = update.Angle
	}
}

func pruneHistory() {
	fence := Clock.GetCurrentTime().Add(TimeDelta)

	for len(history) > 0 {
		if history[0].Time.Before(fence) {
			// Remove this item if it's too old
			delete(historyMap, history[0].Nonce)

			if len(history) > 1 {
				history = history[1:]
			} else {
				history = nil
			}
		} else {
			// otherwise, quit pruning
			break
		}
	}
}

func RecordWorker() {
	for {
		// Get the next incoming update
		update := <-RecordUpdates

		if update.Time.Before(Clock.GetCurrentTime().Add(TimeDelta)) {
			// Ignore very old updates
			continue
		}

		// Prune history
		pruneHistory()

		if _, exists := historyMap[update.Nonce]; exists {
			// We've already seen this update
			continue
		}

		// Write down that we've seen this update
		historyMap[update.Nonce] = nil
		history = append(history, update)

		// Add this player to our records if we haven't heard of them before
		if records[update.PlayerID] == nil {
			log.Println("Heard of new player", update.PlayerID)
			records[update.PlayerID] = &PlayerRecord{
				ID: update.PlayerID,
			}
		}

		// Accept the update
		switch update.Kind {
		case clientlib.DEAD:
			// Remove the player if it's dead
			delete(records, update.PlayerID)
		default:
			// Otherwise update its record with whatever came in
			records[update.PlayerID].Accept(update)
		}

		// Display the update
		UpdateChannel <- update

		switch update.Kind {
		case clientlib.DEAD:
			// do nothing
		default:
			// Send the update out if it's not a death update
			OutgoingUpdates <- update
		}
	}
}
