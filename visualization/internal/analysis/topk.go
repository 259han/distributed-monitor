package analysis

import (
	"container/heap"
	"sort"
	"sync"
	"time"

	"github.com/han-fei/monitor/visualization/internal/models"
)

// TopKAnalyzer TopK分析器
type TopKAnalyzer struct {
	data      map[string][]*models.MetricsData
	dataMutex sync.RWMutex
	topkCache map[string][]*TopKResult
	cacheExp  time.Duration
	cacheTime map[string]time.Time
}

// TopKResult TopK结果
type TopKResult struct {
	HostID     string  `json:"host_id"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
	Rank       int     `json:"rank"`
	Timestamp  time.Time `json:"timestamp"`
}

// NewTopKAnalyzer 创建TopK分析器
func NewTopKAnalyzer() *TopKAnalyzer {
	return &TopKAnalyzer{
		data:      make(map[string][]*models.MetricsData),
		topkCache: make(map[string][]*TopKResult),
		cacheTime: make(map[string]time.Time),
		cacheExp:  5 * time.Minute,
	}
}

// AddData 添加数据
func (t *TopKAnalyzer) AddData(data *models.MetricsData) {
	t.dataMutex.Lock()
	defer t.dataMutex.Unlock()

	hostID := data.HostID
	t.data[hostID] = append(t.data[hostID], data)

	// 保持数据量，防止内存泄漏
	maxDataSize := 1000
	if len(t.data[hostID]) > maxDataSize {
		t.data[hostID] = t.data[hostID][len(t.data[hostID])-maxDataSize:]
	}

	// 清除相关缓存
	for metricName := range t.topkCache {
		for _, metric := range data.Metrics {
			if metric.Name == metricName {
				delete(t.topkCache, metricName)
				delete(t.cacheTime, metricName)
				break
			}
		}
	}
}

// GetTopK 获取TopK结果
func (t *TopKAnalyzer) GetTopK(metricName string, k int) []*TopKResult {
	t.dataMutex.RLock()
	defer t.dataMutex.RUnlock()

	// 检查缓存
	if cache, exists := t.topkCache[metricName]; exists {
		if time.Since(t.cacheTime[metricName]) < t.cacheExp {
			if len(cache) >= k {
				return cache[:k]
			}
			return cache
		}
	}

	// 计算TopK
	results := t.calculateTopK(metricName, k)

	// 更新缓存
	t.topkCache[metricName] = results
	t.cacheTime[metricName] = time.Now()

	return results
}

// calculateTopK 计算TopK
func (t *TopKAnalyzer) calculateTopK(metricName string, k int) []*TopKResult {
	// 使用最小堆来维护TopK
	h := &TopKHeap{}
	heap.Init(h)

	// 遍历所有主机的数据
	for hostID, data := range t.data {
		if len(data) == 0 {
			continue
		}

		// 获取该主机最新指标的值
		latestData := data[len(data)-1]
		var value float64
		var found bool

		for _, metric := range latestData.Metrics {
			if metric.Name == metricName {
				value = metric.Value
				found = true
				break
			}
		}

		if !found {
			continue
		}

		// 添加到堆中
		heap.Push(h, &TopKResult{
			HostID:     hostID,
			MetricName: metricName,
			Value:      value,
			Timestamp:  latestData.Timestamp,
		})

		// 保持堆大小不超过k
		if h.Len() > k {
			heap.Pop(h)
		}
	}

	// 从堆中提取结果
	results := make([]*TopKResult, 0, h.Len())
	for h.Len() > 0 {
		result := heap.Pop(h).(*TopKResult)
		results = append(results, result)
	}

	// 反转结果，使最高值排在前面
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	// 设置排名
	for i, result := range results {
		result.Rank = i + 1
	}

	return results
}

// GetTopKByTimeRange 获取时间范围内的TopK
func (t *TopKAnalyzer) GetTopKByTimeRange(metricName string, k int, startTime, endTime time.Time) []*TopKResult {
	t.dataMutex.RLock()
	defer t.dataMutex.RUnlock()

	// 计算时间范围内的平均值
	hostValues := make(map[string]float64)
	hostCounts := make(map[string]int)

	for hostID, data := range t.data {
		for _, d := range data {
			if d.Timestamp.After(startTime) && d.Timestamp.Before(endTime) {
				for _, metric := range d.Metrics {
					if metric.Name == metricName {
						hostValues[hostID] += metric.Value
						hostCounts[hostID]++
					}
				}
			}
		}
	}

	// 计算平均值
	var avgResults []*TopKResult
	for hostID, sum := range hostValues {
		if count := hostCounts[hostID]; count > 0 {
			avgResults = append(avgResults, &TopKResult{
				HostID:     hostID,
				MetricName: metricName,
				Value:      sum / float64(count),
				Timestamp:  time.Now(),
			})
		}
	}

	// 排序
	sort.Slice(avgResults, func(i, j int) bool {
		return avgResults[i].Value > avgResults[j].Value
	})

	// 取前k个
	if len(avgResults) > k {
		avgResults = avgResults[:k]
	}

	// 设置排名
	for i, result := range avgResults {
		result.Rank = i + 1
	}

	return avgResults
}

// ClearData 清除数据
func (t *TopKAnalyzer) ClearData() {
	t.dataMutex.Lock()
	defer t.dataMutex.Unlock()

	t.data = make(map[string][]*models.MetricsData)
	t.topkCache = make(map[string][]*TopKResult)
	t.cacheTime = make(map[string]time.Time)
}

// TopKHeap 最小堆实现
type TopKHeap []*TopKResult

func (h TopKHeap) Len() int           { return len(h) }
func (h TopKHeap) Less(i, j int) bool { return h[i].Value < h[j].Value }
func (h TopKHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TopKHeap) Push(x interface{}) {
	*h = append(*h, x.(*TopKResult))
}

func (h *TopKHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}