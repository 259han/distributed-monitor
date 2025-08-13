package host

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// HostState 主机状态
type HostState int

const (
	HostStateUnknown HostState = iota
	HostStateHealthy
	HostStateUnhealthy
	HostStateOffline
	HostStateMaintenance
)

// HostInfo 主机信息
type HostInfo struct {
	ID            string                 `json:"id"`
	Hostname      string                 `json:"hostname"`
	IPAddress     string                 `json:"ip_address"`
	Port          int                    `json:"port"`
	State         HostState              `json:"state"`
	Capacity      HostCapacity           `json:"capacity"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	LastCheck     time.Time              `json:"last_check"`
	Metadata      map[string]interface{} `json:"metadata"`
	Tags          []string               `json:"tags"`
	Load          HostLoad               `json:"load"`
}

// HostCapacity 主机容量信息
type HostCapacity struct {
	CPU       int `json:"cpu_cores"`
	MemoryMB  int `json:"memory_mb"`
	DiskGB    int `json:"disk_gb"`
	NetworkMB int `json:"network_mbps"`
}

// HostLoad 主机负载信息
type HostLoad struct {
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
	DiskUsage    float64 `json:"disk_usage"`
	NetworkUsage float64 `json:"network_usage"`
	Connections  int     `json:"connections"`
}

// HostRegistry 主机注册表
type HostRegistry struct {
	hosts    map[string]*HostInfo
	mu       sync.RWMutex
	config   *RegistryConfig
	stopChan chan struct{}
}

// RegistryConfig 注册表配置
type RegistryConfig struct {
	HeartbeatInterval time.Duration
	HealthCheckInterval time.Duration
	OfflineTimeout     time.Duration
	CleanupInterval    time.Duration
}

// NewHostRegistry 创建主机注册表
func NewHostRegistry(config *RegistryConfig) *HostRegistry {
	return &HostRegistry{
		hosts:    make(map[string]*HostInfo),
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// RegisterHost 注册主机
func (r *HostRegistry) RegisterHost(host *HostInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, exists := r.hosts[host.ID]; exists {
		existing.Hostname = host.Hostname
		existing.IPAddress = host.IPAddress
		existing.Port = host.Port
		existing.Capacity = host.Capacity
		existing.Metadata = host.Metadata
		existing.Tags = host.Tags
		existing.LastHeartbeat = time.Now()
		existing.State = HostStateHealthy
		log.Printf("主机 %s 已更新注册信息", host.ID)
	} else {
		host.LastHeartbeat = time.Now()
		host.State = HostStateHealthy
		r.hosts[host.ID] = host
		log.Printf("主机 %s 已注册", host.ID)
	}

	return nil
}

// UnregisterHost 注销主机
func (r *HostRegistry) UnregisterHost(hostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.hosts[hostID]; !exists {
		return fmt.Errorf("主机 %s 不存在", hostID)
	}

	delete(r.hosts, hostID)
	log.Printf("主机 %s 已注销", hostID)
	return nil
}

// GetHost 获取主机信息
func (r *HostRegistry) GetHost(hostID string) (*HostInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	host, exists := r.hosts[hostID]
	if !exists {
		return nil, fmt.Errorf("主机 %s 不存在", hostID)
	}
	return host, nil
}

// GetAllHosts 获取所有主机
func (r *HostRegistry) GetAllHosts() []*HostInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hosts := make([]*HostInfo, 0, len(r.hosts))
	for _, host := range r.hosts {
		hosts = append(hosts, host)
	}
	return hosts
}

// GetHealthyHosts 获取健康主机
func (r *HostRegistry) GetHealthyHosts() []*HostInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hosts := make([]*HostInfo, 0)
	for _, host := range r.hosts {
		if host.State == HostStateHealthy {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// UpdateHeartbeat 更新心跳
func (r *HostRegistry) UpdateHeartbeat(hostID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	host, exists := r.hosts[hostID]
	if !exists {
		return fmt.Errorf("主机 %s 不存在", hostID)
	}

	host.LastHeartbeat = time.Now()
	if host.State == HostStateOffline {
		host.State = HostStateHealthy
		log.Printf("主机 %s 恢复健康状态", hostID)
	}

	return nil
}

// UpdateHostState 更新主机状态
func (r *HostRegistry) UpdateHostState(hostID string, state HostState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	host, exists := r.hosts[hostID]
	if !exists {
		return fmt.Errorf("主机 %s 不存在", hostID)
	}

	oldState := host.State
	host.State = state
	host.LastCheck = time.Now()

	if oldState != state {
		log.Printf("主机 %s 状态从 %v 变更为 %v", hostID, oldState, state)
	}

	return nil
}

// UpdateHostLoad 更新主机负载
func (r *HostRegistry) UpdateHostLoad(hostID string, load HostLoad) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	host, exists := r.hosts[hostID]
	if !exists {
		return fmt.Errorf("主机 %s 不存在", hostID)
	}

	host.Load = load
	host.LastCheck = time.Now()
	return nil
}

// GetHostCount 获取主机数量
func (r *HostRegistry) GetHostCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hosts)
}

// GetHostCountByState 根据状态获取主机数量
func (r *HostRegistry) GetHostCountByState(state HostState) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, host := range r.hosts {
		if host.State == state {
			count++
		}
	}
	return count
}

// Start 启动注册表服务
func (r *HostRegistry) Start(ctx context.Context) {
	go r.healthChecker(ctx)
	go r.cleanup(ctx)
	log.Println("主机注册表服务已启动")
}

// Stop 停止注册表服务
func (r *HostRegistry) Stop() {
	close(r.stopChan)
	log.Println("主机注册表服务已停止")
}

// healthChecker 健康检查器
func (r *HostRegistry) healthChecker(ctx context.Context) {
	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.checkHostsHealth()
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		}
	}
}

// checkHostsHealth 检查主机健康状态
func (r *HostRegistry) checkHostsHealth() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, host := range r.hosts {
		if now.Sub(host.LastHeartbeat) > r.config.OfflineTimeout {
			if host.State != HostStateOffline {
				host.State = HostStateOffline
				log.Printf("主机 %s 超时，标记为离线", host.ID)
			}
		}
	}
}

// cleanup 清理器
func (r *HostRegistry) cleanup(ctx context.Context) {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupStaleHosts()
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		}
	}
}

// cleanupStaleHosts 清理过期主机
func (r *HostRegistry) cleanupStaleHosts() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for id, host := range r.hosts {
		if now.Sub(host.LastHeartbeat) > r.config.OfflineTimeout*2 {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(r.hosts, id)
		log.Printf("清理过期主机: %s", id)
	}
}

// ToJSON 转换为JSON
func (r *HostRegistry) ToJSON() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data := struct {
		Hosts []*HostInfo `json:"hosts"`
		Count int         `json:"count"`
	}{
		Hosts: r.GetAllHosts(),
		Count: len(r.hosts),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}