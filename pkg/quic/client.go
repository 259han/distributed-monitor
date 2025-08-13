package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/han-fei/monitor/proto"
	"github.com/quic-go/quic-go"
)

// Client QUIC客户端
type Client struct {
	config      *Config
	protocol    *ProtocolHandler
	dataHandler DataHandler
	conn        *quic.Conn
	streams     sync.Map
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	connected   int32
	reconnectCh chan struct{}
}

// NewClient 创建新的QUIC客户端
func NewClient(config *Config, dataHandler DataHandler) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:      config,
		protocol:    NewProtocolHandler(config.MaxMessageSize),
		dataHandler: dataHandler,
		ctx:         ctx,
		cancel:      cancel,
		reconnectCh: make(chan struct{}, 1),
	}
}

// Connect 连接到服务器
func (c *Client) Connect(addr string) error {
	if atomic.LoadInt32(&c.connected) == 1 {
		return fmt.Errorf("客户端已连接")
	}

	// 创建TLS配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		NextProtos:         c.config.NextProtos,
		ServerName:         "localhost", // 用于证书验证
	}

	// 创建QUIC配置
	quicConfig := &quic.Config{
		MaxIncomingStreams:    int64(c.config.MaxStreamsPerConn),
		MaxIncomingUniStreams: int64(c.config.MaxStreamsPerConn),
		KeepAlivePeriod:       c.config.KeepAlivePeriod,
		MaxIdleTimeout:        c.config.MaxIdleTimeout,
		HandshakeIdleTimeout:  c.config.HandshakeTimeout,
		EnableDatagrams:       true,
	}

	// 连接到服务器
	conn, err := quic.DialAddr(c.ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("连接QUIC服务器失败: %v", err)
	}

	c.conn = conn
	atomic.StoreInt32(&c.connected, 1)

	log.Printf("QUIC客户端已连接到: %s", addr)

	// 启动连接处理协程
	c.wg.Add(1)
	go c.handleConnection()

	// 启动重连监控协程
	c.wg.Add(1)
	go c.reconnectMonitor(addr)

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return fmt.Errorf("客户端未连接")
	}

	log.Println("正在断开QUIC连接...")

	// 取消上下文
	c.cancel()

	// 关闭连接
	if c.conn != nil {
		c.conn.CloseWithError(0, "客户端断开")
	}

	// 等待所有协程完成
	c.wg.Wait()

	atomic.StoreInt32(&c.connected, 0)

	log.Println("QUIC客户端已断开")
	return nil
}

// SendMetrics 发送指标数据
func (c *Client) SendMetrics(metrics *pb.MetricsData) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return fmt.Errorf("客户端未连接")
	}

	// 创建消息
	msg, err := c.protocol.CreateMetricsMessage(metrics.HostId, metrics)
	if err != nil {
		return fmt.Errorf("创建指标消息失败: %v", err)
	}

	// 发送消息
	return c.sendMessage(msg)
}

// SendHeartbeat 发送心跳
func (c *Client) SendHeartbeat(hostID string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return fmt.Errorf("客户端未连接")
	}

	// 创建心跳消息
	msg := c.protocol.CreateHeartbeatMessage(hostID)

	// 发送消息
	return c.sendMessage(msg)
}

// Subscribe 订阅数据
func (c *Client) Subscribe(hostID, streamID string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return fmt.Errorf("客户端未连接")
	}

	// 创建订阅信息
	sub := &Subscription{
		HostID:    hostID,
		StreamID:  streamID,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	// 创建订阅消息
	msg, err := c.protocol.CreateSubscribeMessage(sub)
	if err != nil {
		return fmt.Errorf("创建订阅消息失败: %v", err)
	}

	// 发送消息
	return c.sendMessage(msg)
}

// Unsubscribe 取消订阅
func (c *Client) Unsubscribe(hostID, streamID string) error {
	if atomic.LoadInt32(&c.connected) == 0 {
		return fmt.Errorf("客户端未连接")
	}

	// 创建订阅信息
	sub := &Subscription{
		HostID:   hostID,
		StreamID: streamID,
	}

	// 创建取消订阅消息
	msg, err := c.protocol.CreateSubscribeMessage(sub)
	if err != nil {
		return fmt.Errorf("创建取消订阅消息失败: %v", err)
	}
	msg.Type = MessageTypeUnsubscribe

	// 发送消息
	return c.sendMessage(msg)
}

// sendMessage 发送消息
func (c *Client) sendMessage(msg *Message) error {
	// 打开新流
	stream, err := c.conn.OpenStreamSync(c.ctx)
	if err != nil {
		return fmt.Errorf("打开流失败: %v", err)
	}
	defer stream.Close()

	// 发送消息
	if err := c.protocol.WriteMessage(stream, msg); err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}

	// 读取响应（如果需要）
	if msg.Type != MessageTypeHeartbeat {
		respMsg, err := c.protocol.ReadMessage(stream)
		if err != nil {
			return fmt.Errorf("读取响应失败: %v", err)
		}

		if respMsg.Type == MessageTypeError {
			return fmt.Errorf("服务器返回错误: %s", string(respMsg.Data))
		}
	}

	return nil
}

