package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/han-fei/monitor/proto"
	"github.com/quic-go/quic-go"
)

// Server QUIC服务器
type Server struct {
	config        *Config
	listener      *quic.Listener
	protocol      *ProtocolHandler
	dataHandler   DataHandler
	connections   sync.Map // map[string]*quic.Conn
	subscriptions sync.Map // map[string]*Subscription
	statistics    *Statistics
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       int32
}

// NewServer 创建新的QUIC服务器
func NewServer(config *Config, dataHandler DataHandler) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 生成TLS配置
	tlsConfig, err := generateTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("生成TLS配置失败: %v", err)
	}

	// 创建QUIC配置
	quicConfig := &quic.Config{
		MaxIncomingStreams:    int64(config.MaxStreamsPerConn),
		MaxIncomingUniStreams: int64(config.MaxStreamsPerConn),
		KeepAlivePeriod:       config.KeepAlivePeriod,
		MaxIdleTimeout:        config.MaxIdleTimeout,
		HandshakeIdleTimeout:  config.HandshakeTimeout,
		EnableDatagrams:       true,
	}

	// 创建监听器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("创建QUIC监听器失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:      config,
		listener:    listener,
		protocol:    NewProtocolHandler(config.MaxMessageSize),
		dataHandler: dataHandler,
		statistics: &Statistics{
			LastUpdateTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return fmt.Errorf("服务器已经在运行")
	}

	log.Printf("QUIC服务器启动，监听地址: %s:%d", s.config.Host, s.config.Port)

	s.wg.Add(1)
	go s.acceptConnections()

	s.wg.Add(1)
	go s.updateStatistics()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return fmt.Errorf("服务器未在运行")
	}

	log.Println("正在关闭QUIC服务器...")

	// 取消上下文
	s.cancel()

	// 关闭监听器
	if err := s.listener.Close(); err != nil {
		log.Printf("关闭监听器失败: %v", err)
	}

	// 关闭所有连接
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*quic.Conn); ok {
			conn.CloseWithError(0, "服务器关闭")
		}
		return true
	})

	// 等待所有goroutine完成
	s.wg.Wait()

	log.Println("QUIC服务器已关闭")
	return nil
}

// acceptConnections 接受连接
func (s *Server) acceptConnections() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("接受连接失败: %v", err)
				continue
			}
		}

		// 记录连接
		connID := conn.RemoteAddr().String()
		s.connections.Store(connID, conn)
		atomic.AddInt64(&s.statistics.ConnectionsTotal, 1)
		atomic.AddInt64(&s.statistics.ConnectionsActive, 1)

		log.Printf("新的QUIC连接: %s", connID)

		// 处理连接
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *Server) handleConnection(conn *quic.Conn) {
	defer s.wg.Done()
	defer func() {
		connID := conn.RemoteAddr().String()
		s.connections.Delete(connID)
		atomic.AddInt64(&s.statistics.ConnectionsActive, -1)
		log.Printf("连接已关闭: %s", connID)
	}()

	connCtx := conn.Context()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-connCtx.Done():
			return
		default:
			// 接受新流
			stream, err := conn.AcceptStream(s.ctx)
			if err != nil {
				select {
				case <-s.ctx.Done():
					return
				case <-connCtx.Done():
					return
				default:
					log.Printf("接受流失败: %v", err)
					return
				}
			}

			atomic.AddInt64(&s.statistics.StreamsTotal, 1)
			atomic.AddInt64(&s.statistics.StreamsActive, 1)

			// 处理流
			s.wg.Add(1)
			go s.handleStream(stream)
		}
	}
}

// handleStream 处理流
func (s *Server) handleStream(stream *quic.Stream) {
	defer s.wg.Done()
	defer func() {
		stream.Close()
		atomic.AddInt64(&s.statistics.StreamsActive, -1)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// 读取消息
			msg, err := s.protocol.ReadMessage(stream)
			if err != nil {
				log.Printf("读取消息失败: %v", err)
				return
			}

			atomic.AddInt64(&s.statistics.MessagesReceived, 1)
			atomic.AddInt64(&s.statistics.BytesReceived, int64(len(msg.Data)))

			// 处理消息
			if err := s.handleMessage(stream, msg); err != nil {
				log.Printf("处理消息失败: %v", err)

				// 发送错误响应
				errMsg := s.protocol.CreateErrorMessage(msg.ID, err.Error())
				if writeErr := s.protocol.WriteMessage(stream, errMsg); writeErr != nil {
					log.Printf("发送错误消息失败: %v", writeErr)
				}

				atomic.AddInt64(&s.statistics.ErrorsTotal, 1)
			}
		}
	}
}

// handleMessage 处理消息
func (s *Server) handleMessage(stream *quic.Stream, msg *Message) error {
	switch msg.Type {
	case MessageTypeMetrics:
		return s.handleMetricsMessage(stream, msg)
	case MessageTypeHeartbeat:
		return s.handleHeartbeatMessage(stream, msg)
	case MessageTypeSubscribe:
		return s.handleSubscribeMessage(stream, msg)
	case MessageTypeUnsubscribe:
		return s.handleUnsubscribeMessage(stream, msg)
	case MessageTypeQuery:
		return s.handleQueryMessage(stream, msg)
	default:
		return fmt.Errorf("未知消息类型: %d", msg.Type)
	}
}

