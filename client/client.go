package main

import (
	"../clientlib"
	"../clocklib"
	"../crdtlib"
	"../serverlib"
	"github.com/DistributedClocks/GoVector/govec"
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
	"image"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	windowCfg = pixelgl.WindowConfig{
		Title:  "Wednesday",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync:  true,
	}
)

var (
	NetworkSettings clientlib.PeerNetSettings
	LocalAddr       *net.UDPAddr
	RPCAddr         *net.TCPAddr
	UpdateChannel   = make(chan clientlib.Update, 1000)
	Clock           = &clocklib.ClockManager{}
	KVMap           = struct {
		sync.RWMutex
		M map[uint64]crdtlib.ValueType
	}{M: make(map[uint64]crdtlib.ValueType)}
	KVDir    = "stats-directory"
	Server   serverlib.ServerAPI
	Logger   *govec.GoLog
	KVLogger *govec.GoLog
	PeerLogger *govec.GoLog
	IsLogUpdates bool
)

var (
	playerPic   pixel.Picture
	bulletPic   pixel.Picture
	localPlayer *Player
	players     = make(map[uint64]*Player)
	// Keep a separate list of player IDs around because go maps don't have a stable iteration order
	playerIds []uint64
	bullets []*Bullet
	alive = true
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Connect to the server
	serverAddr := os.Args[1]
	localAddrString := os.Args[2]
	display_name := os.Args[3]

	var err error
	LocalAddr, err = net.ResolveUDPAddr("udp", localAddrString)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Println(http.ListenAndServe("localhost:"+strconv.Itoa(LocalAddr.Port+20), nil))
	}()

	address := LocalAddr.IP.String() + ":" + strconv.Itoa(LocalAddr.Port+5)
	RPCAddr, err = net.ResolveTCPAddr("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	go ClockWorker()

	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Setup govector loggers
	clientName := "client_" + display_name
	statsName := clientName + "_stats"
	peersName := clientName + "_peers"
	Logger = govec.InitGoVector(clientName, clientName+"_logfile")
	KVLogger = govec.InitGoVector(statsName, statsName+"_logfile")
	PeerLogger = govec.InitGoVector(peersName, peersName+"_logfile")
	PeerLogger.EnableBufferedWrites()

	v := os.Getenv("LOG_UPDATES")
	IsLogUpdates = true
	if v == "" {
		IsLogUpdates = false
	}

	// KV: Setup the key-value store.
	KVMap.M, err = KVStoreSetup()
	if err != nil {
		log.Fatal(err)
	}

	Server = serverlib.NewRPCServerAPI(client)
	// TODO : Only register if a client ID is not already present
	NetworkSettings, err = Server.Register(localAddrString, address, rand.Uint64(), display_name, Logger)
	if err != nil {
		log.Fatal(err)
	}

	ack, err := Server.Connect(NetworkSettings.UniqueUserID, Logger)
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

	// Load the bullet picture
	bulletPic, err = loadPicture("images/bullet.png")
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
	PeerLogger.Flush()
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

		// Update existing bullets
		doUpdateBullets(dt)

		// Update the local player with local input, if we're alive
		if alive {doLocalInput(dt)}

		// Accept all waiting events
		doAcceptUpdates()

		// Draw everything
		doDraw()
	}
}

func doUpdateBullets(dt float64) {
	for i, bullet := range bullets {
		bullet.Update(dt)

		if !win.Bounds().Contains(bullet.Pos) {
			// kill this bullet
			if i+1 < len(bullets) {
				bullets = append(bullets[:i], bullets[i+1:]...)
			} else {
				bullets = bullets[:i]
			}
		} else if bullet.Pos.Sub(localPlayer.Pos).Len() < PlayerHitBounds {
			// we've been hit!
			alive = false
			RecordUpdates <- clientlib.DeadPlayer(localPlayer.ID)
		}
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
				playerIds = append(playerIds, update.PlayerID)
			}

			// Update the player with what we received
			switch update.Kind {
			case clientlib.DEAD:
				// Remove the player if they're dead
				delete(players, update.PlayerID)
				// make a new list of players
				playerIds = nil
				for id := range players {
					playerIds = append(playerIds, id)
				}
			case clientlib.FIRE:
				// Add a bullet
				bullets = append(bullets, NewBullet(update.Pos, update.Angle))
			default:
				players[update.PlayerID].Accept(update)
			}
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

	update = update.
		UpdateAngle(win.MousePosition()).
		Timestamp(Clock.GetCurrentTime())

	// Update our local player immediately
	localPlayer.Accept(update)

	if win.JustPressed(pixelgl.MouseButtonLeft) {
		// fire a bullet if the mouse button was pressed
		offset := pixel.V(math.Cos(localPlayer.Angle), math.Sin(localPlayer.Angle)).Scaled(30)
		position := localPlayer.Pos.Add(offset)

		// Add the bullet to our list
		bullets = append(bullets, NewBullet(position, localPlayer.Angle))

		// Send an update about this bullet that was fired
		RecordUpdates <- clientlib.FireBullet(localPlayer.ID, position, localPlayer.Angle).Timestamp(Clock.GetCurrentTime())
	}

	// Tell everybody else about it
	RecordUpdates <- update
}

var imd = imdraw.New(nil)

func doDrawLocal() {
	imd.Clear()

	lineLength := win.Bounds().Max.Sub(win.Bounds().Min).Len()
	endPoint := pixel.V(math.Cos(localPlayer.Angle), math.Sin(localPlayer.Angle)).
		Scaled(lineLength).Add(localPlayer.Pos)

	imd.Color = colornames.Darkred
	imd.Push(localPlayer.Pos, endPoint)
	imd.Line(3)

	imd.Draw(win)
	localPlayer.Draw(win)
}

func doDraw() {
	// Clear the screen
	win.Clear(colornames.Whitesmoke)

	// Draw ourselves if we're alive
	if alive {doDrawLocal()}

	// draw all the other players
	for _, id := range playerIds {
		players[id].Draw(win)
	}

	// then draw bullets
	for _, bullet := range bullets {
		bullet.Draw(win)
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
