package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	VOTEDGRANTED = "granted"
	VOTEDDENIED  = "denied"
	TERMOUTDATED = "outdated"
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

type RequestVotesArgs struct {
	Term    uint
	voteFor uint
}

func (rpc *RpcProxy) Quit(args *Args, reply *Reply) error {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Printf("Quit called -> %v %v ", args, reply)
	fmt.Printf("%v", rpc.server)
	close(rpc.server.quit)
	return nil
}

func (rpc *RpcProxy) resetVoteState() {
	// requires to have mutex
	rpc.voteFor = 0
	rpc.votes = 0
}

func (rpc *RpcProxy) BecomeALeader() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Println("Becoming a leader")

	rpc.server.state = LEADER
	rpc.resetVoteState()
	go rpc.SendHeartbeats()
}

func (rpc *RpcProxy) BecomeAFollower() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Println("Becoming a follower")
	rpc.server.state = FOLLOWER
	rpc.resetVoteState()
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

func (rpc *RpcProxy) RequestVotes(args *RequestVotesArgs, reply *Reply) error {
	if args.Term < rpc.term {
		reply.Message = fmt.Sprintf("term is %s", TERMOUTDATED)
		return nil
	}
	if rpc.voteFor == 0 || rpc.voteFor == args.voteFor {
		rpc.mu.Lock()
		rpc.term = args.Term
		rpc.voteFor = args.voteFor
		reply.Message = VOTEDGRANTED
		rpc.mu.Unlock()
	} else {
		reply.Message = VOTEDDENIED
	}

	return nil
}

func (rpc *RpcProxy) StartElections() {
	if rpc.server.state != FOLLOWER {
		return
	}

	fmt.Printf("New Elections started by %d \n", rpc.server.serverId)

	rpc.mu.Lock()

	newTerm := rpc.term + 1
	serverId := rpc.server.serverId
	votesRequired := uint(len(rpc.server.peerIds)/2) + 1

	rpc.server.state = CANDIDATE
	rpc.term = newTerm
	rpc.voteFor = rpc.server.serverId
	rpc.votes = 1

	rpc.mu.Unlock()

	for _, peerId := range rpc.server.peerIds {
		go func() {
			client := rpc.server.GetPeerClient(peerId)
			if client == nil {
				return
			}

			args := RequestVotesArgs{
				Term:    newTerm,
				voteFor: serverId,
			}
			reply := Reply{}
			err := client.Call("RaftConsensus.RequestVotes", &args, &reply)
			if err != nil {
				fmt.Printf("ERROR while requesting votes %v\n", err)
				return
			}

			rpc.mu.Lock()
			if rpc.server.state == LEADER {
				rpc.mu.Unlock()
				return
			}
			rpc.mu.Unlock()

			if strings.Contains(reply.Message, VOTEDGRANTED) {
				rpc.mu.Lock()
				rpc.votes += 1
				rpc.mu.Unlock()
			} else if strings.Contains(reply.Message, TERMOUTDATED) {
				rpc.BecomeAFollower()
				return
			}

			if rpc.votes > votesRequired {
				rpc.BecomeALeader()
			}
		}()
	}

}

func (rpc *RpcProxy) RunElectionTimeout() {
	timeTicker := time.NewTicker((time.Millisecond * 250) + (time.Millisecond * time.Duration(rand.Intn(250))))
	for {
		if rpc.server.state == LEADER {
			return
		}
		<-timeTicker.C

		rpc.mu.Lock()
		timeSinceLastHeartbeat := rpc.timeSinceLastHeartbeat
		rpc.mu.Unlock()

		elapsedTime := time.Since(timeSinceLastHeartbeat)
		if elapsedTime > time.Millisecond*500 {
			// fmt.Printf("elapsed time %v since heartbeat \n", elapsedTime)
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
	timeTicker := time.NewTicker(time.Millisecond * 75)
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

			fmt.Printf("Sended heartbeat to %d -> responded with %v current term is %d \n", peerId, reply, currentTerm)
			if reply.Term > currentTerm {
				fmt.Println("Term is outdated")
				rpc.BecomeAFollower()
				return
			}
		}

	}
}
