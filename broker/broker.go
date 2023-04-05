package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Pair struct {
	stubs.Request
	*stubs.Response
}

var (
	publishCh    = make([]chan Pair, 16)
	subscriberCh = make([]chan [][]byte, 16)
	clientsCh    = make([]*rpc.Client, 16)
	quitCh       = make([]chan bool, 16)
	done         = make(chan bool)

	numServers int
	paused     = false
	worldSave  [][]byte
	turn       int
	size       int
)
var mu = new(sync.Mutex)

var cond = sync.NewCond(mu)

func publish(request stubs.Request, response *stubs.Response) (err error) {
	splitSize := request.ImageSize / numServers
	diff := request.ImageSize % numServers
	pos := 0
	count := 0
	for n := 0; n < numServers; n++ {
		start := pos
		pos += splitSize - 1
		if diff != 0 {
			pos++
			diff--
		}
		end := pos
		pos++
		pair := Pair{stubs.Request{World: request.World, ImageSize: request.ImageSize, SplitSize: splitSize, StartY: start, EndY: end, Threads: request.Threads}, response}
		publishCh[count%numServers] <- pair
		count++
	}
	return
}

func subscriberLoop(client *rpc.Client, clientId int) {
	for !paused {
		select {
		case pair := <-publishCh[clientId]:
			request := pair.Request
			response := new(stubs.Response)
			err := client.Call(stubs.Work, request, response)
			subscriberCh[clientId] <- response.World

			if err != nil {
				fmt.Println("Error")
				fmt.Println(err)
				fmt.Println("Closing subscriber thread.")
				publishCh[clientId] <- pair
				break
			}
		case <-quitCh[clientId]:
			client.Call(stubs.QuitServer, stubs.QuitCall{}, stubs.StatusReport{})
			client.Close()
			if clientId == numServers {
				done <- true
			}
			os.Exit(0)
			return
		}
	}
}

func subscribe(servers []string) (err error) {
	for id := range servers {

		publishCh[id] = make(chan Pair)
		subscriberCh[id] = make(chan [][]byte)
		quitCh[id] = make(chan bool)
		server := flag.String(servers[id], servers[id], "ssss")
		flag.Parse()

		client, err := rpc.Dial("tcp", *server)
		clientsCh[id] = client
		if err == nil {
			go subscriberLoop(client, id)
		} else {
			fmt.Println("Error subscribing", servers[id])
			fmt.Println(err)
			break
		}

	}
	return

}
func checkLock() bool {
	if paused {
		mu.Lock()
		defer mu.Unlock()
		for paused {
			cond.Wait()
		}

	}
	return true

}

type Broker struct{}

func (b *Broker) CheckStates(req stubs.Request, res *stubs.StateCheck) (err error) {
	if paused {
		if size == req.ImageSize {
			res.SameSize = true
		} else {
			res.SameSize = false
		}
		res.World = worldSave

		res.Turn = turn

	}
	res.Paused = paused
	paused = false
	cond.Broadcast()
	return

}

func (b *Broker) Pause(req stubs.PauseCall, res *stubs.StatusReport) (err error) {
	if req.P {
		turn = req.Turn
		paused = true
		size = req.Dimension

	} else {
		paused = false
		cond.Broadcast()
	}

	return
}

func (b *Broker) Publish(req stubs.Request, res *stubs.Response) (err error) {
	fmt.Println("workin")
	res.InitialWorld = req.World
	size = req.ImageSize

	if checkLock() {
		err = publish(req, res)

		if err != nil {
			fmt.Println(err)
		}
		newWorld := make([][]byte, 0)
		for n := 0; n < numServers; n++ {
			splitWorld := <-subscriberCh[n]
			for _, row := range splitWorld {
				newWorld = append(newWorld, row)
			}
		}
		worldSave = newWorld
		res.World = newWorld

	}
	return err
}

func (b *Broker) Quit(req stubs.QuitCall, res *stubs.StatusReport) (err error) {
	for n := 0; n < numServers; n++ {
		quitCh[n] <- true
	}
	<-done
	os.Exit(0)
	return err
}

func main() {
	servers := []string{"44.204.144.1:8031", "3.237.65.63:8031", "3.232.131.120:8031", "3.237.187.141:8031"}
	subscribe(servers)
	fmt.Println("Done connecting")
	//what
	numServers = len(servers)
	pAddr := flag.String("port", "8031", "port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Broker{})

	listener, _ := net.Listen("tcp", ":"+*pAddr)

	rpc.Accept(listener)
}
