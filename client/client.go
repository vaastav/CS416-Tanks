package main

import (
	"../clientlib"
	"../clocklib"
	"../crdtlib"
	"../serverlib"
	"flag"
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

const (
	MinX = 0
	MinY = 0
	MaxX = 1024
	MaxY = 668
)

var (
	windowCfg = pixelgl.WindowConfig{
		Title:  "Battle Royale",
		Bounds: pixel.R(MinX, MinY, MaxX, MaxY),
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
	KVDir        = "stats-directory"
	Server       serverlib.ServerAPI
	Logger       *govec.GoLog
	KVLogger     *govec.GoLog
	PeerLogger   *govec.GoLog
	IsLogUpdates bool
)

var (
	playerPic   pixel.Picture
	localPlayer *Player
	players     = make(map[uint64]*Player)
	// Keep a separate list of player IDs around because go maps don't have a stable iteration order
	playerIds []uint64
	isBot     bool
)

func main() {
	rand.Seed(time.Now().UnixNano())

	botFlag := flag.Bool("bot", false, "Runs the bot player")
	flag.Parse()
	isBot = *botFlag

	// Connect to the server
	serverAddr := flag.Arg(0)
	localAddrString := flag.Arg(1)
	displayName := flag.Arg(2)

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
	clientName := "client_" + displayName
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
	NetworkSettings, err = Server.Register(localAddrString, address, rand.Uint64(), displayName, Logger)
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

	// Create the local player
	localPlayer = NewPlayer(NetworkSettings.UniqueUserID)
	localPlayer.Pos = windowCfg.Bounds.Center()

	// Start workers
	go PeerWorker()
	go RecordWorker()
	go OutgoingWorker()
	go ListenerWorker()

	// Run the main thread
	if isBot {
		runBot()
	} else {
		pixelgl.Run(run)
	}
	PeerLogger.Flush() // TODO: this won't work for bots; they run until the process is killed :(
}

var win *pixelgl.Window

func runBot() {
	go GenerateMoves()
	for {
		doAcceptUpdates()
	}
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
	for _, id := range playerIds {
		players[id].Draw(win)
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
