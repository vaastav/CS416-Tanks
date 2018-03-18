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
	"../clientlib"
)

var (
	windowCfg = pixelgl.WindowConfig{
		Title: "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
	}
)

var (
	ClientID uint64
	UpdateChannel = make(chan clientlib.Update, 100)
)

var (
	playerPic pixel.Picture
	localPlayer   *Player
	players = make(map[uint64]*Player)
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

	ClientID = localPlayer.ID

	// Start the peer worker
	go PeerWorker()

	// Run the main thread
	pixelgl.Run(run)
}

var win *pixelgl.Window

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
		doAcceptUpdates()

		// Draw everything
		doDraw()
	}
}

func doAcceptUpdates() {
	for {
		select {
		case <-UpdateChannel:
			// TODO deal with other players as well
		default:
			// Done if there are no more events waiting
			return
		}
	}
}

func doLocalInput(dt float64) {
	update := localPlayer.Update()

	if win.Pressed(pixelgl.KeyA) {
		update = update.MoveLeft(dt)
	}

	if win.Pressed(pixelgl.KeyD) {
		update = update.MoveRight(dt)
	}

	if win.Pressed(pixelgl.KeyS) {
		update = update.MoveDown(dt)
	}

	if win.Pressed(pixelgl.KeyW) {
		update = update.MoveUp(dt)
	}

	update = update.UpdateAngle(win.MousePosition())

	// Update our local player immediately
	localPlayer.Accept(update)

	// Tell everybody else about it
	RecordUpdates <- update
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