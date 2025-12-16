package fsm

import (
	"github.com/hashicorp/raft"
)

type KVStoreFSM interface {
	raft.FSM

	Get(key string) (string, bool)
	GetAll() map[string]string
}
