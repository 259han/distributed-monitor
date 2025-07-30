package algorithm

import (
	"sync"
	"time"
)

// SlidingWindow 滑动窗口实现
type SlidingWindow struct {
	size       int           // 窗口大小
	values     []float64     // 窗口中的值
	timestamps []time.Time   // 对应的时间戳
	sum        float64       // 当前窗口值的总和
	mu         sync.RWMutex  // 读写锁
	maxAge     time.Duration // 数据最大存活时间
}

// NewSlidingWindow 创建新的滑动窗口
func NewSlidingWindow(size int, maxAge time.Duration) *SlidingWindow {
	return &SlidingWindow{
		size:       size,
		values:     make([]float64, 0, size),
		timestamps: make([]time.Time, 0, size),
		maxAge:     maxAge,
	}
}

// Add 添加新值到窗口
func (sw *SlidingWindow) Add(value float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()

	// 先清理过期数据
	sw.cleanExpired(now)

	// 如果窗口已满，移除最旧的值
	if len(sw.values) >= sw.size {
		sw.sum -= sw.values[0]
		sw.values = sw.values[1:]
		sw.timestamps = sw.timestamps[1:]
	}

	// 添加新值
	sw.values = append(sw.values, value)
	sw.timestamps = append(sw.timestamps, now)
	sw.sum += value
}

// Average 获取窗口平均值
func (sw *SlidingWindow) Average() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	if len(sw.values) == 0 {
		return 0
	}

	return sw.sum / float64(len(sw.values))
}

// Min 获取窗口最小值
func (sw *SlidingWindow) Min() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	if len(sw.values) == 0 {
		return 0
	}

	min := sw.values[0]
	for _, v := range sw.values {
		if v < min {
			min = v
		}
	}

	return min
}

// Max 获取窗口最大值
func (sw *SlidingWindow) Max() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	if len(sw.values) == 0 {
		return 0
	}

	max := sw.values[0]
	for _, v := range sw.values {
		if v > max {
			max = v
		}
	}

	return max
}

// Count 获取窗口中的值数量
func (sw *SlidingWindow) Count() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	return len(sw.values)
}

// Reset 重置窗口
func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.values = make([]float64, 0, sw.size)
	sw.timestamps = make([]time.Time, 0, sw.size)
	sw.sum = 0
}

// Values 获取窗口中的所有值
func (sw *SlidingWindow) Values() []float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	// 复制一份数据返回，避免外部修改
	result := make([]float64, len(sw.values))
	copy(result, sw.values)

	return result
}

// cleanExpired 清理过期数据
// 注意：调用此方法前必须已获取锁
func (sw *SlidingWindow) cleanExpired(now time.Time) {
	if sw.maxAge <= 0 || len(sw.values) == 0 {
		return
	}

	cutoff := now.Add(-sw.maxAge)
	i := 0

	// 找到第一个未过期的数据索引
	for ; i < len(sw.timestamps); i++ {
		if !sw.timestamps[i].Before(cutoff) {
			break
		}
		sw.sum -= sw.values[i]
	}

	// 如果有过期数据，移除它们
	if i > 0 {
		sw.values = sw.values[i:]
		sw.timestamps = sw.timestamps[i:]
	}
}
