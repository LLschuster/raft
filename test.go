package main

import (
	"fmt"
	"time"
)

func test() {
	proxyArray := make([]*RpcProxy, 0, 5)
	var ports []uint = []uint{6969, 6970, 6971}
	for i := 0; i < len(ports); i++ {
		port := ports[i]
		peerIds := func() []uint {
			var ids []uint = append(ports[i:], ports[0:i]...)
			return ids
		}()
		proxyArray = append(proxyArray, NewServerAndListen(uint(port), peerIds, int(port)))
	}

	for {
		fmt.Println("List servers state")

		amountOfLeaders := 0
		for _, proxy := range proxyArray {
			proxy.server.Info()
			if proxy.server.state == LEADER {
				amountOfLeaders++
			}
		}

		if amountOfLeaders > 1 {
			fmt.Printf("There are more than 1 leader")
		}

		sleep(5000)
	}
}

func sleep(milliseconds int) {
	time.Sleep(time.Millisecond * time.Duration(milliseconds))
}
