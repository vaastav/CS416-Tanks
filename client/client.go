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

func run() {
	cfg := pixelgl.WindowConfig{
		Title: "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		log.Fatal(err)
	}

	playerPic, err := loadPicture("images/player.png")
	if err != nil {
		log.Fatal(err)
	}

	player := pixel.NewSprite(playerPic, playerPic.Bounds())
	playerMat := pixel.IM.Scaled(pixel.ZV, 0.5).Moved(win.Bounds().Center())
	playerPos := win.Bounds().Center().Scaled(0.5)

	const playerSpeed = 150.0

	win.SetSmooth(true)

	imd := imdraw.New(nil)

	last := time.Now()
	for !win.Closed() {
		dt := time.Since(last).Seconds()
		last = time.Now()

		if win.Pressed(pixelgl.KeyA) {
			playerPos.X -= playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyD) {
			playerPos.X += playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyS) {
			playerPos.Y -= playerSpeed * dt
		}

		if win.Pressed(pixelgl.KeyW) {
			playerPos.Y += playerSpeed * dt
		}

		mousePosition := win.MousePosition().Sub(playerPos)

		imd.Clear()

		lineLength := win.Bounds().Max.Len()

		endPoint := mousePosition.Scaled(lineLength / mousePosition.Len()).Add(win.Bounds().Center())

		imd.Color = colornames.Darkred
		imd.Push(playerPos, endPoint)
		imd.Line(3)

		angle := mousePosition.Angle()
		playerMat = pixel.IM.Scaled(pixel.ZV, 0.25).Rotated(pixel.ZV, angle).Moved(playerPos)

		win.Clear(colornames.Whitesmoke)
		imd.Draw(win)
		player.Draw(win, playerMat)
		win.Update()
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	pixelgl.Run(run)
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