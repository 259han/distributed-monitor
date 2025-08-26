package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/han-fei/monitor/broker/internal/host"
	"github.com/han-fei/monitor/broker/internal/queue"
	"github.com/han-fei/monitor/broker/internal/raft"
	"github.com/han-fei/monitor/broker/internal/storage"
	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/han-fei/monitor/pkg/hash"
	pb "github.com/han-fei/monitor/proto"
)

// GRPCServer gRPC服务器
type GRPCServer struct {
	config          *Config
	server          *grpc.Server
	hashRing        *hash.ConsistentHash
	raftServer      *raft.Server
	storage         *storage.RedisStorage
	hostManager     *host.HostManager
	messageQueue    *queue.RedisMessageQueue
	streamProcessor *queue.StreamProcessor
	mu              sync.RWMutex
	pb.UnimplementedMonitorServiceServer
}

// Config gRPC服务器配置
type Config struct {
	Port           int
	MaxRecvMsgSize int
	MaxSendMsgSize int
	VirtualNodes   int
	StorageConfig  storage.RedisConfig
	RaftConfig     *raft.Config
	HostManager    *host.HostManager
}

// NewGRPCServer 创建新的gRPC服务器
func NewGRPCServer(config *Config) (*GRPCServer, error) {
	// 创建一致性哈希环
	hashRing := hash.NewConsistentHash(config.VirtualNodes, nil)

	// 创建Redis存储
	redisStorage, err := storage.NewRedisStorage(config.StorageConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Redis存储失败: %v", err)
	}
	log.Printf("Redis存储连接成功: %s", config.StorageConfig.Address)

	// 创建Raft服务器，并传入Redis存储
	raftServer, err := raft.NewServer(config.RaftConfig, redisStorage)
	if err != nil {
		return nil, fmt.Errorf("创建Raft服务器失败: %v", err)
	}

	// 创建gRPC服务器选项
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
	}

	// 创建gRPC服务器
	server := grpc.NewServer(opts...)

	// 创建消息队列
	messageQueue := queue.NewRedisMessageQueue(redisStorage, "queue", 24*time.Hour)

	// 创建流处理器
	streamProcessor := queue.NewStreamProcessor(messageQueue)

	// 创建服务器实例
	s := &GRPCServer{
		config:          config,
		server:          server,
		hashRing:        hashRing,
		raftServer:      raftServer,
		storage:         redisStorage,
		hostManager:     config.HostManager,
		messageQueue:    messageQueue,
		streamProcessor: streamProcessor,
	}

	// 注册默认处理器
	if err := streamProcessor.RegisterProcessor("metrics", s.handleMetricsMessage); err != nil {
		return nil, fmt.Errorf("注册指标处理器失败: %v", err)
	}

	if err := streamProcessor.RegisterProcessor("alerts", s.handleAlertMessage); err != nil {
		return nil, fmt.Errorf("注册告警处理器失败: %v", err)
	}

	// 注册服务
	pb.RegisterMonitorServiceServer(server, s)

	// 启用反射服务，用于grpcurl等工具
	reflection.Register(server)

	return s, nil
}

// Start 启动服务器
func (s *GRPCServer) Start() error {
	log.Printf("正在启动gRPC服务器...")

	// 启动Raft服务器
	log.Printf("正在启动Raft服务器...")
	if err := s.raftServer.Start(); err != nil {
		log.Printf("启动Raft服务器失败: %v", err)
		return err
	}
	log.Printf("Raft服务器启动成功")

	// 添加节点到一致性哈希环
	log.Printf("正在初始化一致性哈希环...")
	if len(s.config.RaftConfig.Peers) > 0 {
		s.hashRing.Add(s.config.RaftConfig.Peers...)
		log.Printf("已添加 %d 个节点到一致性哈希环", len(s.config.RaftConfig.Peers))
	} else {
		// 如果没有配置节点，添加本地节点
		s.hashRing.Add(s.config.RaftConfig.NodeAddr)
		log.Printf("已添加本地节点到一致性哈希环: %s", s.config.RaftConfig.NodeAddr)
	}
	log.Printf("一致性哈希环初始化完成")

	// 监听端口
	log.Printf("正在监听端口: %d", s.config.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		log.Printf("监听端口失败: %v", err)
		return err
	}

	// 启动gRPC服务器
	log.Printf("gRPC服务器启动，监听端口: %d", s.config.Port)
	return s.server.Serve(lis)
}

