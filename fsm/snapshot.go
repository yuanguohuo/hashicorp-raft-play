package fsm

import (
	"encoding/binary"

	"github.com/hashicorp/raft"
)

var _ raft.FSMSnapshot = (*kvSnapshot)(nil)

type kvSnapshot struct {
	data map[string]string
}

// implement raft.FSMSnapshot
func (s *kvSnapshot) Persist(sink raft.SnapshotSink) error {
	// 写入数据大小
	size := uint64(len(s.data))
	if err := binary.Write(sink, binary.LittleEndian, size); err != nil {
		sink.Cancel()
		return err
	}

	// 写入所有键值对
	for k, v := range s.data {
		keyBytes := []byte(k)
		valBytes := []byte(v)

		// 写入键长度和键
		if err := binary.Write(sink, binary.LittleEndian, uint32(len(keyBytes))); err != nil {
			sink.Cancel()
			return err
		}
		if _, err := sink.Write(keyBytes); err != nil {
			sink.Cancel()
			return err
		}

		// 写入值长度和值
		if err := binary.Write(sink, binary.LittleEndian, uint32(len(valBytes))); err != nil {
			sink.Cancel()
			return err
		}
		if _, err := sink.Write(valBytes); err != nil {
			sink.Cancel()
			return err
		}
	}

	return sink.Close()
}

// implement raft.FSMSnapshot
func (s *kvSnapshot) Release() {
	// 内存快照无需特殊清理
}
