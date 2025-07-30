package collector

import (
	"context"
	"sync"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/han-fei/monitor/agent/internal/service"
)

// Collector 数据采集器接口
type Collector interface {
	Start(ctx context.Context) error
	Stop() error
	Collect() ([]models.Metric, error)
}

// MetricsCollector 指标采集器
type MetricsCollector struct {
	config      *config.Config
	collectors  map[string]Collector
	metricsChan chan models.MetricsData
	wg          sync.WaitGroup
	mu          sync.Mutex
	stopCh      chan struct{}
	hostInfo    models.HostInfo
	grpcClient  *service.GRPCClient
}

// NewMetricsCollector 创建新的指标采集器
func NewMetricsCollector(cfg *config.Config) *MetricsCollector {
	return &MetricsCollector{
		config:      cfg,
		collectors:  make(map[string]Collector),
		metricsChan: make(chan models.MetricsData, cfg.Advanced.RingBufferSize),
		stopCh:      make(chan struct{}),
		hostInfo: models.HostInfo{
			ID:       cfg.Agent.HostID,
			Hostname: cfg.Agent.Hostname,
			IP:       cfg.Agent.IP,
		},
		grpcClient: service.NewGRPCClient(cfg),
	}
}

// RegisterCollector 注册采集器
func (mc *MetricsCollector) RegisterCollector(name string, collector Collector) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.collectors[name] = collector
}

// Start 启动采集
func (mc *MetricsCollector) Start(ctx context.Context) error {
	// 连接到broker
	if err := mc.grpcClient.Connect(); err != nil {
		return err
	}

	// 启动所有采集器
	for _, collector := range mc.collectors {
		if err := collector.Start(ctx); err != nil {
			return err
		}
	}

	// 启动采集协程
	mc.wg.Add(1)
	go mc.collectLoop(ctx)

	// 启动处理协程
	mc.wg.Add(1)
	go mc.processLoop(ctx)

	return nil
}

// Stop 停止采集
func (mc *MetricsCollector) Stop() error {
	close(mc.stopCh)

	// 停止所有采集器
	for _, collector := range mc.collectors {
		if err := collector.Stop(); err != nil {
			return err
		}
	}

	// 关闭gRPC客户端
	if err := mc.grpcClient.Close(); err != nil {
		return err
	}

	// 等待所有协程退出
	mc.wg.Wait()
	return nil
}

// collectLoop 采集循环
func (mc *MetricsCollector) collectLoop(ctx context.Context) {
	defer mc.wg.Done()

	ticker := time.NewTicker(mc.config.Collect.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.collectMetrics()
		}
	}
}

// collectMetrics 采集指标
func (mc *MetricsCollector) collectMetrics() {
	var allMetrics []models.Metric

	// 从每个采集器获取指标
	for _, collector := range mc.collectors {
		metrics, err := collector.Collect()
		if err != nil {
			// 记录错误，但继续采集其他指标
			continue
		}
		allMetrics = append(allMetrics, metrics...)
	}

	// 创建指标数据
	metricsData := models.MetricsData{
		HostID:    mc.hostInfo.ID,
		Timestamp: time.Now(),
		Metrics:   allMetrics,
	}

	// 发送到通道
	select {
	case mc.metricsChan <- metricsData:
		// 成功发送
	default:
		// 通道已满，丢弃数据
	}
}

// processLoop 处理循环
func (mc *MetricsCollector) processLoop(ctx context.Context) {
	defer mc.wg.Done()

	// 批处理缓冲区
	buffer := make([]models.MetricsData, 0, mc.config.Collect.BatchSize)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case data := <-mc.metricsChan:
			// 添加到缓冲区
			buffer = append(buffer, data)

			// 如果缓冲区已满，处理批量数据
			if len(buffer) >= mc.config.Collect.BatchSize {
				mc.processBatch(ctx, buffer)
				buffer = buffer[:0] // 清空缓冲区
			}
		case <-ticker.C:
			// 定时处理批量数据，避免数据长时间不处理
			if len(buffer) > 0 {
				mc.processBatch(ctx, buffer)
				buffer = buffer[:0] // 清空缓冲区
			}
		}
	}
}

// processBatch 处理批量数据
func (mc *MetricsCollector) processBatch(ctx context.Context, batch []models.MetricsData) {
	// 发送数据到中转层
	if err := mc.grpcClient.SendMetricsWithRetry(ctx, batch); err != nil {
		// 记录错误，但不中断处理
	}
}

// GetMetricsChannel 获取指标通道，用于测试
func (mc *MetricsCollector) GetMetricsChannel() <-chan models.MetricsData {
	return mc.metricsChan
}
