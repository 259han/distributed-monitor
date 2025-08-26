package collector

/*
#cgo CFLAGS: -I../c
#cgo LDFLAGS: -lpthread
#include "lockfree_queue.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/han-fei/monitor/agent/internal/c"
	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/han-fei/monitor/agent/internal/service"
	"github.com/han-fei/monitor/pkg/interfaces"
)

// 使用pkg/interfaces中的Collector接口
type Collector = interfaces.Collector

// MetricsCollector 指标采集器
type MetricsCollector struct {
	config           *config.Config
	collectors       map[string]Collector
	metricsBuffer    *c.LockFreeQueue
	wg               sync.WaitGroup
	mu               sync.Mutex
	stopCh           chan struct{}
	hostInfo         models.HostInfo
	grpcClient       *service.GRPCClient
	kafkaProducer    *service.KafkaProducer
	useLockFreeQueue bool // 是否使用无锁队列
	useKafka         bool // 是否使用Kafka
}

// NewMetricsCollector 创建新的指标采集器
func NewMetricsCollector(cfg *config.Config) *MetricsCollector {
	mc := &MetricsCollector{
		config:     cfg,
		collectors: make(map[string]Collector),
		stopCh:     make(chan struct{}),
		hostInfo: models.HostInfo{
			ID:       cfg.Agent.HostID,
			Hostname: cfg.Agent.Hostname,
			IP:       cfg.Agent.IP,
		},
		grpcClient:       service.NewGRPCClient(cfg),
		kafkaProducer:    service.NewKafkaProducer(&cfg.Kafka),
		useLockFreeQueue: true, // 默认使用无锁队列
		useKafka:         cfg.Kafka.Enabled, // 是否使用Kafka
	}

	// 初始化无锁队列
	mc.metricsBuffer = c.NewLockFreeQueue(cfg.Advanced.RingBufferSize)
	if mc.metricsBuffer == nil {
		// 如果无锁队列创建失败，回退到Go channel
		mc.useLockFreeQueue = false
		log.Printf("警告：无锁队列创建失败，回退到Go channel")
	}

	// 日志记录Kafka状态
	if mc.useKafka {
		log.Printf("Kafka生产者已启用，连接到: %v, 主题: %s", cfg.Kafka.Brokers, cfg.Kafka.Topic)
	} else {
		log.Printf("Kafka生产者未启用，将直接使用gRPC发送数据")
	}

	return mc
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

	// 关闭Kafka生产者
	if mc.useKafka && mc.kafkaProducer != nil {
		if err := mc.kafkaProducer.Close(); err != nil {
			log.Printf("关闭Kafka生产者失败: %v", err)
		}
	}

	// 关闭无锁队列
	if mc.useLockFreeQueue && mc.metricsBuffer != nil {
		mc.metricsBuffer.Close()
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

	log.Printf("开始采集循环，间隔: %v", mc.config.Collect.Interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("采集循环收到ctx.Done信号，正在退出")
			return
		case <-mc.stopCh:
			log.Printf("采集循环收到停止信号，正在退出")
			return
		case <-ticker.C:
			log.Printf("采集循环触发器响应，开始采集指标")
			mc.collectMetrics()
		}
	}
}

// collectMetrics 采集指标
func (mc *MetricsCollector) collectMetrics() {
	var allMetrics []models.Metric

	// 从每个采集器获取指标
	for name, collector := range mc.collectors {
		interfaceMetrics, err := collector.Collect()
		if err != nil {
			// 记录错误，但继续采集其他指标
			log.Printf("采集%s指标失败: %v", name, err)
			continue
		}
		// 将接口 Metric 转换为内部 Metric
		metrics := ConvertBackMetrics(interfaceMetrics)
		allMetrics = append(allMetrics, metrics...)
	}

	// 创建指标数据
	metricsData := models.MetricsData{
		HostID:    mc.hostInfo.ID,
		Timestamp: time.Now(),
		Metrics:   allMetrics,
	}

	log.Printf("采集到 %d 个指标，主机ID: %s", len(allMetrics), mc.hostInfo.ID)

	// 发送到buffer
	if mc.useLockFreeQueue {
		// 使用无锁队列处理数据
		if err := mc.sendToLockFreeQueue(metricsData); err != nil {
			log.Printf("发送到无锁队列失败: %v", err)
			// 回退到直接处理
			mc.processBatchDirectly(metricsData)
		}
	} else {
		// 使用Go channel方式（简化实现）
		mc.processBatchDirectly(metricsData)
	}
}

// processLoop 处理循环
func (mc *MetricsCollector) processLoop(ctx context.Context) {
	defer mc.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.stopCh:
			return
		case <-ticker.C:
			// 从无锁队列中处理数据
			if mc.useLockFreeQueue {
				mc.processFromLockFreeQueue()
			}
		}
	}
}

// processBatch 处理批量数据
func (mc *MetricsCollector) processBatch(ctx context.Context, batch []models.MetricsData) {
	// 发送数据到中转层
	if err := mc.grpcClient.SendMetricsWithRetry(ctx, batch); err != nil {
		// 记录错误，但不中断处理
		log.Printf("发送批量数据失败: %v", err)
	}
}

// processBatchDirectly 直接处理单个MetricsData
func (mc *MetricsCollector) processBatchDirectly(data models.MetricsData) {
	ctx := context.Background()
	log.Printf("正在处理 %d 个指标，主机ID: %s", len(data.Metrics), data.HostID)
	
	// 如果启用了Kafka，发送到Kafka
	if mc.useKafka && mc.kafkaProducer != nil {
		if err := mc.kafkaProducer.SendMetricsWithRetry(ctx, []models.MetricsData{data}); err != nil {
			log.Printf("发送数据到Kafka失败: %v，尝试使用gRPC发送", err)
			// Kafka发送失败，回退到gRPC
			if err := mc.grpcClient.SendMetricsWithRetry(ctx, []models.MetricsData{data}); err != nil {
				log.Printf("通过gRPC发送数据失败: %v", err)
			} else {
				log.Printf("成功通过gRPC发送 %d 个指标", len(data.Metrics))
			}
		} else {
			log.Printf("成功发送 %d 个指标到 Kafka", len(data.Metrics))
		}
	} else {
		// 直接通过gRPC发送
		log.Printf("正在发送 %d 个指标到 Broker，主机ID: %s", len(data.Metrics), data.HostID)
		if err := mc.grpcClient.SendMetricsWithRetry(ctx, []models.MetricsData{data}); err != nil {
			log.Printf("发送数据失败: %v", err)
		} else {
			log.Printf("成功发送 %d 个指标到 Broker", len(data.Metrics))
		}
	}
}

// sendToLockFreeQueue 发送数据到无锁队列
func (mc *MetricsCollector) sendToLockFreeQueue(data models.MetricsData) error {
	if mc.metricsBuffer == nil {
		return fmt.Errorf("无锁队列未初始化")
	}

	// 将Go结构转换为C结构
	cData, err := mc.convertToCMetricsData(data)
	if err != nil {
		return fmt.Errorf("转换数据失败: %v", err)
	}

	// 发送到无锁队列
	if err := mc.metricsBuffer.Enqueue(unsafe.Pointer(cData)); err != nil {
		// 释放内存
		C.free(unsafe.Pointer(cData.metrics))
		C.free(unsafe.Pointer(cData))
		return fmt.Errorf("推送数据到无锁队列失败: %v", err)
	}

	return nil
}

// processFromLockFreeQueue 从无锁队列处理数据
func (mc *MetricsCollector) processFromLockFreeQueue() {
	if mc.metricsBuffer == nil {
		return
	}

	// 批量处理数据
	batchSize := 10
	var batch []models.MetricsData

	for i := 0; i < batchSize; i++ {
		dataPtr, err := mc.metricsBuffer.Dequeue()
		if err != nil {
			if err == c.ErrQueueEmpty {
				break
			}
			log.Printf("从无锁队列获取数据失败: %v", err)
			continue
		}

		// 转换C结构为Go结构
		cData := (*C.MetricsData)(dataPtr)
		goData, err := mc.convertFromCMetricsData(cData)
		if err != nil {
			log.Printf("转换C数据失败: %v", err)
			// 释放内存
			C.free(unsafe.Pointer(cData.metrics))
			C.free(unsafe.Pointer(cData))
			continue
		}

		batch = append(batch, goData)

		// 释放C结构内存
		C.free(unsafe.Pointer(cData.metrics))
		C.free(unsafe.Pointer(cData))
	}

	// 处理批量数据
	if len(batch) > 0 {
		ctx := context.Background()
		
		// 如果启用了Kafka，发送到Kafka
		if mc.useKafka && mc.kafkaProducer != nil {
			log.Printf("从无锁队列处理 %d 条数据，发送到Kafka", len(batch))
			if err := mc.kafkaProducer.SendMetricsWithRetry(ctx, batch); err != nil {
				log.Printf("发送批量数据到Kafka失败: %v，尝试使用gRPC发送", err)
				// Kafka发送失败，回退到gRPC
				if err := mc.grpcClient.SendMetricsWithRetry(ctx, batch); err != nil {
					log.Printf("通过gRPC发送批量数据失败: %v", err)
				}
			}
		} else {
			// 直接通过gRPC发送
			log.Printf("从无锁队列处理 %d 条数据，发送到Broker", len(batch))
			if err := mc.grpcClient.SendMetricsWithRetry(ctx, batch); err != nil {
				log.Printf("发送批量数据失败: %v", err)
			}
		}
	}
}

// convertToCMetricsData 将Go MetricsData转换为C MetricsData
func (mc *MetricsCollector) convertToCMetricsData(data models.MetricsData) (*C.MetricsData, error) {
	// 分配C结构内存
	cData := (*C.MetricsData)(C.malloc(C.sizeof_MetricsData))
	if cData == nil {
		return nil, fmt.Errorf("分配C MetricsData内存失败")
	}

	// 设置基本信息
	cHostID := C.CString(data.HostID)
	C.strcpy((*C.char)(unsafe.Pointer(&cData.host_id[0])), cHostID)
	C.free(unsafe.Pointer(cHostID))
	cData.timestamp = C.int64_t(data.Timestamp.Unix())
	cData.metrics_count = C.int(len(data.Metrics))

	// 分配指标数组内存
	if len(data.Metrics) > 0 {
		cMetrics := (*C.Metric)(C.malloc(C.size_t(len(data.Metrics)) * C.sizeof_Metric))
		if cMetrics == nil {
			C.free(unsafe.Pointer(cData))
			return nil, fmt.Errorf("分配C Metric数组内存失败")
		}

		// 转换每个指标
		for i, metric := range data.Metrics {
			cMetric := (*C.Metric)(unsafe.Pointer(uintptr(unsafe.Pointer(cMetrics)) + uintptr(i)*C.sizeof_Metric))

			cName := C.CString(metric.Name)
			C.strcpy((*C.char)(unsafe.Pointer(&cMetric.name[0])), cName)
			C.free(unsafe.Pointer(cName))

			cMetric.value = C.double(metric.Value)

			cUnit := C.CString(metric.Unit)
			C.strcpy((*C.char)(unsafe.Pointer(&cMetric.unit[0])), cUnit)
			C.free(unsafe.Pointer(cUnit))
		}

		cData.metrics = cMetrics
	} else {
		cData.metrics = nil
	}

	return cData, nil
}

// convertFromCMetricsData 将C MetricsData转换为Go MetricsData
func (mc *MetricsCollector) convertFromCMetricsData(cData *C.MetricsData) (models.MetricsData, error) {
	// 转换基本信息
	hostID := C.GoString((*C.char)(unsafe.Pointer(&cData.host_id[0])))
	timestamp := time.Unix(int64(cData.timestamp), 0)

	// 转换指标
	var metrics []models.Metric
	metricsCount := int(cData.metrics_count)

	if metricsCount > 0 && cData.metrics != nil {
		for i := 0; i < metricsCount; i++ {
			cMetric := (*C.Metric)(unsafe.Pointer(uintptr(unsafe.Pointer(cData.metrics)) + uintptr(i)*C.sizeof_Metric))

			metric := models.Metric{
				Name:  C.GoString((*C.char)(unsafe.Pointer(&cMetric.name[0]))),
				Value: float64(cMetric.value),
				Unit:  C.GoString((*C.char)(unsafe.Pointer(&cMetric.unit[0]))),
			}
			metrics = append(metrics, metric)
		}
	}

	return models.MetricsData{
		HostID:    hostID,
		Timestamp: timestamp,
		Metrics:   metrics,
	}, nil
}
