package config

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 可视化分析端配置
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Broker    BrokerConfig    `yaml:"broker"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	QUIC      QUICConfig      `yaml:"quic"`
	Auth      AuthConfig      `yaml:"auth"`
	TopK      TopKConfig      `yaml:"topk"`
	Cache     CacheConfig     `yaml:"cache"`
	Log       LogConfig       `yaml:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// BrokerConfig 中转层配置
type BrokerConfig struct {
	Endpoints []string      `yaml:"endpoints"`
	Timeout   time.Duration `yaml:"timeout"`
	MaxRetry  int           `yaml:"max_retry"`
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	Enable             bool          `yaml:"enable"`
	Path               string        `yaml:"path"`
	ReadBufferSize     int           `yaml:"read_buffer_size"`
	WriteBufferSize    int           `yaml:"write_buffer_size"`
	MaxMessageSize     int64         `yaml:"max_message_size"`
	PingInterval       time.Duration `yaml:"ping_interval"`
	PingTimeout        time.Duration `yaml:"ping_timeout"`
	PongTimeout        time.Duration `yaml:"pong_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	BufferSize         int           `yaml:"buffer_size"`
	HeartbeatInterval  time.Duration `yaml:"heartbeat_interval"`
	HeartbeatTimeout   time.Duration `yaml:"heartbeat_timeout"`
}

// QUICConfig QUIC配置
type QUICConfig struct {
	Enable            bool          `yaml:"enable"`
	Port              int           `yaml:"port"`
	MaxStreamsPerConn int           `yaml:"max_streams_per_conn"`
	KeepAlivePeriod   time.Duration `yaml:"keep_alive_period"`
	MaxIdleTimeout    time.Duration `yaml:"max_idle_timeout"`
	BufferSize        int           `yaml:"buffer_size"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret            string        `yaml:"jwt_secret"`
	TokenExpiry          time.Duration `yaml:"token_expiration"`
	RefreshTokenExpiry   time.Duration `yaml:"refresh_token_expiration"`
	CookieSecure         bool          `yaml:"cookie_secure"`
	CookieHTTPOnly       bool          `yaml:"cookie_http_only"`
	CookieName           string        `yaml:"cookie_name"`
	RefreshCookieName    string        `yaml:"refresh_cookie_name"`
}

// TopKConfig Top-K配置
type TopKConfig struct {
	Size int           `yaml:"size"`
	TTL  time.Duration `yaml:"ttl"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	TTL             time.Duration `yaml:"ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
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
		if config.Server.Port == 0 {
			config.Server.Port = 8080
		}
		if config.Server.Host == "" {
			config.Server.Host = "localhost"
		}
		if config.Server.ReadTimeout == 0 {
			config.Server.ReadTimeout = 10 * time.Second
		}
		if config.Server.WriteTimeout == 0 {
			config.Server.WriteTimeout = 10 * time.Second
		}
		if config.Server.IdleTimeout == 0 {
			config.Server.IdleTimeout = 60 * time.Second
		}

		if config.Broker.Timeout == 0 {
			config.Broker.Timeout = 5 * time.Second
		}

		if config.WebSocket.Path == "" {
			config.WebSocket.Path = "/ws"
		}
		if config.WebSocket.ReadBufferSize == 0 {
			config.WebSocket.ReadBufferSize = 1024
		}
		if config.WebSocket.WriteBufferSize == 0 {
			config.WebSocket.WriteBufferSize = 1024
		}
		if config.WebSocket.HeartbeatInterval == 0 {
			config.WebSocket.HeartbeatInterval = 10 * time.Second
		}
		if config.WebSocket.HeartbeatTimeout == 0 {
			config.WebSocket.HeartbeatTimeout = 30 * time.Second
		}

		if config.QUIC.Port == 0 {
			config.QUIC.Port = 8081
		}
		if config.QUIC.MaxStreamsPerConn == 0 {
			config.QUIC.MaxStreamsPerConn = 100
		}
		if config.QUIC.KeepAlivePeriod == 0 {
			config.QUIC.KeepAlivePeriod = 15 * time.Second
		}
		if config.QUIC.MaxIdleTimeout == 0 {
			config.QUIC.MaxIdleTimeout = 30 * time.Second
		}

		if config.Auth.TokenExpiry == 0 {
			config.Auth.TokenExpiry = 1 * time.Hour
		}

		if config.TopK.Size == 0 {
			config.TopK.Size = 10
		}

		if config.Cache.TTL == 0 {
			config.Cache.TTL = 5 * time.Minute
		}
		if config.Cache.CleanupInterval == 0 {
			config.Cache.CleanupInterval = 10 * time.Minute
		}
	})
	return config, err
}

// GetConfig 获取配置
func GetConfig() *Config {
	return config
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	// 服务器默认值
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 10 * time.Second
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 10 * time.Second
	}
	if config.Server.IdleTimeout == 0 {
		config.Server.IdleTimeout = 30 * time.Second
	}

	// Broker默认值
	if len(config.Broker.Endpoints) == 0 {
		config.Broker.Endpoints = []string{"localhost:9090"}
	}
	if config.Broker.Timeout == 0 {
		config.Broker.Timeout = 5 * time.Second
	}
	if config.Broker.MaxRetry == 0 {
		config.Broker.MaxRetry = 3
	}

	// WebSocket默认值
	if config.WebSocket.Path == "" {
		config.WebSocket.Path = "/ws"
	}
	if config.WebSocket.ReadBufferSize == 0 {
		config.WebSocket.ReadBufferSize = 1024
	}
	if config.WebSocket.WriteBufferSize == 0 {
		config.WebSocket.WriteBufferSize = 1024
	}
	if config.WebSocket.MaxMessageSize == 0 {
		config.WebSocket.MaxMessageSize = 512 * 1024 // 512KB
	}
	if config.WebSocket.PingInterval == 0 {
		config.WebSocket.PingInterval = 30 * time.Second
	}
	if config.WebSocket.PingTimeout == 0 {
		config.WebSocket.PingTimeout = 60 * time.Second
	}
	if config.WebSocket.PongTimeout == 0 {
		config.WebSocket.PongTimeout = 60 * time.Second
	}
	if config.WebSocket.WriteTimeout == 0 {
		config.WebSocket.WriteTimeout = 10 * time.Second
	}
	if config.WebSocket.BufferSize == 0 {
		config.WebSocket.BufferSize = 256
	}

	// QUIC默认值
	if config.QUIC.Port == 0 {
		config.QUIC.Port = 8081
	}
	if config.QUIC.BufferSize == 0 {
		config.QUIC.BufferSize = 256
	}

	// Auth默认值
	if config.Auth.JWTSecret == "" {
		config.Auth.JWTSecret = "default-secret-key"
	}

	// TopK默认值
	if config.TopK.Size == 0 {
		config.TopK.Size = 10
	}

	// Log默认值
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	/* 暂时注释掉有问题的配置
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}
	*/
	if config.Log.Output == "" {
		config.Log.Output = "stdout"
	}
}
