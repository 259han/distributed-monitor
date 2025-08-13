package quic

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"

	"github.com/quic-go/quic-go"

	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// EnhancedServer 增强的QUIC服务器
type EnhancedServer struct {
	config      *config.Config
	listener    *quic.Listener
	dataStream  chan *models.MetricsData
	stopCh      chan struct{}
	connections map[*quic.Conn]struct{}
	connMutex   sync.RWMutex
}

// NewEnhancedServer 创建新的增强QUIC服务器
func NewEnhancedServer(cfg *config.Config) (*EnhancedServer, error) {
	// 生成TLS配置
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("生成TLS配置失败: %v", err)
	}

	// 创建UDP监听器，并设置更大的缓冲区
	udpConn, err := createUDPSocketWithLargeBuffer(fmt.Sprintf(":%d", cfg.QUIC.Port))
	if err != nil {
		return nil, fmt.Errorf("创建UDP套接字失败: %v", err)
	}

	// 创建QUIC监听器
	listener, err := quic.Listen(udpConn, tlsConfig, &quic.Config{
		MaxIdleTimeout:        cfg.QUIC.MaxIdleTimeout,
		KeepAlivePeriod:       cfg.QUIC.KeepAlivePeriod,
		MaxIncomingStreams:    int64(cfg.QUIC.MaxStreamsPerConn),
		MaxIncomingUniStreams: int64(cfg.QUIC.MaxStreamsPerConn),
	})
	if err != nil {
		return nil, fmt.Errorf("创建QUIC监听器失败: %v", err)
	}

	return &EnhancedServer{
		config:      cfg,
		listener:    listener,
		dataStream:  make(chan *models.MetricsData, cfg.QUIC.BufferSize),
		stopCh:      make(chan struct{}),
		connections: make(map[*quic.Conn]struct{}),
	}, nil
}

// createUDPSocketWithLargeBuffer 创建具有大缓冲区的UDP套接字
func createUDPSocketWithLargeBuffer(addr string) (net.PacketConn, error) {
	// 解析地址
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	// 创建套接字
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	// 设置接收缓冲区大小
	// 尝试设置为7MB (7168KB)
	if err := conn.SetReadBuffer(7 * 1024 * 1024); err != nil {
		log.Printf("警告: 无法设置UDP读取缓冲区大小: %v", err)
	}

	// 设置发送缓冲区大小
	if err := conn.SetWriteBuffer(7 * 1024 * 1024); err != nil {
		log.Printf("警告: 无法设置UDP写入缓冲区大小: %v", err)
	}

	// 获取文件描述符并设置SO_REUSEADDR选项
	file, err := conn.File()
	if err != nil {
		log.Printf("警告: 无法获取UDP套接字文件描述符: %v", err)
		return conn, nil
	}
	defer file.Close()

	fd := int(file.Fd())
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		log.Printf("警告: 无法设置SO_REUSEADDR选项: %v", err)
	}

	return conn, nil
}

// Start 启动服务器
func (s *EnhancedServer) Start() error {
	log.Printf("集成QUIC服务器启动，监听端口: %d", s.config.QUIC.Port)

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
func (s *EnhancedServer) Stop() error {
	close(s.stopCh)
	err := s.listener.Close()
	close(s.dataStream)
	return err
}

// SendData 发送数据
func (s *EnhancedServer) SendData(data *models.MetricsData) {
	select {
	case s.dataStream <- data:
		// 成功发送到通道
	default:
		// 通道已满，丢弃数据
		log.Printf("QUIC数据通道已满，丢弃数据")
	}
}

// BroadcastData 广播数据
func (s *EnhancedServer) BroadcastData(data *models.MetricsData) {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for conn := range s.connections {
		go func(c *quic.Conn) {
			// 打开新的流
			stream, err := c.OpenStreamSync(context.Background())
			if err != nil {
				log.Printf("打开QUIC流失败: %v", err)
				return
			}
			defer stream.Close()

			// 序列化数据
			// 简化实现，实际应该使用更高效的序列化方式
			message := fmt.Sprintf("HostID: %s, Timestamp: %v, Metrics: %d", 
				data.HostID, data.Timestamp, len(data.Metrics))
			
			// 发送数据
			if _, err := stream.Write([]byte(message)); err != nil {
				log.Printf("发送QUIC数据失败: %v", err)
			}
		}(conn)
	}
}

// GetMetricsData 获取指标数据通道
func (s *EnhancedServer) GetMetricsData() <-chan *models.MetricsData {
	return s.dataStream
}

// GetConnectionCount 获取连接数
func (s *EnhancedServer) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return len(s.connections)
}

// handleConnection 处理QUIC连接
func (s *EnhancedServer) handleConnection(conn *quic.Conn) {
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
func (s *EnhancedServer) addConnection(conn *quic.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = struct{}{}
	log.Printf("QUIC连接数: %d", len(s.connections))
}

// removeConnection 移除连接
func (s *EnhancedServer) removeConnection(conn *quic.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	delete(s.connections, conn)
	log.Printf("QUIC连接数: %d", len(s.connections))
}

// handleStream 处理QUIC数据流
func (s *EnhancedServer) handleStream(stream *quic.Stream) {
	defer stream.Close()

	// 处理客户端请求
	buffer := make([]byte, 4096)
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