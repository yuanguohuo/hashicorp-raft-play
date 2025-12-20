package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"

	kvstorefsm "goraft/test/fsm"
)

type RaftNode struct {
	raft *raft.Raft
	conf *raft.Config

	localID            raft.ServerID
	localBindAddr      string
	localAdvertiseAddr string
	localTrans         raft.Transport

	// data managed by raft, storing logStore, stableStore and snapshotStore
	raftPrivDir string
	// data managed by fsm (not used now)
	fsmDataDir string

	logStore      raft.LogStore
	stableStore   raft.StableStore
	snapshotStore raft.SnapshotStore
	localFSM      kvstorefsm.KVStoreFSM
}

func NewRaftNode(members []raft.Server, localIdx int, localBindAddr string, joinMode bool) (*RaftNode, error) {
	var (
		err error

		conf               *raft.Config   = raft.DefaultConfig()
		localServer        *raft.Server   = &members[localIdx]
		localAdvertiseAddr string         = string(localServer.Address)
		localTrans         raft.Transport = nil

		raftPrivDir string = filepath.Join("./data", string(localServer.ID), "raft_private")
		fsmDataDir  string = filepath.Join("./data", string(localServer.ID), "kv_store_fsm") // not used now

		logStorePath      string = filepath.Join(raftPrivDir, "raft_log_store")
		stableStorePath   string = filepath.Join(raftPrivDir, "raft_stable_store")
		snapshotStorePath string = filepath.Join(raftPrivDir, "fsm_snap")

		// we could have usd one boltDB as both logStore and stableStore, but for clearness, seprate them!
		logStore      raft.LogStore         = nil
		stableStore   raft.StableStore      = nil
		snapshotStore raft.SnapshotStore    = nil
		localFSM      kvstorefsm.KVStoreFSM = nil
	)

	conf.LocalID = localServer.ID
	conf.SnapshotThreshold = 20
	conf.SnapshotInterval = time.Millisecond * 1000

	pair := strings.Split(localAdvertiseAddr, ":")
	port, err := strconv.Atoi(pair[1])
	if err != nil {
		return nil, err
	}
	var advertise net.Addr = &net.TCPAddr{IP: net.ParseIP(pair[0]), Port: port}

	if localTrans, err = raft.NewTCPTransport(localBindAddr, advertise, 32, 5*time.Second, nil); err != nil {
		return nil, err
	}

	if err = os.MkdirAll(raftPrivDir, 0755); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(fsmDataDir, 0755); err != nil {
		return nil, err
	}

	if logStore, err = raftboltdb.NewBoltStore(logStorePath); err != nil {
		return nil, err
	}
	if stableStore, err = raftboltdb.NewBoltStore(stableStorePath); err != nil {
		return nil, err
	}

	if snapshotStore, err = raft.NewFileSnapshotStore(snapshotStorePath, 5, nil); err != nil {
		return nil, err
	}

	if localFSM, err = kvstorefsm.NewStupidFSM(); err != nil {
		return nil, err
	}

	exist, err := raft.HasExistingState(logStore, stableStore, snapshotStore)
	if err != nil {
		log.Printf("failed to check existing state. %v\n", err)
		return nil, err
	}

	// raft.BootstrapCluster() 必须放在 raftInst 创建之前。
	// 对比：raftInst.BootstrapCluster()
	/*
		if !exist {
			log.Printf("%s: Raft has no existing State\n", string(localServer.ID))
			if localIdx == 0 && !joinMode {
				log.Printf("%s: Raft bootstrapping ... \n", string(localServer.ID))
				err = raft.BootstrapCluster(
					conf,
					logStore,
					stableStore,
					snapshotStore,
					localTrans,
					raft.Configuration{Servers: members})
				if err != nil {
					log.Printf("failed to boot strap cluster. %v\n", err)
					return nil, err
				}
			} else {
				log.Printf("%s: Raft is not the first member, skip bootstrapping ... \n", string(localServer.ID))
			}
		} else {
			log.Printf("%s: Raft has existing State\n", string(localServer.ID))
		}
	*/

	raftInst, err := raft.NewRaft(conf, localFSM, logStore, stableStore, snapshotStore, localTrans)
	if err != nil {
		log.Printf("%s: failed to create raft. %s\n", string(localServer.ID), err)
		return nil, err
	}

	r := &RaftNode{
		raft: raftInst,
		conf: conf,

		localID:            localServer.ID,
		localBindAddr:      localBindAddr,
		localAdvertiseAddr: localAdvertiseAddr,
		localTrans:         localTrans,

		raftPrivDir: raftPrivDir,
		fsmDataDir:  fsmDataDir,

		logStore:      logStore,
		stableStore:   stableStore,
		snapshotStore: snapshotStore,
		localFSM:      localFSM,
	}

	if !exist {
		log.Printf("%s: Raft has no existing State\n", string(localServer.ID))
		if localIdx == 0 && !joinMode {
			log.Printf("%s: Raft bootstrapping ... \n", string(localServer.ID))
			if f := raftInst.BootstrapCluster(raft.Configuration{Servers: members}); f.Error() != nil {
				log.Printf("failed to boot strap cluster. %v\n", err)
				return nil, f.Error()
			}
		} else {
			log.Printf("%s: Raft is not the first member, skip bootstrapping ... \n", string(localServer.ID))
		}
	} else {
		log.Printf("%s: Raft has existing State\n", string(localServer.ID))
	}

	return r, nil
}