// handleMetricsMessage 处理指标数据消息
func (s *Server) handleMetricsMessage(stream *quic.Stream, msg *Message) error {
	// 解码指标数据
	metrics, err := s.protocol.DecodeMetrics(msg.Data)
	if err != nil {
		return fmt.Errorf("解码指标数据失败: %v", err)
	}

	// 调用数据处理器
	if err := s.dataHandler.HandleMetrics(s.ctx, metrics); err != nil {
		return fmt.Errorf("处理指标数据失败: %v", err)
	}

	// 发送确认消息
	ackMsg := s.protocol.CreateAckMessage(msg.ID)
	return s.writeMessage(stream, ackMsg)
}

// handleHeartbeatMessage 处理心跳消息
func (s *Server) handleHeartbeatMessage(stream *quic.Stream, msg *Message) error {
	hostID := string(msg.Data)

	// 调用数据处理器
	if err := s.dataHandler.HandleHeartbeat(s.ctx, hostID); err != nil {
		return fmt.Errorf("处理心跳失败: %v", err)
	}

	// 发送心跳响应
	heartbeatMsg := s.protocol.CreateHeartbeatMessage("server")
	return s.writeMessage(stream, heartbeatMsg)
}

// handleSubscribeMessage 处理订阅消息
func (s *Server) handleSubscribeMessage(stream *quic.Stream, msg *Message) error {
	// 解码订阅请求
	sub, err := s.protocol.DecodeSubscription(msg.Data)
	if err != nil {
		return fmt.Errorf("解码订阅请求失败: %v", err)
	}

	// 记录订阅
	subKey := fmt.Sprintf("%s:%s", sub.HostID, sub.StreamID)
	s.subscriptions.Store(subKey, sub)

	// 调用数据处理器
	if err := s.dataHandler.HandleSubscribe(s.ctx, sub.HostID, sub.StreamID); err != nil {
		return fmt.Errorf("处理订阅失败: %v", err)
	}

	// 发送确认消息
	ackMsg := s.protocol.CreateAckMessage(msg.ID)
	return s.writeMessage(stream, ackMsg)
}

// handleUnsubscribeMessage 处理取消订阅消息
func (s *Server) handleUnsubscribeMessage(stream *quic.Stream, msg *Message) error {
	// 解码订阅请求
	sub, err := s.protocol.DecodeSubscription(msg.Data)
	if err != nil {
		return fmt.Errorf("解码订阅请求失败: %v", err)
	}

	// 删除订阅
	subKey := fmt.Sprintf("%s:%s", sub.HostID, sub.StreamID)
	s.subscriptions.Delete(subKey)

	// 调用数据处理器
	if err := s.dataHandler.HandleUnsubscribe(s.ctx, sub.HostID, sub.StreamID); err != nil {
		return fmt.Errorf("处理取消订阅失败: %v", err)
	}

	// 发送确认消息
	ackMsg := s.protocol.CreateAckMessage(msg.ID)
	return s.writeMessage(stream, ackMsg)
}

// handleQueryMessage 处理查询消息
func (s *Server) handleQueryMessage(stream *quic.Stream, msg *Message) error {
	// 这里可以实现查询逻辑
	// 暂时返回空响应
	respMsg := s.protocol.CreateMessage(MessageTypeResponse, msg.ID, []byte("{}"))
	return s.writeMessage(stream, respMsg)
}

// writeMessage 写入消息
func (s *Server) writeMessage(stream *quic.Stream, msg *Message) error {
	if err := s.protocol.WriteMessage(stream, msg); err != nil {
		return err
	}

	atomic.AddInt64(&s.statistics.MessagesSent, 1)
	atomic.AddInt64(&s.statistics.BytesSent, int64(len(msg.Data)))

	return nil
}

// BroadcastMetrics 广播指标数据
func (s *Server) BroadcastMetrics(metrics *pb.MetricsData) error {
	msg, err := s.protocol.CreateMetricsMessage(metrics.HostId, metrics)
	if err != nil {
		return err
	}

	// 向所有连接广播
	var broadcastErr error
	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*quic.Conn); ok {
			// 打开新流
			stream, err := conn.OpenStreamSync(s.ctx)
			if err != nil {
				log.Printf("打开流失败: %v", err)
				broadcastErr = err
				return false
			}

			// 发送消息
			if err := s.writeMessage(stream, msg); err != nil {
				log.Printf("发送广播消息失败: %v", err)
				broadcastErr = err
			}

			stream.Close()
		}
		return true
	})

	return broadcastErr
}

// GetStatistics 获取统计信息
func (s *Server) GetStatistics() *Statistics {
	stats := *s.statistics
	stats.Uptime = time.Since(stats.LastUpdateTime)
	return &stats
}

// updateStatistics 更新统计信息
func (s *Server) updateStatistics() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.statistics.LastUpdateTime = time.Now()
		}
	}
}

// generateTLSConfig 生成TLS配置
func generateTLSConfig(config *Config) (*tls.Config, error) {
	// 如果提供了证书文件，使用文件中的证书
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载证书文件失败: %v", err)
		}

		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   config.NextProtos,
		}, nil
	}

	// 生成自签名证书
	return generateSelfSignedTLSConfig(config)
}

// generateSelfSignedTLSConfig 生成自签名TLS配置
func generateSelfSignedTLSConfig(config *Config) (*tls.Config, error) {
	// 生成私钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Monitor QUIC Server"},
			Country:       []string{"CN"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:    []string{"localhost"},
	}

	// 创建证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	// 编码证书和私钥
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	// 加载证书
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   config.NextProtos,
	}, nil
}