// Stop 停止服务器
func (s *GRPCServer) Stop() {
	// 停止gRPC服务器
	s.server.GracefulStop()

	// 停止Raft服务器
	s.raftServer.Stop()

	// 关闭存储
	s.storage.Close()
}

// SendMetrics 实现MonitorService.SendMetrics
func (s *GRPCServer) SendMetrics(stream pb.MonitorService_SendMetricsServer) error {
	var count int
	batchMap := make(map[string][]interface{})

	for {
		// 接收数据
		data, err := stream.Recv()
		if err == io.EOF {
			// 流结束，处理剩余批次
			log.Printf("gRPC流结束，开始处理剩余批次")
			for node, items := range batchMap {
				if len(items) > 0 {
					log.Printf("处理节点 %s 的剩余批次，数量: %d", node, len(items))
					if err := s.processBatch(node, items); err != nil {
						return err
					}
				}
			}

			// 返回响应
			return stream.SendAndClose(&pb.MetricsResponse{
				Success: true,
				Message: fmt.Sprintf("成功处理%d条数据", count),
			})
		}
		if err != nil {
			return err
		}

		// 记录接收到的数据
		log.Printf("Broker接收到来自Agent的数据: HostID=%s, 指标数=%d, Timestamp=%d", data.HostId, len(data.Metrics), data.Timestamp)

		// 计数
		count++

		// 使用一致性哈希确定存储节点
		key := fmt.Sprintf("%s:%d", data.HostId, data.Timestamp)
		node, err := s.hashRing.Get(key)
		if err != nil {
			log.Printf("获取存储节点失败: %v", err)
			continue
		}

		// 添加到对应节点的批次
		if _, ok := batchMap[node]; !ok {
			batchMap[node] = make([]interface{}, 0)
		}
		batchMap[node] = append(batchMap[node], data)

		// 如果批次达到一定大小，处理批次（设置为1，立即处理）
		if len(batchMap[node]) >= 1 {
			log.Printf("批次大小达到阈值，开始处理 %d 条数据", len(batchMap[node]))
			if err := s.processBatch(node, batchMap[node]); err != nil {
				return err
			}
			batchMap[node] = batchMap[node][:0]
		}
	}
}

// GetMetrics 实现MonitorService.GetMetrics
func (s *GRPCServer) GetMetrics(req *pb.MetricsRequest, stream pb.MonitorService_GetMetricsServer) error {
	// 使用存储层的时间范围查询，兼容哈希结构与时间索引
	ctx := context.Background()
	metricsList, err := s.storage.GetMetricsDataRange(ctx, req.HostId, req.StartTime, req.EndTime)
	if err != nil {
		return err
	}

	for _, data := range metricsList {
		// 将存储数据转换为 protobuf 结构（兼容 value 或 {value,unit} 存储格式）
		var pbMetrics []*pb.Metric
		for name, raw := range data.Metrics {
			valStr := ""
			unitStr := ""
			// 兼容两种形态
			switch v := raw.(type) {
			case map[string]interface{}:
				if vv, ok := v["value"]; ok {
					valStr = fmt.Sprintf("%v", vv)
				} else {
					valStr = fmt.Sprintf("%v", v)
				}
				if uu, ok := v["unit"].(string); ok {
					unitStr = uu
				}
			default:
				valStr = fmt.Sprintf("%v", raw)
			}

			pbMetrics = append(pbMetrics, &pb.Metric{
				Name:  name,
				Value: valStr,
				Unit:  unitStr,
			})
		}

		out := &pb.MetricsData{
			HostId:    data.HostID,
			Timestamp: data.Timestamp,
			Metrics:   pbMetrics,
		}

		if err := stream.Send(out); err != nil {
			return err
		}
	}
	return nil
}

// processBatch 处理批量数据
func (s *GRPCServer) processBatch(node string, batch []interface{}) error {
	// 如果当前节点是领导者，直接处理
	if s.raftServer.IsLeader() {
		return s.applyBatch(batch)
	}

	// 否则，转发到领导者
	leaderID := s.raftServer.Leader()
	if leaderID == "" {
		return fmt.Errorf("没有可用的领导者")
	}

	// 实现重试机制
	return s.forwardToLeaderWithRetry(leaderID, batch, 3)
}