func (r *RaftNode) Set(key, value string) error {
	if r.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	instruct := kvstorefsm.Instruction{
		Op:    "set",
		Key:   key,
		Value: value,
	}

	data, err := json.Marshal(instruct)
	if err != nil {
		return err
	}

	future := r.raft.Apply(data, 10*time.Second)

	// 必须先调用 future.Eorr() 才能调用它的 future.Index() 和 future.Response()
	if err := future.Error(); err != nil {
		return err
	}

	// 可以获取Apply的返回值
	response := future.Response()
	log.Printf("Set操作响应: %v", response)
	return nil
}

func (r *RaftNode) Delete(key string) error {
	if r.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	instruct := kvstorefsm.Instruction{
		Op:  "delete",
		Key: key,
	}

	data, err := json.Marshal(instruct)
	if err != nil {
		return err
	}

	future := r.raft.Apply(data, 10*time.Second)

	// 必须先调用 future.Eorr() 才能调用它的 future.Index() 和 future.Response()
	if err := future.Error(); err != nil {
		return err
	}

	response := future.Response()
	log.Printf("Delete操作响应: %v", response)
	return nil
}

func (r *RaftNode) Get(key string) (string, bool) {
	return r.localFSM.Get(key)
}

func (r *RaftNode) GetAll() map[string]string {
	return r.localFSM.GetAll()
}

func (r *RaftNode) AddVoter(id string, address string) error {
	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(id) || srv.Address == raft.ServerAddress(address) {
			return errors.New("conflict ServerID or ServerAddress")
		}
	}
	future := r.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(address), 0, 10*time.Second)
	return future.Error()
}

func (r *RaftNode) ListMembers() (map[string]string, error) {
	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("failed to get raft configuration: %v", err)
		return nil, err
	}

	var ret = make(map[string]string)
	for _, srv := range configFuture.Configuration().Servers {
		ret[string(srv.ID)] = string(srv.Address)
	}

	return ret, nil
}

func (r *RaftNode) DelVoter(id string) error {
	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("failed to get raft configuration: %v", err)
		return err
	}

	var del raft.Server
	var found bool = false
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(id) {
			del = srv
			found = true
			break
		}
	}

	if !found {
		return errors.New("no node with given id was found")
	}

	future := r.raft.RemoveServer(del.ID, 0, 10*time.Second)
	return future.Error()
}

func (r *RaftNode) IsLeader() bool {
	return r.raft != nil && r.raft.State() == raft.Leader
}

func (r *RaftNode) Leader() (string, string) {
	leadAddr, leaderID := r.raft.LeaderWithID()
	return string(leadAddr), string(leaderID)
}

func (r *RaftNode) State() raft.RaftState {
	return r.raft.State()
}
