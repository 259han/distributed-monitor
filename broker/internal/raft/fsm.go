package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/han-fei/monitor/broker/internal/storage"
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
	Type  CommandType            `json:"type"`
	Key   string                 `json:"key"`
	Value []byte                 `json:"value,omitempty"`
	TTL   time.Duration          `json:"ttl,omitempty"`
	Meta  map[string]interface{} `json:"meta,omitempty"`
}

// FSM 有限状态机
type FSM struct {
	data      map[string][]byte
	mu        sync.RWMutex
	lastIndex uint64
	lastTerm  uint64
	storage   *storage.RedisStorage
}

// NewFSM 创建新的FSM
func NewFSM(storage *storage.RedisStorage) *FSM {
	return &FSM{
		data:    make(map[string][]byte),
		storage: storage,
	}
}

// Apply 应用日志条目
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 更新索引和任期
	f.lastIndex = logEntry.Index
	f.lastTerm = logEntry.Term

	// 优先检查是否为通用字符串命令格式（如 {"type":"batch_metrics", ...}）
	var genericProbe map[string]interface{}
	if err := json.Unmarshal(logEntry.Data, &genericProbe); err == nil {
		if t, ok := genericProbe["type"].(string); ok && t != "" {
			var cmd Command // 创建空的cmd结构体
			return f.applyGenericCommand(cmd, logEntry.Data)
		}
	}

	// 尝试解析为标准命令格式
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
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
		return f.applyGenericCommand(cmd, logEntry.Data)
	}
}

// applyGenericCommand 应用通用命令
func (f *FSM) applyGenericCommand(cmd Command, rawData []byte) interface{} {
	log.Printf("applyGenericCommand被调用，数据大小: %d bytes", len(rawData))

	// 解析为map格式
	var genericCmd map[string]interface{}
	if err := json.Unmarshal(rawData, &genericCmd); err != nil {
		log.Printf("解析通用命令失败: %v", err)
		return fmt.Errorf("failed to unmarshal generic command: %v", err)
	}

	log.Printf("通用命令解析成功，命令类型: %v", genericCmd["type"])

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

		// 尝试解析为protobuf MetricsData格式
		if metricsData, ok := item.(map[string]interface{}); ok {
			// 生成键（兼容 hostId 与 host_id 两种字段）
			hostID, _ := metricsData["hostId"].(string)
			if hostID == "" {
				hostID, _ = metricsData["host_id"].(string)
			}
			tsf, _ := metricsData["timestamp"].(float64)
			key := fmt.Sprintf("monitor:metrics:%s:%d", hostID, int64(tsf))

			// 存储数据到内存
			data, _ := json.Marshal(metricsData)
			f.data[key] = data

			// 同时将数据存储到Redis
			ctx := context.Background()

			// 将数据转换为MetricsData结构
			storageData := &models.MetricsData{
				HostID:    hostID,
				Timestamp: int64(tsf),
				Metrics:   make(map[string]interface{}),
				Tags:      make(map[string]string),
			}

			// 解析指标 - 处理protobuf格式的metrics数组（保留单位）
			if metricsArray, ok := metricsData["metrics"].([]interface{}); ok {
				for _, metricInterface := range metricsArray {
					if metric, ok := metricInterface.(map[string]interface{}); ok {
						name, _ := metric["name"].(string)
						if name == "" {
							name, _ = metric["Name"].(string)
						}
						// 读取值与单位
						var val interface{}
						if vStr, ok := metric["value"].(string); ok {
							val = vStr
						} else if vNum, ok := metric["value"].(float64); ok {
							val = vNum
						}
						unit, _ := metric["unit"].(string)
						if name != "" && val != nil {
							// 统一以对象形式存储，保留单位
							storageData.Metrics[name] = map[string]interface{}{
								"value": val,
								"unit":  unit,
							}
						}
					}
				}
			}

			// 解析标签 - 处理protobuf格式的tags数组
			if tagsArray, ok := metricsData["tags"].([]interface{}); ok {
				for _, tagInterface := range tagsArray {
					if tag, ok := tagInterface.(map[string]interface{}); ok {
						key, _ := tag["key"].(string)
						value, _ := tag["value"].(string)
						if key != "" && value != "" {
							storageData.Tags[key] = value
						}
					}
				}
			}

			// 使用Redis存储实例保存数据
			if f.storage != nil {
				log.Printf("开始保存数据到Redis，HostID: %s, Timestamp: %d", storageData.HostID, storageData.Timestamp)

				// 使用defer-recover捕获可能的panic
				var storageErr error
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("保存指标数据到Redis时发生panic: %v", r)
							storageErr = fmt.Errorf("panic in SaveMetricsData: %v", r)
						}
					}()

					if err := f.storage.SaveMetricsData(ctx, storageData); err != nil {
						log.Printf("保存指标数据到Redis失败: %v", err)
						storageErr = err
					} else {
						log.Printf("成功保存指标数据到Redis: host=%s ts=%d metrics=%d", storageData.HostID, storageData.Timestamp, len(storageData.Metrics))
					}
				}()

				// 如果Redis存储失败，返回错误
				if storageErr != nil {
					log.Printf("Redis存储失败，返回错误: %v", storageErr)
					return storageErr
				}
			} else {
				log.Printf("警告: Redis存储实例为空")
			}
		} else {
			log.Printf("数据项格式错误，期望map[string]interface{}，实际: %T", item)
		}
	}

	log.Printf("批量指标数据处理完成")
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
