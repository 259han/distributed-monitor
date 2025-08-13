package host

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// HealthMonitor 健康监控器
type HealthMonitor struct {
	registry     *HostRegistry
	config       *HealthConfig
	checkers     map[string]HealthChecker
	mu           sync.RWMutex
	stopChan     chan struct{}
	eventChan    chan HealthEvent
	subscribers  map[string]chan HealthEvent
}

// HealthConfig 健康监控配置
type HealthConfig struct {
	CheckInterval    time.Duration
	Timeout          time.Duration
	MaxRetries       int
	EnableTCP        bool
	EnableHTTP       bool
	EnableCustom     bool
	TCPPorts         []int
	HTTPPaths        []string
	CustomChecks     []CustomHealthCheck
	AlertThresholds map[string]float64
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Check(host *HostInfo) (bool, error)
	Name() string
}

// HealthEvent 健康事件
type HealthEvent struct {
	HostID      string    `json:"host_id"`
	EventType   string    `json:"event_type"`
	OldState    HostState `json:"old_state"`
	NewState    HostState `json:"new_state"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// CustomHealthCheck 自定义健康检查
type CustomHealthCheck struct {
	Name        string
	Command     string
	Timeout     time.Duration
	ExpectCode  int
	ExpectValue string
}

// TCPHealthChecker TCP健康检查器
type TCPHealthChecker struct {
	ports []int
}

// HTTPHealthChecker HTTP健康检查器
type HTTPHealthChecker struct {
	paths []string
}

// CustomHealthChecker 自定义健康检查器
type CustomHealthChecker struct {
	config CustomHealthCheck
}

// NewHealthMonitor 创建健康监控器
func NewHealthMonitor(registry *HostRegistry, config *HealthConfig) *HealthMonitor {
	monitor := &HealthMonitor{
		registry:    registry,
		config:      config,
		checkers:    make(map[string]HealthChecker),
		stopChan:    make(chan struct{}),
		eventChan:   make(chan HealthEvent, 100),
		subscribers: make(map[string]chan HealthEvent),
	}

	// 初始化检查器
	monitor.initCheckers()

	return monitor
}

// initCheckers 初始化检查器
func (m *HealthMonitor) initCheckers() {
	if m.config.EnableTCP {
		m.checkers["tcp"] = &TCPHealthChecker{ports: m.config.TCPPorts}
	}

	if m.config.EnableHTTP {
		m.checkers["http"] = &HTTPHealthChecker{paths: m.config.HTTPPaths}
	}

	if m.config.EnableCustom {
		for _, check := range m.config.CustomChecks {
			m.checkers[check.Name] = &CustomHealthChecker{config: check}
		}
	}
}

// Start 启动健康监控
func (m *HealthMonitor) Start(ctx context.Context) {
	go m.monitorLoop(ctx)
	go m.eventDispatcher(ctx)
	log.Println("健康监控器已启动")
}

// Stop 停止健康监控
func (m *HealthMonitor) Stop() {
	close(m.stopChan)
	log.Println("健康监控器已停止")
}

// monitorLoop 监控循环
func (m *HealthMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAllHosts()
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		}
	}
}

// checkAllHosts 检查所有主机
func (m *HealthMonitor) checkAllHosts() {
	hosts := m.registry.GetAllHosts()
	for _, host := range hosts {
		go m.checkHost(host)
	}
}

// checkHost 检查单个主机
func (m *HealthMonitor) checkHost(host *HostInfo) {
	oldState := host.State
	var newState HostState = HostStateHealthy
	var message string
	details := make(map[string]interface{})

	// 执行所有检查器
	for name, checker := range m.checkers {
		healthy, err := checker.Check(host)
		details[name] = map[string]interface{}{
			"healthy": healthy,
			"error":   err != nil,
			"message": fmt.Sprintf("%v", err),
		}

		if !healthy {
			newState = HostStateUnhealthy
			message = fmt.Sprintf("健康检查失败: %s", name)
			break
		}
	}

	// 检查负载阈值
	if m.isOverThreshold(host) {
		newState = HostStateUnhealthy
		message = "负载超过阈值"
	}

	// 更新主机状态
	if newState != oldState {
		if err := m.registry.UpdateHostState(host.ID, newState); err != nil {
			log.Printf("更新主机状态失败: %v", err)
			return
		}

		// 发送健康事件
		event := HealthEvent{
			HostID:    host.ID,
			EventType: "state_change",
			OldState:  oldState,
			NewState:  newState,
			Message:   message,
			Timestamp: time.Now(),
			Details:   details,
		}

		m.eventChan <- event
	}
}

// isOverThreshold 检查是否超过阈值
func (m *HealthMonitor) isOverThreshold(host *HostInfo) bool {
	if m.config.AlertThresholds == nil {
		return false
	}

	if cpuThreshold, exists := m.config.AlertThresholds["cpu"]; exists {
		if host.Load.CPUUsage > cpuThreshold {
			return true
		}
	}

	if memoryThreshold, exists := m.config.AlertThresholds["memory"]; exists {
		if host.Load.MemoryUsage > memoryThreshold {
			return true
		}
	}

	if diskThreshold, exists := m.config.AlertThresholds["disk"]; exists {
		if host.Load.DiskUsage > diskThreshold {
			return true
		}
	}

	return false
}

// eventDispatcher 事件分发器
func (m *HealthMonitor) eventDispatcher(ctx context.Context) {
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
func (m *HealthMonitor) dispatchEvent(event HealthEvent) {
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

// Subscribe 订阅健康事件
func (m *HealthMonitor) Subscribe(id string) chan HealthEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan HealthEvent, 50)
	m.subscribers[id] = ch
	return ch
}

// Unsubscribe 取消订阅
func (m *HealthMonitor) Unsubscribe(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.subscribers[id]; exists {
		close(ch)
		delete(m.subscribers, id)
	}
}

// GetHealthStatus 获取健康状态统计
func (m *HealthMonitor) GetHealthStatus() map[string]interface{} {
	hosts := m.registry.GetAllHosts()
	
	status := map[string]interface{}{
		"total": len(hosts),
		"healthy": 0,
		"unhealthy": 0,
		"offline": 0,
		"maintenance": 0,
	}

	for _, host := range hosts {
		switch host.State {
		case HostStateHealthy:
			status["healthy"] = status["healthy"].(int) + 1
		case HostStateUnhealthy:
			status["unhealthy"] = status["unhealthy"].(int) + 1
		case HostStateOffline:
			status["offline"] = status["offline"].(int) + 1
		case HostStateMaintenance:
			status["maintenance"] = status["maintenance"].(int) + 1
		}
	}

	return status
}

// TCPHealthChecker 实现
func (c *TCPHealthChecker) Check(host *HostInfo) (bool, error) {
	for _, port := range c.ports {
		address := fmt.Sprintf("%s:%d", host.IPAddress, port)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			return false, fmt.Errorf("TCP连接失败: %v", err)
		}
		conn.Close()
	}
	return true, nil
}

func (c *TCPHealthChecker) Name() string {
	return "tcp"
}

// HTTPHealthChecker 实现
func (c *HTTPHealthChecker) Check(host *HostInfo) (bool, error) {
	// 简化的HTTP检查，实际实现需要HTTP客户端
	// 这里暂时使用TCP检查作为替代
	for range c.paths {
		address := fmt.Sprintf("%s:80", host.IPAddress)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			return false, fmt.Errorf("HTTP连接失败: %v", err)
		}
		conn.Close()
	}
	return true, nil
}

func (c *HTTPHealthChecker) Name() string {
	return "http"
}

// CustomHealthChecker 实现
func (c *CustomHealthChecker) Check(host *HostInfo) (bool, error) {
	// 简化的自定义检查，实际实现需要执行命令并检查结果
	// 这里暂时返回成功
	return true, nil
}

func (c *CustomHealthChecker) Name() string {
	return c.config.Name
}