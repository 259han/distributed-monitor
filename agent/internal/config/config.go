package config

import (
	"io/ioutil"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 采集代理配置
type Config struct {
	Agent    AgentConfig    `yaml:"agent"`
	Collect  CollectConfig  `yaml:"collect"`
	Broker   BrokerConfig   `yaml:"broker"`
	Log      LogConfig      `yaml:"log"`
	Advanced AdvancedConfig `yaml:"advanced"`
}

// AgentConfig 代理基本配置
type AgentConfig struct {
	HostID   string `yaml:"host_id"`   // 主机ID
	Hostname string `yaml:"hostname"`  // 主机名
	IP       string `yaml:"ip"`        // IP地址
	Port     int    `yaml:"port"`      // 监听端口
}

// CollectConfig 采集配置
type CollectConfig struct {
	Interval  time.Duration `yaml:"interval"`   // 采集间隔，单位秒
	Timeout   time.Duration `yaml:"timeout"`    // 超时时间，单位秒
	BatchSize int           `yaml:"batch_size"` // 批处理大小
	Metrics   []string      `yaml:"metrics"`    // 要采集的指标列表
	MaxRetry  int           `yaml:"max_retry"`  // 最大重试次数
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

var (
	config *Config
	once   sync.Once
)

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	var err error
	once.Do(func() {
		config = &Config{}
		data, readErr := ioutil.ReadFile(path)
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
	})
	return config, err
}

// GetConfig 获取配置
func GetConfig() *Config {
	return config
} 