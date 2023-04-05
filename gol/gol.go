package gol

import "time"

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}
	go startIo(p, ioChannels)

	distributorChannels := DistributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
		keyPresses: keyPresses,
	}
	managerChannels := managerChannels{
		worldState: make(chan [][]byte),
		turnChan:   make(chan int),
		pause:      make(chan bool),
	}

	countChannels := countChannels{
		worldState: make(chan [][]byte),
		turnChan:   make(chan int),
		tickerChan: make(chan *time.Ticker),
		doneChan:   make(chan bool),
		pause:      make(chan bool),
	}

	distributor(p, distributorChannels, countChannels, managerChannels)
}
