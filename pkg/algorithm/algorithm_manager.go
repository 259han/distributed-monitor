package algorithm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	config "github.com/han-fei/monitor/pkg/agent"
	"github.com/willf/bloom"
)

// AlgorithmManager 算法管理器
type AlgorithmManager struct {
	config         *config.Config
	mu             sync.RWMutex
	slidingWindows map[string]*SlidingWindow
	bloomFilter    *bloom.BloomFilter
	timeWheel      *TimeWheel
	prefixTree     *Trie
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
	metrics        map[string]interface{}
}

// AlgorithmStats 算法统计信息
type AlgorithmStats struct {
	SlidingWindowStats map[string]interface{} `json:"sliding_window_stats"`
	BloomFilterStats   map[string]interface{} `json:"bloom_filter_stats"`
	TimeWheelStats     map[string]interface{} `json:"time_wheel_stats"`
	PrefixTreeStats    map[string]interface{} `json:"prefix_tree_stats"`
	TotalOperations    int64                  `json:"total_operations"`
	Uptime             time.Duration          `json:"uptime"`
}

// NewAlgorithmManager 创建算法管理器
func NewAlgorithmManager(cfg *config.Config) *AlgorithmManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &AlgorithmManager{
		config:         cfg,
		slidingWindows: make(map[string]*SlidingWindow),
		ctx:            ctx,
		cancel:         cancel,
		metrics:        make(map[string]interface{}),
	}

	// 初始化算法
	if err := manager.initializeAlgorithms(); err != nil {
		log.Printf("初始化算法失败: %v", err)
	}

	return manager
}

// initializeAlgorithms 初始化算法
func (am *AlgorithmManager) initializeAlgorithms() error {
	// 初始化滑动窗口
	if am.config.Algorithms.SlidingWindow.Enable {
		am.initializeSlidingWindows()
	}

	// 初始化布隆过滤器
	if am.config.Algorithms.BloomFilter.Enable {
		am.initializeBloomFilter()
	}

	// 初始化时间轮
	if am.config.Algorithms.TimeWheel.Enable {
		am.initializeTimeWheel()
	}

	// 初始化前缀树
	if am.config.Algorithms.PrefixTree.Enable {
		am.initializePrefixTree()
	}

	return nil
}

// initializeSlidingWindows 初始化滑动窗口
func (am *AlgorithmManager) initializeSlidingWindows() {
	// 创建默认滑动窗口
	defaultWindow := NewAdaptiveSlidingWindow(
		am.config.Algorithms.SlidingWindow.Size,
		am.config.Algorithms.SlidingWindow.MaxAge,
		am.config.Algorithms.SlidingWindow.Adaptive,
	)

	if am.config.Algorithms.SlidingWindow.Adaptive {
		defaultWindow.SetAdaptive(
			true,
			am.config.Algorithms.SlidingWindow.MinSize,
			am.config.Algorithms.SlidingWindow.MaxSize,
			am.config.Algorithms.SlidingWindow.GrowthFactor,
			am.config.Algorithms.SlidingWindow.ShrinkFactor,
		)
	}

	am.slidingWindows["default"] = defaultWindow

	// 创建针对不同指标的滑动窗口
	metrics := []string{"cpu", "memory", "disk", "network"}
	for _, metric := range metrics {
		window := NewAdaptiveSlidingWindow(
			am.config.Algorithms.SlidingWindow.Size,
			am.config.Algorithms.SlidingWindow.MaxAge,
			am.config.Algorithms.SlidingWindow.Adaptive,
		)
		am.slidingWindows[metric] = window
	}

	log.Printf("初始化滑动窗口完成，数量: %d", len(am.slidingWindows))
}

// initializeBloomFilter 初始化布隆过滤器
func (am *AlgorithmManager) initializeBloomFilter() {
	am.bloomFilter = bloom.NewWithEstimates(
		uint(am.config.Algorithms.BloomFilter.Size),
		am.config.Algorithms.BloomFilter.FalsePositive,
	)

	log.Printf("初始化布隆过滤器完成，大小: %d, 误判率: %.4f",
		am.config.Algorithms.BloomFilter.Size,
		am.config.Algorithms.BloomFilter.FalsePositive)
}

// initializeTimeWheel 初始化时间轮
func (am *AlgorithmManager) initializeTimeWheel() {
	am.timeWheel = NewTimeWheel(
		am.config.Algorithms.TimeWheel.TickInterval,
		am.config.Algorithms.TimeWheel.SlotNum,
	)

	log.Printf("初始化时间轮完成，间隔: %v, 槽位数: %d",
		am.config.Algorithms.TimeWheel.TickInterval,
		am.config.Algorithms.TimeWheel.SlotNum)
}

