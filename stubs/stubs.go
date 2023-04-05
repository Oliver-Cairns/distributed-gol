package stubs

import "uk.ac.bris.cs/gameoflife/util"

var Quit = "Broker.Quit"
var QuitServer = "GolOP.Quit"
var Work = "GolOP.Work"

var Publish = "Broker.Publish"
var Pause = "Broker.Pause"

var CheckStates = "Broker.CheckStates"
var CheckState = "GolOP.CheckState"

type Response struct {
	World        [][]byte
	Turn         int
	FlipCells    []util.Cell
	InitialWorld [][]byte
}

type Request struct {
	World     [][]byte
	ImageSize int
	SplitSize int
	StartY    int
	EndY      int
	Threads   int
}

type PauseCall struct {
	P         bool
	World     [][]byte
	Turn      int
	Dimension int
}
type StateCheck struct {
	Paused   bool
	SameSize bool
	World    [][]byte
	Turn     int
}

type Subscription struct {
	ServerAddress string
}

type StatusReport struct {
	Message string
}

type QuitCall struct{}
