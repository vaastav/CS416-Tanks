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
	"math"
)

var (
	windowCfg = pixelgl.WindowConfig{
		Title: "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
	}
	win *pixelgl.Window
	playerPic pixel.Picture
)

var (
	localPlayer *Player
	players []*Player
	updateChannel = make(chan *Update, 100)
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Load the player picture
	var err error
	playerPic, err = loadPicture("images/player.png")
	if err != nil {
		log.Fatal(err)
	}

	// Create the local player
	localPlayer = NewPlayer()
	localPlayer.Pos = windowCfg.Bounds.Center()

	pixelgl.Run(run)
}

func run() {
	var err error
	win, err = pixelgl.NewWindow(windowCfg)
	if err != nil {
		log.Fatal(err)
	}

	win.SetSmooth(true)

	last := time.Now()
	for !win.Closed() {
		dt := time.Since(last).Seconds()
		last = time.Now()

		// Update the local player with local input
		doLocalInput(dt)

		// Accept all waiting events
	outer:
		for {
			select {
			case update := <-updateChannel:
				// TODO deal with other players as well
				localPlayer.Accept(update)
			default:
				// Quit if there are no events waiting
				break outer
			}
		}

		doDraw()
	}
}

func doLocalInput(dt float64) {
	var update *Update

	if win.Pressed(pixelgl.KeyA) {
		update = localPlayer.MoveLeft(dt, win.MousePosition())
	}

	if win.Pressed(pixelgl.KeyD) {
		update = localPlayer.MoveRight(dt, win.MousePosition())
	}

	if win.Pressed(pixelgl.KeyS) {
		update = localPlayer.MoveDown(dt, win.MousePosition())
	}

	if win.Pressed(pixelgl.KeyW) {
		update = localPlayer.MoveUp(dt, win.MousePosition())
	}

	if update == nil {
		update = localPlayer.AngleUpdate(win.MousePosition())
	}

	updateChannel <- update
}

var imd = imdraw.New(nil)

func doDraw() {
	imd.Clear()

	lineLength := win.Bounds().Max.Sub(win.Bounds().Min).Len()
	endPoint := pixel.V(math.Cos(localPlayer.Angle), math.Sin(localPlayer.Angle)).
		Scaled(lineLength).Add(localPlayer.Pos)

	imd.Color = colornames.Darkred
	imd.Push(localPlayer.Pos, endPoint)
	imd.Line(3)

	win.Clear(colornames.Whitesmoke)

	imd.Draw(win)
	localPlayer.Draw(win)

	win.Update()
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