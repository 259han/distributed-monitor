package host

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// HostManager 主机管理器
type HostManager struct {
	registry        *HostRegistry
	discovery       *DiscoveryService
	health          *HealthMonitor
	config          *ManagerConfig
	mu              sync.RWMutex
	stopChan        chan struct{}
	eventChan       chan ManagerEvent
	subscribers     map[string]chan ManagerEvent
	scalingPolicy   *ScalingPolicy
	loadBalancer    *LoadBalancer
	costAnalyzer    *CostAnalyzer
	metricsCollector *MetricsCollector
	predictor       *LoadPredictor
	lastScaleAction time.Time
	scaleHistory   []ScaleEvent
	resourceHistory []ResourceSnapshot
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	RegistryConfig  *RegistryConfig
	DiscoveryConfig *DiscoveryConfig
	HealthConfig    *HealthConfig
	AutoRemove      bool
	RemoveTimeout   time.Duration
	MaxHosts        int
	MinHosts        int
	EnableScaling   bool
	ScalingConfig   *ScalingConfig
	LoadBalancer    *LoadBalancerConfig
	CostConfig      *CostConfig
}

// ScalingConfig 扩缩容配置
type ScalingConfig struct {
	EnableHorizontalScale bool          `json:"enable_horizontal_scale"`
	EnableVerticalScale   bool          `json:"enable_vertical_scale"`
	ScaleUpThreshold      float64       `json:"scale_up_threshold"`
	ScaleDownThreshold    float64       `json:"scale_down_threshold"`
	CooldownPeriod        time.Duration `json:"cooldown_period"`
	ScaleStep             int            `json:"scale_step"`
	MaxScaleUpPerCycle    int            `json:"max_scale_up_per_cycle"`
	MaxScaleDownPerCycle  int            `json:"max_scale_down_per_cycle"`
	PredictionWindow     time.Duration `json:"prediction_window"`
	EnablePrediction     bool           `json:"enable_prediction"`
	ScaleStrategy         string         `json:"scale_strategy"` // "threshold", "predictive", "cost_optimized"
	ResourceWeights       ResourceWeights `json:"resource_weights"`
	TimeBasedRules        []TimeBasedRule `json:"time_based_rules"`
	MinHosts              int            `json:"min_hosts"`
	MaxHosts              int            `json:"max_hosts"`
}

// ResourceWeights 资源权重配置
type ResourceWeights struct {
	CPU     float64 `json:"cpu_weight"`
	Memory  float64 `json:"memory_weight"`
	Disk    float64 `json:"disk_weight"`
	Network float64 `json:"network_weight"`
	Custom  float64 `json:"custom_weight"`
}

// TimeBasedRule 基于时间的扩缩容规则
type TimeBasedRule struct {
	Name        string        `json:"name"`
	StartTime   string        `json:"start_time"`
	EndTime     string        `json:"end_time"`
	DaysOfWeek  []int         `json:"days_of_week"`
	MinHosts    int           `json:"min_hosts"`
	MaxHosts    int           `json:"max_hosts"`
	Action      string        `json:"action"` // "scale_up", "scale_down", "maintain"
	Description string        `json:"description"`
}

// ResourceMetrics 资源指标
type ResourceMetrics struct {
	CPUUsage           float64 `json:"cpu_usage"`
	MemoryUsage        float64 `json:"memory_usage"`
	DiskUsage          float64 `json:"disk_usage"`
	NetworkUsage       float64 `json:"network_usage"`
	ActiveConnections  int     `json:"active_connections"`
	RequestRate        float64 `json:"request_rate"`
	ResponseTime       float64 `json:"response_time"`
	ErrorRate          float64 `json:"error_rate"`
}

// ResourceSnapshot 资源快照
type ResourceSnapshot struct {
	Timestamp time.Time        `json:"timestamp"`
	Metrics   *ResourceMetrics `json:"metrics"`
	HostID    string           `json:"host_id"`
}

// ScalingDecision 扩缩容决策
type ScalingDecision struct {
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	TargetCount int    `json:"target_count"`
	Priority    int    `json:"priority"`
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy           string `json:"strategy"` // "round_robin", "least_connections", "weighted", "resource_based"
	EnableHealthCheck  bool   `json:"enable_health_check"`
	SessionPersistence bool   `json:"session_persistence"`
	WeightAlgorithm    string `json:"weight_algorithm"` // "static", "dynamic", "adaptive"
}