// handleConnection 处理连接
func (c *Client) handleConnection() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.conn.Context().Done():
			// 连接断开，触发重连
			select {
			case c.reconnectCh <- struct{}{}:
			default:
			}
			return
		default:
			// 接受服务器推送的流
			stream, err := c.conn.AcceptStream(c.ctx)
			if err != nil {
				select {
				case <-c.ctx.Done():
					return
				case <-c.conn.Context().Done():
					return
				default:
					log.Printf("接受流失败: %v", err)
					continue
				}
			}

			// 处理流
			c.wg.Add(1)
			go c.handleStream(stream)
		}
	}
}

// handleStream 处理流
func (c *Client) handleStream(stream *quic.Stream) {
	defer c.wg.Done()
	defer stream.Close()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// 读取消息
			msg, err := c.protocol.ReadMessage(stream)
			if err != nil {
				log.Printf("读取消息失败: %v", err)
				return
			}

			// 处理消息
			if err := c.handleMessage(stream, msg); err != nil {
				log.Printf("处理消息失败: %v", err)
			}
		}
	}
}

// handleMessage 处理消息
func (c *Client) handleMessage(stream *quic.Stream, msg *Message) error {
	switch msg.Type {
	case MessageTypeMetrics:
		return c.handleMetricsMessage(msg)
	case MessageTypeHeartbeat:
		return c.handleHeartbeatMessage(stream, msg)
	case MessageTypeResponse:
		return c.handleResponseMessage(msg)
	default:
		log.Printf("收到未知消息类型: %d", msg.Type)
		return nil
	}
}

// handleMetricsMessage 处理指标数据消息
func (c *Client) handleMetricsMessage(msg *Message) error {
	// 解码指标数据
	metrics, err := c.protocol.DecodeMetrics(msg.Data)
	if err != nil {
		return fmt.Errorf("解码指标数据失败: %v", err)
	}

	// 调用数据处理器
	if c.dataHandler != nil {
		return c.dataHandler.HandleMetrics(c.ctx, metrics)
	}

	return nil
}

// handleHeartbeatMessage 处理心跳消息
func (c *Client) handleHeartbeatMessage(stream *quic.Stream, msg *Message) error {
	hostID := string(msg.Data)

	// 调用数据处理器
	if c.dataHandler != nil {
		if err := c.dataHandler.HandleHeartbeat(c.ctx, hostID); err != nil {
			return err
		}
	}

	// 发送心跳响应
	heartbeatMsg := c.protocol.CreateHeartbeatMessage("client")
	return c.protocol.WriteMessage(stream, heartbeatMsg)
}

// handleResponseMessage 处理响应消息
func (c *Client) handleResponseMessage(msg *Message) error {
	// 这里可以实现响应处理逻辑
	log.Printf("收到响应消息: %s", string(msg.Data))
	return nil
}

// reconnectMonitor 重连监控
func (c *Client) reconnectMonitor(addr string) {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.reconnectCh:
			if atomic.LoadInt32(&c.connected) == 0 {
				return // 已断开，不需要重连
			}

			log.Println("连接断开，尝试重连...")
			atomic.StoreInt32(&c.connected, 0)

			// 等待一段时间后重连
			timer := time.NewTimer(c.config.RetryInterval)
			select {
			case <-c.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			// 尝试重连
			for i := 0; i < c.config.MaxRetries; i++ {
				if err := c.reconnect(addr); err != nil {
					log.Printf("重连失败 (尝试 %d/%d): %v", i+1, c.config.MaxRetries, err)

					// 等待后重试
					timer := time.NewTimer(c.config.RetryInterval)
					select {
					case <-c.ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
					continue
				}

				log.Println("重连成功")
				break
			}
		}
	}
}

// reconnect 重连
func (c *Client) reconnect(addr string) error {
	// 创建TLS配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.config.InsecureSkipVerify,
		NextProtos:         c.config.NextProtos,
		ServerName:         "localhost",
	}

	// 创建QUIC配置
	quicConfig := &quic.Config{
		MaxIncomingStreams:    int64(c.config.MaxStreamsPerConn),
		MaxIncomingUniStreams: int64(c.config.MaxStreamsPerConn),
		KeepAlivePeriod:       c.config.KeepAlivePeriod,
		MaxIdleTimeout:        c.config.MaxIdleTimeout,
		HandshakeIdleTimeout:  c.config.HandshakeTimeout,
		EnableDatagrams:       true,
	}

	// 连接到服务器
	conn, err := quic.DialAddr(c.ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return err
	}

	c.conn = conn
	atomic.StoreInt32(&c.connected, 1)

	// 启动连接处理协程
	c.wg.Add(1)
	go c.handleConnection()

	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1
}
