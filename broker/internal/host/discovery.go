package host

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// DiscoveryService 主机发现服务
type DiscoveryService struct {
	registry      *HostRegistry
	config        *DiscoveryConfig
	server        *http.Server
	mu            sync.RWMutex
	activeScans   map[string]time.Time
	serviceTypes  map[string]bool
}

// DiscoveryConfig 发现服务配置
type DiscoveryConfig struct {
	ListenPort       int
	ScanInterval     time.Duration
	ScanTimeout      time.Duration
	EnableMulticast  bool
	EnableHTTP       bool
	EnableFileWatch  bool
	NetworkRanges    []string
	DiscoveryTypes   []string
	ServicePortRange string
}

// HostDiscoveryRequest 主机发现请求
type HostDiscoveryRequest struct {
	NetworkRange string   `json:"network_range"`
	PortRange    string   `json:"port_range"`
	Timeout      int      `json:"timeout"`
	Tags         []string `json:"tags"`
}

// HostDiscoveryResponse 主机发现响应
type HostDiscoveryResponse struct {
	Success      bool         `json:"success"`
	Discovered   []*HostInfo  `json:"discovered"`
	Failed       []string     `json:"failed"`
	TotalScanned int          `json:"total_scanned"`
	Message      string       `json:"message"`
}

// NewDiscoveryService 创建主机发现服务
func NewDiscoveryService(registry *HostRegistry, config *DiscoveryConfig) *DiscoveryService {
	return &DiscoveryService{
		registry:     registry,
		config:       config,
		activeScans:  make(map[string]time.Time),
		serviceTypes: make(map[string]bool),
	}
}

// Start 启动发现服务
func (s *DiscoveryService) Start(ctx context.Context) error {
	// 初始化服务类型
	s.serviceTypes["http"] = true
	s.serviceTypes["grpc"] = true
	s.serviceTypes["tcp"] = true

	// 启动HTTP服务器
	if s.config.EnableHTTP {
		go s.startHTTPServer()
	}

	// 启动定期扫描
	go s.periodicScan(ctx)

	// 启动多播发现
	if s.config.EnableMulticast {
		go s.multicastDiscovery(ctx)
	}

	// 启动文件监控
	if s.config.EnableFileWatch {
		go s.fileWatchDiscovery(ctx)
	}

	log.Println("主机发现服务已启动")
	return nil
}

// Stop 停止发现服务
func (s *DiscoveryService) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// startHTTPServer 启动HTTP服务器
func (s *DiscoveryService) startHTTPServer() {
	router := mux.NewRouter()

	// 注册路由
	router.HandleFunc("/api/discovery/register", s.handleRegister).Methods("POST")
	router.HandleFunc("/api/discovery/unregister", s.handleUnregister).Methods("POST")
	router.HandleFunc("/api/discovery/heartbeat", s.handleHeartbeat).Methods("POST")
	router.HandleFunc("/api/discovery/scan", s.handleScan).Methods("POST")
	router.HandleFunc("/api/discovery/hosts", s.handleGetHosts).Methods("GET")
	router.HandleFunc("/api/discovery/stats", s.handleGetStats).Methods("GET")

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.ListenPort),
		Handler: router,
	}

	log.Printf("主机发现HTTP服务器启动，监听端口: %d", s.config.ListenPort)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("HTTP服务器启动失败: %v", err)
	}
}

// periodicScan 定期扫描
func (s *DiscoveryService) periodicScan(ctx context.Context) {
	ticker := time.NewTicker(s.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.scanNetworks()
		case <-ctx.Done():
			return
		}
	}
}

// scanNetworks 扫描网络
func (s *DiscoveryService) scanNetworks() {
	for _, networkRange := range s.config.NetworkRanges {
		go s.scanNetwork(networkRange)
	}
}

