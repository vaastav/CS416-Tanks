package main

import (
	"math/rand"
	"time"
)

const (
	DirectionChangeInterval = time.Millisecond * 850
	ShotInterval            = time.Millisecond * 800
	BotSpeed                = 185.0 // Higher speed than regular player b/c bot sleeps in between moves
)

func GenerateMoves() {
	lastDirChange := Clock.GetCurrentTime()
	lastShotFired := lastDirChange
	lastTime := lastDirChange
	var x, y, dt float64

	for {
		// Generate position
		if time.Since(lastDirChange) > DirectionChangeInterval {
			x, y = randomDir()*BotSpeed, randomDir()*BotSpeed
			lastDirChange = Clock.GetCurrentTime()
		}
		dt = time.Since(lastTime).Seconds()
		lastTime = Clock.GetCurrentTime()

		update := localPlayer.Update().Timestamp(Clock.GetCurrentTime())
		newX, newY := update.Pos.X+(x*dt), update.Pos.Y+(y*dt)

		if !(newX < MinX || newX > MaxX) {
			update.Pos.X = newX
		}
		if !(newY < MinY || newY > MaxY) {
			update.Pos.Y = newY
		}

		// Point shot angle at rand player
		if time.Since(lastShotFired) > ShotInterval && len(playerIds) > 0 {
			index := rand.Intn(len(playerIds))
			id := playerIds[index]
			update.Angle = players[id].Pos.Sub(update.Pos).Angle()
			lastShotFired = Clock.GetCurrentTime()
			// TODO: PEW PEW PEW!
		}

		localPlayer.Accept(update)

		RecordUpdates <- update
		time.Sleep(time.Millisecond * 70) // Don't want to bombard the network with updates
	}
}

func randomDir() float64 {
	return (rand.Float64()*2 - 1) * 1.0
}