// CostConfig 成本配置
type CostConfig struct {
	EnableCostOptimization bool          `json:"enable_cost_optimization"`
	CostPerHostHour       float64       `json:"cost_per_host_hour"`
	BudgetLimit           float64       `json:"budget_limit"`
	BillingCycle          time.Duration `json:"billing_cycle"`
	CostThreshold         float64       `json:"cost_threshold"`
}

// ManagerEvent 管理器事件
type ManagerEvent struct {
	Type      string                 `json:"type"`
	HostID    string                 `json:"host_id"`
	HostInfo  *HostInfo              `json:"host_info,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}

// NewHostManager 创建主机管理器
func NewHostManager(config *ManagerConfig) *HostManager {
	manager := &HostManager{
		config:          config,
		stopChan:        make(chan struct{}),
		eventChan:       make(chan ManagerEvent, 100),
		subscribers:     make(map[string]chan ManagerEvent),
		lastScaleAction: time.Time{},
		scaleHistory:   make([]ScaleEvent, 0),
		resourceHistory: make([]ResourceSnapshot, 0),
	}

	// 初始化各个组件
	manager.registry = NewHostRegistry(config.RegistryConfig)
	manager.discovery = NewDiscoveryService(manager.registry, config.DiscoveryConfig)
	manager.health = NewHealthMonitor(manager.registry, config.HealthConfig)
	
	// 初始化扩缩容相关组件
	if config.EnableScaling && config.ScalingConfig != nil {
		manager.scalingPolicy = NewScalingPolicy(config.ScalingConfig)
		manager.predictor = NewLoadPredictor(config.ScalingConfig.PredictionWindow)
		manager.metricsCollector = NewMetricsCollector()
	}
	
	// 初始化负载均衡器
	if config.LoadBalancer != nil {
		manager.loadBalancer = NewLoadBalancer(config.LoadBalancer)
	}
	
	// 初始化成本分析器
	if config.CostConfig != nil {
		manager.costAnalyzer = NewCostAnalyzer(config.CostConfig)
	}
	
	// 初始化随机数生成器
	rand.Seed(time.Now().UnixNano())

	return manager
}

// Start 启动主机管理器
func (m *HostManager) Start(ctx context.Context) error {
	// 启动各个组件
	if err := m.discovery.Start(ctx); err != nil {
		return fmt.Errorf("启动发现服务失败: %v", err)
	}

	m.health.Start(ctx)
	m.registry.Start(ctx)

	// 启动管理器主循环
	go m.managerLoop(ctx)
	go m.eventDispatcher(ctx)

	log.Println("主机管理器已启动")
	return nil
}

// Stop 停止主机管理器
func (m *HostManager) Stop() {
	close(m.stopChan)
	m.discovery.Stop()
	m.health.Stop()
	m.registry.Stop()
	log.Println("主机管理器已停止")
}

// managerLoop 管理器主循环
func (m *HostManager) managerLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.maintenanceTasks()
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		}
	}
}

// maintenanceTasks 维护任务
func (m *HostManager) maintenanceTasks() {
	if m.config.AutoRemove {
		m.removeUnhealthyHosts()
	}

	if m.config.EnableScaling {
		m.checkScaling()
	}

	m.cleanupResources()
}

// removeUnhealthyHosts 移除不健康的主机
func (m *HostManager) removeUnhealthyHosts() {
	hosts := m.registry.GetAllHosts()
	now := time.Now()

	for _, host := range hosts {
		if host.State == HostStateUnhealthy {
			if now.Sub(host.LastCheck) > m.config.RemoveTimeout {
				if err := m.removeHost(host.ID, "unhealthy_timeout"); err != nil {
					log.Printf("移除不健康主机失败: %v", err)
				}
			}
		} else if host.State == HostStateOffline {
			if now.Sub(host.LastHeartbeat) > m.config.RemoveTimeout*2 {
				if err := m.removeHost(host.ID, "offline_timeout"); err != nil {
					log.Printf("移除离线主机失败: %v", err)
				}
			}
		}
	}
}

// checkScaling 检查扩缩容
func (m *HostManager) checkScaling() {
	// 获取当前主机数量
	hostCount := m.registry.GetHostCount()
	healthyHosts := m.registry.GetHealthyHosts()
	healthyCount := len(healthyHosts)
	
	// 获取资源指标
	resourceMetrics := m.metricsCollector.GetResourceMetrics()
	
	// 计算负载分数
	loadScore := m.calculateLoadScore(resourceMetrics, healthyCount)
	
	// 预测未来负载
	predictedLoad := m.predictor.PredictLoad(time.Minute * 5)
	
	// 基于多种因素进行扩缩容决策
	scalingDecision := m.makeScalingDecision(loadScore, predictedLoad, hostCount, healthyCount)
	
	// 执行扩缩容操作
	if scalingDecision.Action != "none" {
		m.executeScaling(scalingDecision)
	}
}

// calculateLoadScore 计算负载分数
func (m *HostManager) calculateLoadScore(metrics *ResourceMetrics, healthyCount int) float64 {
	if healthyCount == 0 {
		return 100.0 // 最高负载
	}
	
	weights := m.config.ScalingConfig.ResourceWeights
	
	// 计算各项资源利用率
	cpuScore := (metrics.CPUUsage / 100.0) * weights.CPU
	memoryScore := (metrics.MemoryUsage / 100.0) * weights.Memory
	diskScore := (metrics.DiskUsage / 100.0) * weights.Disk
	networkScore := (metrics.NetworkUsage / 100.0) * weights.Network
	
	// 计算主机密度分数
	densityScore := float64(metrics.ActiveConnections) / float64(healthyCount*1000)
	
	// 综合分数
	totalScore := cpuScore + memoryScore + diskScore + networkScore + densityScore*weights.Custom
	
	// 确保分数在0-100范围内
	if totalScore > 100.0 {
		totalScore = 100.0
	}
	
	return totalScore
}

// makeScalingDecision 做出扩缩容决策
func (m *HostManager) makeScalingDecision(loadScore, predictedLoad float64, hostCount, healthyCount int) ScalingDecision {
	config := m.config.ScalingConfig
	decision := ScalingDecision{Action: "none"}
	
	// 检查冷却时间
	if time.Since(m.lastScaleAction) < config.CooldownPeriod {
		return decision
	}
	
	// 检查时间规则
	timeBasedDecision := m.checkTimeBasedRules()
	if timeBasedDecision.Action != "none" {
		return timeBasedDecision
	}
	
	// 基于负载的扩缩容
	currentLoad := math.Max(loadScore, predictedLoad)
	
	// 扩容决策
	if currentLoad > config.ScaleUpThreshold && hostCount < config.MaxHosts {
		decision.Action = "scale_up"
		decision.Reason = "high_load"
		decision.TargetCount = min(hostCount+config.ScaleStep, config.MaxHosts)
	}
	
	// 缩容决策
	if currentLoad < config.ScaleDownThreshold && hostCount > config.MinHosts && healthyCount > config.MinHosts {
		decision.Action = "scale_down"
		decision.Reason = "low_load"
		decision.TargetCount = max(hostCount-config.ScaleStep, config.MinHosts)
	}
	
	// 成本优化决策
	if config.ScaleStrategy == "cost_optimized" && m.costAnalyzer != nil {
		costDecision := m.costAnalyzer.OptimizeCost(hostCount, loadScore)
		if costDecision.Action != "none" {
			decision = costDecision
		}
	}
	
	return decision
}

// cleanupResources 清理资源
func (m *HostManager) cleanupResources() {
	// 清理过期的事件订阅
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ch := range m.subscribers {
		select {
		case <-ch:
			// 通道已关闭，移除订阅
			delete(m.subscribers, id)
		default:
			// 通道正常，保持订阅
		}
	}
}

// removeHost 移除主机
func (m *HostManager) removeHost(hostID, reason string) error {
	// 获取主机信息
	host, err := m.registry.GetHost(hostID)
	if err != nil {
		return err
	}

	// 从注册表中移除
	if err := m.registry.UnregisterHost(hostID); err != nil {
		return err
	}

	// 发送移除事件
	event := ManagerEvent{
		Type:      "host_removed",
		HostID:    hostID,
		HostInfo:  host,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"reason": reason},
	}
	m.eventChan <- event

	log.Printf("主机 %s 已移除，原因: %s", hostID, reason)
	return nil
}

// AddHost 添加主机
func (m *HostManager) AddHost(host *HostInfo) error {
	// 检查主机数量限制
	if m.config.MaxHosts > 0 && m.registry.GetHostCount() >= m.config.MaxHosts {
		return fmt.Errorf("主机数量已达到上限: %d", m.config.MaxHosts)
	}

	// 注册主机
	if err := m.registry.RegisterHost(host); err != nil {
		return err
	}

	// 发送添加事件
	event := ManagerEvent{
		Type:      "host_added",
		HostID:    host.ID,
		HostInfo:  host,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"method": "manual"},
	}
	m.eventChan <- event

	log.Printf("主机 %s 已添加", host.ID)
	return nil
}

// UpdateHost 更新主机信息
func (m *HostManager) UpdateHost(hostID string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	host, err := m.registry.GetHost(hostID)
	if err != nil {
		return err
	}

	// 应用更新
	if val, exists := updates["hostname"]; exists {
		host.Hostname = val.(string)
	}
	if val, exists := updates["ip_address"]; exists {
		host.IPAddress = val.(string)
	}
	if val, exists := updates["port"]; exists {
		host.Port = val.(int)
	}
	if val, exists := updates["state"]; exists {
		host.State = val.(HostState)
	}
	if val, exists := updates["tags"]; exists {
		host.Tags = val.([]string)
	}
	if val, exists := updates["metadata"]; exists {
		host.Metadata = val.(map[string]interface{})
	}

	// 发送更新事件
	event := ManagerEvent{
		Type:      "host_updated",
		HostID:    hostID,
		HostInfo:  host,
		Timestamp: time.Now(),
		Details:   updates,
	}
	m.eventChan <- event

	log.Printf("主机 %s 信息已更新", hostID)
	return nil
}

// SetHostMaintenance 设置主机维护模式
func (m *HostManager) SetHostMaintenance(hostID string, maintenance bool) error {
	state := HostStateHealthy
	if maintenance {
		state = HostStateMaintenance
	}

	if err := m.registry.UpdateHostState(hostID, state); err != nil {
		return err
	}

	event := ManagerEvent{
		Type:      "maintenance_changed",
		HostID:    hostID,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"maintenance": maintenance},
	}
	m.eventChan <- event

	log.Printf("主机 %s 维护模式已设置为: %v", hostID, maintenance)
	return nil
}

// GetHost 获取主机信息
func (m *HostManager) GetHost(hostID string) (*HostInfo, error) {
	return m.registry.GetHost(hostID)
}

// GetAllHosts 获取所有主机
func (m *HostManager) GetAllHosts() []*HostInfo {
	return m.registry.GetAllHosts()
}

// GetHealthyHosts 获取健康主机
func (m *HostManager) GetHealthyHosts() []*HostInfo {
	return m.registry.GetHealthyHosts()
}

// GetStats 获取统计信息
func (m *HostManager) GetStats() map[string]interface{} {
	healthStats := m.health.GetHealthStatus()
	
	stats := map[string]interface{}{
		"total_hosts":       m.registry.GetHostCount(),
		"healthy_hosts":     m.registry.GetHostCountByState(HostStateHealthy),
		"unhealthy_hosts":   m.registry.GetHostCountByState(HostStateUnhealthy),
		"offline_hosts":     m.registry.GetHostCountByState(HostStateOffline),
		"maintenance_hosts": m.registry.GetHostCountByState(HostStateMaintenance),
		"max_hosts":         m.config.MaxHosts,
		"auto_remove":       m.config.AutoRemove,
		"enable_scaling":    m.config.EnableScaling,
		"health_status":     healthStats,
	}

	// 添加资源指标
	if m.metricsCollector != nil {
		stats["resource_metrics"] = m.metricsCollector.GetResourceMetrics()
	}

	// 添加扩缩容历史
	stats["scale_history"] = m.scaleHistory

	return stats
}

// executeScaling 执行扩缩容
func (m *HostManager) executeScaling(decision ScalingDecision) {
	if decision.Action == "none" {
		return
	}

	// 记录扩缩容事件
	event := ScaleEvent{
		Action:      decision.Action,
		Reason:      decision.Reason,
		TargetCount: decision.TargetCount,
		Timestamp:   time.Now(),
		Success:     false,
	}

	// 执行扩缩容
	var err error
	switch decision.Action {
	case "scale_up":
		err = m.performScaleUp(decision.TargetCount)
	case "scale_down":
		err = m.performScaleDown(decision.TargetCount)
	}

	// 更新事件状态
	event.Success = err == nil
	if err != nil {
		event.Error = err.Error()
	}

	// 记录历史
	m.scaleHistory = append(m.scaleHistory, event)
	m.lastScaleAction = time.Now()

	// 发送事件
	managerEvent := ManagerEvent{
		Type:      "scaling_event",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"action":    decision.Action,
			"reason":    decision.Reason,
			"success":   event.Success,
			"target_count": decision.TargetCount,
		},
	}
	m.eventChan <- managerEvent

	if err != nil {
		log.Printf("扩缩容执行失败: %v", err)
	} else {
		log.Printf("扩缩容执行成功: %s -> %d", decision.Action, decision.TargetCount)
	}
}

// performScaleUp 执行扩容
func (m *HostManager) performScaleUp(targetCount int) error {
	currentCount := m.registry.GetHostCount()
	if currentCount >= targetCount {
		return fmt.Errorf("当前主机数量 %d 已达到目标数量 %d", currentCount, targetCount)
	}

	// 这里可以集成云服务API来启动新的主机
	// 例如：AWS EC2、Kubernetes、Docker等
	for i := currentCount; i < targetCount; i++ {
		newHost := &HostInfo{
			ID:         fmt.Sprintf("host-%d-%d", time.Now().Unix(), i),
			Hostname:   fmt.Sprintf("broker-%d", i),
			IPAddress:  fmt.Sprintf("192.168.1.%d", 100+i),
			Port:       9093,
			State:      HostStateHealthy,
			Tags:       []string{"auto_scaled"},
			Capacity: HostCapacity{
				CPU:       4,
				MemoryMB:  8192,
				DiskGB:    100,
				NetworkMB: 1000,
			},
			LastHeartbeat: time.Now(),
			LastCheck:     time.Now(),
			Metadata:      make(map[string]interface{}),
			Load: HostLoad{
				CPUUsage:     0.0,
				MemoryUsage:  0.0,
				DiskUsage:    0.0,
				NetworkUsage: 0.0,
				Connections:  0,
			},
		}

		if err := m.registry.RegisterHost(newHost); err != nil {
			return fmt.Errorf("注册主机 %s 失败: %v", newHost.ID, err)
		}
	}

	return nil
}

// performScaleDown 执行缩容
func (m *HostManager) performScaleDown(targetCount int) error {
	currentCount := m.registry.GetHostCount()
	if currentCount <= targetCount {
		return fmt.Errorf("当前主机数量 %d 已达到目标数量 %d", currentCount, targetCount)
	}

	// 获取健康主机，优先移除负载较低的
	healthyHosts := m.registry.GetHealthyHosts()
	if len(healthyHosts) <= targetCount {
		return fmt.Errorf("健康主机数量 %d 不足，无法缩容到 %d", len(healthyHosts), targetCount)
	}

	// 按负载排序（这里简化处理，实际应该根据真实负载指标）
	sort.Slice(healthyHosts, func(i, j int) bool {
		return healthyHosts[i].ID < healthyHosts[j].ID // 简化处理，实际应该基于负载
	})

	// 移除多余的主机
	for i := targetCount; i < currentCount; i++ {
		if i < len(healthyHosts) {
			if err := m.removeHost(healthyHosts[i].ID, "scale_down"); err != nil {
				return fmt.Errorf("移除主机 %s 失败: %v", healthyHosts[i].ID, err)
			}
		}
	}

	return nil
}

// checkTimeBasedRules 检查基于时间的规则
func (m *HostManager) checkTimeBasedRules() ScalingDecision {
	if m.config.ScalingConfig == nil {
		return ScalingDecision{Action: "none"}
	}

	now := time.Now()
	currentDay := int(now.Weekday())
	currentTime := now.Format("15:04")

	for _, rule := range m.config.ScalingConfig.TimeBasedRules {
		// 检查是否匹配当前时间
		if m.matchesTimeRule(rule, currentDay, currentTime) {
			decision := ScalingDecision{
				Action:      rule.Action,
				Reason:      fmt.Sprintf("time_based_rule:%s", rule.Name),
				TargetCount: m.getRuleTargetCount(rule),
				Priority:    10, // 时间规则优先级较高
			}
			return decision
		}
	}

	return ScalingDecision{Action: "none"}
}

// matchesTimeRule 检查是否匹配时间规则
func (m *HostManager) matchesTimeRule(rule TimeBasedRule, currentDay int, currentTime string) bool {
	// 检查星期
	dayMatch := false
	for _, day := range rule.DaysOfWeek {
		if day == currentDay {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false
	}

	// 检查时间范围
	if currentTime >= rule.StartTime && currentTime <= rule.EndTime {
		return true
	}

	return false
}

// getRuleTargetCount 获取规则目标主机数量
func (m *HostManager) getRuleTargetCount(rule TimeBasedRule) int {
	currentCount := m.registry.GetHostCount()

	switch rule.Action {
	case "scale_up":
		if currentCount < rule.MaxHosts {
			return min(currentCount+1, rule.MaxHosts)
		}
	case "scale_down":
		if currentCount > rule.MinHosts {
			return max(currentCount-1, rule.MinHosts)
		}
	}

	return currentCount
}

// ScaleEvent 扩缩容事件
type ScaleEvent struct {
	Action      string    `json:"action"`
	Reason      string    `json:"reason"`
	TargetCount int       `json:"target_count"`
	Timestamp   time.Time `json:"timestamp"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	metrics *ResourceMetrics
	mu      sync.RWMutex
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &ResourceMetrics{},
	}
}

