package topk

import (
	"container/heap"
	"sync"
	"time"
)

// Item 堆项
type Item struct {
	HostID    string  // 主机ID
	Value     float64 // 指标值
	Timestamp int64   // 时间戳
	Index     int     // 在堆中的索引
}

// MinHeap 最小堆实现
type MinHeap []*Item

// Len 返回堆长度
func (h MinHeap) Len() int {
	return len(h)
}

// Less 比较两个元素
func (h MinHeap) Less(i, j int) bool {
	return h[i].Value < h[j].Value
}

// Swap 交换两个元素
func (h MinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}

// Push 添加元素到堆
func (h *MinHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*Item)
	item.Index = n
	*h = append(*h, item)
}

// Pop 从堆中移除元素
func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // 避免内存泄漏
	item.Index = -1 // 标记为已移除
	*h = old[0 : n-1]
	return item
}

// TopK Top-K算法实现
type TopK struct {
	k       int              // 保留的最大元素数量
	minHeap *MinHeap         // 最小堆
	itemMap map[string]*Item // 主机ID到堆项的映射
	mu      sync.RWMutex     // 读写锁
	ttl     time.Duration    // 数据过期时间
}

// NewTopK 创建新的Top-K实例
func NewTopK(k int, ttl time.Duration) *TopK {
	h := &MinHeap{}
	heap.Init(h)
	return &TopK{
		k:       k,
		minHeap: h,
		itemMap: make(map[string]*Item),
		ttl:     ttl,
	}
}

// Add 添加或更新元素
func (t *TopK) Add(hostID string, value float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().Unix()

	// 如果已存在，更新值
	if item, ok := t.itemMap[hostID]; ok {
		item.Value = value
		item.Timestamp = now
		heap.Fix(t.minHeap, item.Index)
		return
	}

	// 创建新项
	item := &Item{
		HostID:    hostID,
		Value:     value,
		Timestamp: now,
	}

	// 如果堆未满，直接添加
	if t.minHeap.Len() < t.k {
		heap.Push(t.minHeap, item)
		t.itemMap[hostID] = item
		return
	}

	// 如果堆已满，且新值大于堆顶，替换堆顶
	if t.minHeap.Len() > 0 && value > (*t.minHeap)[0].Value {
		// 移除旧的堆顶
		oldItem := heap.Pop(t.minHeap).(*Item)
		delete(t.itemMap, oldItem.HostID)

		// 添加新项
		heap.Push(t.minHeap, item)
		t.itemMap[hostID] = item
	}
}

// GetTopK 获取Top-K元素
func (t *TopK) GetTopK() []*Item {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 清理过期数据
	t.cleanExpired()

	// 复制堆数据
	result := make([]*Item, t.minHeap.Len())
	for i := 0; i < t.minHeap.Len(); i++ {
		result[i] = &Item{
			HostID:    (*t.minHeap)[i].HostID,
			Value:     (*t.minHeap)[i].Value,
			Timestamp: (*t.minHeap)[i].Timestamp,
		}
	}

	// 按值降序排序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Value < result[j].Value {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetSize 获取当前元素数量
func (t *TopK) GetSize() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.minHeap.Len()
}

// Reset 重置Top-K
func (t *TopK) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.minHeap = &MinHeap{}
	heap.Init(t.minHeap)
	t.itemMap = make(map[string]*Item)
}

// cleanExpired 清理过期数据
func (t *TopK) cleanExpired() {
	if t.ttl <= 0 {
		return
	}

	now := time.Now().Unix()
	expireTime := now - int64(t.ttl.Seconds())

	// 找出过期的项
	var expiredItems []*Item
	for _, item := range t.itemMap {
		if item.Timestamp < expireTime {
			expiredItems = append(expiredItems, item)
		}
	}

	// 移除过期项
	for _, item := range expiredItems {
		if item.Index >= 0 && item.Index < t.minHeap.Len() {
			heap.Remove(t.minHeap, item.Index)
		}
		delete(t.itemMap, item.HostID)
	}
}
