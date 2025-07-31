package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	pb "github.com/han-fei/monitor/proto"
)

// GRPCClient gRPC客户端
type GRPCClient struct {
	config     *config.Config
	conns      map[string]*grpc.ClientConn
	clients    map[string]pb.MonitorServiceClient
	mu         sync.RWMutex
	retryCount int
}

// NewGRPCClient 创建新的gRPC客户端
func NewGRPCClient(cfg *config.Config) *GRPCClient {
	return &GRPCClient{
		config:     cfg,
		conns:      make(map[string]*grpc.ClientConn),
		clients:    make(map[string]pb.MonitorServiceClient),
		retryCount: cfg.Collect.MaxRetry,
	}
}

// Connect 连接到所有broker节点
func (c *GRPCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭已有连接
	for endpoint, conn := range c.conns {
		conn.Close()
		delete(c.conns, endpoint)
		delete(c.clients, endpoint)
	}

	// 建立新连接
	for _, endpoint := range c.config.Broker.Endpoints {
		// 设置连接选项
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()), // 使用不安全连接，生产环境应使用TLS
			grpc.WithBlock(), // 阻塞直到连接建立
			grpc.WithTimeout(c.config.Broker.Timeout), // 连接超时
		}

		// 建立连接
		conn, err := grpc.Dial(endpoint, opts...)
		if err != nil {
			return fmt.Errorf("连接到broker %s失败: %v", endpoint, err)
		}

		// 创建客户端
		client := pb.NewMonitorServiceClient(conn)

		// 保存连接和客户端
		c.conns[endpoint] = conn
		c.clients[endpoint] = client
	}

	return nil
}

// Close 关闭所有连接
func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for endpoint, conn := range c.conns {
		if err := conn.Close(); err != nil {
			return err
		}
		delete(c.conns, endpoint)
		delete(c.clients, endpoint)
	}

	return nil
}

// SendMetrics 发送指标数据
func (c *GRPCClient) SendMetrics(ctx context.Context, metrics []models.MetricsData) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.clients) == 0 {
		return fmt.Errorf("没有可用的broker连接")
	}

	// 选择一个客户端（简单的轮询）
	var selectedEndpoint string
	for endpoint := range c.clients {
		selectedEndpoint = endpoint
		break
	}

	client := c.clients[selectedEndpoint]

	// 创建流
	stream, err := client.SendMetrics(ctx)
	if err != nil {
		return fmt.Errorf("创建流失败: %v", err)
	}

	// 发送数据
	for _, data := range metrics {
		// 转换为protobuf格式
		pbMetrics := make([]*pb.Metric, 0, len(data.Metrics))
		for _, m := range data.Metrics {
			pbMetrics = append(pbMetrics, &pb.Metric{
				Name:  m.Name,
				Value: fmt.Sprintf("%.2f", m.Value),
				Unit:  m.Unit,
			})
		}

		pbData := &pb.MetricsData{
			HostId:    data.HostID,
			Timestamp: data.Timestamp.Unix(),
			Metrics:   pbMetrics,
		}

		// 发送数据
		if err := stream.Send(pbData); err != nil {
			return fmt.Errorf("发送数据失败: %v", err)
		}
	}

	// 关闭流并接收响应
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("接收响应失败: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("发送失败: %s", resp.Message)
	}

	return nil
}

// SendMetricsWithRetry 带重试的发送指标数据
func (c *GRPCClient) SendMetricsWithRetry(ctx context.Context, metrics []models.MetricsData) error {
	var err error
	for i := 0; i <= c.retryCount; i++ {
		err = c.SendMetrics(ctx, metrics)
		if err == nil {
			return nil
		}

		// 如果是最后一次重试，直接返回错误
		if i == c.retryCount {
			break
		}

		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			// 重试
		}
	}

	return fmt.Errorf("发送数据失败，已重试%d次: %v", c.retryCount, err)
}
