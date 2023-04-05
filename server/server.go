package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type GolOP struct{}

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func calculateNextState(imageSize int, start int, end int, world [][]byte, out chan<- [][]byte) {
	splitHeight := end - start + 1
	newWorld := make([][]byte, splitHeight)
	for y := 0; y < splitHeight; y++ {
		newWorld[y] = make([]byte, imageSize)
		for x := 0; x < imageSize; x++ {
			newWorld[y][x] = updateCell(imageSize, imageSize, world, x, y+start)
		}
	}
	out <- newWorld
}

func updateCell(height int, width int, world [][]byte, x int, y int) byte {
	aliveCells := countAliveCellsAdjacent(height, width, world, x, y)
	if world[y][x] == 0 {
		if aliveCells == 3 {
			return 255
		}
		return 0
	}
	if world[y][x] == 255 {
		if aliveCells < 2 {
			return 0
		}
		if aliveCells == 2 || aliveCells == 3 {
			return 255
		}
		if aliveCells > 3 {
			return 0
		}
	}
	return 1
}

func countAliveCellsAdjacent(height int, width int, world [][]byte, x int, y int) int {
	left, right, up, down := x-1, x+1, y-1, y+1
	count := 0
	if x == 0 {
		left = width - 1
	}
	if x == width-1 {
		right = 0
	}
	if y == 0 {
		up = height - 1
	}
	if y == height-1 {
		down = 0
	}
	count += int(world[up][left]) + int(world[up][x]) + int(world[up][right]) +
		int(world[y][left]) + int(world[y][right]) +
		int(world[down][left]) + int(world[down][x]) + int(world[down][right])
	count /= 255
	return count
}

func (s *GolOP) Work(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Println("work")
	workerChannels := make([]chan [][]byte, req.Threads)
	for i := range workerChannels {
		workerChannels[i] = make(chan [][]byte)
	}
	splitSize := req.SplitSize / req.Threads
	diff := req.SplitSize % req.Threads
	pos := req.StartY
	for i := 0; i < req.Threads; i++ {
		channel := workerChannels[i]
		start := pos
		pos += splitSize - 1
		if diff > 0 {
			pos++
			diff--
		}
		end := pos
		pos++
		go calculateNextState(req.ImageSize, start, end, req.World, channel)
	}
	newWorld := make([][]byte, 0)
	for _, w := range workerChannels {
		splitWorld := <-w
		for _, row := range splitWorld {
			newWorld = append(newWorld, row)
		}
	}
	res.World = newWorld
	return
}

func (s *GolOP) Quit(req stubs.QuitCall, res *stubs.StatusReport) (err error) {
	os.Exit(0)
	return err
}

func main() {
	fmt.Println("working")
	pAddr := flag.String("port", "8031", "Port to listen on")

	rpc.Register(&GolOP{})

	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println(err)
	}

	defer listener.Close()
	rpc.Accept(listener)
}