// initializePrefixTree 初始化前缀树
func (am *AlgorithmManager) initializePrefixTree() {
	am.prefixTree = NewTrie()

	// 添加一些默认规则
	defaultRules := map[string]string{
		"host-*":     "default_host",
		"server-*":   "server",
		"database-*": "database",
		"cache-*":    "cache",
	}

	for prefix, category := range defaultRules {
		am.prefixTree.Insert(prefix, category)
	}

	log.Printf("初始化前缀树完成，规则数量: %d", len(defaultRules))
}

// Start 启动算法管理器
func (am *AlgorithmManager) Start() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.isRunning {
		return fmt.Errorf("算法管理器已在运行")
	}

	// 启动时间轮
	if am.timeWheel != nil {
		am.timeWheel.Start()
	}

	// 启动后台任务
	go am.cleanupTask()
	go am.metricsTask()

	am.isRunning = true
	log.Printf("算法管理器启动成功")

	return nil
}

// Stop 停止算法管理器
func (am *AlgorithmManager) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.isRunning {
		return
	}

	am.cancel()

	// 停止时间轮
	if am.timeWheel != nil {
		am.timeWheel.Stop()
	}

	am.isRunning = false
	log.Printf("算法管理器已停止")
}

// AddSlidingWindowValue 添加滑动窗口值
func (am *AlgorithmManager) AddSlidingWindowValue(windowName string, value float64) error {
	am.mu.RLock()
	window, exists := am.slidingWindows[windowName]
	am.mu.RUnlock()

	if !exists {
		return fmt.Errorf("滑动窗口不存在: %s", windowName)
	}

	window.Add(value)
	return nil
}

// GetSlidingWindowStats 获取滑动窗口统计
func (am *AlgorithmManager) GetSlidingWindowStats(windowName string) (map[string]float64, error) {
	am.mu.RLock()
	window, exists := am.slidingWindows[windowName]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("滑动窗口不存在: %s", windowName)
	}

	return window.GetStats(), nil
}

// AddToBloomFilter 添加到布隆过滤器
func (am *AlgorithmManager) AddToBloomFilter(item string) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.bloomFilter != nil {
		am.bloomFilter.AddString(item)
	}
}

// TestBloomFilter 测试布隆过滤器
func (am *AlgorithmManager) TestBloomFilter(item string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.bloomFilter != nil {
		return am.bloomFilter.TestString(item)
	}
	return false
}

// AddTimeWheelTask 添加时间轮任务
func (am *AlgorithmManager) AddTimeWheelTask(delay time.Duration, key interface{}, task Task) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.timeWheel == nil {
		return fmt.Errorf("时间轮未启用")
	}

	am.timeWheel.AddTask(delay, key, task)
	return nil
}

// RemoveTimeWheelTask 移除时间轮任务
func (am *AlgorithmManager) RemoveTimeWheelTask(key interface{}) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.timeWheel == nil {
		return fmt.Errorf("时间轮未启用")
	}

	am.timeWheel.RemoveTask(key)
	return nil
}

// AddPrefixTreeRule 添加前缀树规则
func (am *AlgorithmManager) AddPrefixTreeRule(prefix, value string) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.prefixTree == nil {
		return fmt.Errorf("前缀树未启用")
	}

	am.prefixTree.Insert(prefix, value)
	return nil
}

// SearchPrefixTree 搜索前缀树
func (am *AlgorithmManager) SearchPrefixTree(key string) (string, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.prefixTree == nil {
		return "", false
	}

	value, exists := am.prefixTree.Search(key)
	if !exists {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

// GetStats 获取算法统计信息
func (am *AlgorithmManager) GetStats() *AlgorithmStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := &AlgorithmStats{
		SlidingWindowStats: make(map[string]interface{}),
		BloomFilterStats:   make(map[string]interface{}),
		TimeWheelStats:     make(map[string]interface{}),
		PrefixTreeStats:    make(map[string]interface{}),
	}

	// 滑动窗口统计
	for name, window := range am.slidingWindows {
		stats.SlidingWindowStats[name] = window.GetWindowInfo()
	}

	// 布隆过滤器统计
	if am.bloomFilter != nil {
		stats.BloomFilterStats = map[string]interface{}{
			"size":           am.bloomFilter.Cap(),
			"hash_functions": am.bloomFilter.K(),
			"bits":           am.bloomFilter.Cap() * 8,
		}
	}

	// 时间轮统计
	if am.timeWheel != nil {
		stats.TimeWheelStats = map[string]interface{}{
			"interval":    am.timeWheel.interval.String(),
			"slot_num":    am.timeWheel.slotNum,
			"current_pos": am.timeWheel.currentPos,
			"task_count":  am.timeWheel.GetTaskCount(),
			"multi_level": am.timeWheel.nextTimeWheel != nil,
		}
	}

	// 前缀树统计
	if am.prefixTree != nil {
		stats.PrefixTreeStats = am.prefixTree.GetStats()
	}

	// 总体统计
	stats.TotalOperations = am.getTotalOperations()
	if am.isRunning {
		stats.Uptime = time.Minute // 简化处理
	}

	return stats
}

// getTotalOperations 获取总操作数
func (am *AlgorithmManager) getTotalOperations() int64 {
	var total int64 = 0

	for _, window := range am.slidingWindows {
		if info := window.GetWindowInfo(); info != nil {
			if ops, ok := info["total_operations"].(int64); ok {
				total += ops
			}
		}
	}

	return total
}

// cleanupTask 清理任务
func (am *AlgorithmManager) cleanupTask() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.performCleanup()
		}
	}
}

