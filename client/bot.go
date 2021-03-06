package main

import (
	"math/rand"
	"time"
)

const (
	DirectionChangeInterval = time.Millisecond * 875
	ShotInterval            = time.Millisecond * 1400
	BotSpeed                = 70.0
	TickInterval            = time.Second / 60
)

func GenerateMoves() {
	lastDirChange := Clock.GetCurrentTime()
	lastShotFired := lastDirChange
	lastTime := lastDirChange
	var x, y, dt float64

	time.Sleep(time.Second * 3)

	last := time.Now()

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
		if time.Since(lastShotFired) > ShotInterval && len(players) >= 1 {
			index := rand.Intn(len(players))
			playerID, count := uint64(0), 0
			for id := range players {
				if count == index {
					playerID = id
					break
				}
				count++
			}
			if playerID != 0 {
				update.Angle = players[playerID].Pos.Sub(update.Pos).Angle()
			}
		}

		localPlayer.Accept(update)
		RecordUpdates <- update

		//time.Sleep(time.Millisecond * 10)

		if time.Since(lastShotFired) > ShotInterval && len(players) >= 1 {
			FireBullet() // PEW PEW PEW!
			lastShotFired = Clock.GetCurrentTime()
		}

		dt := time.Now().Sub(last)
		if dt < TickInterval {
			time.Sleep(TickInterval - dt)
		}

		last = time.Now()
	}
}

func randomDir() float64 {
	return (rand.Float64()*2 - 1) * 1.0
}