// scanNetwork 扫描指定网络
func (s *DiscoveryService) scanNetwork(networkRange string) {
	s.mu.Lock()
	if _, exists := s.activeScans[networkRange]; exists {
		s.mu.Unlock()
		return
	}
	s.activeScans[networkRange] = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeScans, networkRange)
		s.mu.Unlock()
	}()

	log.Printf("开始扫描网络: %s", networkRange)
	
	ip, ipnet, err := net.ParseCIDR(networkRange)
	if err != nil {
		log.Printf("解析网络范围失败: %v", err)
		return
	}

	var discovered []*HostInfo
	var failed []string

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ipStr := ip.String()
		if s.scanHost(ipStr) {
			hostInfo := s.createHostInfo(ipStr)
			discovered = append(discovered, hostInfo)
		} else {
			failed = append(failed, ipStr)
		}
	}

	log.Printf("网络扫描完成: %s, 发现 %d 个主机", networkRange, len(discovered))

	// 注册发现的主机
	for _, host := range discovered {
		if err := s.registry.RegisterHost(host); err != nil {
			log.Printf("注册主机失败: %v", err)
		}
	}
}

// scanHost 扫描单个主机
func (s *DiscoveryService) scanHost(ip string) bool {
	// 检查常见端口
	commonPorts := []int{22, 80, 443, 9093, 9095, 6379, 3306, 5432}

	for _, port := range commonPorts {
		address := fmt.Sprintf("%s:%d", ip, port)
		timeout := time.Duration(s.config.ScanTimeout) * time.Second
		
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// createHostInfo 创建主机信息
func (s *DiscoveryService) createHostInfo(ip string) *HostInfo {
	return &HostInfo{
		ID:        fmt.Sprintf("host-%s", strings.ReplaceAll(ip, ".", "-")),
		Hostname:  s.resolveHostname(ip),
		IPAddress: ip,
		Port:      9093, // 默认端口
		State:     HostStateHealthy,
		Capacity: HostCapacity{
			CPU:       4,
			MemoryMB:  8192,
			DiskGB:    100,
			NetworkMB: 1000,
		},
		LastHeartbeat: time.Now(),
		LastCheck:     time.Now(),
		Metadata:      make(map[string]interface{}),
		Tags:          []string{"discovered"},
		Load: HostLoad{
			CPUUsage:     0.0,
			MemoryUsage:  0.0,
			DiskUsage:    0.0,
			NetworkUsage: 0.0,
			Connections:  0,
		},
	}
}

// resolveHostname 解析主机名
func (s *DiscoveryService) resolveHostname(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ip
	}
	return strings.TrimSuffix(names[0], ".")
}

// multicastDiscovery 多播发现
func (s *DiscoveryService) multicastDiscovery(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", "239.255.255.250:1900")
	if err != nil {
		log.Printf("解析多播地址失败: %v", err)
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Printf("创建多播连接失败: %v", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, src, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}
			
			message := string(buffer[:n])
			if strings.Contains(message, "monitor-discovery") {
				s.handleDiscoveryMessage(src.IP.String(), message)
			}
		}
	}
}

// handleDiscoveryMessage 处理发现消息
func (s *DiscoveryService) handleDiscoveryMessage(ip, message string) {
	hostInfo := s.createHostInfo(ip)
	hostInfo.Tags = append(hostInfo.Tags, "multicast-discovered")
	
	if err := s.registry.RegisterHost(hostInfo); err != nil {
		log.Printf("注册多播发现的主机失败: %v", err)
	}
}

// fileWatchDiscovery 文件监控发现
func (s *DiscoveryService) fileWatchDiscovery(ctx context.Context) {
	// 监控配置文件变化
	configPath := "/etc/monitor/hosts.conf"
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
			s.checkConfigFile(configPath)
		}
	}
}

