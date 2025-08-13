package algorithm

import (
	"math"
	"sort"
	"sync"
	"time"
)

// SlidingWindow 滑动窗口实现
type SlidingWindow struct {
	size           int           // 窗口大小
	values         []float64     // 窗口中的值
	timestamps     []time.Time   // 对应的时间戳
	sum            float64       // 当前窗口值的总和
	sumSquared     float64       // 平方和（用于标准差计算）
	mu             sync.RWMutex  // 读写锁
	maxAge         time.Duration // 数据最大存活时间
	adaptiveSize   bool          // 是否自适应调整大小
	minSize        int           // 最小窗口大小
	maxSize        int           // 最大窗口大小
	growthFactor   float64       // 增长因子
	shrinkFactor   float64       // 收缩因子
	totalOperations int64        // 总操作数
	lastResize     time.Time     // 上次调整大小时间
}

// NewSlidingWindow 创建新的滑动窗口
func NewSlidingWindow(size int, maxAge time.Duration) *SlidingWindow {
	return NewAdaptiveSlidingWindow(size, maxAge, false)
}

// NewAdaptiveSlidingWindow 创建自适应滑动窗口
func NewAdaptiveSlidingWindow(size int, maxAge time.Duration, adaptive bool) *SlidingWindow {
	return &SlidingWindow{
		size:           size,
		values:         make([]float64, 0, size),
		timestamps:     make([]time.Time, 0, size),
		maxAge:         maxAge,
		adaptiveSize:   adaptive,
		minSize:        10,
		maxSize:        1000,
		growthFactor:   1.5,
		shrinkFactor:   0.8,
		totalOperations: 0,
		lastResize:     time.Now(),
	}
}

// Add 添加新值到窗口
func (sw *SlidingWindow) Add(value float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	sw.totalOperations++

	// 先清理过期数据
	sw.cleanExpired(now)

	// 如果窗口已满，移除最旧的值
	if len(sw.values) >= sw.size {
		sw.sum -= sw.values[0]
		sw.sumSquared -= sw.values[0] * sw.values[0]
		sw.values = sw.values[1:]
		sw.timestamps = sw.timestamps[1:]
	}

	// 添加新值
	sw.values = append(sw.values, value)
	sw.timestamps = append(sw.timestamps, now)
	sw.sum += value
	sw.sumSquared += value * value

	// 自适应调整窗口大小
	if sw.adaptiveSize {
		sw.adjustSize()
	}
}

// AddBatch 批量添加值
func (sw *SlidingWindow) AddBatch(values []float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	sw.totalOperations += int64(len(values))

	// 先清理过期数据
	sw.cleanExpired(now)

	// 批量添加值
	for _, value := range values {
		// 如果窗口已满，移除最旧的值
		if len(sw.values) >= sw.size {
			sw.sum -= sw.values[0]
			sw.sumSquared -= sw.values[0] * sw.values[0]
			sw.values = sw.values[1:]
			sw.timestamps = sw.timestamps[1:]
		}

		// 添加新值
		sw.values = append(sw.values, value)
		sw.timestamps = append(sw.timestamps, now)
		sw.sum += value
		sw.sumSquared += value * value
	}

	// 自适应调整窗口大小
	if sw.adaptiveSize {
		sw.adjustSize()
	}
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

// StandardDeviation 获取标准差
func (sw *SlidingWindow) StandardDeviation() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	count := len(sw.values)
	if count < 2 {
		return 0
	}

	mean := sw.sum / float64(count)
	variance := (sw.sumSquared / float64(count)) - (mean * mean)
	
	if variance < 0 {
		variance = 0
	}
	
	return math.Sqrt(variance)
}

// Variance 获取方差
func (sw *SlidingWindow) Variance() float64 {
	stdDev := sw.StandardDeviation()
	return stdDev * stdDev
}

// Percentile 获取百分位数
func (sw *SlidingWindow) Percentile(p float64) float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	if len(sw.values) == 0 {
		return 0
	}

	// 复制并排序数据
	sorted := make([]float64, len(sw.values))
	copy(sorted, sw.values)
	sort.Float64s(sorted)

	// 计算百分位数
	index := int(float64(len(sorted)-1) * p / 100)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// Median 获取中位数
func (sw *SlidingWindow) Median() float64 {
	return sw.Percentile(50)
}

// Percentile90 获取90百分位数
func (sw *SlidingWindow) Percentile90() float64 {
	return sw.Percentile(90)
}

// Percentile95 获取95百分位数
func (sw *SlidingWindow) Percentile95() float64 {
	return sw.Percentile(95)
}

