#!/bin/bash

sourcefiles="main.go raft.go test.go"
port=$1
peerIds=$2

go run $sourcefiles -port $port -peerIds $peerIds