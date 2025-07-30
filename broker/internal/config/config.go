package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 配置结构
type Config struct {
	Node     NodeConfig     `yaml:"node"`
	Cluster  ClusterConfig  `yaml:"cluster"`
	Raft     RaftConfig     `yaml:"raft"`
	Storage  StorageConfig  `yaml:"storage"`
	Hash     HashConfig     `yaml:"hash"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	Log      LogConfig      `yaml:"log"`
	Advanced AdvancedConfig `yaml:"advanced"`
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
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
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
}
