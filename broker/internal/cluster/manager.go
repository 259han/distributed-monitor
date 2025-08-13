package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/han-fei/monitor/broker/internal/host"
	"github.com/han-fei/monitor/broker/internal/raft"
	"github.com/han-fei/monitor/broker/internal/storage"
)

// ClusterManager 集群管理器
type ClusterManager struct {
	config      *Config
	raftServer  *raft.Server
	hostManager *host.HostManager
	storage     *storage.RedisStorage
	httpServer  *http.Server
	metrics     *ClusterMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// Config 集群管理器配置
type Config struct {
	HTTPPort        int                   `json:"http_port"`
	RaftServer      *raft.Server          `json:"raft_server"`
	HostManager     *host.HostManager     `json:"host_manager"`
	Storage         *storage.RedisStorage `json:"storage"`
	MetricsInterval time.Duration         `json:"metrics_interval"`
}

// ClusterMetrics 集群指标
type ClusterMetrics struct {
	Nodes       map[string]*NodeMetrics `json:"nodes"`
	TotalNodes  int                     `json:"total_nodes"`
	ActiveNodes int                     `json:"active_nodes"`
	LeaderNode  string                  `json:"leader_node"`
	Term        uint64                  `json:"term"`
	LastUpdated time.Time               `json:"last_updated"`
	Throughput  *ThroughputMetrics      `json:"throughput"`
}

// NodeMetrics 节点指标
type NodeMetrics struct {
	ID            string           `json:"id"`
	Address       string           `json:"address"`
	Status        string           `json:"status"`
	LastHeartbeat time.Time        `json:"last_heartbeat"`
	Resources     *ResourceMetrics `json:"resources,omitempty"`
	RaftState     string           `json:"raft_state"`
}

// ResourceMetrics 资源指标
type ResourceMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIn   float64 `json:"network_in"`
	NetworkOut  float64 `json:"network_out"`
}

// ThroughputMetrics 吞吐量指标
type ThroughputMetrics struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	DataRateMB        float64 `json:"data_rate_mb"`
	AvgLatency        float64 `json:"avg_latency_ms"`
	SuccessRate       float64 `json:"success_rate"`
}

// NewClusterManager 创建集群管理器
func NewClusterManager(config *Config) *ClusterManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ClusterManager{
		config:      config,
		raftServer:  config.RaftServer,
		hostManager: config.HostManager,
		storage:     config.Storage,
		metrics: &ClusterMetrics{
			Nodes:      make(map[string]*NodeMetrics),
			Throughput: &ThroughputMetrics{},
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动集群管理器
func (cm *ClusterManager) Start() error {
	// 启动HTTP服务器
	if err := cm.startHTTPServer(); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %v", err)
	}

	// 延迟启动指标收集器，等待Raft服务器完全启动
	go func() {
		// 等待5秒让Raft服务器完全启动
		time.Sleep(5 * time.Second)
		cm.collectMetrics()
	}()

	log.Println("集群管理器启动成功")
	return nil
}

// Stop 停止集群管理器
func (cm *ClusterManager) Stop() {
	cm.cancel()

	if cm.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cm.httpServer.Shutdown(ctx)
	}

	log.Println("集群管理器已停止")
}

// startHTTPServer 启动HTTP服务器
func (cm *ClusterManager) startHTTPServer() error {
	router := mux.NewRouter()

	// API路由
	router.HandleFunc("/api/v1/cluster/status", cm.handleClusterStatus).Methods("GET")
	router.HandleFunc("/api/v1/cluster/nodes", cm.handleClusterNodes).Methods("GET")
	router.HandleFunc("/api/v1/cluster/nodes/{nodeId}", cm.handleNodeDetail).Methods("GET")
	router.HandleFunc("/api/v1/cluster/metrics", cm.handleClusterMetrics).Methods("GET")
	router.HandleFunc("/api/v1/cluster/health", cm.handleClusterHealth).Methods("GET")
	router.HandleFunc("/api/v1/cluster/config", cm.handleClusterConfig).Methods("GET", "POST")

	// 健康检查
	router.HandleFunc("/health", cm.handleHealthCheck).Methods("GET")

	cm.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cm.config.HTTPPort),
		Handler: router,
	}

	go func() {
		log.Printf("集群管理API服务器启动，监听端口: %d", cm.config.HTTPPort)
		if err := cm.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP服务器错误: %v", err)
		}
	}()

	return nil
}

