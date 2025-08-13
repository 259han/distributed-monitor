package quic

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	quicpkg "github.com/han-fei/monitor/pkg/quic"
	pb "github.com/han-fei/monitor/proto"
	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// IntegratedServer 集成的QUIC服务器
type IntegratedServer struct {
	config      *config.Config
	quicServer  *quicpkg.Server
	dataHandler *DataHandler
	enabled     bool
}

// DataHandler 数据处理器，将QUIC数据转换为可视化数据
type DataHandler struct {
	metricsCallback func(*models.MetricsData)
}

// NewIntegratedServer 创建集成的QUIC服务器
func NewIntegratedServer(cfg *config.Config) (*IntegratedServer, error) {
	if !cfg.QUIC.Enable {
		return &IntegratedServer{
			config:  cfg,
			enabled: false,
		}, nil
	}

	// 创建数据处理器
	dataHandler := &DataHandler{}

	// 转换配置
	quicConfig := &quicpkg.Config{
		Host:               "localhost",
		Port:               cfg.QUIC.Port,
		MaxStreamsPerConn:  cfg.QUIC.MaxStreamsPerConn,
		KeepAlivePeriod:    cfg.QUIC.KeepAlivePeriod,
		MaxIdleTimeout:     cfg.QUIC.MaxIdleTimeout,
		BufferSize:         cfg.QUIC.BufferSize,
		MaxMessageSize:     1024 * 1024, // 1MB
		MaxRetries:         3,
		RetryInterval:      cfg.QUIC.KeepAlivePeriod / 2,
		InsecureSkipVerify: true, // 开发环境
		NextProtos:         []string{"monitor-quic/1.0"},
	}

	// 创建QUIC服务器
	server, err := quicpkg.NewServer(quicConfig, dataHandler)
	if err != nil {
		return nil, err
	}

	return &IntegratedServer{
		config:      cfg,
		quicServer:  server,
		dataHandler: dataHandler,
		enabled:     true,
	}, nil
}

// Start 启动QUIC服务器
func (s *IntegratedServer) Start() error {
	if !s.enabled {
		log.Println("QUIC服务器已禁用，跳过启动")
		return nil
	}

	return s.quicServer.Start()
}

// Stop 停止QUIC服务器
func (s *IntegratedServer) Stop() error {
	if !s.enabled {
		return nil
	}

	return s.quicServer.Stop()
}

// SendData 发送数据（兼容原有接口）
func (s *IntegratedServer) SendData(data *models.MetricsData) {
	if !s.enabled {
		return
	}

	// 转换数据格式
	var metrics []*pb.Metric
	for _, metric := range data.Metrics {
		metrics = append(metrics, &pb.Metric{
			Name:  metric.Name,
			Value: fmt.Sprintf("%.2f", metric.Value),
			Unit:  metric.Unit,
		})
	}

	pbData := &pb.MetricsData{
		HostId:    data.HostID,
		Timestamp: data.Timestamp.Unix(),
		Metrics:   metrics,
	}

	// 广播数据
	if err := s.quicServer.BroadcastMetrics(pbData); err != nil {
		log.Printf("QUIC广播数据失败: %v", err)
	}
}

// BroadcastData 广播数据（兼容原有接口）
func (s *IntegratedServer) BroadcastData(data *models.MetricsData) {
	s.SendData(data)
}

// GetMetricsData 获取指标数据通道（兼容原有接口）
func (s *IntegratedServer) GetMetricsData() <-chan *models.MetricsData {
	// 为了兼容性，返回一个空通道
	ch := make(chan *models.MetricsData)
	close(ch)
	return ch
}

// GetConnectionCount 获取连接数
func (s *IntegratedServer) GetConnectionCount() int {
	if !s.enabled {
		return 0
	}

	stats := s.quicServer.GetStatistics()
	return int(stats.ConnectionsActive)
}

// SetMetricsCallback 设置指标数据回调
func (s *IntegratedServer) SetMetricsCallback(callback func(*models.MetricsData)) {
	if s.dataHandler != nil {
		s.dataHandler.metricsCallback = callback
	}
}

// IsEnabled 检查QUIC是否启用
func (s *IntegratedServer) IsEnabled() bool {
	return s.enabled
}

// HandleMetrics 实现DataHandler接口
func (h *DataHandler) HandleMetrics(ctx context.Context, metrics *pb.MetricsData) error {
	if h.metricsCallback != nil {
		// 转换数据格式
		var modelMetrics []models.Metric
		for _, metric := range metrics.Metrics {
			value := 0.0
			if metric.Value != "" {
				if v, err := strconv.ParseFloat(metric.Value, 64); err == nil {
					value = v
				}
			}
			modelMetrics = append(modelMetrics, models.Metric{
				Name:  metric.Name,
				Value: value,
				Unit:  metric.Unit,
			})
		}

		data := &models.MetricsData{
			HostID:    metrics.HostId,
			Timestamp: time.Unix(metrics.Timestamp, 0),
			Metrics:   modelMetrics,
		}
		h.metricsCallback(data)
	}
	return nil
}

// HandleHeartbeat 实现DataHandler接口
func (h *DataHandler) HandleHeartbeat(ctx context.Context, hostID string) error {
	log.Printf("收到QUIC心跳: %s", hostID)
	return nil
}

// HandleSubscribe 实现DataHandler接口
func (h *DataHandler) HandleSubscribe(ctx context.Context, hostID, streamID string) error {
	log.Printf("QUIC订阅请求: HostID=%s, StreamID=%s", hostID, streamID)
	return nil
}

// HandleUnsubscribe 实现DataHandler接口
func (h *DataHandler) HandleUnsubscribe(ctx context.Context, hostID, streamID string) error {
	log.Printf("QUIC取消订阅: HostID=%s, StreamID=%s", hostID, streamID)
	return nil
}
