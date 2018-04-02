package clientlib

import (
	"github.com/faiface/pixel"
	"time"
)

const (
	POSITION UpdateKind = iota
)

const (
	PlayerSpeed = 150.0
)

type UpdateKind int

type Update struct {
	Kind UpdateKind
	Time time.Time
	PlayerID uint64
	Pos pixel.Vec
	Angle float64
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