// GetResourceMetrics 获取资源指标
func (mc *MetricsCollector) GetResourceMetrics() *ResourceMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.metrics
}

// UpdateMetrics 更新指标
func (mc *MetricsCollector) UpdateMetrics(metrics *ResourceMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = metrics
}

// LoadPredictor 负载预测器
type LoadPredictor struct {
	window      time.Duration
	history     []float64
	maxHistory  int
	mu          sync.RWMutex
}

// NewLoadPredictor 创建负载预测器
func NewLoadPredictor(window time.Duration) *LoadPredictor {
	return &LoadPredictor{
		window:     window,
		history:    make([]float64, 0),
		maxHistory: 100,
	}
}

// PredictLoad 预测负载
func (lp *LoadPredictor) PredictLoad(duration time.Duration) float64 {
	lp.mu.RLock()
	defer lp.mu.RUnlock()
	
	if len(lp.history) == 0 {
		return 0.0
	}
	
	// 简单的移动平均预测
	sum := 0.0
	count := 0
	for _, val := range lp.history {
		sum += val
		count++
	}
	
	return sum / float64(count)
}

// AddHistory 添加历史数据
func (lp *LoadPredictor) AddHistory(load float64) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	
	lp.history = append(lp.history, load)
	if len(lp.history) > lp.maxHistory {
		lp.history = lp.history[1:]
	}
}

