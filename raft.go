package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	VOTEDGRANTED              = "granted"
	VOTEDDENIED               = "denied"
	TERMOUTDATED              = "outdated"
	LOG_FILE_ACCESS_FLAGS     = os.O_APPEND | os.O_WRONLY
	LOG_FILE_PERMISSION_FLAGS = os.ModeAppend | os.ModePerm
)

// The type we are gonna register for the RPC communication
// That means all functions declared here will be available for other servers to call.
type RpcProxy struct {
	logEntries             []LogEntry
	logFile                string
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
	Term         uint
	PrevLogIndex uint
	LogEntries   []LogEntry
}

type RequestVotesArgs struct {
	Term    uint
	voteFor uint
}

type StoreKeyArgs struct {
	Key   string
	Value any
}

type LogEntry struct {
	Term  uint
	Index uint
	Data  any
}

func (logEntry *LogEntry) toString() string {
	return fmt.Sprintf("%d %d %v\n", logEntry.Term, logEntry.Index, logEntry.Data)
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

func (rpc *RpcProxy) AppendEntries(args AppendEntriesArgs, reply *Reply) error {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	fmt.Printf("AppendEntries %+v \n", args)

	rpc.timeSinceLastHeartbeat = time.Now()
	reply.Term = rpc.term

	if len(args.LogEntries) <= 0 {
		return nil
	}

	if len(rpc.logEntries) > 0 {
		lastOwnEntry := rpc.logEntries[len(rpc.logEntries)-1]
		if lastOwnEntry.Term > args.LogEntries[len(args.LogEntries)-1].Term {
			reply.Message = "Term is outdated"
			return nil
		}

		if lastOwnEntry.Index > args.PrevLogIndex {
			reply.Message = "The logEntry index is outdated"
			return nil
		} else if lastOwnEntry.Index < args.PrevLogIndex {
			reply.Message = fmt.Sprintf(
				"My log has a hole, i need more entries my index %d, leader index %d\n", lastOwnEntry.Index, args.PrevLogIndex)
			return nil
		}
	}

	reply.Message = "acknowledge"
	rpc.logEntries = append(rpc.logEntries, args.LogEntries...)
	file, err := os.OpenFile(rpc.logFile, LOG_FILE_ACCESS_FLAGS, LOG_FILE_PERMISSION_FLAGS)
	if err != nil {
		return err
	}
	defer file.Close()
	fileWriter := bufio.NewWriter(file)
	finalEntriesString := ""
	for _, entry := range args.LogEntries {
		finalEntriesString += entry.toString()
	}
	_, err = fileWriter.WriteString(finalEntriesString)
	if err != nil {
		return err
	}
	err = fileWriter.Flush()
	if err != nil {
		return err
	}

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

	timeOut := time.NewTimer((150 * time.Millisecond) + (time.Duration(rand.Intn(100)) * time.Millisecond))
	defer timeOut.Stop()
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

	<-timeOut.C
	// the current election timeout without a winner, become a follower to start nextElection
	if rpc.server.state == CANDIDATE {
		rpc.BecomeAFollower()
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
		if elapsedTime > time.Millisecond*750 {
			// fmt.Printf("elapsed time %v since heartbeat \n", elapsedTime)
			rpc.StartElections()
		}
	}
}

// getPreviousLogIndex requires a lock
func (rpc *RpcProxy) getPreviousLogIndex() uint {
	var prevLogIndex uint = 0
	if len(rpc.logEntries) > 0 {
		prevLogIndex = rpc.logEntries[len(rpc.logEntries)-1].Index
	}
	return prevLogIndex
}

func (rpc *RpcProxy) SendHeartbeats() {
	rpc.mu.Lock()

	if rpc.server.state != LEADER {
		rpc.mu.Unlock()
		return
	}

	currentTerm := rpc.term
	rpc.mu.Unlock()
	timeTicker := time.NewTicker(time.Millisecond * 175)
	defer timeTicker.Stop()

	for {
		<-timeTicker.C

		// Added instability
		if false {
			rndNumber := rand.Intn(10)
			if rndNumber > 8 {
				time.Sleep(time.Millisecond * 1200)
			}
		}

		//
		for _, peerId := range rpc.server.peerIds {
			prevLogIndex := rpc.getPreviousLogIndex()
			args := AppendEntriesArgs{
				Term:         currentTerm,
				PrevLogIndex: prevLogIndex,
				LogEntries:   rpc.logEntries,
			}
			reply := Reply{}
			client := rpc.server.GetPeerClient(peerId)
			if client == nil {
				continue
			}
			err := client.Call("RaftConsensus.AppendEntries", args, &reply)
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

func (rpc *RpcProxy) StoreKey(args *StoreKeyArgs, reply *Reply) error {
	if rpc.server.state != LEADER {
		reply.Message = "I am not the leader server"
		return nil
	}

	newLogEntry := LogEntry{}
	// keyvalue := make(map[string]any)
	// keyvalue[args.Key] = args.Value
	keyvalue := fmt.Sprintf("%s->%v", args.Key, args.Value)

	rpc.mu.Lock()
	var prevLogIndex uint = rpc.getPreviousLogIndex()

	newLogEntry.Data = keyvalue
	newLogEntry.Term = rpc.term
	newLogEntry.Index = prevLogIndex + 1
	rpc.logEntries = append(rpc.logEntries, newLogEntry)
	logEntries := rpc.logEntries

	rpc.mu.Unlock()

	var acknowledgeCount struct {
		count uint
		mu    sync.Mutex
	}
	acknowledgeCount.count = 1

	peersCount := len(rpc.server.peerIds)
	acknowledgeGroup := sync.WaitGroup{}
	acknowledgeGroup.Add(peersCount)

	for _, peerId := range rpc.server.peerIds {
		go func() {
			defer acknowledgeGroup.Done()
			client := rpc.server.GetPeerClient(peerId)
			if client == nil {
				return
			}
			args := AppendEntriesArgs{
				Term:         newLogEntry.Term,
				PrevLogIndex: prevLogIndex,
				LogEntries:   logEntries,
			}
			reply := Reply{}
			fmt.Printf("calling store key to %d with args %+v \n", peerId, args)
			err := client.Call("RaftConsensus.AppendEntries", args, &reply)
			if err != nil {
				fmt.Printf("ERROR appending entries %v\n", err)
				return
			}
			if !strings.Contains(reply.Message, "acknowledge") {
				fmt.Printf("ERROR appending entries, no acknowledge %v\n", reply)
				return
			}

			acknowledgeCount.mu.Lock()
			acknowledgeCount.count += 1
			acknowledgeCount.mu.Unlock()
		}()
	}

	acknowledgeGroup.Wait()
	if acknowledgeCount.count < (uint(peersCount)/2)+1 {
		return errors.New("could not process request, must peers are down")
	}

	rpc.mu.Lock()
	rpc.logEntries = append(rpc.logEntries, newLogEntry)

	file, error := os.OpenFile(rpc.logFile, LOG_FILE_ACCESS_FLAGS, LOG_FILE_PERMISSION_FLAGS)
	if error != nil {
		return error
	}
	defer file.Close()

	fileWriter := bufio.NewWriter(file)
	fileWriter.WriteString(newLogEntry.toString())
	rpc.mu.Unlock()

	return nil
}
