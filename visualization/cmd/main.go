package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/han-fei/monitor/visualization/internal/analysis"
	"github.com/han-fei/monitor/visualization/internal/api"
	"github.com/han-fei/monitor/visualization/internal/auth"
	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
	"github.com/han-fei/monitor/visualization/internal/quic"
	"github.com/han-fei/monitor/visualization/internal/radix"
	"github.com/han-fei/monitor/visualization/internal/service"
	"github.com/han-fei/monitor/visualization/internal/websocket"
)

// OldServerAdapter 兼容原有QUIC服务器的适配器
type OldServerAdapter struct {
	server *quic.Server
}

func (a *OldServerAdapter) Start() error                           { return a.server.Start() }
func (a *OldServerAdapter) Stop() error                            { a.server.Stop(); return nil }
func (a *OldServerAdapter) SendData(data *models.MetricsData)      { a.server.SendData(data) }
func (a *OldServerAdapter) BroadcastData(data *models.MetricsData) { a.server.BroadcastData(data) }
func (a *OldServerAdapter) GetMetricsData() <-chan *models.MetricsData {
	return a.server.GetMetricsData()
}
func (a *OldServerAdapter) GetConnectionCount() int { return a.server.GetConnectionCount() }

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/visualization.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建JWT认证
	jwtConfig := auth.JWTConfig{
		Secret:            cfg.Auth.JWTSecret,
		TokenExpiry:       cfg.Auth.TokenExpiry,
		RefreshExpiry:     cfg.Auth.RefreshTokenExpiry,
		CookieSecure:      cfg.Auth.CookieSecure,
		CookieHTTPOnly:    cfg.Auth.CookieHTTPOnly,
		CookieName:        cfg.Auth.CookieName,
		RefreshCookieName: cfg.Auth.RefreshCookieName,
	}
	jwtAuth := auth.NewJWTAuth(jwtConfig)

	// 创建认证处理器
	authHandler := auth.NewAuthHandler(jwtAuth)

	log.Printf("JWT认证初始化完成")

	// 创建基数树
	ruleTree := radix.NewRadixTree()
	log.Printf("基数树初始化完成")

	// 添加一些示例规则
	_ = ruleTree.Insert("host-1", "高优先级")
	_ = ruleTree.Insert("host-2", "中优先级")
	_ = ruleTree.Insert("host-3", "低优先级")

	// 创建数据分析器
	analyzer := analysis.NewAnalyzer()
	log.Printf("数据分析器初始化完成")

	// 创建WebSocket服务器
	wsServer := websocket.NewServer(cfg)
	wsServer.Start()
	log.Printf("WebSocket服务器初始化完成")

	// 创建gRPC客户端
	grpcClient := service.NewGRPCClient(cfg)
	if err := grpcClient.Connect(); err != nil {
		log.Printf("连接到broker失败: %v", err)
	} else {
		log.Printf("已连接到broker")
	}
	defer grpcClient.Close()

	// 创建API处理器
	apiHandler := api.NewAPIHandler(analyzer, grpcClient)
	log.Printf("API处理器初始化完成")

	// 创建集成的QUIC服务器（支持新旧实现切换）
	var quicServer interface {
		Start() error
		Stop() error
		SendData(*models.MetricsData)
		BroadcastData(*models.MetricsData)
		GetMetricsData() <-chan *models.MetricsData
		GetConnectionCount() int
	}

	// 优先使用新的QUIC实现
	integratedServer, err := quic.NewIntegratedServer(cfg)
	if err != nil {
		log.Printf("创建集成QUIC服务器失败: %v", err)
		// 回退到原有实现
		if cfg.QUIC.Enable {
			oldServer, err := quic.NewServer(cfg)
			if err != nil {
				log.Printf("创建原有QUIC服务器失败: %v", err)
			} else {
				quicServer = &OldServerAdapter{server: oldServer}
			}
		}
	} else {
		quicServer = integratedServer
	}

	// 启动QUIC服务器
	if quicServer != nil {
		if err := quicServer.Start(); err != nil {
			log.Printf("启动QUIC服务器失败: %v", err)
		} else {
			log.Printf("QUIC服务器已启动")
			defer quicServer.Stop()
		}
	}

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// 注册认证路由
	authHandler.RegisterRoutes(mux)

	// 注册WebSocket处理程序（需要认证）
	mux.Handle("/ws", jwtAuth.RequireAuth(http.HandlerFunc(wsServer.HandleWebSocket)))

	// 注册API路由
	apiHandler.RegisterRoutes(mux)

	// 注册状态检查路由
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"running","version":"1.0.0"}`))
	})

	// 注册管理员路由
	mux.Handle("/api/admin", jwtAuth.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Admin endpoint"}`))
	})))

	// 注册静态文件服务
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// 启动HTTP服务器
	go func() {
		log.Printf("HTTP服务器启动，监听端口: %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务器错误: %v", err)
		}
	}()

	// 启动数据拉取协程
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pullDataFromBroker(ctx, grpcClient, wsServer, quicServer)

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 优雅关闭
	log.Println("正在关闭服务...")

	// 关闭HTTP服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP服务器关闭错误: %v", err)
	}

	log.Println("服务已关闭")
}

// pullDataFromBroker 从broker拉取数据
func pullDataFromBroker(ctx context.Context, client *service.GRPCClient, wsServer *websocket.Server, quicServer interface {
	SendData(*models.MetricsData)
}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 获取当前时间和1分钟前的时间
			now := time.Now()
			oneMinuteAgo := now.Add(-1 * time.Minute)

			// 获取所有主机的数据（简化处理，实际应该有主机列表）
			hostIDs := []string{"host-1", "host-2", "host-3"}

			for _, hostID := range hostIDs {
				// 从broker获取数据
				metrics, err := client.GetMetricsWithRetry(ctx, hostID, oneMinuteAgo, now)
				if err != nil {
					log.Printf("获取主机 %s 的数据失败: %v", hostID, err)
					continue
				}

				// 发送数据到WebSocket和QUIC
				for _, data := range metrics {
					wsServer.Broadcast(data)

					if quicServer != nil {
						quicServer.SendData(data)
					}
				}
			}
		}
	}
}
