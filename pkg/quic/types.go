package quic

import (
	"context"
	"time"

	pb "github.com/han-fei/monitor/proto"
	"github.com/quic-go/quic-go"
)

// MessageType 消息类型
type MessageType uint8

const (
	// 数据消息类型
	MessageTypeMetrics     MessageType = 1 // 指标数据
	MessageTypeHeartbeat   MessageType = 2 // 心跳消息
	MessageTypeSubscribe   MessageType = 3 // 订阅请求
	MessageTypeUnsubscribe MessageType = 4 // 取消订阅
	MessageTypeQuery       MessageType = 5 // 查询请求
	MessageTypeResponse    MessageType = 6 // 响应消息
	MessageTypeError       MessageType = 7 // 错误消息
	MessageTypeAck         MessageType = 8 // 确认消息
)

// Message QUIC消息结构
type Message struct {
	Type      MessageType `json:"type"`
	ID        string      `json:"id"`
	Timestamp int64       `json:"timestamp"`
	Data      []byte      `json:"data"`
}

// StreamHandler 流处理器接口
type StreamHandler interface {
	HandleStream(ctx context.Context, stream *quic.Stream) error
}

// ConnectionHandler 连接处理器接口
type ConnectionHandler interface {
	HandleConnection(ctx context.Context, conn *quic.Conn) error
}

// DataHandler 数据处理器接口
type DataHandler interface {
	HandleMetrics(ctx context.Context, metrics *pb.MetricsData) error
	HandleHeartbeat(ctx context.Context, hostID string) error
	HandleSubscribe(ctx context.Context, hostID string, streamID string) error
	HandleUnsubscribe(ctx context.Context, hostID string, streamID string) error
}

// Config QUIC配置
type Config struct {
	// 服务器配置
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`

	// 连接配置
	MaxStreamsPerConn int           `yaml:"max_streams_per_conn"`
	KeepAlivePeriod   time.Duration `yaml:"keep_alive_period"`
	MaxIdleTimeout    time.Duration `yaml:"max_idle_timeout"`
	HandshakeTimeout  time.Duration `yaml:"handshake_timeout"`

	// 缓冲区配置
	BufferSize     int `yaml:"buffer_size"`
	MaxMessageSize int `yaml:"max_message_size"`

	// 重试配置
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`

	// TLS配置
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
	NextProtos         []string `yaml:"next_protos"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:               "localhost",
		Port:               8081,
		MaxStreamsPerConn:  100,
		KeepAlivePeriod:    30 * time.Second,
		MaxIdleTimeout:     60 * time.Second,
		HandshakeTimeout:   10 * time.Second,
		BufferSize:         256,
		MaxMessageSize:     1024 * 1024, // 1MB
		MaxRetries:         3,
		RetryInterval:      time.Second,
		InsecureSkipVerify: true, // 仅用于开发环境
		NextProtos:         []string{"monitor-quic/1.0"},
	}
}

// StreamType 流类型
type StreamType string

const (
	StreamTypeMetrics   StreamType = "metrics"   // 指标数据流
	StreamTypeControl   StreamType = "control"   // 控制流
	StreamTypeHeartbeat StreamType = "heartbeat" // 心跳流
	StreamTypeQuery     StreamType = "query"     // 查询流
)

// Subscription 订阅信息
type Subscription struct {
	HostID    string    `json:"host_id"`
	StreamID  string    `json:"stream_id"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// Statistics 统计信息
type Statistics struct {
	ConnectionsTotal  int64         `json:"connections_total"`
	ConnectionsActive int64         `json:"connections_active"`
	StreamsTotal      int64         `json:"streams_total"`
	StreamsActive     int64         `json:"streams_active"`
	MessagesReceived  int64         `json:"messages_received"`
	MessagesSent      int64         `json:"messages_sent"`
	BytesReceived     int64         `json:"bytes_received"`
	BytesSent         int64         `json:"bytes_sent"`
	ErrorsTotal       int64         `json:"errors_total"`
	Uptime            time.Duration `json:"uptime"`
	LastUpdateTime    time.Time     `json:"last_update_time"`
}
