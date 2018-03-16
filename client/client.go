package main

import (
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"log"
	"golang.org/x/image/colornames"
	"image"
	"os"
	"math/rand"
	_ "image/png"
	"time"
)

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

var (
	localPlayer *Player
	windowCfg = pixelgl.WindowConfig{
		Title: "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
	}
	playerPic pixel.Picture
	players []*Player
)

const (
	playerSpeed = 150.0
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Load the player picture
	var err error
	playerPic, err = loadPicture("images/player.png")
	if err != nil {
		log.Fatal(err)
	}

	localPlayer = NewPlayer()
	localPlayer.Pos = windowCfg.Bounds.Center()

	pixelgl.Run(run)
}

func run() {
	win, err := pixelgl.NewWindow(windowCfg)
	if err != nil {
		log.Fatal(err)
	}

	win.SetSmooth(true)

	imd := imdraw.New(nil)

	last := time.Now()
	for !win.Closed() {
		dt := time.Since(last).Seconds()
		last = time.Now()

		if win.Pressed(pixelgl.KeyA) {
			localPlayer.Pos.X -= playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyD) {
			localPlayer.Pos.X += playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyS) {
			localPlayer.Pos.Y -= playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyW) {
			localPlayer.Pos.Y += playerSpeed * dt
		}

		mousePosition := win.MousePosition().Sub(localPlayer.Pos)

		imd.Clear()

		lineLength := win.Bounds().Max.Sub(win.Bounds().Min).Len()

		endPoint := mousePosition.Scaled(lineLength / mousePosition.Len()).Add(localPlayer.Pos)

		imd.Color = colornames.Darkred
		imd.Push(localPlayer.Pos, endPoint)
		imd.Line(3)

		win.Clear(colornames.Whitesmoke)
		imd.Draw(win)
		localPlayer.Draw(win)
		win.Update()
	}
}

func loadPicture(path string) (pixel.Picture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return pixel.PictureDataFromImage(img), nil
}