// forwardToLeaderWithRetry 带重试的转发到领导者
func (s *GRPCServer) forwardToLeaderWithRetry(leaderID string, batch []interface{}, maxRetries int) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// 检查领导者是否变化
		currentLeader := s.raftServer.Leader()
		if currentLeader != leaderID {
			leaderID = currentLeader
			if leaderID == "" {
				return fmt.Errorf("领导者变更，没有可用的领导者")
			}
		}

		// 这里应该实现实际的转发逻辑
		// 简化处理：直接应用到本地（假设领导者最终会同步）
		err := s.applyBatch(batch)
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("转发到领导者 %s 失败 (尝试 %d/%d): %v", leaderID, i+1, maxRetries, err)

		// 指数退避
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}

	return fmt.Errorf("转发到领导者失败，重试 %d 次后: %v", maxRetries, lastErr)
}

// applyBatch 应用批量数据
func (s *GRPCServer) applyBatch(batch []interface{}) error {
	// 将批量数据应用到Raft日志以实现一致性
	log.Printf("开始应用批量数据到Raft，批次大小: %d", len(batch))

	// 构建命令
	command := map[string]interface{}{
		"type":      "batch_metrics",
		"batch":     batch,
		"timestamp": time.Now().Unix(),
	}

	// 序列化命令
	cmdData, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("序列化命令失败: %v", err)
	}

	// 通过Raft应用命令
	log.Printf("调用Raft.Apply，命令大小: %d bytes", len(cmdData))
	result, err := s.raftServer.Apply(cmdData, 5*time.Second)
	if err != nil {
		log.Printf("应用Raft命令失败: %v", err)
		return fmt.Errorf("应用Raft命令失败: %v", err)
	}

	log.Printf("Raft.Apply成功完成，返回结果: %v", result)
	return nil
}

// SaveMetrics 保存指标数据
func (s *GRPCServer) SaveMetrics(ctx context.Context, req *pb.SaveMetricsRequest) (*pb.SaveMetricsResponse, error) {
	// 转换数据格式
	metricsData := &models.MetricsData{
		HostID:    req.HostId,
		Timestamp: req.Timestamp,
		Metrics:   make(map[string]interface{}),
		Tags:      make(map[string]string),
	}

	// 解析指标
	for _, metric := range req.Metrics {
		metricsData.Metrics[metric.Name] = metric.Value
	}

	// 解析标签
	for _, tag := range req.Tags {
		metricsData.Tags[tag.Key] = tag.Value
	}

	// 保存到存储
	if err := s.storage.SaveMetricsData(ctx, metricsData); err != nil {
		return nil, fmt.Errorf("保存指标数据失败: %v", err)
	}

	return &pb.SaveMetricsResponse{
		Success: true,
		Message: "指标数据保存成功",
	}, nil
}

// GetMetricsHistory 获取指标历史数据
func (s *GRPCServer) GetMetricsHistory(ctx context.Context, req *pb.GetMetricsHistoryRequest) (*pb.GetMetricsHistoryResponse, error) {
	// 获取时间范围内的数据
	metrics, err := s.storage.GetMetricsDataRange(ctx, req.HostId, req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("获取指标历史数据失败: %v", err)
	}

	// 转换数据格式
	var pbMetrics []*pb.MetricsData
	for _, data := range metrics {
		pbMetric := &pb.MetricsData{
			HostId:    data.HostID,
			Timestamp: data.Timestamp,
			Metrics:   []*pb.Metric{},
			Tags:      []*pb.Tag{},
		}

		// 转换指标
		for name, value := range data.Metrics {
			pbMetric.Metrics = append(pbMetric.Metrics, &pb.Metric{
				Name:  name,
				Value: fmt.Sprintf("%v", value),
			})
		}

		// 转换标签
		for key, val := range data.Tags {
			pbMetric.Tags = append(pbMetric.Tags, &pb.Tag{
				Key:   key,
				Value: val,
			})
		}

		pbMetrics = append(pbMetrics, pbMetric)
	}

	return &pb.GetMetricsHistoryResponse{
		Success: true,
		Data:    pbMetrics,
		Message: fmt.Sprintf("获取到%d条指标数据", len(pbMetrics)),
	}, nil
}

