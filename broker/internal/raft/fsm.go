package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// Command 命令类型
type CommandType int

const (
	// CommandSet 设置命令
	CommandSet CommandType = iota
	// CommandDelete 删除命令
	CommandDelete
	// CommandUpdate 更新命令
	CommandUpdate
	// CommandIncrement 增量命令
	CommandIncrement
	// CommandExpire 过期命令
	CommandExpire
)

// Command 命令
type Command struct {
	Type  CommandType `json:"type"`
	Key   string      `json:"key"`
	Value []byte      `json:"value,omitempty"`
	TTL   time.Duration `json:"ttl,omitempty"`
	Meta  map[string]interface{} `json:"meta,omitempty"`
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
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	// 执行命令
	switch cmd.Type {
	case CommandSet:
		f.data[cmd.Key] = cmd.Value
		return nil
	case CommandDelete:
		delete(f.data, cmd.Key)
		return nil
	case CommandUpdate:
		// 更新操作，只有键存在时才更新
		if _, exists := f.data[cmd.Key]; exists {
			f.data[cmd.Key] = cmd.Value
		}
		return nil
	case CommandIncrement:
		// 增量操作，用于计数器
		if val, exists := f.data[cmd.Key]; exists {
			// 尝试解析为数字并增加
			var current int
			if err := json.Unmarshal(val, &current); err == nil {
				var increment int
				if err := json.Unmarshal(cmd.Value, &increment); err == nil {
					current += increment
					updated, _ := json.Marshal(current)
					f.data[cmd.Key] = updated
				}
			}
		}
		return nil
	case CommandExpire:
		// 过期操作，删除键
		delete(f.data, cmd.Key)
		return nil
	default:
		// 尝试解析为通用命令格式
		return f.applyGenericCommand(cmd, log.Data)
	}
}

// applyGenericCommand 应用通用命令
func (f *FSM) applyGenericCommand(cmd Command, rawData []byte) interface{} {
	// 解析为map格式
	var genericCmd map[string]interface{}
	if err := json.Unmarshal(rawData, &genericCmd); err != nil {
		return fmt.Errorf("failed to unmarshal generic command: %v", err)
	}

	// 根据命令类型处理
	switch cmdType := genericCmd["type"].(string); cmdType {
	case "batch_metrics":
		return f.applyBatchMetrics(genericCmd)
	case "host_registration":
		return f.applyHostRegistration(genericCmd)
	case "config_update":
		return f.applyConfigUpdate(genericCmd)
	default:
		return fmt.Errorf("unknown generic command type: %s", cmdType)
	}
}

// applyBatchMetrics 应用批量指标数据
func (f *FSM) applyBatchMetrics(cmd map[string]interface{}) interface{} {
	batch, ok := cmd["batch"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid batch format")
	}

	// 处理批量数据
	for _, item := range batch {
		if metricsData, ok := item.(map[string]interface{}); ok {
			// 生成键
			hostID, _ := metricsData["host_id"].(string)
			timestamp, _ := metricsData["timestamp"].(float64)
			key := fmt.Sprintf("metrics:%s:%d", hostID, int64(timestamp))
			
			// 存储数据
			data, _ := json.Marshal(metricsData)
			f.data[key] = data
		}
	}

	return nil
}

// applyHostRegistration 应用主机注册
func (f *FSM) applyHostRegistration(cmd map[string]interface{}) interface{} {
	hostID, _ := cmd["host_id"].(string)
	hostData, _ := json.Marshal(cmd)
	
	key := fmt.Sprintf("host:%s", hostID)
	f.data[key] = hostData
	
	return nil
}

// applyConfigUpdate 应用配置更新
func (f *FSM) applyConfigUpdate(cmd map[string]interface{}) interface{} {
	configData, _ := json.Marshal(cmd)
	f.data["config:current"] = configData
	return nil
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
