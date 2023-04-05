package gol

import (
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type DistributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type countChannels struct {
	worldState chan [][]byte
	turnChan   chan int
	tickerChan chan *time.Ticker
	doneChan   chan bool
	pause      chan bool
	newCount   chan int
}
type managerChannels struct {
	worldState chan [][]byte
	turnChan   chan int
	pause      chan bool
}

func pause(p bool, client *rpc.Client) {
	response := new(stubs.StatusReport)
	err := client.Call(stubs.Pause, stubs.PauseCall{P: p}, response)
	if err != nil {
		fmt.Println(err)
	}

}
func Call(t int, m managerChannels, p Params, d DistributorChannels, c countChannels, world [][]byte, client *rpc.Client) [][]byte {
	request := stubs.Request{World: world, ImageSize: p.ImageHeight, Threads: p.Threads}
	response := new(stubs.Response)
	for turn := t; turn < p.Turns; turn++ {
		err := client.Call(stubs.Publish, request, response)
		if err != nil {
			fmt.Println(err)
		}
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if response.InitialWorld[y][x] != response.World[y][x] {
					d.events <- CellFlipped{turn, util.Cell{X: x, Y: y}}
				}
			}
		}
		request = stubs.Request{World: response.World, ImageSize: p.ImageHeight, Threads: p.Threads}
		c.worldState <- response.World
		c.turnChan <- turn + 1
		m.worldState <- response.World
		m.turnChan <- turn + 1
	}
	return response.World
}

func makeCall(m managerChannels, p Params, d DistributorChannels, c countChannels, world [][]byte, client *rpc.Client) [][]byte {
	r := stubs.Request{ImageSize: p.ImageHeight}
	re := new(stubs.StateCheck)

	if p.Turns == 0 {
		return world
	}
	client.Call(stubs.CheckStates, r, re)
	pause(false, client)

	if re.Paused && re.SameSize {
		fmt.Println("Starting from", re.Turn)
		//c.newCount <- len(calculateAliveCells(re.World))
		newWorld := Call(re.Turn, m, p, d, c, re.World, client)
		return newWorld

	} else {
		newWorld := Call(0, m, p, d, c, world, client)
		return newWorld

	}

}

func generatePGM(p Params, d DistributorChannels, world [][]byte, turn int) {
	filenameOutput := fmt.Sprintf("%dx%dx%dcurrent", p.ImageWidth, p.ImageHeight, p.Turns)
	d.ioCommand <- ioOutput
	d.ioFilename <- filenameOutput
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			d.ioOutput <- world[y][x]
		}
	}
	d.events <- ImageOutputComplete{turn, filenameOutput}
}

func manageKeyPresses(m managerChannels, p Params, d DistributorChannels, c countChannels, client *rpc.Client) {
	world := make([][]byte, p.ImageHeight)
	turn := 0
	paused := false
	for {
		select {
		case w := <-m.worldState:
			world = w
		case t := <-m.turnChan:
			turn = t
		case key := <-d.keyPresses:

			switch key {
			case 's':
				generatePGM(p, d, world, turn)
			case 'k':
				generatePGM(p, d, world, turn)
				d.events <- StateChange{turn, Quitting}
				err := client.Call(stubs.Quit, stubs.QuitCall{}, stubs.StatusReport{})
				if err != nil {
					//fmt.Println(err)
				}
				client.Close()
				d.events <- FinalTurnComplete{0, make([]util.Cell, 0)}
			case 'p':
				paused = !paused
				if paused {
					pause(true, client)
					d.events <- StateChange{turn + 1, Paused}

				} else {
					pause(false, client)
					d.events <- StateChange{turn + 1, Executing}
				}
			case 'q':
				response := new(stubs.StatusReport)
				d.events <- StateChange{turn, Quitting}
				err := client.Call(stubs.Pause, stubs.PauseCall{P: true, Dimension: p.ImageHeight, Turn: turn}, response)
				if err != nil {
					fmt.Println(err)
				}
				client.Close() ///need to properly close!!!
				d.events <- FinalTurnComplete{0, make([]util.Cell, 0)}
			}
		}
	}
}

func calculateAliveCells(world [][]byte) []util.Cell {
	y := 0
	var aliveCells []util.Cell
	for _, row := range world {
		for x := range row {
			if world[y][x] == 255 {
				aliveCell := util.Cell{X: x, Y: y}
				aliveCells = append(aliveCells, aliveCell)
			}
		}
		y += 1
	}
	return aliveCells
}

func countAliveCells(d DistributorChannels, c countChannels, initialWorld [][]byte, client *rpc.Client) {
	t := <-c.tickerChan
	prevTurnAliveCells := 0
	aliveCells := len(calculateAliveCells(initialWorld))
	turn := 0
	done := false
	for !done {
		select {
		case v := <-c.newCount:
			prevTurnAliveCells = v
		case <-t.C:
			d.events <- AliveCellsCount{turn, prevTurnAliveCells}
		case t := <-c.turnChan:
			prevTurnAliveCells = aliveCells
			turn = t
			turnComplete := TurnComplete{turn}
			d.events <- turnComplete
		case world := <-c.worldState:
			aliveCells = len(calculateAliveCells(world))
		case d := <-c.doneChan:
			done = d
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, d DistributorChannels, c countChannels, m managerChannels) {
	fmt.Println("\n running distributor with p", p)
	// TODO: Create a 2D slice to store the world.

	world := make([][]byte, p.ImageHeight)
	for y := range world {
		world[y] = make([]byte, p.ImageWidth)
	}
	turn := 0

	d.ioCommand <- ioInput
	filename := fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	d.ioFilename <- filename

	for y, row := range world {
		for x := range row {
			cell := <-d.ioInput
			world[y][x] = cell
			if cell == 255 {
				d.events <- CellFlipped{turn, util.Cell{X: x, Y: y}}
			}
		}
	}

	brokerAddress := "44.193.6.26:8031"
	client, err := rpc.Dial("tcp", brokerAddress)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer client.Close()

	go manageKeyPresses(m, p, d, c, client)
	rand.Seed(time.Now().UnixNano())

	ticker := time.NewTicker(2000 * time.Millisecond)
	go countAliveCells(d, c, world, client)
	c.tickerChan <- ticker
	world = makeCall(m, p, d, c, world, client)

	// TODO: Report the final state using FinalTurnCompleteEvent.

	aliveCells := calculateAliveCells(world)

	finalTurnComplete := FinalTurnComplete{turn, aliveCells}

	d.events <- finalTurnComplete

	c.doneChan <- true
	close(c.turnChan)
	close(c.tickerChan)
	close(c.doneChan)

	filenameOutput := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, p.Turns)
	d.ioCommand <- ioOutput
	d.ioFilename <- filenameOutput
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			d.ioOutput <- world[y][x]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.ioCommand <- ioCheckIdle
	<-d.ioIdle

	d.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(d.events)
}
