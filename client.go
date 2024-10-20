package main

import (
	"flag"
	"fmt"
	"net/rpc"
)

func client() {
	port := flag.Int("leaderPort", 6969, "example -leaderPort 6969")
	flag.Parse()
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", serverAddress, *port))

	if err != nil {
		panic("Could not dial rpc server ")
	}

	for i := range 1000 {
		sleep(1000)
		args := &StoreKeyArgs{
			Key:   fmt.Sprintf("k-%d", i),
			Value: 5,
		}
		reply := &Reply{}
		err := client.Call("RaftConsensus.StoreKey", args, reply)
		if err != nil {
			fmt.Printf("ERROR calling storeKey %v\n", err)
		}

		fmt.Printf("reply from server: %v\n", reply)
	}
}
