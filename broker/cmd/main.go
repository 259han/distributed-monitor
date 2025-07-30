package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/han-fei/monitor/broker/internal/config"
	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/han-fei/monitor/broker/internal/raft"
	"github.com/han-fei/monitor/broker/internal/service"
	"github.com/han-fei/monitor/broker/internal/storage"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/broker.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建节点
	node := models.NewNode(cfg.Node.ID, cfg.Node.Address)
	log.Printf("节点启动: %s (%s)", node.ID, node.Address)

	// 创建集群
	cluster := models.NewCluster()
	for _, endpoint := range cfg.Cluster.Endpoints {
		cluster.AddNode(models.NewNode(endpoint, endpoint))
	}

	// 创建日志目录
	logPath := cfg.Raft.LogDir
	if err := os.MkdirAll(logPath, 0755); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}

	// 创建快照目录
	snapshotPath := cfg.Raft.SnapshotDir
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		log.Fatalf("创建快照目录失败: %v", err)
	}

	// 初始化日志（暂时不使用，但保留初始化）
	_ = models.NewLog(logPath)

	// 创建Redis配置
	redisConfig := storage.RedisConfig{
		Address:  cfg.Storage.Redis.Addr,
		Password: cfg.Storage.Redis.Password,
		DB:       cfg.Storage.Redis.DB,
	}

	// 创建Raft配置
	raftConfig := &raft.Config{
		NodeID:           cfg.Node.ID,
		NodeAddr:         cfg.Node.Address,
		DataDir:          cfg.Raft.LogDir,
		LogDir:           cfg.Raft.LogDir,
		SnapshotDir:      cfg.Raft.SnapshotDir,
		GRPCPort:         cfg.GRPC.Port,
		Peers:            cfg.Cluster.Endpoints,
		HeartbeatTimeout: cfg.Raft.HeartbeatTimeout,
		ElectionTimeout:  cfg.Raft.ElectionTimeout,
		CommitTimeout:    1 * time.Second, // 默认值
		MaxPool:          3,               // 默认值
	}

	// 创建gRPC服务器配置
	grpcConfig := &service.Config{
		Port:           cfg.GRPC.Port,
		MaxRecvMsgSize: cfg.GRPC.MaxRecvMsgSize,
		MaxSendMsgSize: cfg.GRPC.MaxSendMsgSize,
		VirtualNodes:   cfg.Hash.VirtualNodes,
		StorageConfig:  redisConfig,
		RaftConfig:     raftConfig,
	}

	// 创建gRPC服务器
	grpcServer, err := service.NewGRPCServer(grpcConfig)
	if err != nil {
		log.Fatalf("创建gRPC服务器失败: %v", err)
	}

	// 启动gRPC服务器
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("启动gRPC服务器失败: %v", err)
		}
	}()

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 优雅关闭
	log.Println("正在关闭服务...")
	grpcServer.Stop()
	log.Println("服务已关闭")
}