// Percentile99 获取99百分位数
func (sw *SlidingWindow) Percentile99() float64 {
	return sw.Percentile(99)
}

// Rate 获取变化率（值/秒）
func (sw *SlidingWindow) Rate() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	if len(sw.values) < 2 {
		return 0
	}

	duration := sw.timestamps[len(sw.timestamps)-1].Sub(sw.timestamps[0])
	if duration <= 0 {
		return 0
	}

	change := sw.values[len(sw.values)-1] - sw.values[0]
	return change / duration.Seconds()
}

// GetStats 获取所有统计信息
func (sw *SlidingWindow) GetStats() map[string]float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	// 先清理过期数据
	sw.cleanExpired(time.Now())

	stats := make(map[string]float64)
	count := len(sw.values)
	
	if count > 0 {
		stats["count"] = float64(count)
		stats["sum"] = sw.sum
		stats["avg"] = sw.sum / float64(count)
		stats["min"] = sw.Min()
		stats["max"] = sw.Max()
		stats["std_dev"] = sw.StandardDeviation()
		stats["variance"] = sw.Variance()
		stats["median"] = sw.Median()
		stats["p90"] = sw.Percentile90()
		stats["p95"] = sw.Percentile95()
		stats["p99"] = sw.Percentile99()
		stats["rate"] = sw.Rate()
		stats["last_value"] = sw.values[len(sw.values)-1]
	} else {
		stats["count"] = 0
		stats["sum"] = 0
		stats["avg"] = 0
		stats["min"] = 0
		stats["max"] = 0
		stats["std_dev"] = 0
		stats["variance"] = 0
		stats["median"] = 0
		stats["p90"] = 0
		stats["p95"] = 0
		stats["p99"] = 0
		stats["rate"] = 0
		stats["last_value"] = 0
	}

	stats["window_size"] = float64(sw.size)
	stats["operations"] = float64(sw.totalOperations)
	
	return stats
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
		sw.sumSquared -= sw.values[i] * sw.values[i]
	}

	// 如果有过期数据，移除它们
	if i > 0 {
		sw.values = sw.values[i:]
		sw.timestamps = sw.timestamps[i:]
	}
}

// adjustSize 自适应调整窗口大小
// 注意：调用此方法前必须已获取锁
func (sw *SlidingWindow) adjustSize() {
	now := time.Now()
	
	// 限制调整频率，避免频繁调整
	if now.Sub(sw.lastResize) < time.Minute {
		return
	}
	
	count := len(sw.values)
	fillRatio := float64(count) / float64(sw.size)
	
	// 根据填充比例调整窗口大小
	if fillRatio > 0.8 && sw.size < sw.maxSize {
		// 增加窗口大小
		newSize := int(float64(sw.size) * sw.growthFactor)
		if newSize > sw.maxSize {
			newSize = sw.maxSize
		}
		if newSize > sw.size {
			sw.size = newSize
			sw.lastResize = now
		}
	} else if fillRatio < 0.3 && sw.size > sw.minSize {
		// 减少窗口大小
		newSize := int(float64(sw.size) * sw.shrinkFactor)
		if newSize < sw.minSize {
			newSize = sw.minSize
		}
		if newSize < sw.size {
			// 需要移除部分数据
			removeCount := sw.size - newSize
			if removeCount > 0 {
				sw.values = sw.values[removeCount:]
				sw.timestamps = sw.timestamps[removeCount:]
			}
			sw.size = newSize
			sw.lastResize = now
		}
	}
}

// SetAdaptive 设置自适应模式
func (sw *SlidingWindow) SetAdaptive(adaptive bool, minSize, maxSize int, growthFactor, shrinkFactor float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	sw.adaptiveSize = adaptive
	if minSize > 0 {
		sw.minSize = minSize
	}
	if maxSize > 0 {
		sw.maxSize = maxSize
	}
	if growthFactor > 0 {
		sw.growthFactor = growthFactor
	}
	if shrinkFactor > 0 {
		sw.shrinkFactor = shrinkFactor
	}
}

// GetWindowInfo 获取窗口信息
func (sw *SlidingWindow) GetWindowInfo() map[string]interface{} {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	
	info := make(map[string]interface{})
	info["size"] = sw.size
	info["count"] = len(sw.values)
	info["max_age"] = sw.maxAge.String()
	info["adaptive"] = sw.adaptiveSize
	info["min_size"] = sw.minSize
	info["max_size"] = sw.maxSize
	info["growth_factor"] = sw.growthFactor
	info["shrink_factor"] = sw.shrinkFactor
	info["total_operations"] = sw.totalOperations
	info["last_resize"] = sw.lastResize.Format(time.RFC3339)
	
	return info
}
