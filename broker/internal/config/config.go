package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 配置结构
type Config struct {
	Node           NodeConfig           `yaml:"node"`
	Cluster        ClusterConfig        `yaml:"cluster"`
	Raft           RaftConfig           `yaml:"raft"`
	Storage        StorageConfig        `yaml:"storage"`
	Hash           HashConfig           `yaml:"hash"`
	GRPC           GRPCConfig           `yaml:"grpc"`
	RaftGRPC       RaftGRPCConfig       `yaml:"raft_grpc"`
	Log            LogConfig            `yaml:"log"`
	Advanced       AdvancedConfig       `yaml:"advanced"`
	HostManagement HostManagementConfig `yaml:"host_management"`
}

// NodeConfig 节点配置
type NodeConfig struct {
	ID      string `yaml:"id"`
	Address string `yaml:"address"`
}

// ClusterConfig 集群配置
type ClusterConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

// RaftConfig Raft配置
type RaftConfig struct {
	LogDir           string        `yaml:"log_dir"`
	SnapshotDir      string        `yaml:"snapshot_dir"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`
	ElectionTimeout  time.Duration `yaml:"election_timeout"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Redis RedisConfig `yaml:"redis"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr      string `yaml:"addr"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}

// HashConfig 哈希配置
type HashConfig struct {
	VirtualNodes int `yaml:"virtual_nodes"`
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Port           int `yaml:"port"`
	MaxRecvMsgSize int `yaml:"max_recv_msg_size"`
	MaxSendMsgSize int `yaml:"max_send_msg_size"`
}

// RaftGRPCConfig Raft gRPC配置
type RaftGRPCConfig struct {
	Port int `yaml:"port"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// AdvancedConfig 高级配置
type AdvancedConfig struct {
	BufferSize int `yaml:"buffer_size"`
}

// HostManagementConfig 主机管理配置
type HostManagementConfig struct {
	Enabled       bool                   `yaml:"enabled"`
	EnableScaling bool                   `yaml:"enable_scaling"`
	MaxHosts      int                    `yaml:"max_hosts"`
	Registry      RegistryConfig         `yaml:"registry"`
	Discovery     DiscoveryConfig        `yaml:"discovery"`
	Health        HealthConfig           `yaml:"health"`
	AutoRemoveCfg AutoRemoveConfig       `yaml:"auto_remove"`
	Scaling       ScalingConfig          `yaml:"scaling"`
}

// RegistryConfig 注册表配置
type RegistryConfig struct {
	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval"`
	OfflineTimeout       time.Duration `yaml:"offline_timeout"`
	CleanupInterval      time.Duration `yaml:"cleanup_interval"`
}

// DiscoveryConfig 发现配置
type DiscoveryConfig struct {
	ListenPort      int           `yaml:"listen_port"`
	ScanInterval    time.Duration `yaml:"scan_interval"`
	ScanTimeout     time.Duration `yaml:"scan_timeout"`
	EnableMulticast bool          `yaml:"enable_multicast"`
	EnableHTTP      bool          `yaml:"enable_http"`
	EnableFileWatch bool          `yaml:"enable_file_watch"`
	NetworkRanges   []string      `yaml:"network_ranges"`
	DiscoveryTypes  []string      `yaml:"discovery_types"`
	ServicePortRange string        `yaml:"service_port_range"`
}

// HealthConfig 健康配置
type HealthConfig struct {
	CheckInterval   time.Duration         `yaml:"check_interval"`
	Timeout         time.Duration         `yaml:"timeout"`
	MaxRetries      int                   `yaml:"max_retries"`
	EnableTCP       bool                  `yaml:"enable_tcp"`
	EnableHTTP      bool                  `yaml:"enable_http"`
	EnableCustom    bool                  `yaml:"enable_custom"`
	TCPPorts        []int                 `yaml:"tcp_ports"`
	HTTPPaths       []string              `yaml:"http_paths"`
	AlertThresholds map[string]float64    `yaml:"alert_thresholds"`
}

// AutoRemoveConfig 自动移除配置
type AutoRemoveConfig struct {
	Enabled           bool          `yaml:"enabled"`
	UnhealthyTimeout  time.Duration `yaml:"unhealthy_timeout"`
	OfflineTimeout    time.Duration `yaml:"offline_timeout"`
}

// ScalingConfig 扩缩容配置
type ScalingConfig struct {
	Enabled            bool `yaml:"enabled"`
	ScaleUpThreshold   int  `yaml:"scale_up_threshold"`
	ScaleDownThreshold int  `yaml:"scale_down_threshold"`
	MinHosts           int  `yaml:"min_hosts"`
	MaxHosts           int  `yaml:"max_hosts"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 设置默认值
	setDefaults(config)

	return config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	// 节点默认值
	if config.Node.ID == "" {
		config.Node.ID = "broker-1"
	}
	if config.Node.Address == "" {
		config.Node.Address = "localhost:9090"
	}

	// 集群默认值
	if len(config.Cluster.Endpoints) == 0 {
		config.Cluster.Endpoints = []string{config.Node.Address}
	}

	// Raft默认值
	if config.Raft.LogDir == "" {
		config.Raft.LogDir = "data/raft/logs"
	}
	if config.Raft.SnapshotDir == "" {
		config.Raft.SnapshotDir = "data/raft/snapshots"
	}
	if config.Raft.HeartbeatTimeout == 0 {
		config.Raft.HeartbeatTimeout = 1 * time.Second
	}
	if config.Raft.ElectionTimeout == 0 {
		config.Raft.ElectionTimeout = 1 * time.Second
	}

	// Redis默认值
	if config.Storage.Redis.Addr == "" {
		config.Storage.Redis.Addr = "localhost:6379"
	}

	// 哈希默认值
	if config.Hash.VirtualNodes == 0 {
		config.Hash.VirtualNodes = 10
	}

	// gRPC默认值
	if config.GRPC.Port == 0 {
		config.GRPC.Port = 9090
	}
	if config.GRPC.MaxRecvMsgSize == 0 {
		config.GRPC.MaxRecvMsgSize = 4 * 1024 * 1024 // 4MB
	}
	if config.GRPC.MaxSendMsgSize == 0 {
		config.GRPC.MaxSendMsgSize = 4 * 1024 * 1024 // 4MB
	}

	// Raft gRPC默认值
	if config.RaftGRPC.Port == 0 {
		config.RaftGRPC.Port = 9094
	}

	// 日志默认值
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}
	if config.Log.Output == "" {
		config.Log.Output = "stdout"
	}

	// 高级默认值
	if config.Advanced.BufferSize == 0 {
		config.Advanced.BufferSize = 1000
	}

	// 主机管理默认值
	setHostManagementDefaults(config)
}

// setHostManagementDefaults 设置主机管理默认值
func setHostManagementDefaults(config *Config) {
	if !config.HostManagement.Enabled {
		config.HostManagement.Enabled = false
	}
	if !config.HostManagement.AutoRemoveCfg.Enabled {
		config.HostManagement.AutoRemoveCfg.Enabled = false
	}
	if !config.HostManagement.EnableScaling {
		config.HostManagement.EnableScaling = false
	}
	if config.HostManagement.MaxHosts == 0 {
		config.HostManagement.MaxHosts = 100
	}

	// 注册表默认值
	if config.HostManagement.Registry.HeartbeatInterval == 0 {
		config.HostManagement.Registry.HeartbeatInterval = 30 * time.Second
	}
	if config.HostManagement.Registry.HealthCheckInterval == 0 {
		config.HostManagement.Registry.HealthCheckInterval = 60 * time.Second
	}
	if config.HostManagement.Registry.OfflineTimeout == 0 {
		config.HostManagement.Registry.OfflineTimeout = 300 * time.Second
	}
	if config.HostManagement.Registry.CleanupInterval == 0 {
		config.HostManagement.Registry.CleanupInterval = 600 * time.Second
	}

	// 发现服务默认值
	if config.HostManagement.Discovery.ListenPort == 0 {
		config.HostManagement.Discovery.ListenPort = 9096
	}
	if config.HostManagement.Discovery.ScanInterval == 0 {
		config.HostManagement.Discovery.ScanInterval = 300 * time.Second
	}
	if config.HostManagement.Discovery.ScanTimeout == 0 {
		config.HostManagement.Discovery.ScanTimeout = 10 * time.Second
	}
	if len(config.HostManagement.Discovery.NetworkRanges) == 0 {
		config.HostManagement.Discovery.NetworkRanges = []string{"192.168.1.0/24"}
	}
	if len(config.HostManagement.Discovery.DiscoveryTypes) == 0 {
		config.HostManagement.Discovery.DiscoveryTypes = []string{"tcp", "http"}
	}

	// 健康监控默认值
	if config.HostManagement.Health.CheckInterval == 0 {
		config.HostManagement.Health.CheckInterval = 60 * time.Second
	}
	if config.HostManagement.Health.Timeout == 0 {
		config.HostManagement.Health.Timeout = 5 * time.Second
	}
	if config.HostManagement.Health.MaxRetries == 0 {
		config.HostManagement.Health.MaxRetries = 3
	}
	if len(config.HostManagement.Health.TCPPorts) == 0 {
		config.HostManagement.Health.TCPPorts = []int{22, 80, 443, 9093}
	}
	if len(config.HostManagement.Health.HTTPPaths) == 0 {
		config.HostManagement.Health.HTTPPaths = []string{"/health", "/ping"}
	}
	if config.HostManagement.Health.AlertThresholds == nil {
		config.HostManagement.Health.AlertThresholds = map[string]float64{
			"cpu":    80.0,
			"memory": 85.0,
			"disk":   90.0,
		}
	}

	// 自动移除默认值
	if config.HostManagement.AutoRemoveCfg.UnhealthyTimeout == 0 {
		config.HostManagement.AutoRemoveCfg.UnhealthyTimeout = 600 * time.Second
	}
	if config.HostManagement.AutoRemoveCfg.OfflineTimeout == 0 {
		config.HostManagement.AutoRemoveCfg.OfflineTimeout = 1200 * time.Second
	}

	// 扩缩容默认值
	if config.HostManagement.Scaling.ScaleUpThreshold == 0 {
		config.HostManagement.Scaling.ScaleUpThreshold = 50
	}
	if config.HostManagement.Scaling.ScaleDownThreshold == 0 {
		config.HostManagement.Scaling.ScaleDownThreshold = 75
	}
	if config.HostManagement.Scaling.MinHosts == 0 {
		config.HostManagement.Scaling.MinHosts = 3
	}
	if config.HostManagement.Scaling.MaxHosts == 0 {
		config.HostManagement.Scaling.MaxHosts = 100
	}
}
