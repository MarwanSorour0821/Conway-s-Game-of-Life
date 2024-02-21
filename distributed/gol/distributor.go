package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	sdlKeyPresses := c.keyPresses
	server := "127.0.0.1:8030"
	flag.Parse()
	client, err := rpc.Dial("tcp", server)

	if err != nil {
		// Handle error
		fmt.Println("Failed to connect to the server:", err)
		return
	}

	defer client.Close()

	nextWorld := make([][]byte, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		nextWorld[i] = make([]byte, p.ImageWidth)
	}

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageHeight, p.ImageWidth)

	getBytesFromInput(nextWorld, c.ioInput)

	turn := p.Turns

	// Prepare the RPC request
	request := stubs.Request{
		World:       nextWorld,
		Turn:        turn,
		ImageHeight: p.ImageHeight,
		ImageWidth:  p.ImageWidth,
		Threads:     p.Threads,
	}

	response := new(stubs.Response)

	// Make the RPC call to the EvolveWorld method
	//non blocking RPC call to allow for the next GetAliveCells RPC Call
	doneEvolve := client.Go("GameofLife.EvolveWorld", request, response, nil)
	if doneEvolve.Error != nil {
		// Handle error
		fmt.Println("RPC call failed:", err)
		return
	}

	//Make the RPC call to the EvolveWorld method
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-doneEvolve.Done:
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				err1 := client.Call("GameofLife.GetAliveCells", request, response)
				if err1 != nil {
					fmt.Println("RPC call number of cells failed:", err1)
					return
				}

				// Retrieve the alive cells count
				aliveCellsCount := len(response.AliveCells)

				// Send the event down the events channel
				c.events <- AliveCellsCount{response.Turn, aliveCellsCount}
			case key := <-sdlKeyPresses:
				if key == 's' {
					revs := KeyRequest("save", client)
					c.ioCommand <- ioOutput
					c.ioFilename <- fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, revs.CurrentTurn)
					sendToOutput(revs.World, c.ioOutput)
					c.ioCommand <- ioCheckIdle
					<-c.ioIdle
					c.events <- ImageOutputComplete{revs.CurrentTurn, fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, revs.CurrentTurn)}
					fmt.Println("current state of the board saved")
				} else if key == 'q' {
					KeyRequest("quit", client)
					return
				}
			}
		}
	}()

	<-doneEvolve.Done

	finishedWorld := response.World
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%dx%dx%d", p.ImageHeight, p.ImageWidth, p.Turns)

	sendToOutput(finishedWorld, c.ioOutput)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{p.Turns, response.AliveCells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func sendToOutput(world [][]byte, outputChannel chan<- byte) {
	for y := range world {
		for x := range world[y] {
			outputChannel <- world[y][x]
		}
	}
}

func getBytesFromInput(world [][]byte, inputChannel <-chan byte) {
	for y := range world {
		for x := range world[y] {
			// Read values from the input channel and initialize the world.
			world[y][x] = <-inputChannel
		}
	}
}

func KeyRequest(event string, client *rpc.Client) *stubs.KeyPressResponse {
	request := stubs.StateRequest{Start: event}
	response := new(stubs.KeyPressResponse)

	err := client.Call(stubs.DealWithKeyPresses, request, response)
	if err != nil {
		fmt.Println("Call to Deal with key presses failed")
	}
	return response
}
