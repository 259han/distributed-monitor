package topk

import (
	"container/heap"
	"sync"
	"time"

	"github.com/han-fei/monitor/visualization/internal/cpp"
)

// Item Top-K项
type Item struct {
	HostID    string  `json:"host_id"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Index     int     `json:"index"` // 堆索引
}

// MinHeap 最小堆实现
type MinHeap []*Item

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].Value < h[j].Value }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	item := x.(*Item)
	item.Index = len(*h)
	*h = append(*h, item)
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.Index = -1
	*h = old[0 : n-1]
	return item
}

// TopKProcessor Top-K处理器接口
type TopKProcessor interface {
	Add(hostID string, value float64)
	GetTopK() []Item
	GetCount() int
	Clear()
	SetTTL(seconds int)
	Close()
}

// GoTopKProcessor Go实现的Top-K处理器
type GoTopKProcessor struct {
	mu     sync.Mutex
	items  map[string]*Item
	heap   MinHeap
	k      int
	ttl    time.Duration
	closed bool
}

// NewGoTopKProcessor 创建Go实现的Top-K处理器
func NewGoTopKProcessor(k int, ttlSeconds int) *GoTopKProcessor {
	processor := &GoTopKProcessor{
		items: make(map[string]*Item),
		k:     k,
		ttl:   time.Duration(ttlSeconds) * time.Second,
	}
	heap.Init(&processor.heap)
	return processor
}

// Add 添加或更新元素
func (p *GoTopKProcessor) Add(hostID string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// 检查是否已存在
	if item, exists := p.items[hostID]; exists {
		item.Value = value
		item.Timestamp = time.Now().Unix()
		heap.Fix(&p.heap, item.Index)
		return
	}

	// 创建新项
	item := &Item{
		HostID:    hostID,
		Value:     value,
		Timestamp: time.Now().Unix(),
	}

	// 如果堆未满，直接添加
	if len(p.heap) < p.k {
		p.items[hostID] = item
		heap.Push(&p.heap, item)
		return
	}

	// 如果堆已满，比较最小值
	if value > p.heap[0].Value {
		// 移除最小项
		oldMin := heap.Pop(&p.heap).(*Item)
		delete(p.items, oldMin.HostID)

		// 添加新项
		p.items[hostID] = item
		heap.Push(&p.heap, item)
	}
}

// GetTopK 获取Top-K元素
func (p *GoTopKProcessor) GetTopK() []Item {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	// 清理过期项
	p.cleanupExpired()

	// 复制结果
	result := make([]Item, len(p.heap))
	for i, item := range p.heap {
		result[i] = *item
	}

	// 按值从大到小排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Value < result[j].Value {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetCount 获取当前元素数量
func (p *GoTopKProcessor) GetCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0
	}

	p.cleanupExpired()
	return len(p.items)
}

// Clear 清空所有元素
func (p *GoTopKProcessor) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.items = make(map[string]*Item)
	p.heap = p.heap[:0]
}

// SetTTL 设置TTL
func (p *GoTopKProcessor) SetTTL(seconds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.ttl = time.Duration(seconds) * time.Second
}

// Close 关闭处理器
func (p *GoTopKProcessor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.items = nil
	p.heap = nil
	p.closed = true
}

// cleanupExpired 清理过期项
func (p *GoTopKProcessor) cleanupExpired() {
	now := time.Now().Unix()
	for hostID, item := range p.items {
		if now-item.Timestamp > int64(p.ttl.Seconds()) {
			// 从堆中移除
			heap.Remove(&p.heap, item.Index)
			delete(p.items, hostID)
		}
	}
}

// CppTopKProcessor C++实现的Top-K处理器包装器
type CppTopKProcessor struct {
	processor *cpp.TopKManager
	mu        sync.Mutex
	closed    bool
}

// NewCppTopKProcessor 创建C++实现的Top-K处理器
func NewCppTopKProcessor(k int, ttlSeconds int) *CppTopKProcessor {
	processor := cpp.NewTopKManager(k, ttlSeconds)
	if processor == nil {
		return nil
	}

	return &CppTopKProcessor{
		processor: processor,
	}
}

// Add 添加或更新元素
func (p *CppTopKProcessor) Add(hostID string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	_ = p.processor.Add(hostID, value)
}

// GetTopK 获取Top-K元素
func (p *CppTopKProcessor) GetTopK() []Item {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	cppItems := p.processor.GetTopK()
	if cppItems == nil {
		return nil
	}

	items := make([]Item, len(cppItems))
	for i, cppItem := range cppItems {
		items[i] = Item{
			HostID:    cppItem.HostID,
			Value:     cppItem.Value,
			Timestamp: cppItem.Timestamp,
		}
	}

	return items
}

// GetCount 获取当前元素数量
func (p *CppTopKProcessor) GetCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0
	}

	return p.processor.GetCount()
}

// Clear 清空所有元素
func (p *CppTopKProcessor) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.processor.Clear()
}

// SetTTL 设置TTL
func (p *CppTopKProcessor) SetTTL(seconds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.processor.SetTTL(seconds)
}

// Close 关闭处理器
func (p *CppTopKProcessor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.processor.Close()
	p.closed = true
}

// NewTopKProcessor 创建Top-K处理器（优先使用C++实现）
func NewTopKProcessor(k int, ttlSeconds int, useCpp bool) TopKProcessor {
	if useCpp {
		processor := NewCppTopKProcessor(k, ttlSeconds)
		if processor != nil {
			return processor
		}
		// 如果C++创建失败，回退到Go实现
	}

	return NewGoTopKProcessor(k, ttlSeconds)
}