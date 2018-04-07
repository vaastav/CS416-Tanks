package clientlib

import (
	"github.com/faiface/pixel"
	"math/rand"
	"time"
)

const (
	POSITION UpdateKind = iota
	FIRE
	DEAD
)

const (
	PlayerSpeed = 150.0
)

type UpdateKind int

type Update struct {
	Kind        UpdateKind
	Time        time.Time
	Nonce       uint64
	PlayerID    uint64
	OtherPlayer uint64
	Pos         pixel.Vec
	Angle       float64
}

func DeadPlayer(playerID uint64, cause uint64) Update {
	return Update{
		Kind:        DEAD,
		PlayerID:    playerID,
		OtherPlayer: cause,
	}
}

func FireBullet(playerID uint64, pos pixel.Vec, angle float64) Update {
	return Update{
		Kind:     FIRE,
		PlayerID: playerID,
		Pos:      pos,
		Angle:    angle,
	}
}

func (u Update) UpdateAngle(mousePos pixel.Vec) Update {
	u.Angle = mousePos.Sub(u.Pos).Angle()

	return u
}

func (u Update) MoveLeft(dt float64) Update {
	u.Pos.X -= PlayerSpeed * dt

	return u
}

func (u Update) MoveRight(dt float64) Update {
	u.Pos.X += PlayerSpeed * dt

	return u
}

func (u Update) MoveDown(dt float64) Update {
	u.Pos.Y -= PlayerSpeed * dt

	return u
}

func (u Update) MoveUp(dt float64) Update {
	u.Pos.Y += PlayerSpeed * dt

	return u
}

func (u Update) Bound(bounds pixel.Rect) Update {
	u.Pos.X = pixel.Clamp(u.Pos.X, bounds.Min.X, bounds.Max.X)
	u.Pos.Y = pixel.Clamp(u.Pos.Y, bounds.Min.Y, bounds.Max.Y)

	return u
}

func (u Update) Timestamp(time time.Time) Update {
	u.Time = time
	// Set a nonce now
	u.Nonce = rand.Uint64()

	return u
}
