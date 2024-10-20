package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
)

const (
	LEADER = iota
	FOLLOWER
	CANDIDATE
)

const serverAddress = "localhost"

type CMDpeerIds struct {
	ids *[]uint
}

func (cmd CMDpeerIds) String() string {
	return "CMDpeerIds"
}

func (cmd CMDpeerIds) Set(value string) error {
	parts := strings.Split(value, ",")
	var err error
	for _, part := range parts {
		var id uint64
		id, err = strconv.ParseUint(part, 10, 64)
		*cmd.ids = append(*cmd.ids, uint(id))
	}
	if err != nil {
		return errors.New("Invalid format for cmd peer ids")
	}
	return nil
}

type any interface{}

type Server struct {
	serverId       uint
	peerIds        []uint
	mapPeerClients map[uint]*rpc.Client
	rpc            *rpc.Server
	state          uint

	quit chan any
}

func (s *Server) Info() {
	stateString := func() string {
		switch s.state {
		case LEADER:
			return "Leader"
		case FOLLOWER:
			return "Follower"
		case CANDIDATE:
			return "Candidate"
		default:
			return "wtf state"
		}
	}()
	fmt.Printf("serverId: %d has state: %s\n", s.serverId, stateString)
}

func NewServerAndListen(serverId uint, peerIds []uint, port int) *RpcProxy {
	server := new(Server)
	rpcProxy := new(RpcProxy)
	server.peerIds = peerIds
	server.serverId = serverId
	server.rpc = rpc.NewServer()
	server.state = FOLLOWER
	server.quit = make(chan any)
	server.mapPeerClients = make(map[uint]*rpc.Client)

	rpcProxy.server = server
	rpcProxy.logFile = fmt.Sprintf("logfile-%d.txt", serverId)
	server.rpc.RegisterName("RaftConsensus", rpcProxy)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("Listener error %v", err)
		os.Exit(1)
	}

	fmt.Printf("Listening on address %s \n", listener.Addr())

	go http.Serve(listener, server.rpc)

	for _, peerId := range peerIds {
		client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", serverAddress, peerId))
		if err != nil {
			fmt.Printf("Cannot dial to server %d -> %v\n", peerId, err)
			continue
		}
		server.mapPeerClients[peerId] = client
	}

	rpcProxy.BecomeAFollower()

	return rpcProxy
}

func (s *Server) GetPeerClient(peerId uint) *rpc.Client {
	if client, ok := s.mapPeerClients[peerId]; ok {
		return client
	}
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", serverAddress, peerId))
	if err != nil {
		fmt.Printf("Cannot dial to server %d -> %v\n", peerId, err)
		return nil
	}
	s.mapPeerClients[peerId] = client
	return client
}

func main() {
	if os.Getenv("test") == "1" {
		test()
		return
	}

	if os.Getenv("client") == "1" {
		client()
		return
	}

	port := flag.Int("port", 6969, "example: -port 6969")
	var ids = make([]uint, 0, 10)
	cmdPeerIds := CMDpeerIds{&ids}
	flag.Var(cmdPeerIds, "peerIds", "example: -peerIds 6969,6949,6969")

	flag.Parse()

	fmt.Println("start of Raft program")
	fmt.Printf("cmdPeerIds.ids %v\n", *cmdPeerIds.ids)
	rpcProxy := NewServerAndListen(uint(*port), *cmdPeerIds.ids, *port)

	if os.Getenv("isLeader") == "1" {
		rpcProxy.BecomeALeader()
	}

	rpcProxy.server.Info()
	for {
		select {
		case <-rpcProxy.server.quit:
			return
		default:
			// TODO
		}
	}
}
