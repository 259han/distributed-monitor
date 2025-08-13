package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// Server QUIC服务器
type Server struct {
	config      *config.Config
	listener    *quic.Listener
	dataStream  chan *models.MetricsData
	stopCh      chan struct{}
	connections map[*quic.Conn]struct{}
	connMutex   sync.RWMutex
}

// NewServer 创建新的QUIC服务器
func NewServer(cfg *config.Config) (*Server, error) {
	// 生成TLS配置
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("生成TLS配置失败: %v", err)
	}

	// 创建QUIC监听器
	listener, err := quic.ListenAddr(fmt.Sprintf(":%d", cfg.QUIC.Port), tlsConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("创建QUIC监听器失败: %v", err)
	}

	return &Server{
		config:      cfg,
		listener:    listener,
		dataStream:  make(chan *models.MetricsData, cfg.QUIC.BufferSize),
		stopCh:      make(chan struct{}),
		connections: make(map[*quic.Conn]struct{}),
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	log.Printf("QUIC服务器启动，监听端口: %d", s.config.QUIC.Port)

	go func() {
		for {
			// 接受连接
			conn, err := s.listener.Accept(context.Background())
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					log.Printf("接受QUIC连接失败: %v", err)
					continue
				}
			}

			// 处理连接
			go s.handleConnection(conn)
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() {
	close(s.stopCh)
	s.listener.Close()
	close(s.dataStream)
}

// SendData 发送数据到所有连接的客户端
func (s *Server) SendData(data *models.MetricsData) {
	select {
	case s.dataStream <- data:
		// 成功发送到通道
	default:
		// 通道已满，丢弃数据
		log.Printf("QUIC数据通道已满，丢弃数据")
	}
}

// BroadcastData 广播数据到所有连接
func (s *Server) BroadcastData(data *models.MetricsData) {
	// 这里可以实现向所有连接的客户端广播数据
	// 暂时通过通道发送，实际实现需要维护连接列表
	log.Printf("广播QUIC数据: HostID=%s", data.HostID)
}

// GetMetricsData 获取指标数据通道
func (s *Server) GetMetricsData() <-chan *models.MetricsData {
	return s.dataStream
}

// handleConnection 处理QUIC连接
func (s *Server) handleConnection(conn *quic.Conn) {
	log.Printf("QUIC连接已建立，远程地址: %s", conn.RemoteAddr())

	// 添加连接到管理列表
	s.addConnection(conn)
	defer func() {
		conn.CloseWithError(0, "正常关闭")
		s.removeConnection(conn)
		log.Printf("QUIC连接已关闭: %s", conn.RemoteAddr())
	}()

	// 接受数据流
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("接受QUIC流失败: %v", err)
				return
			}
		}

		go s.handleStream(stream)
	}
}

// addConnection 添加连接
func (s *Server) addConnection(conn *quic.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = struct{}{}
	log.Printf("QUIC连接数: %d", len(s.connections))
}

// removeConnection 移除连接
func (s *Server) removeConnection(conn *quic.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	delete(s.connections, conn)
	log.Printf("QUIC连接数: %d", len(s.connections))
}

// GetConnectionCount 获取连接数
func (s *Server) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return len(s.connections)
}

// handleStream 处理QUIC数据流
func (s *Server) handleStream(stream *quic.Stream) {
	defer stream.Close()

	// 处理客户端请求
	buffer := make([]byte, 1024)
	for {
		n, err := stream.Read(buffer)
		if err != nil {
			log.Printf("读取QUIC流失败: %v", err)
			return
		}

		// 处理接收到的数据
		data := buffer[:n]
		log.Printf("接收到QUIC数据: %d 字节", len(data))

		// 简单的回显处理
		if _, err := stream.Write(data); err != nil {
			log.Printf("写入QUIC流失败: %v", err)
			return
		}
	}
}

// generateTLSConfig 生成TLS配置
func generateTLSConfig() (*tls.Config, error) {
	// 生成私钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour * 24 * 180), // 180天
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	// 创建自签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	// 编码为PEM格式
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	// 加载证书
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	// 创建TLS配置
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"},
	}, nil
}

// Client QUIC客户端 (暂时简化实现)
type Client struct {
	config   *config.Config
	stopCh   chan struct{}
	dataChan chan *models.MetricsData
}

// NewClient 创建新的QUIC客户端
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config:   cfg,
		stopCh:   make(chan struct{}),
		dataChan: make(chan *models.MetricsData, cfg.QUIC.BufferSize),
	}
}

// Connect 连接到QUIC服务器
func (c *Client) Connect(addr string) error {
	// 生成TLS配置
	tlsConfig, err := c.generateTLSConfig()
	if err != nil {
		return fmt.Errorf("生成TLS配置失败: %v", err)
	}

	// 创建QUIC连接
	conn, err := quic.DialAddr(context.Background(), addr, tlsConfig, &quic.Config{
		KeepAlivePeriod: c.config.QUIC.KeepAlivePeriod,
		MaxIdleTimeout:  c.config.QUIC.MaxIdleTimeout,
	})
	if err != nil {
		return fmt.Errorf("连接QUIC服务器失败: %v", err)
	}

	log.Printf("QUIC客户端已连接到: %s", addr)

	// 启动数据接收协程
	go c.receiveData(conn)

	return nil
}

// receiveData 接收数据
func (c *Client) receiveData(conn *quic.Conn) {
	defer conn.CloseWithError(0, "客户端关闭")

	for {
		select {
		case <-c.stopCh:
			return
		default:
			// 打开新的流
			stream, err := conn.OpenStreamSync(context.Background())
			if err != nil {
				log.Printf("打开QUIC流失败: %v", err)
				return
			}

			// 发送测试数据
			testData := []byte("Hello QUIC Server")
			if _, err := stream.Write(testData); err != nil {
				log.Printf("发送数据失败: %v", err)
				stream.Close()
				continue
			}

			// 读取响应
			buffer := make([]byte, 1024)
			n, err := stream.Read(buffer)
			if err != nil {
				log.Printf("读取响应失败: %v", err)
				stream.Close()
				continue
			}

			log.Printf("接收到响应: %s", string(buffer[:n]))
			stream.Close()

			// 等待一段时间再发送下一个请求
			time.Sleep(5 * time.Second)
		}
	}
}

// generateTLSConfig 生成客户端TLS配置
func (c *Client) generateTLSConfig() (*tls.Config, error) {
	return &tls.Config{
		InsecureSkipVerify: true, // 测试环境跳过证书验证
		NextProtos:         []string{"h3"},
	}, nil
}

// Close 关闭连接
func (c *Client) Close() {
	close(c.stopCh)
	close(c.dataChan)
}

// GetDataChannel 获取数据通道
func (c *Client) GetDataChannel() <-chan *models.MetricsData {
	return c.dataChan
}
