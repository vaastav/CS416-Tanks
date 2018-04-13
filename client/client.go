package main

import (
	"../clientlib"
	"../clocklib"
	"../crdtlib"
	"../serverlib"
	"bitbucket.org/bestchai/dinv/dinvRT"
	"flag"
	"fmt"
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
	"runtime/pprof"
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
	NetworkSettings        clientlib.PeerNetSettings
	MinimumPeerConnections int
	LocalAddr              *net.UDPAddr
	RPCAddr                *net.TCPAddr
	UpdateChannel          = make(chan clientlib.Update, 1000)
	Clock                  = &clocklib.ClockManager{}
	KVMap                  = struct {
		sync.RWMutex
		M map[uint64]crdtlib.ValueType
	}{M: make(map[uint64]crdtlib.ValueType)}
	KVDir        = "stats-directory"
	Server       serverlib.ServerAPI
	Logger       *govec.GoLog
	KVLogger     *govec.GoLog
	PeerLogger   *govec.GoLog
	IsLogUpdates bool
	UseDinv      bool
)

var (
	playerPic   pixel.Picture
	bulletPic   pixel.Picture
	localPlayer *Player
	players     = make(map[uint64]*Player)
	// Keep a separate list of player IDs around because go maps don't have a stable iteration order
	playerIds []uint64
	bullets   []*Bullet
	alive     = true
	isBot     bool
)

func main() {
	rand.Seed(time.Now().UnixNano())

	botFlag := flag.Bool("bot", false, "Runs the bot player")
	cpuprofile := flag.String("cpuprofile", "", "write a cpu profile")
	flag.Parse()
	isBot = *botFlag

	// start profiling
	if *cpuprofile != "" {
		log.Println("Starting cpu profile")
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Connect to the server
	serverAddr := flag.Arg(0)
	localAddrString := flag.Arg(1)
	displayName := flag.Arg(2)

  // Update the stats directory name.
  KVDir = displayName + "-stats-directory"
  fmt.Println("KVDir: " + KVDir)

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

	v2 := os.Getenv("USE_DINV")
	UseDinv = true
	if v2 == "" {
		log.Println("Not using Dinv")
		UseDinv = false
	}

	// Needed to make dinv work on windows as this is a function I have added
	// Comment after generating relevant logs
	if UseDinv {
		//dinvRT.DoFast()
	}

	// KV: Setup the key-value store.
	KVMap.M, err = KVStoreSetup()
	if err != nil {
		log.Fatal(err)
	}

	Server = serverlib.NewRPCServerAPI(client)
	// TODO : Only register if a client ID is not already present
	ID, err := findIDFile(displayName)
	if err != nil {
		NetworkSettings, err = Server.Register(displayName, rand.Uint64(), Logger, UseDinv)
		if err != nil {
			log.Fatal(err)
		}
		ID = NetworkSettings.UniqueUserID

		f, err := os.Create("./" + displayName + ".ID")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		_, err = f.WriteString(fmt.Sprintf("%d\n", ID))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		NetworkSettings.UniqueUserID = ID
	}

	log.Print("ID is")
	log.Println(ID)
	if UseDinv {
		dinvRT.Track(clientName, "display_name", displayName)
	}

	ready := make(chan error)

	// Start the clock worker now
	go ClockWorker(serverAddr, ready)

	// Await the clock worker starting
	_ = <-ready

	MinimumPeerConnections, err = Server.Connect(localAddrString, address, ID, displayName, Logger, UseDinv)
	if err != nil {
		log.Fatal(err)
	}
	if MinimumPeerConnections == 0 {
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
	if isBot {
		go FlushLogs()
		runBot()
	} else {
		pixelgl.Run(run)
	}
	ack, err := Server.Disconnect(ID, Logger, UseDinv)
	if !ack {
		fmt.Println("Failed to disconnect from server")
	}
	PeerLogger.Flush()
}

var win *pixelgl.Window

func runBot() {
	go GenerateMoves()
	for {
		// Since this doesn't update bullets, you can't kill the bot!
		doAcceptUpdates()
	}
}

func FlushLogs() {
	for {
		time.Sleep(time.Second * 30)
		PeerLogger.Flush()
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

		// Update existing bullets
		doUpdateBullets(dt)

		// Update the local player with local input, if we're alive
		if alive {
			doLocalInput(dt)
		}

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
		} else if bullet.Pos.Sub(localPlayer.Pos).Len() < PlayerHitBounds && alive {
			// we've been hit!
			alive = false

			go func() {
				// Increment our death count
				// Ignore error in this case
				value, _ := KVGet(localPlayer.ID)

				value.NumDeaths += 1

				err := KVPut(localPlayer.ID, value)
				if err != nil {
					log.Fatal(err)
				}
			}()

			RecordUpdates <- clientlib.DeadPlayer(localPlayer.ID, bullet.PlayerID).Timestamp(Clock.GetCurrentTime())
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

				if update.OtherPlayer == localPlayer.ID {
					go func() {
						// Increment our kill count
						// Ignore error in this case
						value, _ := KVGet(localPlayer.ID)

						value.NumKills += 1

						err := KVPut(localPlayer.ID, value)
						if err != nil {
							log.Fatal(err)
						}
					}()
				}
			case clientlib.FIRE:
				// Add a bullet
				bullets = append(bullets, NewBullet(update.PlayerID, update.Pos, update.Angle))
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

	if win.Pressed(pixelgl.KeyEnter) {
		// move to the mouse position when enter is pressed (bad behavior)
		update.Pos = win.MousePosition()
	}

	update = update.
		UpdateAngle(win.MousePosition()).
		Bound(windowCfg.Bounds).
		Timestamp(Clock.GetCurrentTime())

	// Update our local player immediately
	localPlayer.Accept(update)

	if win.JustPressed(pixelgl.MouseButtonLeft) {
		FireBullet()
	}

	// Tell everybody else about it
	RecordUpdates <- update
}

func FireBullet() {
	// fire a bullet if the mouse button was pressed
	offset := pixel.V(math.Cos(localPlayer.Angle), math.Sin(localPlayer.Angle)).Scaled(30)
	position := localPlayer.Pos.Add(offset)

	// Add the bullet to our list
	bullets = append(bullets, NewBullet(localPlayer.ID, position, localPlayer.Angle))

	// Send an update about this bullet that was fired
	RecordUpdates <- clientlib.FireBullet(localPlayer.ID, position, localPlayer.Angle).Timestamp(Clock.GetCurrentTime())
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
	if alive {
		doDrawLocal()
	}

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

func findIDFile(displayName string) (id uint64, err error) {
	filePath := "./" + displayName + ".ID"
	if _, err = os.Stat(filePath); err != nil {
		return 0, err
	}

	f, err := os.Open(filePath)
	_, err = fmt.Fscanf(f, "%d\n", &id)
	return id, nil
}
