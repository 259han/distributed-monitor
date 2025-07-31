package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/han-fei/monitor/proto"
	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
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
		retryCount: cfg.Broker.MaxRetry,
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

// GetMetrics 获取指标数据
func (c *GRPCClient) GetMetrics(ctx context.Context, hostID string, startTime, endTime time.Time) ([]*models.MetricsData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.clients) == 0 {
		return nil, fmt.Errorf("没有可用的broker连接")
	}

	// 选择一个客户端（简单的轮询）
	var selectedEndpoint string
	for endpoint := range c.clients {
		selectedEndpoint = endpoint
		break
	}

	client := c.clients[selectedEndpoint]

	// 创建请求
	req := &pb.MetricsRequest{
		HostId:    hostID,
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	}

	// 创建流
	stream, err := client.GetMetrics(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("创建流失败: %v", err)
	}

	// 接收数据
	var result []*models.MetricsData
	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("接收数据失败: %v", err)
		}

		// 转换为模型
		metrics := make([]models.Metric, 0, len(data.Metrics))
		for _, m := range data.Metrics {
			value, err := strconv.ParseFloat(m.Value, 64)
			if err != nil {
				continue // Skip invalid values
			}
			metrics = append(metrics, models.Metric{
				Name:  m.Name,
				Value: value,
				Unit:  m.Unit,
			})
		}

		metricsData := &models.MetricsData{
			HostID:    data.HostId,
			Timestamp: time.Unix(data.Timestamp, 0),
			Metrics:   metrics,
		}

		result = append(result, metricsData)
	}

	return result, nil
}

// GetMetricsWithRetry 带重试的获取指标数据
func (c *GRPCClient) GetMetricsWithRetry(ctx context.Context, hostID string, startTime, endTime time.Time) ([]*models.MetricsData, error) {
	var (
		result []*models.MetricsData
		err    error
	)

	for i := 0; i <= c.retryCount; i++ {
		result, err = c.GetMetrics(ctx, hostID, startTime, endTime)
		if err == nil {
			return result, nil
		}

		// 如果是最后一次重试，直接返回错误
		if i == c.retryCount {
			break
		}

		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1 * time.Second):
			// 重试
		}
	}

	return nil, fmt.Errorf("获取数据失败，已重试%d次: %v", c.retryCount, err)
}
