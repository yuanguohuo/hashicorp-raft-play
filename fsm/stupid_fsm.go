package fsm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/hashicorp/raft"
)

var _ KVStoreFSM = (*StupidFSM)(nil)

type StupidFSM struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewStupidFSM() (*StupidFSM, error) {
	stupid_fsm := &StupidFSM{
		data: make(map[string]string),
	}
	return stupid_fsm, nil
}

// implement KVStoreFSM
func (k *StupidFSM) Get(key string) (string, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	val, ok := k.data[key]
	return val, ok
}

// implement KVStoreFSM
func (k *StupidFSM) GetAll() map[string]string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	result := make(map[string]string)
	for k, v := range k.data {
		result[k] = v
	}
	return result
}

// implement raft.FSM
func (k *StupidFSM) Apply(logEntry *raft.Log) interface{} {
	var instruct Instruction
	if err := json.Unmarshal(logEntry.Data, &instruct); err != nil {
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	switch instruct.Op {
	case "set":
		oldValue, existed := k.data[instruct.Key]
		log.Printf("FSM: apply %s => %s", instruct.Key, instruct.Value)
		k.data[instruct.Key] = instruct.Value
		return map[string]interface{}{
			"success":  true,
			"existed":  existed,
			"oldValue": oldValue,
		}
	case "delete":
		oldValue, existed := k.data[instruct.Key]
		if existed {
			delete(k.data, instruct.Key)
		}
		return map[string]interface{}{
			"success":  true,
			"existed":  existed,
			"oldValue": oldValue,
		}
	default:
		return fmt.Errorf("unknown command op: %s", instruct.Op)
	}
}

// implement raft.FSM
func (k *StupidFSM) Snapshot() (raft.FSMSnapshot, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// 创建数据副本
	data := make(map[string]string)
	for k, v := range k.data {
		data[k] = v
	}

	return &kvSnapshot{data: data}, nil
}

// implement raft.FSM
func (k *StupidFSM) Restore(snapshot io.ReadCloser) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// 清空现有数据
	k.data = make(map[string]string)

	// 读取快照大小
	var size uint64
	if err := binary.Read(snapshot, binary.LittleEndian, &size); err != nil {
		return err
	}

	// 读取所有键值对
	for i := uint64(0); i < size; i++ {
		var keyLen, valLen uint32
		if err := binary.Read(snapshot, binary.LittleEndian, &keyLen); err != nil {
			return err
		}
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(snapshot, key); err != nil {
			return err
		}

		if err := binary.Read(snapshot, binary.LittleEndian, &valLen); err != nil {
			return err
		}
		value := make([]byte, valLen)
		if _, err := io.ReadFull(snapshot, value); err != nil {
			return err
		}

		k.data[string(key)] = string(value)
	}

	return snapshot.Close()
}
