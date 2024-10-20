# Raft Algorithm

## Implementation
```
Start every server as a follower
all servers should have an id and a map to all other peers in the network
servers have a state, which could be leader, follower, candidate
tasks divided by state
    Leader:
        Has to emit a heartbeat to every follower on a consistent basis.
        has to handle write requests from the clients.
        If it is too slow to send the heartbeats, and on the heartbeat reply a new has started then
        should change state to follower and adjust.
    follower:
        handles read request from the clients
        If follower doesn't receive the leaders heartbeat it should change to candidate state and start an election.
    Candidate:
        while on the election process it will vote and request votes.
        if it wins the selection it becomes the leader, follower otherwise.
    
    ElectionDetails:

        startElection
            increase current term
            change server state to candidate
            votes for itself
            request other server votes, if votes > n / 2 + 1
```

## Running the servers

```bash
isLeader=1 go run main.go raft.go test.go  -port 6969 -peerIds 6970
go run main.go raft.go test.go client.go -port 6970 -peerIds 6969
```
