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
	"../clocklib"
	"../serverlib"
	"net/rpc"
	"net"

	_ "net/http/pprof"
	"net/http"
	"strconv"
)

var (
	windowCfg = pixelgl.WindowConfig{
		Title: "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
	}
)

var (
	NetworkSettings clientlib.PeerNetSettings
	LocalAddr *net.UDPAddr
	RPCAddr *net.TCPAddr
	UpdateChannel = make(chan clientlib.Update, 1000)
	Clock *clocklib.ClockManager = &clocklib.ClockManager{0}
)

var (
	playerPic pixel.Picture
	localPlayer   *Player
	server serverlib.ServerAPI
	players = make(map[uint64]*Player)
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Connect to the server
	serverAddr := os.Args[1]
	localAddrString := os.Args[2]

	var err error
	LocalAddr, err = net.ResolveUDPAddr("udp", localAddrString)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:" + strconv.Itoa(LocalAddr.Port + 20), nil))
	}()

	address := LocalAddr.IP.String() + ":" + strconv.Itoa(LocalAddr.Port + 5)
	RPCAddr, err = net.ResolveTCPAddr("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	go ClockWorker()

	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	server = serverlib.NewRPCServerAPI(client)
	NetworkSettings, err = server.Register(localAddrString, address, rand.Uint64(), "Wednesday")
	if err != nil {
		log.Fatal(err)
	}

	ack, err := server.Connect(NetworkSettings.UniqueUserID)
	if err != nil {
		log.Fatal(err)
	}
	if !ack {
		log.Fatal("Failed to connect to server")
	}

	// Load the player picture
	playerPic, err = loadPicture("images/player.png")
	if err != nil {
		log.Fatal(err)
	}

	// Create the local player
	localPlayer = NewPlayer(NetworkSettings.UniqueUserID)
	localPlayer.Pos = windowCfg.Bounds.Center()

	// Start workers
	go PeerWorker()
	go RecordWorker()
	go OutgoingWorker()
	go ListenerWorker()

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
		case update := <-UpdateChannel:
			if update.PlayerID == localPlayer.ID {
				// We already know about ourselves
				continue
			}

			if players[update.PlayerID] == nil {
				// New player, create it
				players[update.PlayerID] = NewPlayer(update.PlayerID)
			}

			// Update the player with what we received
			players[update.PlayerID].Accept(update)
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

	// draw all the other players
	for _, p := range players {
		p.Draw(win)
	}

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