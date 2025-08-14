package analysis

import (
	"sync"
	"time"

	"github.com/han-fei/monitor/visualization/internal/cpp"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// TopKResult TopK结果
type TopKResult struct {
	HostID     string    `json:"host_id"`
	MetricName string    `json:"metric_name"`
	Value      float64   `json:"value"`
	Rank       int       `json:"rank"`
	Timestamp  time.Time `json:"timestamp"`
}

// StreamingTopKAnalyzer 流式Top-K分析器（使用C++实现）
type StreamingTopKAnalyzer struct {
	manager     *cpp.StreamingTopKManager
	processors  map[string]*cpp.StreamingTopKProcessor // metric_name -> processor
	mutex       sync.RWMutex
	initialized bool
}

// NewStreamingTopKAnalyzer 创建流式Top-K分析器
func NewStreamingTopKAnalyzer() *StreamingTopKAnalyzer {
	analyzer := &StreamingTopKAnalyzer{
		processors:  make(map[string]*cpp.StreamingTopKProcessor),
		initialized: false,
	}

	// 初始化管理器
	analyzer.initializeManager()

	return analyzer
}

// initializeManager 初始化管理器
func (s *StreamingTopKAnalyzer) initializeManager() {
	s.manager = cpp.GetStreamingTopKManagerInstance()
	if s.manager == nil {
		panic("Failed to get StreamingTopKManager instance")
	}

	// 初始化管理器
	err := s.manager.Initialize(10, 3600) // 默认K=10, TTL=3600秒
	if err != nil {
		panic("Failed to initialize StreamingTopKManager: " + err.Error())
	}

	s.initialized = true
}

// AddData 添加数据
func (s *StreamingTopKAnalyzer) AddData(data *models.MetricsData) {
	if !s.initialized {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 为每个指标创建或获取处理器
	for _, metric := range data.Metrics {
		processor, exists := s.processors[metric.Name]
		if !exists {
			// 创建新的处理器
			var err error
			processor, err = s.manager.CreateProcessor(metric.Name, 10, 3600)
			if err != nil {
				continue // 跳过创建失败的处理器
			}
			s.processors[metric.Name] = processor
		}

		// 流式处理数据
		err := processor.ProcessStream(data.HostID, metric.Value)
		if err != nil {
			// 记录错误但继续处理
			continue
		}
	}
}

// GetTopK 获取Top-K结果
func (s *StreamingTopKAnalyzer) GetTopK(metricName string, k int) []*TopKResult {
	if !s.initialized {
		return nil
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 获取处理器
	processor, exists := s.processors[metricName]
	if !exists {
		return nil
	}

	// 获取Top-K结果
	cppResults := processor.GetTopK()
	if len(cppResults) == 0 {
		return nil
	}

	// 转换为TopKResult格式
	results := make([]*TopKResult, 0, len(cppResults))
	for i, item := range cppResults {
		if i >= k {
			break
		}

		result := &TopKResult{
			HostID:     item.HostID,
			MetricName: metricName,
			Value:      item.Value,
			Rank:       i + 1,
			Timestamp:  time.Unix(item.Timestamp, 0),
		}
		results = append(results, result)
	}

	return results
}

// GetTopKByTimeRange 获取时间范围内的Top-K（流式实现不支持，返回当前Top-K）
func (s *StreamingTopKAnalyzer) GetTopKByTimeRange(metricName string, k int, startTime, endTime time.Time) []*TopKResult {
	// 流式Top-K不支持时间范围查询，返回当前Top-K
	return s.GetTopK(metricName, k)
}

// ClearData 清除数据
func (s *StreamingTopKAnalyzer) ClearData() {
	if !s.initialized {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 清除所有处理器
	for _, processor := range s.processors {
		processor.Clear()
	}
}

// GetStatistics 获取统计信息
func (s *StreamingTopKAnalyzer) GetStatistics() map[string]interface{} {
	if !s.initialized {
		return nil
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := make(map[string]interface{})

	// 获取系统统计信息
	systemStats := s.manager.GetSystemStats()
	if systemStats != nil {
		stats["system"] = systemStats
	}

	// 获取每个处理器的统计信息
	processorStats := make(map[string]interface{})
	for metricName, processor := range s.processors {
		procStats := processor.GetStats()
		if procStats != nil {
			processorStats[metricName] = procStats
		}
	}
	stats["processors"] = processorStats

	return stats
}

// GetProcessorNames 获取所有处理器名称
func (s *StreamingTopKAnalyzer) GetProcessorNames() []string {
	if !s.initialized {
		return nil
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	names := make([]string, 0, len(s.processors))
	for name := range s.processors {
		names = append(names, name)
	}

	return names
}

// Close 关闭分析器
func (s *StreamingTopKAnalyzer) Close() {
	if !s.initialized {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 关闭所有处理器
	for _, processor := range s.processors {
		processor.Close()
	}

	// 关闭管理器
	s.manager.Shutdown()

	s.initialized = false
}

// IsInitialized 检查是否已初始化
func (s *StreamingTopKAnalyzer) IsInitialized() bool {
	return s.initialized
}
