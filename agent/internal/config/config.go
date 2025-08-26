package config

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 采集代理配置
type Config struct {
	Agent      AgentConfig      `yaml:"agent"`
	Collect    CollectConfig    `yaml:"collect"`
	Broker     BrokerConfig     `yaml:"broker"`
	Kafka      KafkaConfig      `yaml:"kafka"`      // Kafka配置
	Log        LogConfig        `yaml:"log"`
	Advanced   AdvancedConfig   `yaml:"advanced"`
	HostRegistry HostRegistryConfig `yaml:"host_registry"`
	Algorithms AlgorithmConfig `yaml:"algorithms"`
}

// AgentConfig 代理基本配置
type AgentConfig struct {
	HostID   string `yaml:"host_id"`  // 主机ID
	Hostname string `yaml:"hostname"` // 主机名
	IP       string `yaml:"ip"`       // IP地址
	Port     int    `yaml:"port"`     // 监听端口
}

// CollectConfig 采集配置
type CollectConfig struct {
	Interval      time.Duration `yaml:"interval"`        // 采集间隔，单位秒
	Timeout       time.Duration `yaml:"timeout"`         // 超时时间，单位秒
	BatchSize     int           `yaml:"batch_size"`      // 批处理大小
	Metrics       []string      `yaml:"metrics"`         // 要采集的指标列表
	MaxRetry      int           `yaml:"max_retry"`       // 最大重试次数
	CacheDuration time.Duration `yaml:"cache_duration"`  // 缓存持续时间
	EnablePerCore bool          `yaml:"enable_per_core"` // 是否启用每核心CPU指标
	EnableVMStat  bool          `yaml:"enable_vmstat"`   // 是否启用VM统计
	EnableNUMA    bool          `yaml:"enable_numa"`     // 是否启用NUMA指标
}

