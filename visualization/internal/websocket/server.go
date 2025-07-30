package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// Client WebSocket客户端
type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	server   *Server
	mu       sync.Mutex
	closed   bool
	lastPing time.Time
}

// Server WebSocket服务器
type Server struct {
	config     *config.Config
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	upgrader   websocket.Upgrader
	mu         sync.RWMutex
}

// NewServer 创建新的WebSocket服务器
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config:     cfg,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, cfg.WebSocket.BufferSize),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境应该限制
			},
		},
	}
}

// Start 启动WebSocket服务器
func (s *Server) Start() {
	go s.run()
}

// Stop 停止WebSocket服务器
func (s *Server) Stop() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		client.close()
	}
}

// HandleWebSocket 处理WebSocket连接
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("升级WebSocket连接失败: %v", err)
		return
	}

	// 创建客户端
	client := &Client{
		conn:     conn,
		send:     make(chan []byte, s.config.WebSocket.BufferSize),
		server:   s,
		lastPing: time.Now(),
	}

	// 注册客户端
	s.register <- client

	// 启动读写协程
	go client.readPump()
	go client.writePump()
}

// Broadcast 广播数据
func (s *Server) Broadcast(data *models.MetricsData) {
	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("序列化数据失败: %v", err)
		return
	}

	s.broadcast <- jsonData
}

// run WebSocket服务器主循环
func (s *Server) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-s.register:
			// 注册客户端
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()
			log.Printf("客户端已连接，当前连接数: %d", len(s.clients))

		case client := <-s.unregister:
			// 注销客户端
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			s.mu.Unlock()
			log.Printf("客户端已断开，当前连接数: %d", len(s.clients))

		case message := <-s.broadcast:
			// 广播消息
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
					// 成功发送
				default:
					// 客户端缓冲区已满，关闭连接
					client.close()
				}
			}
			s.mu.RUnlock()

		case <-ticker.C:
			// 定期检查客户端心跳
			s.mu.RLock()
			for client := range s.clients {
				if time.Since(client.lastPing) > s.config.WebSocket.PingTimeout {
					client.close()
				} else {
					client.ping()
				}
			}
			s.mu.RUnlock()
		}
	}
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.server.config.WebSocket.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.server.config.WebSocket.PongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.server.config.WebSocket.PongTimeout))
		c.mu.Lock()
		c.lastPing = time.Now()
		c.mu.Unlock()
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("读取WebSocket消息错误: %v", err)
			}
			break
		}
		// 目前不处理客户端消息
	}
}

// writePump 发送消息到客户端
func (c *Client) writePump() {
	ticker := time.NewTicker(c.server.config.WebSocket.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.server.config.WebSocket.WriteTimeout))
			if !ok {
				// 通道已关闭
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 将队列中的所有消息添加到当前WebSocket消息中
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.ping()
		}
	}
}

// ping 发送ping消息
func (c *Client) ping() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.server.config.WebSocket.WriteTimeout))
	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		c.close()
	}
}

// close 关闭客户端连接
func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	c.conn.Close()
	c.server.unregister <- c
}
