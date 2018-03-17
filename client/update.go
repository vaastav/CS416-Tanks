package main

import "github.com/faiface/pixel"

// TODO maybe move this into peerclientlib?

const (
	POSITION UpdateKind = iota
)

type UpdateKind int

type Update struct {
	Kind UpdateKind
	PlayerID uint64
	Pos pixel.Vec
	Angle float64
}