// metricsTask 指标收集任务
func (am *AlgorithmManager) metricsTask() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.collectMetrics()
		}
	}
}

// performCleanup 执行清理
func (am *AlgorithmManager) performCleanup() {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 清理过期的滑动窗口数据
	for _, window := range am.slidingWindows {
		// 滑动窗口会自动清理过期数据
		windowInfo := window.GetWindowInfo()
		if count, ok := windowInfo["count"].(int); ok && count == 0 {
			// 如果窗口为空，考虑重置
			if time.Since(windowInfo["last_resize"].(time.Time)) > time.Hour {
				window.Reset()
			}
		}
	}

	// 清理时间轮中的过期任务
	if am.timeWheel != nil {
		// 时间轮会自动清理过期任务
	}

	log.Printf("算法管理器清理完成")
}

// collectMetrics 收集指标
func (am *AlgorithmManager) collectMetrics() {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 收集各种算法的指标
	am.metrics["timestamp"] = time.Now()
	am.metrics["sliding_window_count"] = len(am.slidingWindows)

	if am.bloomFilter != nil {
		am.metrics["bloom_filter_size"] = am.bloomFilter.Cap()
		am.metrics["bloom_filter_bits"] = am.bloomFilter.Cap() * 8
	}

	if am.timeWheel != nil {
		am.metrics["time_wheel_tasks"] = am.timeWheel.GetTaskCount()
	}

	if am.prefixTree != nil {
		am.metrics["prefix_tree_rules"] = am.prefixTree.Size()
	}
}

// GetMetrics 获取指标
func (am *AlgorithmManager) GetMetrics() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// 返回指标的副本
	metrics := make(map[string]interface{})
	for k, v := range am.metrics {
		metrics[k] = v
	}

	return metrics
}

// Reset 重置算法管理器
func (am *AlgorithmManager) Reset() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 重置滑动窗口
	for _, window := range am.slidingWindows {
		window.Reset()
	}

	// 重置布隆过滤器
	if am.bloomFilter != nil {
		am.bloomFilter.ClearAll()
	}

	// 重置时间轮
	if am.timeWheel != nil {
		am.timeWheel.Stop()
		am.initializeTimeWheel()
		am.timeWheel.Start()
	}

	// 重置前缀树
	if am.prefixTree != nil {
		am.prefixTree = NewTrie()
		am.initializePrefixTree()
	}

	// 重置指标
	am.metrics = make(map[string]interface{})

	log.Printf("算法管理器重置完成")
	return nil
}

// ExportState 导出状态
func (am *AlgorithmManager) ExportState() (map[string]interface{}, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	state := make(map[string]interface{})

	// 导出滑动窗口状态
	slidingWindowState := make(map[string]interface{})
	for name, window := range am.slidingWindows {
		slidingWindowState[name] = map[string]interface{}{
			"values":     window.Values(),
			"timestamps": window.GetWindowInfo(),
		}
	}
	state["sliding_windows"] = slidingWindowState

	// 导出布隆过滤器状态
	if am.bloomFilter != nil {
		state["bloom_filter"] = map[string]interface{}{
			"size":    am.bloomFilter.Cap(),
			"k":       am.bloomFilter.K(),
			"bit_set": "bitmap_data", // 简化处理
		}
	}

	// 导出前缀树状态
	if am.prefixTree != nil {
		state["prefix_tree"] = am.prefixTree.Export()
	}

	return state, nil
}

// ImportState 导入状态
func (am *AlgorithmManager) ImportState(state map[string]interface{}) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 导入滑动窗口状态
	if slidingWindows, ok := state["sliding_windows"].(map[string]interface{}); ok {
		for name, windowState := range slidingWindows {
			if window, exists := am.slidingWindows[name]; exists {
				if values, ok := windowState.(map[string]interface{})["values"].([]interface{}); ok {
					for _, v := range values {
						if value, ok := v.(float64); ok {
							window.Add(value)
						}
					}
				}
			}
		}
	}

	// 导入前缀树状态
	if prefixTree, ok := state["prefix_tree"].(map[string]interface{}); ok {
		if am.prefixTree != nil {
			am.prefixTree.Import(prefixTree)
		}
	}

	log.Printf("算法管理器状态导入完成")
	return nil
}
