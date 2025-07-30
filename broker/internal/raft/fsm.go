package raft

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// Command 命令类型
type CommandType int

const (
	// CommandSet 设置命令
	CommandSet CommandType = iota
	// CommandDelete 删除命令
	CommandDelete
)

// Command 命令
type Command struct {
	Type  CommandType `json:"type"`
	Key   string      `json:"key"`
	Value []byte      `json:"value,omitempty"`
}

// FSM 有限状态机
type FSM struct {
	data      map[string][]byte
	mu        sync.RWMutex
	lastIndex uint64
	lastTerm  uint64
}

// NewFSM 创建新的FSM
func NewFSM() *FSM {
	return &FSM{
		data: make(map[string][]byte),
	}
}

// Apply 应用日志条目
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 更新索引和任期
	f.lastIndex = log.Index
	f.lastTerm = log.Term

	// 解析命令
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return err
	}

	// 执行命令
	switch cmd.Type {
	case CommandSet:
		f.data[cmd.Key] = cmd.Value
		return nil
	case CommandDelete:
		delete(f.data, cmd.Key)
		return nil
	default:
		return nil
	}
}

// Snapshot 创建快照
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// 复制当前状态
	data := make(map[string][]byte)
	for k, v := range f.data {
		data[k] = v
	}

	return &FSMSnapshot{data: data}, nil
}

// Restore 从快照恢复
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 解码快照数据
	var data map[string][]byte
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return err
	}

	// 恢复状态
	f.data = data
	return nil
}

// Get 获取值
func (f *FSM) Get(key string) ([]byte, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	value, ok := f.data[key]
	return value, ok
}

// GetLastIndex 获取最后应用的日志索引
func (f *FSM) GetLastIndex() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.lastIndex
}

// GetLastTerm 获取最后应用的日志任期
func (f *FSM) GetLastTerm() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.lastTerm
}

// FSMSnapshot FSM快照
type FSMSnapshot struct {
	data map[string][]byte
}

// Persist 持久化快照
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	// 编码数据
	err := json.NewEncoder(sink).Encode(s.data)
	if err != nil {
		sink.Cancel()
		return err
	}

	// 关闭快照
	return sink.Close()
}

// Release 释放快照资源
func (s *FSMSnapshot) Release() {
	// 不需要做任何事情
}
