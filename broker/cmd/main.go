package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	cluster "github.com/han-fei/monitor/broker/internal/cluster"
	"github.com/han-fei/monitor/broker/internal/config"
	"github.com/han-fei/monitor/broker/internal/host"
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
	brokerCluster := models.NewCluster()
	for _, endpoint := range cfg.Cluster.Endpoints {
		brokerCluster.AddNode(models.NewNode(endpoint, endpoint))
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

	// Note: Log initialization removed to avoid conflicts with Raft

	// 创建Redis配置
	redisConfig := storage.RedisConfig{
		Address:   cfg.Storage.Redis.Addr,
		Password:  cfg.Storage.Redis.Password,
		DB:        cfg.Storage.Redis.DB,
		KeyPrefix: cfg.Storage.Redis.KeyPrefix,
	}

	// 创建Raft配置
	raftConfig := &raft.Config{
		NodeID:           cfg.Node.ID,
		NodeAddr:         cfg.Node.Address,
		DataDir:          cfg.Raft.LogDir + "/data",
		LogDir:           cfg.Raft.LogDir,
		SnapshotDir:      cfg.Raft.SnapshotDir,
		GRPCPort:         cfg.RaftGRPC.Port,
		Peers:            cfg.Cluster.Endpoints,
		HeartbeatTimeout: cfg.Raft.HeartbeatTimeout,
		ElectionTimeout:  cfg.Raft.ElectionTimeout,
		CommitTimeout:    1 * time.Second, // 默认值
		MaxPool:          3,               // 默认值
	}

	// 创建Redis存储
	redisStorage, err := storage.NewRedisStorage(redisConfig)
	if err != nil {
		log.Fatalf("创建Redis存储失败: %v", err)
	}
	defer redisStorage.Close()

	// 创建Raft服务器，并传入Redis存储
	raftServer, err := raft.NewServer(raftConfig, redisStorage)
	if err != nil {
		log.Fatalf("创建Raft服务器失败: %v", err)
	}

	// 创建主机管理器配置
	hostManagerConfig := &host.ManagerConfig{
		RegistryConfig: &host.RegistryConfig{
			HeartbeatInterval:   cfg.HostManagement.Registry.HeartbeatInterval,
			HealthCheckInterval: cfg.HostManagement.Registry.HealthCheckInterval,
			OfflineTimeout:      cfg.HostManagement.Registry.OfflineTimeout,
			CleanupInterval:     cfg.HostManagement.Registry.CleanupInterval,
		},
		DiscoveryConfig: &host.DiscoveryConfig{
			EnableHTTP:       true,
			EnableMulticast:  cfg.HostManagement.Discovery.EnableMulticast,
			EnableFileWatch:  cfg.HostManagement.Discovery.EnableFileWatch,
			ScanInterval:     cfg.HostManagement.Discovery.ScanInterval,
			ScanTimeout:      cfg.HostManagement.Discovery.ScanTimeout,
			NetworkRanges:    cfg.HostManagement.Discovery.NetworkRanges,
			DiscoveryTypes:   cfg.HostManagement.Discovery.DiscoveryTypes,
			ServicePortRange: cfg.HostManagement.Discovery.ServicePortRange,
		},
		HealthConfig: &host.HealthConfig{
			CheckInterval:   cfg.HostManagement.Health.CheckInterval,
			Timeout:         cfg.HostManagement.Health.Timeout,
			MaxRetries:      cfg.HostManagement.Health.MaxRetries,
			EnableTCP:       cfg.HostManagement.Health.EnableTCP,
			EnableHTTP:      cfg.HostManagement.Health.EnableHTTP,
			TCPPorts:        cfg.HostManagement.Health.TCPPorts,
			HTTPPaths:       cfg.HostManagement.Health.HTTPPaths,
			AlertThresholds: cfg.HostManagement.Health.AlertThresholds,
		},
		AutoRemove:    cfg.HostManagement.AutoRemoveCfg.Enabled,
		RemoveTimeout: cfg.HostManagement.AutoRemoveCfg.UnhealthyTimeout,
		MaxHosts:      cfg.HostManagement.MaxHosts,
		MinHosts:      cfg.HostManagement.Scaling.MinHosts,
		EnableScaling: cfg.HostManagement.EnableScaling,
		ScalingConfig: &host.ScalingConfig{
			EnableHorizontalScale: true,
			ScaleUpThreshold:      float64(cfg.HostManagement.Scaling.ScaleUpThreshold),
			ScaleDownThreshold:    float64(cfg.HostManagement.Scaling.ScaleDownThreshold),
			CooldownPeriod:        5 * time.Minute,
			ScaleStep:             1,
			MinHosts:              cfg.HostManagement.Scaling.MinHosts,
			MaxHosts:              cfg.HostManagement.Scaling.MaxHosts,
			ResourceWeights: host.ResourceWeights{
				CPU:     0.3,
				Memory:  0.3,
				Disk:    0.2,
				Network: 0.2,
			},
		},
	}

	// 创建主机管理器
	hostManager := host.NewHostManager(hostManagerConfig)
	if cfg.HostManagement.Enabled {
		if err := hostManager.Start(context.Background()); err != nil {
			log.Fatalf("启动主机管理器失败: %v", err)
		}
		defer hostManager.Stop()
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

	// 创建Kafka消费者（如果启用）
	var kafkaConsumer *service.KafkaConsumer
	if cfg.Kafka.Enabled {
		kafkaConsumer = service.NewKafkaConsumer(&cfg.Kafka, redisStorage)
		log.Printf("Kafka消费者已创建，连接到: %v, 主题: %s", cfg.Kafka.Brokers, cfg.Kafka.Topic)
		
		// 启动Kafka消费者
		if err := kafkaConsumer.Start(context.Background()); err != nil {
			log.Fatalf("启动Kafka消费者失败: %v", err)
		}
		defer kafkaConsumer.Stop()
	}

	// 启动gRPC服务器（这会同时启动Raft服务器）
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("启动gRPC服务器失败: %v", err)
		}
	}()

	// 等待gRPC服务器启动
	time.Sleep(3 * time.Second)

	// 创建集群管理器（在Raft服务器启动之后）
	clusterManagerConfig := &cluster.Config{
		HTTPPort:        9097, // 集群管理API端口
		RaftServer:      raftServer,
		HostManager:     hostManager,
		Storage:         redisStorage,
		MetricsInterval: 30 * time.Second,
	}

	clusterManager := cluster.NewClusterManager(clusterManagerConfig)
	if err := clusterManager.Start(); err != nil {
		log.Fatalf("启动集群管理器失败: %v", err)
	}
	defer clusterManager.Stop()

	// 检查gRPC服务器是否真的启动了
	log.Println("Broker启动完成，等待连接...")
	log.Println("集群管理API启动完成，监听端口: 9097")

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动后台任务
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 打印统计信息
				if cfg.HostManagement.Enabled {
					stats := hostManager.GetStats()
					log.Printf("主机统计: %+v", stats)
				}
			case <-sigCh:
				return
			}
		}
	}()

	<-sigCh

	// 优雅关闭
	log.Println("正在关闭服务...")
	grpcServer.Stop()
	log.Println("服务已关闭")
}
