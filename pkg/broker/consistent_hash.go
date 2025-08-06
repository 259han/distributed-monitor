package hash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// ConsistentHash 一致性哈希实现
type ConsistentHash struct {
	hashFunc HashFunc          // 哈希函数
	replicas int               // 虚拟节点数
	hashRing []uint32          // 哈希环
	nodeMap  map[uint32]string // 哈希值到节点的映射
	nodes    []string          // 节点列表
	mu       sync.RWMutex      // 读写锁
}

// HashFunc 哈希函数类型
type HashFunc func(data []byte) uint32

// NewConsistentHash 创建一致性哈希
// replicas: 虚拟节点数量
// fn: 自定义哈希函数，如果为nil则使用默认的CRC32
func NewConsistentHash(replicas int, fn HashFunc) *ConsistentHash {
	h := &ConsistentHash{
		replicas: replicas,
		hashFunc: fn,
		hashRing: make([]uint32, 0),
		nodeMap:  make(map[uint32]string),
		nodes:    make([]string, 0),
	}

	if h.hashFunc == nil {
		h.hashFunc = crc32.ChecksumIEEE
	}

	return h
}

// Add 添加节点
func (c *ConsistentHash) Add(nodes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range nodes {
		// 检查节点是否已存在
		exists := false
		for _, n := range c.nodes {
			if n == node {
				exists = true
				break
			}
		}

		if exists {
			continue
		}

		c.nodes = append(c.nodes, node)

		// 为每个节点创建虚拟节点
		for i := 0; i < c.replicas; i++ {
			hash := c.hashFunc([]byte(strconv.Itoa(i) + node))
			c.hashRing = append(c.hashRing, hash)
			c.nodeMap[hash] = node
		}
	}

	// 排序哈希环
	sort.Slice(c.hashRing, func(i, j int) bool {
		return c.hashRing[i] < c.hashRing[j]
	})
}

// Remove 移除节点
func (c *ConsistentHash) Remove(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查节点是否存在
	var idx = -1
	for i, n := range c.nodes {
		if n == node {
			idx = i
			break
		}
	}

	if idx == -1 {
		return
	}

	// 从节点列表中移除
	c.nodes = append(c.nodes[:idx], c.nodes[idx+1:]...)

	// 移除相关的虚拟节点
	var newRing []uint32
	for i := 0; i < c.replicas; i++ {
		hash := c.hashFunc([]byte(strconv.Itoa(i) + node))
		delete(c.nodeMap, hash)
	}

	for _, hash := range c.hashRing {
		if _, ok := c.nodeMap[hash]; ok {
			newRing = append(newRing, hash)
		}
	}

	c.hashRing = newRing
}

// Get 获取负责处理key的节点
func (c *ConsistentHash) Get(key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.hashRing) == 0 {
		return "", fmt.Errorf("hash ring is empty")
	}

	// 计算key的哈希值
	hash := c.hashFunc([]byte(key))

	// 二分查找找到第一个大于等于hash的索引
	idx := sort.Search(len(c.hashRing), func(i int) bool {
		return c.hashRing[i] >= hash
	})

	// 如果没有找到，则使用第一个节点（环形结构）
	if idx == len(c.hashRing) {
		idx = 0
	}

	return c.nodeMap[c.hashRing[idx]], nil
}

// GetN 获取负责处理key的N个节点
func (c *ConsistentHash) GetN(key string, n int) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.hashRing) == 0 {
		return nil, fmt.Errorf("hash ring is empty")
	}

	if len(c.nodes) < n {
		return nil, fmt.Errorf("not enough nodes, have %d, need %d", len(c.nodes), n)
	}

	// 计算key的哈希值
	hash := c.hashFunc([]byte(key))

	// 二分查找找到第一个大于等于hash的索引
	idx := sort.Search(len(c.hashRing), func(i int) bool {
		return c.hashRing[i] >= hash
	})

	// 如果没有找到，则使用第一个节点（环形结构）
	if idx == len(c.hashRing) {
		idx = 0
	}

	result := make([]string, 0, n)
	visited := make(map[string]struct{})

	for len(result) < n {
		node := c.nodeMap[c.hashRing[idx]]
		if _, ok := visited[node]; !ok {
			result = append(result, node)
			visited[node] = struct{}{}
		}

		idx = (idx + 1) % len(c.hashRing)
	}

	return result, nil
}

// GetNodes 获取所有节点
func (c *ConsistentHash) GetNodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]string, len(c.nodes))
	copy(result, c.nodes)

	return result
}

// GetNodeCount 获取节点数量
func (c *ConsistentHash) GetNodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.nodes)
}

// GetVirtualNodeCount 获取虚拟节点数量
func (c *ConsistentHash) GetVirtualNodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.hashRing)
}

// GetReplicas 获取每个节点的虚拟节点数量
func (c *ConsistentHash) GetReplicas() int {
	return c.replicas
}

// GetDistribution 获取节点分布情况
func (c *ConsistentHash) GetDistribution() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]int)
	for _, node := range c.nodes {
		result[node] = 0
	}

	// 计算每个节点负责的哈希环区间数量
	for i := 0; i < len(c.hashRing); i++ {
		node := c.nodeMap[c.hashRing[i]]
		result[node]++
	}

	return result
}

// Reset 重置哈希环
func (c *ConsistentHash) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hashRing = make([]uint32, 0)
	c.nodeMap = make(map[uint32]string)
	c.nodes = make([]string, 0)
}
