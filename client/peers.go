package main

import (
	"../clientlib"
)

var (
	RecordUpdates = make(chan clientlib.Update, 100)
)

var (
	records = make(map[uint64]*clientlib.PlayerRecord)
)

func PeerWorker() {
	for {
		// Get the next incoming update
		update := <- RecordUpdates

		// Add this player to our records if we haven't heard of them before
		if records[update.PlayerID] == nil {
			records[update.PlayerID] = &clientlib.PlayerRecord{
				ID: update.PlayerID,
			}
		}

		// Accept the update
		records[update.PlayerID].Accept(update)

		// TODO validate incoming updates
		UpdateChannel <- update
	}
}