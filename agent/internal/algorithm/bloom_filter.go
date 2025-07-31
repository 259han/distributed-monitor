package algorithm

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/willf/bloom"
)

// BloomFilter 布隆过滤器实现
type BloomFilter struct {
	filter *bloom.BloomFilter
	mu     sync.RWMutex
}

// NewBloomFilter 创建新的布隆过滤器
// size: 位数组大小
// hashFuncs: 哈希函数数量
func NewBloomFilter(size int, hashFuncs int) *BloomFilter {
	return &BloomFilter{
		filter: bloom.New(uint(size), uint(hashFuncs)),
	}
}

// Add 添加元素到过滤器
func (bf *BloomFilter) Add(data []byte) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.Add(data)
}

// AddString 添加字符串到过滤器
func (bf *BloomFilter) AddString(data string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.AddString(data)
}

// Test 检查元素是否可能在过滤器中
func (bf *BloomFilter) Test(data []byte) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.filter.Test(data)
}

// TestString 检查字符串是否可能在过滤器中
func (bf *BloomFilter) TestString(data string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.filter.TestString(data)
}

// TestAndAdd 检查元素是否在过滤器中，如果不在则添加
func (bf *BloomFilter) TestAndAdd(data []byte) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	return bf.filter.TestAndAdd(data)
}

// TestAndAddString 检查字符串是否在过滤器中，如果不在则添加
func (bf *BloomFilter) TestAndAddString(data string) bool {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	return bf.filter.TestAndAddString(data)
}

// Reset 重置过滤器
func (bf *BloomFilter) Reset() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.ClearAll()
}

// EstimateFalsePositiveRate 估计误判率
func (bf *BloomFilter) EstimateFalsePositiveRate(n int) float64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.filter.EstimateFalsePositiveRate(uint(n))
}

// AlertFilter 告警过滤器，用于过滤重复告警
type AlertFilter struct {
	filter    *BloomFilter
	alertKeys map[string]struct{} // 用于记录已添加的告警键
	mu        sync.RWMutex
}

// NewAlertFilter 创建新的告警过滤器
func NewAlertFilter(size int) *AlertFilter {
	return &AlertFilter{
		filter:    NewBloomFilter(size, 3), // 使用3个哈希函数
		alertKeys: make(map[string]struct{}),
	}
}

// AddAlert 添加告警
func (af *AlertFilter) AddAlert(hostID, metricName string, timestamp int64) {
	af.mu.Lock()
	defer af.mu.Unlock()

	// 生成告警键
	key := generateAlertKey(hostID, metricName, timestamp)

	// 添加到布隆过滤器
	af.filter.AddString(key)

	// 同时记录到map中
	af.alertKeys[key] = struct{}{}
}

// HasAlert 检查告警是否存在
func (af *AlertFilter) HasAlert(hostID, metricName string, timestamp int64) bool {
	key := generateAlertKey(hostID, metricName, timestamp)

	// 先检查布隆过滤器
	if !af.filter.TestString(key) {
		return false
	}

	// 如果布隆过滤器返回可能存在，再精确检查map
	af.mu.RLock()
	defer af.mu.RUnlock()
	_, exists := af.alertKeys[key]
	return exists
}

// generateAlertKey 生成告警键
func generateAlertKey(hostID, metricName string, timestamp int64) string {
	h := fnv.New64a()
	h.Write([]byte(hostID))
	h.Write([]byte(metricName))
	h.Write([]byte{
		byte(timestamp >> 56),
		byte(timestamp >> 48),
		byte(timestamp >> 40),
		byte(timestamp >> 32),
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp),
	})

	// 使用时间戳的小时部分，这样可以让相同的告警在一小时后可以再次触发
	hourTimestamp := timestamp / 3600

	return hostID + ":" + metricName + ":" + fmt.Sprintf("%d", hourTimestamp)
}