// BrokerConfig 中转层配置
type BrokerConfig struct {
	Endpoints []string      `yaml:"endpoints"` // 中转层节点列表
	Timeout   time.Duration `yaml:"timeout"`   // 连接超时时间，单位秒
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`       // 日志级别
	Path       string `yaml:"path"`        // 日志路径
	MaxSize    int    `yaml:"max_size"`    // 单个日志文件最大大小，单位MB
	MaxBackups int    `yaml:"max_backups"` // 最大备份数量
	MaxAge     int    `yaml:"max_age"`     // 最大保留天数
	Compress   bool   `yaml:"compress"`    // 是否压缩
}

// AdvancedConfig 高级配置
type AdvancedConfig struct {
	WindowSize       int  `yaml:"window_size"`        // 滑动窗口大小
	BloomFilterSize  int  `yaml:"bloom_filter_size"`  // 布隆过滤器大小
	EnableTimeWheel  bool `yaml:"enable_time_wheel"`  // 是否启用时间轮
	TimeWheelTick    int  `yaml:"time_wheel_tick"`    // 时间轮刻度，单位毫秒
	TimeWheelSize    int  `yaml:"time_wheel_size"`    // 时间轮大小
	EnablePrefixTree bool `yaml:"enable_prefix_tree"` // 是否启用前缀树
	RingBufferSize   int  `yaml:"ring_buffer_size"`   // 环形缓冲区大小
	MaxGoroutines    int  `yaml:"max_goroutines"`     // 最大goroutine数量
}

// HostRegistryConfig 主机注册配置
type HostRegistryConfig struct {
	Enable        bool          `yaml:"enable"`         // 是否启用主机注册
	UpdateInterval time.Duration `yaml:"update_interval"` // 更新间隔
	MaxHosts      int           `yaml:"max_hosts"`      // 最大主机数量
	HealthCheck   bool          `yaml:"health_check"`   // 是否启用健康检查
	RetryInterval time.Duration `yaml:"retry_interval"` // 重试间隔
}

// AlgorithmConfig 算法配置
type AlgorithmConfig struct {
	SlidingWindow SlidingWindowConfig `yaml:"sliding_window"`
	BloomFilter   BloomFilterConfig   `yaml:"bloom_filter"`
	TimeWheel     TimeWheelConfig     `yaml:"time_wheel"`
	PrefixTree    PrefixTreeConfig    `yaml:"prefix_tree"`
}

// SlidingWindowConfig 滑动窗口配置
type SlidingWindowConfig struct {
	Enable        bool          `yaml:"enable"`
	Size          int           `yaml:"size"`
	MaxAge        time.Duration `yaml:"max_age"`
	Adaptive      bool          `yaml:"adaptive"`
	MinSize       int           `yaml:"min_size"`
	MaxSize       int           `yaml:"max_size"`
	GrowthFactor  float64       `yaml:"growth_factor"`
	ShrinkFactor  float64       `yaml:"shrink_factor"`
}

// BloomFilterConfig 布隆过滤器配置
type BloomFilterConfig struct {
	Enable      bool    `yaml:"enable"`
	Size        int     `yaml:"size"`
	FalsePositive float64 `yaml:"false_positive"`
	HashFuncs   int     `yaml:"hash_funcs"`
}

// TimeWheelConfig 时间轮配置
type TimeWheelConfig struct {
	Enable        bool          `yaml:"enable"`
	TickInterval  time.Duration `yaml:"tick_interval"`
	SlotNum       int           `yaml:"slot_num"`
	MultiLevel    bool          `yaml:"multi_level"`
	MaxTasks      int           `yaml:"max_tasks"`
}

// PrefixTreeConfig 前缀树配置
type PrefixTreeConfig struct {
	Enable        bool   `yaml:"enable"`
	MaxDepth      int    `yaml:"max_depth"`
	Compress      bool   `yaml:"compress"`
	Wildcard      bool   `yaml:"wildcard"`
	CaseSensitive bool   `yaml:"case_sensitive"`
}

// KafkaConfig Kafka配置
type KafkaConfig struct {
	Enabled      bool          `yaml:"enabled"`       // 是否启用Kafka
	Brokers      []string      `yaml:"brokers"`       // Kafka服务器地址列表
	Topic        string        `yaml:"topic"`         // 主题名称
	BatchSize    int           `yaml:"batch_size"`    // 批处理大小
	BatchTimeout time.Duration `yaml:"batch_timeout"` // 批处理超时时间
	Encoding     string        `yaml:"encoding"`      // 编码格式 (json 或 protobuf)
	MaxRetry     int           `yaml:"max_retry"`     // 最大重试次数
}

var (
	config *Config
	once   sync.Once
)

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	var err error
	once.Do(func() {
		config = &Config{}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
			return
		}
		err = yaml.Unmarshal(data, config)
		if err != nil {
			return
		}

		// 设置默认值
		if config.Collect.Interval == 0 {
			config.Collect.Interval = 1 * time.Second
		}
		if config.Collect.Timeout == 0 {
			config.Collect.Timeout = 3 * time.Second
		}
		if config.Collect.BatchSize == 0 {
			config.Collect.BatchSize = 100
		}
		if config.Collect.MaxRetry == 0 {
			config.Collect.MaxRetry = 3
		}
		if config.Collect.CacheDuration == 0 {
			config.Collect.CacheDuration = 5 * time.Second
		}
		if config.Advanced.WindowSize == 0 {
			config.Advanced.WindowSize = 10
		}
		if config.Advanced.BloomFilterSize == 0 {
			config.Advanced.BloomFilterSize = 100000
		}
		if config.Advanced.TimeWheelTick == 0 {
			config.Advanced.TimeWheelTick = 100
		}
		if config.Advanced.TimeWheelSize == 0 {
			config.Advanced.TimeWheelSize = 60
		}
		if config.Advanced.RingBufferSize == 0 {
			config.Advanced.RingBufferSize = 1024
		}
		if config.Advanced.MaxGoroutines == 0 {
			config.Advanced.MaxGoroutines = 1000
		}
		// 设置主机注册默认值
		if config.HostRegistry.UpdateInterval == 0 {
			config.HostRegistry.UpdateInterval = 30 * time.Second
		}
		if config.HostRegistry.MaxHosts == 0 {
			config.HostRegistry.MaxHosts = 1000
		}
		if config.HostRegistry.RetryInterval == 0 {
			config.HostRegistry.RetryInterval = 5 * time.Second
		}
		// 设置算法默认值
		if config.Algorithms.SlidingWindow.Size == 0 {
			config.Algorithms.SlidingWindow.Size = 10
		}
		if config.Algorithms.SlidingWindow.MaxAge == 0 {
			config.Algorithms.SlidingWindow.MaxAge = 10 * time.Second
		}
		if config.Algorithms.SlidingWindow.MinSize == 0 {
			config.Algorithms.SlidingWindow.MinSize = 5
		}
		if config.Algorithms.SlidingWindow.MaxSize == 0 {
			config.Algorithms.SlidingWindow.MaxSize = 100
		}
		if config.Algorithms.BloomFilter.Size == 0 {
			config.Algorithms.BloomFilter.Size = 100000
		}
		if config.Algorithms.BloomFilter.FalsePositive == 0 {
			config.Algorithms.BloomFilter.FalsePositive = 0.001
		}
		if config.Algorithms.BloomFilter.HashFuncs == 0 {
			config.Algorithms.BloomFilter.HashFuncs = 3
		}
		if config.Algorithms.TimeWheel.TickInterval == 0 {
			config.Algorithms.TimeWheel.TickInterval = 100 * time.Millisecond
		}
		if config.Algorithms.TimeWheel.SlotNum == 0 {
			config.Algorithms.TimeWheel.SlotNum = 60
		}
		if config.Algorithms.TimeWheel.MaxTasks == 0 {
			config.Algorithms.TimeWheel.MaxTasks = 10000
		}
		if config.Algorithms.PrefixTree.MaxDepth == 0 {
			config.Algorithms.PrefixTree.MaxDepth = 32
		}
		
		// 设置Kafka默认值
		if config.Kafka.BatchSize == 0 {
			config.Kafka.BatchSize = 100
		}
		if config.Kafka.BatchTimeout == 0 {
			config.Kafka.BatchTimeout = 1 * time.Second
		}
		if config.Kafka.MaxRetry == 0 {
			config.Kafka.MaxRetry = 3
		}
		if config.Kafka.Encoding == "" {
			config.Kafka.Encoding = "json"
		}
		if config.Kafka.Topic == "" {
			config.Kafka.Topic = "metrics"
		}
	})
	return config, err
}

// GetConfig 获取配置
func GetConfig() *Config {
	return config
}
