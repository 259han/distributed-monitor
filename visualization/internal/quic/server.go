package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// Server QUIC服务器
type Server struct {
	config     *config.Config
	listener   quic.Listener
	dataStream chan *models.MetricsData
	stopCh     chan struct{}
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
		config:     cfg,
		listener:   listener,
		dataStream: make(chan *models.MetricsData, cfg.QUIC.BufferSize),
		stopCh:     make(chan struct{}),
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

// SendData 发送数据
func (s *Server) SendData(data *models.MetricsData) {
	select {
	case s.dataStream <- data:
		// 成功发送
	default:
		// 通道已满，丢弃数据
		log.Printf("QUIC数据通道已满，丢弃数据")
	}
}

// handleConnection 处理连接
func (s *Server) handleConnection(conn quic.Connection) {
	// 打开流
	stream, err := conn.OpenStream()
	if err != nil {
		log.Printf("打开QUIC流失败: %v", err)
		conn.CloseWithError(1, "打开流失败")
		return
	}
	defer stream.Close()

	// 发送数据
	encoder := json.NewEncoder(stream)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case data := <-s.dataStream:
			if err := encoder.Encode(data); err != nil {
				log.Printf("编码数据失败: %v", err)
				return
			}
		case <-ticker.C:
			// 定期检查连接状态
			if !conn.Context().Done().Done() {
				continue
			}
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

// Client QUIC客户端
type Client struct {
	config   *config.Config
	conn     quic.Connection
	stream   quic.Stream
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

// Connect 连接到服务器
func (c *Client) Connect(addr string) error {
	// 创建TLS配置
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}

	// 连接到服务器
	conn, err := quic.DialAddr(addr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("连接到QUIC服务器失败: %v", err)
	}
	c.conn = conn

	// 打开流
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		conn.CloseWithError(1, "接受流失败")
		return fmt.Errorf("接受QUIC流失败: %v", err)
	}
	c.stream = stream

	// 启动接收协程
	go c.receiveData()

	return nil
}

// Close 关闭连接
func (c *Client) Close() {
	close(c.stopCh)
	if c.stream != nil {
		c.stream.Close()
	}
	if c.conn != nil {
		c.conn.CloseWithError(0, "正常关闭")
	}
	close(c.dataChan)
}

// GetDataChannel 获取数据通道
func (c *Client) GetDataChannel() <-chan *models.MetricsData {
	return c.dataChan
}

// receiveData 接收数据
func (c *Client) receiveData() {
	decoder := json.NewDecoder(c.stream)
	for {
		select {
		case <-c.stopCh:
			return
		default:
			// 解码数据
			var data models.MetricsData
			if err := decoder.Decode(&data); err != nil {
				if err == io.EOF {
					return
				}
				log.Printf("解码数据失败: %v", err)
				continue
			}

			// 发送到通道
			select {
			case c.dataChan <- &data:
				// 成功发送
			default:
				// 通道已满，丢弃数据
				log.Printf("QUIC客户端数据通道已满，丢弃数据")
			}
		}
	}
}