// CostAnalyzer 成本分析器
type CostAnalyzer struct {
	config *CostConfig
}

// NewCostAnalyzer 创建成本分析器
func NewCostAnalyzer(config *CostConfig) *CostAnalyzer {
	return &CostAnalyzer{
		config: config,
	}
}

// OptimizeCost 优化成本
func (ca *CostAnalyzer) OptimizeCost(hostCount int, loadScore float64) ScalingDecision {
	if !ca.config.EnableCostOptimization {
		return ScalingDecision{Action: "none"}
	}
	
	// 简单的成本优化逻辑
	currentCost := float64(hostCount) * ca.config.CostPerHostHour
	
	if currentCost > ca.config.BudgetLimit {
		return ScalingDecision{
			Action:      "scale_down",
			Reason:      "cost_optimization",
			TargetCount: hostCount - 1,
			Priority:    5,
		}
	}
	
	return ScalingDecision{Action: "none"}
}

// ScalingPolicy 扩缩容策略
type ScalingPolicy struct {
	config *ScalingConfig
}

// NewScalingPolicy 创建扩缩容策略
func NewScalingPolicy(config *ScalingConfig) *ScalingPolicy {
	return &ScalingPolicy{
		config: config,
	}
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	config *LoadBalancerConfig
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(config *LoadBalancerConfig) *LoadBalancer {
	return &LoadBalancer{
		config: config,
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Subscribe 订阅管理器事件
func (m *HostManager) Subscribe(id string) chan ManagerEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan ManagerEvent, 50)
	m.subscribers[id] = ch
	return ch
}

// Unsubscribe 取消订阅
func (m *HostManager) Unsubscribe(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.subscribers[id]; exists {
		close(ch)
		delete(m.subscribers, id)
	}
}

// eventDispatcher 事件分发器
func (m *HostManager) eventDispatcher(ctx context.Context) {
	for {
		select {
		case event := <-m.eventChan:
			m.dispatchEvent(event)
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		}
	}
}

// dispatchEvent 分发事件
func (m *HostManager) dispatchEvent(event ManagerEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			log.Printf("事件分发失败，订阅者 %s 通道已满", id)
		}
	}
}

// ScanNetwork 扫描网络
func (m *HostManager) ScanNetwork(networkRange string) (*HostDiscoveryResponse, error) {
	request := HostDiscoveryRequest{
		NetworkRange: networkRange,
		PortRange:    "9093",
		Timeout:      10,
		Tags:         []string{"manual_scan"},
	}

	response := m.discovery.performNetworkScan(request)
	return response, nil
}

// ForceRemoveHost 强制移除主机
func (m *HostManager) ForceRemoveHost(hostID, reason string) error {
	return m.removeHost(hostID, reason)
}