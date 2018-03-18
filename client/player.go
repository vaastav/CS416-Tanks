package main

import (
	"github.com/faiface/pixel"
	"math/rand"
	"../clientlib"
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

func (p *Player) Update() clientlib.Update {
	return clientlib.Update{
		Kind: clientlib.POSITION,
		PlayerID: p.ID,
		Pos: p.Pos,
		Angle: p.Angle,
	}
}

func (p *Player) Accept(update clientlib.Update) {
	switch update.Kind{
	case clientlib.POSITION:
		p.Pos = update.Pos
		p.Angle = update.Angle
	}
}