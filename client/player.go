package main

import (
	"github.com/faiface/pixel"
	"math/rand"
)

const (
	playerSpeed = 150.0
)

// TODO add shooting mechanic

type Player struct {
	sprite *pixel.Sprite
	ID uint64
	Pos pixel.Vec
	Angle float64
}

func NewPlayer() *Player {
	return &Player {
		ID: rand.Uint64(),
		sprite: pixel.NewSprite(playerPic, playerPic.Bounds()),
	}
}

func (p *Player) Draw(t pixel.Target) {
	mat := pixel.IM.Scaled(pixel.ZV, 0.25).
		Rotated(pixel.ZV, p.Angle).Moved(p.Pos)

	p.sprite.Draw(t, mat)
}

func (p *Player) MoveLeft(dt float64, mousePos pixel.Vec) *Update {
	return p.PositionUpdate(-playerSpeed*dt, 0, win.MousePosition())
}

func (p *Player) MoveRight(dt float64, mousePos pixel.Vec) *Update {
	return p.PositionUpdate(playerSpeed*dt, 0, win.MousePosition())
}

func (p *Player) MoveDown(dt float64, mousePos pixel.Vec) *Update {
	return p.PositionUpdate(0, -playerSpeed*dt, win.MousePosition())
}

func (p *Player) MoveUp(dt float64, mousePos pixel.Vec) *Update {
	return p.PositionUpdate(0, playerSpeed*dt, win.MousePosition())
}

func (p *Player) AngleUpdate(mousePos pixel.Vec) *Update {
	return p.PositionUpdate(0, 0, mousePos)
}

func (p *Player) PositionUpdate(dx float64, dy float64, mousePos pixel.Vec) *Update {
	pos := p.Pos.Add(pixel.V(dx, dy))

	return &Update{
		Kind: POSITION,
		PlayerID: p.ID,
		Pos: pos,
		Angle: mousePos.Sub(pos).Angle(),
	}
}

func (p *Player) Accept(update *Update) {
	switch update.Kind{
	case POSITION:
		p.Pos = update.Pos
		p.Angle = update.Angle
	}
}
