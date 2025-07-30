package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// LogEntry 日志条目
type LogEntry struct {
	Term    uint64      `json:"term"`    // 任期
	Index   uint64      `json:"index"`   // 索引
	Command interface{} `json:"command"` // 命令
	Type    LogType     `json:"type"`    // 类型
}

// LogType 日志类型
type LogType int

const (
	// LogCommand 命令日志
	LogCommand LogType = iota
	// LogConfiguration 配置日志
	LogConfiguration
	// LogNoop 空操作日志
	LogNoop
)

// Log 日志管理
type Log struct {
	entries     []LogEntry   // 日志条目
	mu          sync.RWMutex // 读写锁
	commitIndex uint64       // 已提交索引
	lastApplied uint64       // 已应用索引
	storePath   string       // 存储路径
}

// NewLog 创建新日志
func NewLog(storePath string) *Log {
	log := &Log{
		entries:     make([]LogEntry, 0),
		commitIndex: 0,
		lastApplied: 0,
		storePath:   storePath,
	}

	// 确保存储目录存在
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		fmt.Printf("创建日志存储目录失败: %v\n", err)
	}

	// 加载已有日志
	if err := log.load(); err != nil {
		fmt.Printf("加载日志失败: %v\n", err)
	}

	return log
}

// Append 追加日志条目
func (l *Log) Append(entry LogEntry) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置索引
	if len(l.entries) > 0 {
		entry.Index = l.entries[len(l.entries)-1].Index + 1
	} else {
		entry.Index = 1
	}

	l.entries = append(l.entries, entry)

	// 持久化日志
	if err := l.save(); err != nil {
		fmt.Printf("保存日志失败: %v\n", err)
	}

	return entry.Index
}

// AppendEntries 追加多个日志条目
func (l *Log) AppendEntries(prevLogIndex, prevLogTerm uint64, entries []LogEntry) (uint64, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查前一个日志条目是否匹配
	if prevLogIndex > 0 {
		prevEntry := l.getEntry(prevLogIndex)
		if prevEntry == nil || prevEntry.Term != prevLogTerm {
			return 0, false
		}
	}

	// 如果没有新条目，这是一个心跳
	if len(entries) == 0 {
		return l.getLastIndex(), true
	}

	// 追加或覆盖日志条目
	for i, entry := range entries {
		index := prevLogIndex + uint64(i) + 1

		// 如果已有该索引的条目，检查任期
		if index <= l.getLastIndex() {
			existingEntry := l.getEntry(index)

			// 如果任期不同，删除该索引及之后的所有条目
			if existingEntry.Term != entry.Term {
				l.entries = l.entries[:index-1]
				l.entries = append(l.entries, entry)
			}
			// 如果任期相同，不做任何操作
		} else {
			// 追加新条目
			l.entries = append(l.entries, entry)
		}
	}

	// 持久化日志
	if err := l.save(); err != nil {
		fmt.Printf("保存日志失败: %v\n", err)
	}

	return l.getLastIndex(), true
}

// GetEntries 获取指定范围的日志条目
func (l *Log) GetEntries(startIndex, endIndex uint64) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if startIndex > l.getLastIndex() {
		return []LogEntry{}
	}

	if endIndex > l.getLastIndex() {
		endIndex = l.getLastIndex()
	}

	result := make([]LogEntry, 0, endIndex-startIndex+1)
	for i := startIndex; i <= endIndex; i++ {
		entry := l.getEntry(i)
		if entry != nil {
			result = append(result, *entry)
		}
	}

	return result
}

// GetLastIndex 获取最后一个日志条目的索引
func (l *Log) GetLastIndex() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.getLastIndex()
}

// getLastIndex 获取最后一个日志条目的索引（内部使用，不加锁）
func (l *Log) getLastIndex() uint64 {
	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Index
}

// GetLastTerm 获取最后一个日志条目的任期
func (l *Log) GetLastTerm() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.entries) == 0 {
		return 0
	}

	return l.entries[len(l.entries)-1].Term
}

// GetEntry 获取指定索引的日志条目
func (l *Log) GetEntry(index uint64) *LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.getEntry(index)
}

// getEntry 获取指定索引的日志条目（内部使用，不加锁）
func (l *Log) getEntry(index uint64) *LogEntry {
	if index == 0 || index > l.getLastIndex() {
		return nil
	}

	// 由于索引从1开始，而切片从0开始，需要减1
	adjustedIndex := index - 1
	if adjustedIndex >= uint64(len(l.entries)) {
		return nil
	}

	entry := l.entries[adjustedIndex]
	return &entry
}

// SetCommitIndex 设置已提交索引
func (l *Log) SetCommitIndex(index uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index > l.getLastIndex() {
		index = l.getLastIndex()
	}

	l.commitIndex = index
}

// GetCommitIndex 获取已提交索引
func (l *Log) GetCommitIndex() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.commitIndex
}

// SetLastApplied 设置已应用索引
func (l *Log) SetLastApplied(index uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lastApplied = index
}

// GetLastApplied 获取已应用索引
func (l *Log) GetLastApplied() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.lastApplied
}

// save 保存日志到文件
func (l *Log) save() error {
	data, err := json.Marshal(l.entries)
	if err != nil {
		return err
	}

	tempFile := l.storePath + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	// 原子替换文件
	return os.Rename(tempFile, l.storePath)
}

// load 从文件加载日志
func (l *Log) load() error {
	// 如果文件不存在，不报错
	if _, err := os.Stat(l.storePath); os.IsNotExist(err) {
		return nil
	}

	data, err := ioutil.ReadFile(l.storePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	var entries []LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	l.entries = entries
	return nil
}

// Truncate 截断日志
func (l *Log) Truncate(index uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if index >= l.getLastIndex() {
		return
	}

	l.entries = l.entries[:index]

	// 持久化日志
	if err := l.save(); err != nil {
		fmt.Printf("保存日志失败: %v\n", err)
	}
}

// Snapshot 创建快照
func (l *Log) Snapshot(snapshotIndex uint64, snapshotPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if snapshotIndex > l.getLastIndex() {
		return fmt.Errorf("快照索引 %d 大于最后日志索引 %d", snapshotIndex, l.getLastIndex())
	}

	// 获取要保存的日志条目
	entries := l.entries[:snapshotIndex]

	// 序列化
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	// 创建快照目录
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		return err
	}

	// 写入快照文件
	tempFile := snapshotPath + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	// 原子替换文件
	if err := os.Rename(tempFile, snapshotPath); err != nil {
		return err
	}

	// 截断日志
	l.entries = l.entries[snapshotIndex:]

	// 持久化日志
	return l.save()
}
