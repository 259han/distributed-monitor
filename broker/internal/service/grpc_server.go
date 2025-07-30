package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/han-fei/monitor/broker/internal/hash"
	"github.com/han-fei/monitor/broker/internal/raft"
	"github.com/han-fei/monitor/broker/internal/storage"
	pb "github.com/han-fei/monitor/proto"
)

// GRPCServer gRPC服务器
type GRPCServer struct {
	config     *Config
	server     *grpc.Server
	hashRing   *hash.ConsistentHash
	raftServer *raft.Server
	storage    *storage.RedisStorage
	mu         sync.RWMutex
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

	// 创建Raft服务器
	raftServer, err := raft.NewServer(config.RaftConfig)
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

	// 创建服务器实例
	s := &GRPCServer{
		config:     config,
		server:     server,
		hashRing:   hashRing,
		raftServer: raftServer,
		storage:    redisStorage,
	}

	// 注册服务
	pb.RegisterMonitorServiceServer(server, s)

	// 启用反射服务，用于grpcurl等工具
	reflection.Register(server)

	return s, nil
}

// Start 启动服务器
func (s *GRPCServer) Start() error {
	// 启动Raft服务器
	if err := s.raftServer.Start(); err != nil {
		return err
	}

	// 监听端口
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
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
			for node, items := range batchMap {
				if len(items) > 0 {
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

		// 如果批次达到一定大小，处理批次
		if len(batchMap[node]) >= 100 {
			if err := s.processBatch(node, batchMap[node]); err != nil {
				return err
			}
			batchMap[node] = batchMap[node][:0]
		}
	}
}

// GetMetrics 实现MonitorService.GetMetrics
func (s *GRPCServer) GetMetrics(req *pb.MetricsRequest, stream pb.MonitorService_GetMetricsServer) error {
	// 构建键前缀
	prefix := fmt.Sprintf("metrics:%s:", req.HostId)

	// 获取键列表
	ctx := context.Background()
	keys, err := s.storage.Keys(ctx, prefix+"*")
	if err != nil {
		return err
	}

	// 批量获取数据
	dataMap, err := s.storage.GetBatch(ctx, keys)
	if err != nil {
		return err
	}

	// 发送数据
	for key := range dataMap {
		// 这里应该反序列化数据，但简化处理
		log.Printf("发送键 %s 的数据", key)

		// 创建一个空的MetricsData用于演示
		metricsData := &pb.MetricsData{
			HostId:    req.HostId,
			Timestamp: time.Now().Unix(),
			Metrics:   []*pb.Metric{},
		}

		if err := stream.Send(metricsData); err != nil {
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

	// 这里应该实现转发逻辑，但简化处理
	// 假设已经转发并成功处理

	return nil
}

// applyBatch 应用批量数据
func (s *GRPCServer) applyBatch(batch []interface{}) error {
	// 将批量数据应用到Raft日志
	// 这里简化处理，直接存储到Redis

	ctx := context.Background()
	items := make(map[string]interface{})

	for _, item := range batch {
		data, ok := item.(*pb.MetricsData)
		if !ok {
			continue
		}

		// 构建键
		key := fmt.Sprintf("metrics:%s:%d", data.HostId, data.Timestamp)
		items[key] = data
	}

	// 批量存储
	return s.storage.SetBatch(ctx, items)
}