// GetMetricsStats 获取指标统计信息
func (s *GRPCServer) GetMetricsStats(ctx context.Context, req *pb.GetMetricsStatsRequest) (*pb.GetMetricsStatsResponse, error) {
	// 计算聚合数据
	aggregated, err := s.storage.AggregateMetrics(ctx, req.HostId, req.StartTime, req.EndTime, time.Duration(req.Interval)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("获取指标统计信息失败: %v", err)
	}

	// 转换数据格式
	stats := make(map[string]*pb.MetricStats)
	for metricName, statData := range aggregated {
		if stat, ok := statData.(map[string]interface{}); ok {
			stats[metricName] = &pb.MetricStats{
				Count: uint64(stat["count"].(float64)),
				Sum:   stat["sum"].(float64),
				Avg:   stat["avg"].(float64),
				Min:   stat["min"].(float64),
				Max:   stat["max"].(float64),
			}
		}
	}

	return &pb.GetMetricsStatsResponse{
		Success: true,
		Stats:   stats,
		Message: "指标统计信息获取成功",
	}, nil
}

// DeleteMetrics 删除指标数据
func (s *GRPCServer) DeleteMetrics(ctx context.Context, req *pb.DeleteMetricsRequest) (*pb.DeleteMetricsResponse, error) {
	// 删除指定时间点的数据
	if err := s.storage.DeleteMetricsData(ctx, req.HostId, req.Timestamp); err != nil {
		return nil, fmt.Errorf("删除指标数据失败: %v", err)
	}

	return &pb.DeleteMetricsResponse{
		Success: true,
		Message: "指标数据删除成功",
	}, nil
}

// GetBrokerStats 获取Broker统计信息
func (s *GRPCServer) GetBrokerStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Raft统计
	if s.raftServer != nil {
		stats["raft"] = map[string]interface{}{
			"leader_id":      s.raftServer.Leader(),
			"term":           s.raftServer.Term(),
			"state":          s.raftServer.State().String(),
			"last_log_index": s.raftServer.LastLogIndex(),
		}
	}

	// 主机管理器统计
	if s.hostManager != nil {
		stats["host_manager"] = s.hostManager.GetStats()
	}

	// 存储统计
	if s.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		storageStats, err := s.storage.GetStats(ctx)
		if err == nil {
			stats["storage"] = storageStats
		}
	}

	// 消息队列统计
	if s.streamProcessor != nil {
		if queueStats, err := s.streamProcessor.GetStats(context.Background()); err == nil {
			stats["message_queue"] = queueStats
		}
	}

	// 一致性哈希统计
	if s.hashRing != nil {
		stats["hash_ring"] = map[string]interface{}{
			"node_count":         s.hashRing.GetNodeCount(),
			"virtual_node_count": s.hashRing.GetVirtualNodeCount(),
			"distribution":       s.hashRing.GetDistribution(),
		}
	}

	return stats
}

// handleMetricsMessage 处理指标消息
func (s *GRPCServer) handleMetricsMessage(ctx context.Context, message *queue.Message) error {
	// 解析指标数据
	var metricsData models.MetricsData
	if err := json.Unmarshal(message.Payload, &metricsData); err != nil {
		return fmt.Errorf("解析指标数据失败: %v", err)
	}

	// 保存到存储
	if err := s.storage.SaveMetricsData(ctx, &metricsData); err != nil {
		return fmt.Errorf("保存指标数据失败: %v", err)
	}

	log.Printf("处理指标消息: %s, 包含%d个指标", metricsData.HostID, len(metricsData.Metrics))
	return nil
}

// handleAlertMessage 处理告警消息
func (s *GRPCServer) handleAlertMessage(ctx context.Context, message *queue.Message) error {
	// 解析告警数据
	var alertData map[string]interface{}
	if err := json.Unmarshal(message.Payload, &alertData); err != nil {
		return fmt.Errorf("解析告警数据失败: %v", err)
	}

	// 处理告警逻辑
	alertID := alertData["id"].(string)
	alertLevel := alertData["level"].(string)
	alertMessage := alertData["message"].(string)

	log.Printf("处理告警消息: ID=%s, Level=%s, Message=%s", alertID, alertLevel, alertMessage)

	// 这里可以添加告警通知逻辑，如发送邮件、短信等

	return nil
}

// PublishMessage 发布消息到队列
func (s *GRPCServer) PublishMessage(ctx context.Context, topic string, data []byte, metadata map[string]interface{}) error {
	return s.streamProcessor.Process(ctx, topic, data, metadata)
}

// SubscribeToTopic 订阅主题
func (s *GRPCServer) SubscribeToTopic(ctx context.Context, topic string, handler queue.MessageHandler) error {
	return s.messageQueue.Subscribe(ctx, topic, handler)
}
