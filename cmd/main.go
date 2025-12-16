package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/raft"

	"goraft/test/http_srv"
	"goraft/test/node"
)

func main() {
	var (
		mode               string
		httpBindAddr       string
		raftBindAddr       string
		raftAdvertiseAddrs string
		localIdx           int = -1

		localAdvertiseAddr string
		leaderHttpBindAddr string

		rnode *node.RaftNode
		err   error
	)

	flag.StringVar(&mode, "mode", "", "create or join")
	flag.StringVar(&httpBindAddr, "http-bind-addr", "", "http addr of local member")
	flag.StringVar(&raftBindAddr, "raft-bind-addr", "", "bind addr of local member")

	// in create mode: we need to specify
	//   - list of all raft members in format {ID}@{advertise-addr}:{port},{ID}@{advertise-addr}:{port},...
	//   - the index of local member in the above list
	flag.StringVar(&raftAdvertiseAddrs, "raft-advertise-addrs", "r0@127.0.0.1:7890,r1@127.0.0.1:7891,r2@127.0.0.1:7892", "create mode only: advertise addrs of all members in format {ID}@{advertise-addr}:{port},...")
	flag.IntVar(&localIdx, "local-index", -1, "create mode only: the index of local member in advertise addr list")

	// in join mode: we need to specify
	//   - local member {ID}@{advertise-addr}:{port}
	//   - the leader's http-bind-addr (we could poll all members; just tell it for now)
	flag.StringVar(&localAdvertiseAddr, "local-advertise-addr", "", "join mode only, local {ID}@{advertise-addr}:{port}")
	flag.StringVar(&leaderHttpBindAddr, "leader-http-bind-addr", "", "join mode only, leader's http addr")

	flag.Parse()

	if httpBindAddr == "" || raftBindAddr == "" {
		log.Fatalf("invalid argument: {http-bind-addr} or {raft-bind-addr} is empty\n")
	}

	switch mode {
	case "create":
		if raftAdvertiseAddrs == "" || localIdx == -1 {
			log.Fatalf("invalid argument: {raft-advertise-addrs} or {local-index} is empty\n")
		}
		if rnode, err = createRaftCluster(raftAdvertiseAddrs, localIdx, raftBindAddr); err != nil {
			log.Fatalf("failed to create raft cluster. %v\n", err)
		}
	case "join":
		if localAdvertiseAddr == "" || leaderHttpBindAddr == "" {
			log.Fatalf("invalid argument: {local-advertise-addr} or {leader-http-bind-addr} is empty\n")
		}
		if rnode, err = joinRaftCluster(localAdvertiseAddr, raftBindAddr, leaderHttpBindAddr); err != nil {
			log.Fatalf("failed to create raft cluster. %v\n", err)
		}
	default:
		log.Fatalf("invalid mode %s\n", mode)
	}

	log.Printf("\n\n======== Start http service on %s ========\n\n", httpBindAddr)
	http.ListenAndServe(httpBindAddr, http_srv.NewHttpHandler(rnode))
}

func createRaftCluster(raftAdvertiseAddrs string, localIdx int, raftBindAddr string) (*node.RaftNode, error) {
	var (
		err         error
		raftMembers []raft.Server  = nil
		rnode       *node.RaftNode = nil

		leaderAddr string
		leaderID   string
	)

	for _, m := range strings.Split(raftAdvertiseAddrs, ",") {
		raftMembers = append(raftMembers, parseRaftServer(m))
	}

	if rnode, err = node.NewRaftNode(raftMembers, localIdx, raftBindAddr, false); err != nil {
		log.Fatalf("failed to create raft node\n")
	}

	for range 10 {
		leaderAddr, leaderID = rnode.Leader()
		if string(leaderAddr) != "" && string(leaderID) != "" {
			log.Printf("Leader %s %s\n", leaderAddr, leaderID)
			break
		}
		log.Println("wait for election ...")
		time.Sleep(3 * time.Second)
	}

	if string(leaderID) == "" {
		log.Fatalf("election failed.\n")
	}

	log.Printf("Leader %s %s\n", leaderID, leaderAddr)

	if raft.ServerID(leaderID) == raftMembers[localIdx].ID {
		rnode.Set(fmt.Sprintf("key_%s", leaderID), fmt.Sprintf("val_%s", leaderID))
	}

	all := rnode.GetAll()
	log.Printf("All Key-Value Pairs: %v\n", all)
	return rnode, nil
}

func joinRaftCluster(localAdvertiseAddr string, raftBindAddr string, leaderHttpBindAddr string) (*node.RaftNode, error) {
	var (
		err         error
		raftMembers []raft.Server  = nil
		rnode       *node.RaftNode = nil

		leaderAddr string
		leaderID   string
	)

	localServer := parseRaftServer(localAdvertiseAddr)
	raftMembers = append(raftMembers, localServer)

	if rnode, err = node.NewRaftNode(raftMembers, 0, raftBindAddr, true); err != nil {
		log.Fatalf("failed to create raft node\n")
	}

	leaderAddr, leaderID = rnode.Leader()
	log.Printf("Leader: %s %s\n", leaderAddr, leaderID) //should be empty, because has not joined the cluster

	// send http to leader, asking leader to add me
	url := fmt.Sprintf("http://%s/join?id=%s&advertise=%s", leaderHttpBindAddr, localServer.ID, localServer.Address)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to join cluster. %v\n", err)
		return nil, err
	}

	fmt.Printf("%s", resp.Body)

	leaderAddr, leaderID = rnode.Leader()
	log.Printf("Leader: %s %s\n", leaderAddr, leaderID)

	return rnode, nil
}

func parseRaftServer(member string) raft.Server {
	// format: {ID}@{advertise-addr}:{port}
	pair := strings.Split(member, "@")
	if len(pair) != 2 {
		log.Fatalf("invalid member %s: should be {ID}@{advertise-addr}:{port}\n", member)
	}
	return raft.Server{
		Suffrage: 0,
		ID:       raft.ServerID(pair[0]),
		Address:  raft.ServerAddress(pair[1]),
	}
}
