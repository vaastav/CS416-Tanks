package main

import (
	"github.com/faiface/pixel"
	"math"
)

const (
	BulletSpeed = 200.0
)

type Bullet struct {
	sprite   *pixel.Sprite
	PlayerID uint64
	Pos      pixel.Vec
	Angle    float64
}

func NewBullet(playerID uint64, pos pixel.Vec, angle float64) *Bullet {
	return &Bullet{
		PlayerID: playerID,
		Pos:      pos,
		Angle:    angle,
		sprite:   pixel.NewSprite(bulletPic, bulletPic.Bounds()),
	}
}

func (b *Bullet) Draw(t pixel.Target) {
	mat := pixel.IM.Moved(b.Pos)

	b.sprite.Draw(t, mat)
}

func (b *Bullet) Update(dt float64) {
	b.Pos = b.Pos.Add(pixel.V(
		dt*BulletSpeed*math.Cos(b.Angle),
		dt*BulletSpeed*math.Sin(b.Angle),
	))
}