// checkConfigFile 检查配置文件
func (s *DiscoveryService) checkConfigFile(configPath string) {
	if _, err := os.Stat(configPath); err != nil {
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	var hosts []string
	if err := json.Unmarshal(data, &hosts); err != nil {
		return
	}

	for _, host := range hosts {
		hostInfo := s.createHostInfo(host)
		hostInfo.Tags = append(hostInfo.Tags, "config-discovered")
		
		if err := s.registry.RegisterHost(hostInfo); err != nil {
			log.Printf("注册配置发现的主机失败: %v", err)
		}
	}
}

// HTTP处理函数
func (s *DiscoveryService) handleRegister(w http.ResponseWriter, r *http.Request) {
	var hostInfo HostInfo
	if err := json.NewDecoder(r.Body).Decode(&hostInfo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.registry.RegisterHost(&hostInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "主机注册成功"})
}

func (s *DiscoveryService) handleUnregister(w http.ResponseWriter, r *http.Request) {
	var request struct {
		HostID string `json:"host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.registry.UnregisterHost(request.HostID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "主机注销成功"})
}

func (s *DiscoveryService) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var request struct {
		HostID string `json:"host_id"`
		Load   HostLoad `json:"load"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.registry.UpdateHeartbeat(request.HostID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.registry.UpdateHostLoad(request.HostID, request.Load); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "心跳更新成功"})
}

func (s *DiscoveryService) handleScan(w http.ResponseWriter, r *http.Request) {
	var request HostDiscoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := s.performNetworkScan(request)
	json.NewEncoder(w).Encode(response)
}

func (s *DiscoveryService) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	hosts := s.registry.GetAllHosts()
	json.NewEncoder(w).Encode(map[string]interface{}{"hosts": hosts})
}

func (s *DiscoveryService) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"total_hosts":        s.registry.GetHostCount(),
		"healthy_hosts":      s.registry.GetHostCountByState(HostStateHealthy),
		"unhealthy_hosts":    s.registry.GetHostCountByState(HostStateUnhealthy),
		"offline_hosts":      s.registry.GetHostCountByState(HostStateOffline),
		"maintenance_hosts":  s.registry.GetHostCountByState(HostStateMaintenance),
	}
	json.NewEncoder(w).Encode(stats)
}

// performNetworkScan 执行网络扫描
func (s *DiscoveryService) performNetworkScan(request HostDiscoveryRequest) *HostDiscoveryResponse {
	response := &HostDiscoveryResponse{
		Success:    false,
		Discovered: make([]*HostInfo, 0),
		Failed:     make([]string, 0),
	}

	// 解析网络范围
	ip, ipnet, err := net.ParseCIDR(request.NetworkRange)
	if err != nil {
		response.Message = fmt.Sprintf("网络范围解析失败: %v", err)
		return response
	}

	timeout := time.Duration(request.Timeout) * time.Second
	if timeout == 0 {
		timeout = s.config.ScanTimeout
	}

	// 扫描网络
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ipStr := ip.String()
		response.TotalScanned++

		if s.scanHostWithTimeout(ipStr, timeout) {
			hostInfo := s.createHostInfo(ipStr)
			hostInfo.Tags = append(hostInfo.Tags, request.Tags...)
			response.Discovered = append(response.Discovered, hostInfo)
			
			// 注册主机
			if err := s.registry.RegisterHost(hostInfo); err != nil {
				log.Printf("注册扫描发现的主机失败: %v", err)
			}
		} else {
			response.Failed = append(response.Failed, ipStr)
		}
	}

	response.Success = true
	response.Message = fmt.Sprintf("扫描完成，发现 %d 个主机", len(response.Discovered))
	return response
}

// scanHostWithTimeout 带超时的主机扫描
func (s *DiscoveryService) scanHostWithTimeout(ip string, timeout time.Duration) bool {
	ch := make(chan bool, 1)
	
	go func() {
		ch <- s.scanHost(ip)
	}()

	select {
	case result := <-ch:
		return result
	case <-time.After(timeout):
		return false
	}
}

// inc IP地址递增
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}