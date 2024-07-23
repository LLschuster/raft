# Raft Algorithm

## Implementation

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