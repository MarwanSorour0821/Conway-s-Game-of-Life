package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type GameofLife struct{}
type SecretKeyPressOperation struct{}
type ResetState struct {
	aliveCells int
	beginTurn  int
	world      [][]byte
}

var quit bool
var initialState ResetState

func calculateLiveNeighbours(world [][]byte, i int, j int) int {

	numOfLiveNeighbours := 0

	//positive modules
	up := ((i-1)%len(world) + len(world)) % len(world)
	down := ((i+1)%len(world) + len(world)) % len(world)
	right := ((j+1)%len(world[i]) + len(world[i])) % len(world[i])
	left := ((j-1)%len(world[i]) + len(world[i])) % len(world[i])

	neighbours := [8]byte{world[up][j], world[down][j], world[i][left], world[i][right], world[up][left], world[up][right], world[down][right], world[down][left]}

	for _, neighbour := range neighbours {
		if neighbour == 255 {
			numOfLiveNeighbours++
		}
	}

	return numOfLiveNeighbours
}

func calculateNextState(world [][]byte, startY, endY, startX, endX int, t int) [][]byte {

	nextWorld := make([][]byte, endY-startY)

	for i := startY; i < endY; i++ {
		nextWorld[i-startY] = make([]byte, endX)
		for j := startX; j < endX; j++ {
			liveNeighbours := calculateLiveNeighbours(world, i, j)

			// Apply the rules
			if world[i][j] == 255 {
				if liveNeighbours < 2 || liveNeighbours > 3 {
					nextWorld[i-startY][j] = 0 // Cell dies

				} else {
					nextWorld[i-startY][j] = 255 // Cell remains alive
				}
			} else {
				if liveNeighbours == 3 {
					nextWorld[i-startY][j] = 255 // Dead cell becomes alive

				} else {
					nextWorld[i-startY][j] = 0 // Cell remains dead
				}
			}
		}
	}
	return nextWorld
}

func calculateAliveCells(world [][]byte) []util.Cell {
	numRows := len(world)
	numColumns := len(world[0])
	cells := make([]util.Cell, 0)
	for row := 0; row < numRows; row++ {
		for col := 0; col < numColumns; col++ {
			cell1 := world[row][col]
			if cell1 == 255 {
				c := util.Cell{X: col, Y: row}
				cells = append(cells, c)
			}
		}
	}
	return cells
}

var world [][]byte
var turn int

/** Super-Secret `reversing a string' method we can't allow clients to see. **/
func (s *GameofLife) EvolveWorld(req stubs.Request, res *stubs.Response) (err error) {
	// calculate next state for all trns in request

	world = req.World
	res.Turn = turn

	for t := 0; t < req.Turn; t++ {

		world = calculateNextState(world, 0, len(world), 0, len(world[0]), t)
		turn++

		if quit {
			break
		}
	}

	fmt.Println(world)
	res.World = world
	res.AliveCells = calculateAliveCells(world)
	initialState = ResetState{
		aliveCells: 0,
		beginTurn:  0,
		world:      nil,
	}
	quit = false
	return
}

func (s *GameofLife) GetAliveCells(req stubs.Request, res *stubs.Response) (err error) {
	res.AliveCells = calculateAliveCells(world)
	res.AliveCells2 = len(res.AliveCells)
	res.Turn = turn

	return
}

func (s *GameofLife) DealWithKeyPresses(req stubs.StateRequest, dres *stubs.KeyPressResponse) (err error) {
	if req.Start == "save" {
		dres.World = world
		dres.CurrentTurn = turn
	} else if req.Start == "quit" {
		dres.World = world
		dres.CurrentTurn = turn
		quit = true
	}

	return
}

func main() {

	pAddr := flag.String("port", ":8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// Create an instance of GameofLife (you might want to initialize it with necessary data)

	// Register GameofLife with the RPC server
	game := new(GameofLife)
	rpc.Register(game)

	// Start RPC server
	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		// Handle error
		panic(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Handle error
			continue
		}
		go rpc.ServeConn(conn)
	}

}