// collectMetrics 收集集群指标
func (cm *ClusterManager) collectMetrics() {
	ticker := time.NewTicker(cm.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cm.updateMetrics()
		case <-cm.ctx.Done():
			return
		}
	}
}

// updateMetrics 更新集群指标
func (cm *ClusterManager) updateMetrics() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查Raft服务器是否可用
	if cm.raftServer == nil {
		log.Printf("警告: Raft服务器未初始化")
		return
	}

	// 更新基本信息 - 添加额外的空指针检查
	leader := cm.raftServer.Leader()
	if leader != "" {
		cm.metrics.LeaderNode = leader
	}

	term := cm.raftServer.Term()
	if term > 0 {
		cm.metrics.Term = term
	}

	cm.metrics.LastUpdated = time.Now()

	// 更新节点信息
	if cm.hostManager != nil {
		hostStats := cm.hostManager.GetStats()

		// 从统计信息中提取节点数量
		if total, ok := hostStats["total_hosts"].(float64); ok {
			cm.metrics.TotalNodes = int(total)
		}
		if active, ok := hostStats["active_hosts"].(float64); ok {
			cm.metrics.ActiveNodes = int(active)
		}

		// 更新节点指标
		if hosts, ok := hostStats["hosts"].([]interface{}); ok {
			for _, hostInfo := range hosts {
				if hostMap, ok := hostInfo.(map[string]interface{}); ok {
					nodeMetrics := &NodeMetrics{
						ID:            fmt.Sprintf("%v", hostMap["id"]),
						Address:       fmt.Sprintf("%v", hostMap["address"]),
						Status:        fmt.Sprintf("%v", hostMap["status"]),
						LastHeartbeat: time.Now(), // 简化处理
						RaftState:     cm.getRaftNodeState(fmt.Sprintf("%v", hostMap["id"])),
					}

					// 如果有资源信息，添加资源指标
					if resources, ok := hostMap["resources"].(map[string]interface{}); ok {
						if cpu, ok := resources["cpu"].(float64); ok {
							if memory, ok := resources["memory"].(float64); ok {
								if disk, ok := resources["disk"].(float64); ok {
									nodeMetrics.Resources = &ResourceMetrics{
										CPUUsage:    cpu,
										MemoryUsage: memory,
										DiskUsage:   disk,
									}
								}
							}
						}
					}

					cm.metrics.Nodes[nodeMetrics.ID] = nodeMetrics
				}
			}
		}
	}

	// 更新吞吐量指标
	cm.updateThroughputMetrics()
}

// getRaftNodeState 获取Raft节点状态
func (cm *ClusterManager) getRaftNodeState(nodeID string) string {
	if cm.raftServer == nil {
		return "unknown"
	}

	if cm.raftServer.Leader() == nodeID {
		return "leader"
	}

	// 这里可以通过Raft API获取更详细的状态
	return "follower"
}

// updateThroughputMetrics 更新吞吐量指标
func (cm *ClusterManager) updateThroughputMetrics() {
	// 简化的吞吐量计算
	// 实际应用中应该从Redis或其他存储系统获取真实的指标
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if cm.storage != nil {
		_, err := cm.storage.GetStats(ctx)
		if err == nil {
			// 从Redis统计信息中计算吞吐量
			cm.metrics.Throughput.RequestsPerSecond = 100.0 // 示例值
			cm.metrics.Throughput.DataRateMB = 1.5          // 示例值
			cm.metrics.Throughput.AvgLatency = 10.0         // 示例值
			cm.metrics.Throughput.SuccessRate = 99.5        // 示例值
		}
	}
}

