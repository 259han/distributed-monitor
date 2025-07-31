package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
)

// HostInfo 主机信息
type HostInfo struct {
	ID        string            `json:"id"`
	Hostname  string            `json:"hostname"`
	IP        string            `json:"ip"`
	Port      int               `json:"port"`
	Status    string            `json:"status"` // online, offline, error
	LastSeen  time.Time         `json:"last_seen"`
	Metrics   map[string]string `json:"metrics"`
	Tags      map[string]string `json:"tags"`
	CPUCount  int               `json:"cpu_count"`
	MemoryMB  int64             `json:"memory_mb"`
	DiskGB    int64             `json:"disk_gb"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	Uptime    int64             `json:"uptime"`
	Version   string            `json:"version"`
	Location  string            `json:"location"`
	Group     string            `json:"group"`
	Priority  int               `json:"priority"`
}

// HostRegistry 主机注册管理器
type HostRegistry struct {
	config         *config.Config
	hosts          map[string]*HostInfo
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	updateTicker   *time.Ticker
	healthTicker   *time.Ticker
	eventChan      chan HostEvent
	subscribers    map[chan HostEvent]bool
	subscribersMu  sync.RWMutex
	localHost      *HostInfo
	failedHosts    map[string]int
	retryTicker    *time.Ticker
	maxRetries     int
	retryInterval  time.Duration
	enableHealth   bool
}

// HostEvent 主机事件
type HostEvent struct {
	Type      string     `json:"type"`      // add, update, remove, health_check
	HostInfo  *HostInfo  `json:"host_info"`
	Timestamp time.Time  `json:"timestamp"`
	Error     error      `json:"error,omitempty"`
}

// NewHostRegistry 创建主机注册管理器
func NewHostRegistry(cfg *config.Config) *HostRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	
	registry := &HostRegistry{
		config:        cfg,
		hosts:         make(map[string]*HostInfo),
		ctx:           ctx,
		cancel:        cancel,
		eventChan:     make(chan HostEvent, 1000),
		subscribers:   make(map[chan HostEvent]bool),
		failedHosts:   make(map[string]int),
		maxRetries:    3,
		retryInterval: 5 * time.Second,
		enableHealth:  cfg.HostRegistry.HealthCheck,
	}

	// 获取本地主机信息
	registry.localHost = registry.getLocalHostInfo()
	
	// 注册本地主机
	if registry.localHost != nil {
		registry.RegisterHost(registry.localHost)
	}

	return registry
}

// Start 启动主机注册管理器
func (hr *HostRegistry) Start() error {
	if !hr.config.HostRegistry.Enable {
		return nil
	}

	log.Printf("启动主机注册管理器...")

	// 启动更新定时器
	hr.updateTicker = time.NewTicker(hr.config.HostRegistry.UpdateInterval)
	go hr.updateLoop()

	// 启动健康检查定时器
	if hr.enableHealth {
		hr.healthTicker = time.NewTicker(30 * time.Second)
		go hr.healthCheckLoop()
	}

	// 启动重试定时器
	hr.retryTicker = time.NewTicker(hr.retryInterval)
	go hr.retryLoop()

	// 启动事件处理器
	go hr.eventHandler()

	log.Printf("主机注册管理器启动成功")
	return nil
}

// Stop 停止主机注册管理器
func (hr *HostRegistry) Stop() {
	log.Printf("正在停止主机注册管理器...")

	hr.cancel()

	if hr.updateTicker != nil {
		hr.updateTicker.Stop()
	}

	if hr.healthTicker != nil {
		hr.healthTicker.Stop()
	}

	if hr.retryTicker != nil {
		hr.retryTicker.Stop()
	}

	close(hr.eventChan)

	log.Printf("主机注册管理器已停止")
}

// RegisterHost 注册主机
func (hr *HostRegistry) RegisterHost(host *HostInfo) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if host == nil {
		return fmt.Errorf("主机信息不能为空")
	}

	// 检查主机数量限制
	if len(hr.hosts) >= hr.config.HostRegistry.MaxHosts {
		return fmt.Errorf("主机数量超过限制: %d", hr.config.HostRegistry.MaxHosts)
	}

	// 验证主机信息
	if err := hr.validateHostInfo(host); err != nil {
		return err
	}

	existing, exists := hr.hosts[host.ID]
	if exists {
		// 更新现有主机信息
		hr.updateHostInfo(existing, host)
		host.Status = "online"
		host.LastSeen = time.Now()
	} else {
		// 添加新主机
		host.Status = "online"
		host.LastSeen = time.Now()
		hr.hosts[host.ID] = host
	}

	// 发送事件
	event := HostEvent{
		Type:      "add",
		HostInfo:  host,
		Timestamp: time.Now(),
	}
	hr.eventChan <- event

	log.Printf("主机注册成功: %s (%s)", host.ID, host.IP)
	return nil
}

// UnregisterHost 注销主机
func (hr *HostRegistry) UnregisterHost(hostID string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	host, exists := hr.hosts[hostID]
	if !exists {
		return fmt.Errorf("主机不存在: %s", hostID)
	}

	delete(hr.hosts, hostID)

	// 发送事件
	event := HostEvent{
		Type:      "remove",
		HostInfo:  host,
		Timestamp: time.Now(),
	}
	hr.eventChan <- event

	log.Printf("主机注销成功: %s", hostID)
	return nil
}

// UpdateHostStatus 更新主机状态
func (hr *HostRegistry) UpdateHostStatus(hostID string, status string, errorMsg string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	host, exists := hr.hosts[hostID]
	if !exists {
		return fmt.Errorf("主机不存在: %s", hostID)
	}

	oldStatus := host.Status
	host.Status = status
	host.LastSeen = time.Now()

	if errorMsg != "" {
		if host.Metrics == nil {
			host.Metrics = make(map[string]string)
		}
		host.Metrics["error"] = errorMsg
	}

	// 发送事件
	event := HostEvent{
		Type:      "update",
		HostInfo:  host,
		Timestamp: time.Now(),
	}
	if oldStatus != status {
		event.Type = "status_change"
	}
	hr.eventChan <- event

	return nil
}

// GetHost 获取主机信息
func (hr *HostRegistry) GetHost(hostID string) (*HostInfo, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	host, exists := hr.hosts[hostID]
	if !exists {
		return nil, fmt.Errorf("主机不存在: %s", hostID)
	}

	// 返回副本
	hostCopy := *host
	return &hostCopy, nil
}

// GetAllHosts 获取所有主机信息
func (hr *HostRegistry) GetAllHosts() []*HostInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	hosts := make([]*HostInfo, 0, len(hr.hosts))
	for _, host := range hr.hosts {
		hostCopy := *host
		hosts = append(hosts, &hostCopy)
	}

	return hosts
}

// GetOnlineHosts 获取在线主机
func (hr *HostRegistry) GetOnlineHosts() []*HostInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	var hosts []*HostInfo
	for _, host := range hr.hosts {
		if host.Status == "online" {
			hostCopy := *host
			hosts = append(hosts, &hostCopy)
		}
	}

	return hosts
}

// GetHostsByGroup 按组获取主机
func (hr *HostRegistry) GetHostsByGroup(group string) []*HostInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	var hosts []*HostInfo
	for _, host := range hr.hosts {
		if host.Group == group {
			hostCopy := *host
			hosts = append(hosts, &hostCopy)
		}
	}

	return hosts
}

// GetHostsByTag 按标签获取主机
func (hr *HostRegistry) GetHostsByTag(tagKey, tagValue string) []*HostInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	var hosts []*HostInfo
	for _, host := range hr.hosts {
		if value, exists := host.Tags[tagKey]; exists && value == tagValue {
			hostCopy := *host
			hosts = append(hosts, &hostCopy)
		}
	}

	return hosts
}

// Subscribe 订阅主机事件
func (hr *HostRegistry) Subscribe() chan HostEvent {
	hr.subscribersMu.Lock()
	defer hr.subscribersMu.Unlock()

	ch := make(chan HostEvent, 100)
	hr.subscribers[ch] = true
	return ch
}

// Unsubscribe 取消订阅
func (hr *HostRegistry) Unsubscribe(ch chan HostEvent) {
	hr.subscribersMu.Lock()
	defer hr.subscribersMu.Unlock()

	delete(hr.subscribers, ch)
	close(ch)
}

// GetLocalHost 获取本地主机信息
func (hr *HostRegistry) GetLocalHost() *HostInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if hr.localHost == nil {
		return nil
	}

	hostCopy := *hr.localHost
	return &hostCopy
}

// GetStats 获取统计信息
func (hr *HostRegistry) GetStats() map[string]interface{} {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_hosts"] = len(hr.hosts)
	
	onlineCount := 0
	offlineCount := 0
	errorCount := 0
	
	for _, host := range hr.hosts {
		switch host.Status {
		case "online":
			onlineCount++
		case "offline":
			offlineCount++
		case "error":
			errorCount++
		}
	}
	
	stats["online_hosts"] = onlineCount
	stats["offline_hosts"] = offlineCount
	stats["error_hosts"] = errorCount
	stats["subscribers"] = len(hr.subscribers)
	stats["failed_hosts"] = len(hr.failedHosts)
	
	return stats
}

// updateLoop 更新循环
func (hr *HostRegistry) updateLoop() {
	for {
		select {
		case <-hr.ctx.Done():
			return
		case <-hr.updateTicker.C:
			hr.updateHostList()
		}
	}
}

// healthCheckLoop 健康检查循环
func (hr *HostRegistry) healthCheckLoop() {
	for {
		select {
		case <-hr.ctx.Done():
			return
		case <-hr.healthTicker.C:
			hr.performHealthChecks()
		}
	}
}

// retryLoop 重试循环
func (hr *HostRegistry) retryLoop() {
	for {
		select {
		case <-hr.ctx.Done():
			return
		case <-hr.retryTicker.C:
			hr.retryFailedHosts()
		}
	}
}

// eventHandler 事件处理器
func (hr *HostRegistry) eventHandler() {
	for event := range hr.eventChan {
		hr.subscribersMu.RLock()
		for ch := range hr.subscribers {
			select {
			case ch <- event:
			default:
				log.Printf("警告: 事件通道满，丢弃事件")
			}
		}
		hr.subscribersMu.RUnlock()
	}
}

// updateHostList 更新主机列表
func (hr *HostRegistry) updateHostList() {
	// 这里可以实现从配置文件、数据库或其他服务动态获取主机列表
	// 目前简化处理，只更新本地主机信息
	if hr.localHost != nil {
		hr.localHost.Uptime = time.Now().Unix() - hr.localHost.LastSeen.Unix()
		hr.UpdateHostStatus(hr.localHost.ID, "online", "")
	}
}

// performHealthChecks 执行健康检查
func (hr *HostRegistry) performHealthChecks() {
	hr.mu.RLock()
	hosts := make([]*HostInfo, 0, len(hr.hosts))
	for _, host := range hr.hosts {
		hostCopy := *host
		hosts = append(hosts, &hostCopy)
	}
	hr.mu.RUnlock()

	for _, host := range hosts {
		if host.ID == hr.localHost.ID {
			continue // 跳过本地主机
		}

		// 简单的健康检查：尝试连接主机端口
		if hr.checkHostHealth(host) {
			hr.UpdateHostStatus(host.ID, "online", "")
			// 清除失败计数
			delete(hr.failedHosts, host.ID)
		} else {
			hr.failedHosts[host.ID]++
			if hr.failedHosts[host.ID] >= hr.maxRetries {
				hr.UpdateHostStatus(host.ID, "offline", "健康检查失败")
			}
		}
	}
}

// retryFailedHosts 重试失败的主机
func (hr *HostRegistry) retryFailedHosts() {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	for hostID, retryCount := range hr.failedHosts {
		if retryCount >= hr.maxRetries {
			continue // 超过最大重试次数
		}

		host, exists := hr.hosts[hostID]
		if !exists {
			delete(hr.failedHosts, hostID)
			continue
		}

		// 尝试重新连接
		if hr.checkHostHealth(host) {
			host.Status = "online"
			host.LastSeen = time.Now()
			delete(hr.failedHosts, hostID)
			
			// 发送事件
			event := HostEvent{
				Type:      "update",
				HostInfo:  host,
				Timestamp: time.Now(),
			}
			hr.eventChan <- event
		}
	}
}

// checkHostHealth 检查主机健康状态
func (hr *HostRegistry) checkHostHealth(host *HostInfo) bool {
	if host.Port <= 0 {
		return false
	}

	address := fmt.Sprintf("%s:%d", host.IP, host.Port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// validateHostInfo 验证主机信息
func (hr *HostRegistry) validateHostInfo(host *HostInfo) error {
	if host.ID == "" {
		return fmt.Errorf("主机ID不能为空")
	}
	if host.IP == "" {
		return fmt.Errorf("主机IP不能为空")
	}
	if net.ParseIP(host.IP) == nil {
		return fmt.Errorf("无效的IP地址: %s", host.IP)
	}
	return nil
}

// updateHostInfo 更新主机信息
func (hr *HostRegistry) updateHostInfo(existing, new *HostInfo) {
	if new.Hostname != "" {
		existing.Hostname = new.Hostname
	}
	if new.IP != "" {
		existing.IP = new.IP
	}
	if new.Port > 0 {
		existing.Port = new.Port
	}
	if new.CPUCount > 0 {
		existing.CPUCount = new.CPUCount
	}
	if new.MemoryMB > 0 {
		existing.MemoryMB = new.MemoryMB
	}
	if new.DiskGB > 0 {
		existing.DiskGB = new.DiskGB
	}
	if new.OS != "" {
		existing.OS = new.OS
	}
	if new.Arch != "" {
		existing.Arch = new.Arch
	}
	if new.Version != "" {
		existing.Version = new.Version
	}
	if new.Location != "" {
		existing.Location = new.Location
	}
	if new.Group != "" {
		existing.Group = new.Group
	}
	if new.Priority > 0 {
		existing.Priority = new.Priority
	}
	
	// 合并标签
	if new.Tags != nil {
		if existing.Tags == nil {
			existing.Tags = make(map[string]string)
		}
		for k, v := range new.Tags {
			existing.Tags[k] = v
		}
	}
	
	// 合并指标
	if new.Metrics != nil {
		if existing.Metrics == nil {
			existing.Metrics = make(map[string]string)
		}
		for k, v := range new.Metrics {
			existing.Metrics[k] = v
		}
	}
}

// getLocalHostInfo 获取本地主机信息
func (hr *HostRegistry) getLocalHostInfo() *HostInfo {
	hostID := hr.config.Agent.HostID
	if hostID == "" {
		hostname, _ := os.Hostname()
		hostID = hostname
		if hostID == "" {
			hostID = "localhost"
		}
	}

	hostname := hr.config.Agent.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	ip := hr.config.Agent.IP
	if ip == "" {
		ip = "127.0.0.1"
	}

	return &HostInfo{
		ID:       hostID,
		Hostname: hostname,
		IP:       ip,
		Port:     hr.config.Agent.Port,
		Status:   "online",
		LastSeen: time.Now(),
		Metrics:  make(map[string]string),
		Tags:     make(map[string]string),
		Version:  "1.0.0",
		Group:    "default",
		Priority: 1,
	}
}

// ExportHosts 导出主机列表
func (hr *HostRegistry) ExportHosts() ([]byte, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	hosts := make([]*HostInfo, 0, len(hr.hosts))
	for _, host := range hr.hosts {
		hosts = append(hosts, host)
	}

	return json.MarshalIndent(hosts, "", "  ")
}

// ImportHosts 导入主机列表
func (hr *HostRegistry) ImportHosts(data []byte) error {
	var hosts []*HostInfo
	if err := json.Unmarshal(data, &hosts); err != nil {
		return err
	}

	for _, host := range hosts {
		if err := hr.RegisterHost(host); err != nil {
			log.Printf("导入主机失败 %s: %v", host.ID, err)
		}
	}

	return nil
}