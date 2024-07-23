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

// The type we are gonna register for the RPC communication
// That means all functions declared here will be available for other servers to call.
type RpcProxy struct {
	server *Server
}

type Args struct {
	ok uint
}

type Reply struct {
	message string
}

func (rpc *RpcProxy) Quit(args *Args, reply *Reply) error {
	close(rpc.server.quit)
	return nil
}

func NewServerAndListen(serverId uint, peerIds []uint, port int) *Server {
	server := new(Server)
	rpcProxy := new(RpcProxy)
	server.peerIds = peerIds
	server.serverId = serverId
	server.rpc = rpc.NewServer()
	server.state = FOLLOWER
	server.quit = make(chan any)
	server.mapPeerClients = make(map[uint]*rpc.Client)

	server.rpc.RegisterName("RaftConsensus", rpcProxy)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("Listener error %v", err)
		os.Exit(1)
	}

	fmt.Printf("Listening on address %s \n", listener.Addr())

	go http.Serve(listener, nil)

	for _, peerId := range peerIds {
		client, err := rpc.DialHTTP("tcp", fmt.Sprintf(":%d", peerId))
		if err != nil {
			fmt.Printf("Cannot dial to server %d -> %v", peerId, err)
		}
		server.mapPeerClients[peerId] = client
	}

	return server
}

func main() {
	port := flag.Int("port", 6969, "example: -port 6969")
	var ids = make([]uint, 0, 10)
	cmdPeerIds := CMDpeerIds{&ids}
	flag.Var(cmdPeerIds, "peerIds", "example: -peerIds 6969,6949,6969")

	flag.Parse()

	fmt.Println("start of Raft program")
	fmt.Printf("cmdPeerIds.ids %v\n", *cmdPeerIds.ids)
	s := NewServerAndListen(1, *cmdPeerIds.ids, *port)

	fmt.Printf("%v", s.peerIds)

	for {
		select {
		case <-s.quit:
			return
		default:
			// TODO
		}
	}
}