// HTTP处理函数
func (cm *ClusterManager) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	status := map[string]interface{}{
		"cluster_id":   "monitor-cluster",
		"leader":       cm.metrics.LeaderNode,
		"term":         cm.metrics.Term,
		"total_nodes":  cm.metrics.TotalNodes,
		"active_nodes": cm.metrics.ActiveNodes,
		"status":       "healthy",
		"last_updated": cm.metrics.LastUpdated,
	}

	cm.writeJSONResponse(w, http.StatusOK, status)
}

func (cm *ClusterManager) handleClusterNodes(w http.ResponseWriter, r *http.Request) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	nodes := make([]map[string]interface{}, 0)
	for _, node := range cm.metrics.Nodes {
		nodeData := map[string]interface{}{
			"id":             node.ID,
			"address":        node.Address,
			"status":         node.Status,
			"raft_state":     node.RaftState,
			"last_heartbeat": node.LastHeartbeat,
		}
		if node.Resources != nil {
			nodeData["resources"] = node.Resources
		}
		nodes = append(nodes, nodeData)
	}

	response := map[string]interface{}{
		"nodes":        nodes,
		"total_count":  len(nodes),
		"active_count": cm.metrics.ActiveNodes,
	}

	cm.writeJSONResponse(w, http.StatusOK, response)
}

func (cm *ClusterManager) handleNodeDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeId"]

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.metrics.Nodes[nodeID]
	if !exists {
		cm.writeJSONResponse(w, http.StatusNotFound, map[string]string{"error": "节点不存在"})
		return
	}

	cm.writeJSONResponse(w, http.StatusOK, node)
}

func (cm *ClusterManager) handleClusterMetrics(w http.ResponseWriter, r *http.Request) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cm.writeJSONResponse(w, http.StatusOK, cm.metrics)
}

func (cm *ClusterManager) handleClusterHealth(w http.ResponseWriter, r *http.Request) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 计算健康状态
	healthStatus := "healthy"
	if cm.metrics.ActiveNodes < cm.metrics.TotalNodes/2 {
		healthStatus = "degraded"
	}
	if cm.metrics.ActiveNodes == 0 {
		healthStatus = "unhealthy"
	}

	health := map[string]interface{}{
		"status":        healthStatus,
		"total_nodes":   cm.metrics.TotalNodes,
		"active_nodes":  cm.metrics.ActiveNodes,
		"leader_exists": cm.metrics.LeaderNode != "",
		"checks": map[string]interface{}{
			"raft_consensus": cm.metrics.LeaderNode != "",
			"node_health":    cm.metrics.ActiveNodes > 0,
			"storage_health": true, // 简化处理
		},
	}

	cm.writeJSONResponse(w, http.StatusOK, health)
}

func (cm *ClusterManager) handleClusterConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		config := map[string]interface{}{
			"cluster_id":              "monitor-cluster",
			"http_port":               cm.config.HTTPPort,
			"raft_enabled":            cm.raftServer != nil,
			"host_management_enabled": cm.hostManager != nil,
			"storage_enabled":         cm.storage != nil,
		}
		cm.writeJSONResponse(w, http.StatusOK, config)

	case "POST":
		var config map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			cm.writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "无效的配置"})
			return
		}

		// 这里可以处理配置更新逻辑
		cm.writeJSONResponse(w, http.StatusOK, map[string]string{"message": "配置更新成功"})
	}
}

func (cm *ClusterManager) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	cm.writeJSONResponse(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "cluster-manager",
	})
}

// writeJSONResponse 写入JSON响应
func (cm *ClusterManager) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("写入JSON响应失败: %v", err)
	}
}

// GetClusterMetrics 获取集群指标
func (cm *ClusterManager) GetClusterMetrics() *ClusterMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回副本
	metrics := *cm.metrics
	metrics.Nodes = make(map[string]*NodeMetrics)
	for k, v := range cm.metrics.Nodes {
		nodeCopy := *v
		metrics.Nodes[k] = &nodeCopy
	}

	return &metrics
}

// GetNodeMetrics 获取节点指标
func (cm *ClusterManager) GetNodeMetrics(nodeID string) (*NodeMetrics, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	node, exists := cm.metrics.Nodes[nodeID]
	if !exists {
		return nil, false
	}

	// 返回副本
	nodeCopy := *node
	return &nodeCopy, true
}
