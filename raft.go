package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// The type we are gonna register for the RPC communication
// That means all functions declared here will be available for other servers to call.
type RpcProxy struct {
	server                 *Server
	timeSinceLastHeartbeat time.Time
	term                   uint
	voteFor                uint
	votes                  uint
	mu                     sync.Mutex
}

type Args struct {
	Ok uint
}

type Reply struct {
	Message string
	Term    uint
}

type AppendEntriesArgs struct {
	Term uint
}

func (rpc *RpcProxy) Quit(args *Args, reply *Reply) error {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Printf("Quit called -> %v %v ", args, reply)
	fmt.Printf("%v", rpc.server)
	close(rpc.server.quit)
	return nil
}

func (rpc *RpcProxy) BecomeALeader() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Println("Becoming a leader")
	rpc.server.state = LEADER
	go rpc.SendHeartbeats()
}

func (rpc *RpcProxy) BecomeAFollower() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Println("Becoming a follower")
	rpc.server.state = FOLLOWER
	go rpc.RunElectionTimeout()
}

func (rpc *RpcProxy) AppendEntries(args *AppendEntriesArgs, reply *Reply) error {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Printf("AppendEntries %v \n", args)

	rpc.timeSinceLastHeartbeat = time.Now()
	reply.Term = rpc.term

	return nil
}

func (rpc *RpcProxy) StartElections() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	if rpc.server.state != FOLLOWER {
		return
	}

	fmt.Printf("New Elections started by %d \n", rpc.server.serverId)

	rpc.server.state = CANDIDATE
	rpc.term += 1
	rpc.voteFor = rpc.server.serverId
	rpc.votes = 1
}

func (rpc *RpcProxy) RunElectionTimeout() {
	if rpc.server.state != FOLLOWER {
		return
	}

	timeTicker := time.NewTicker(time.Millisecond * 150)
	for {
		<-timeTicker.C

		rpc.mu.Lock()
		timeSinceLastHeartbeat := rpc.timeSinceLastHeartbeat
		rpc.mu.Unlock()

		elapsedTime := time.Since(timeSinceLastHeartbeat)
		fmt.Printf("elapsed time %v since heartbeat \n", elapsedTime)
		if elapsedTime > time.Millisecond*500 {
			rpc.StartElections()
		}
	}
}

func (rpc *RpcProxy) SendHeartbeats() {
	rpc.mu.Lock()

	if rpc.server.state != LEADER {
		rpc.mu.Unlock()
		return
	}

	currentTerm := rpc.term
	rpc.mu.Unlock()
	timeTicker := time.NewTicker(time.Millisecond * 150)
	defer timeTicker.Stop()

	for {
		<-timeTicker.C

		// Added instability
		rndNumber := rand.Intn(10)
		if rndNumber > 8 {
			time.Sleep(time.Millisecond * 1200)
		}

		//
		for _, peerId := range rpc.server.peerIds {
			args := AppendEntriesArgs{
				Term: currentTerm,
			}
			reply := Reply{}
			client := rpc.server.GetPeerClient(peerId)
			if client == nil {
				continue
			}
			err := client.Call("RaftConsensus.AppendEntries", &args, &reply)
			if err != nil {
				fmt.Printf("could not call AppendEntries on peer %d -> %v\n", peerId, err)
			}

			if reply.Term > rpc.term {
				fmt.Println("Term is outdated")
				rpc.BecomeAFollower()
				return
			}
		}

	}
}
