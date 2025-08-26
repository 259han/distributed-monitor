package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/han-fei/monitor/broker/internal/config"
	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/han-fei/monitor/broker/internal/storage"
	"github.com/segmentio/kafka-go"
)

// KafkaConsumer Kafka消费者
type KafkaConsumer struct {
	config      *config.KafkaConfig
	reader      *kafka.Reader
	enabled     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
	dataStorage storage.MetricsStorage
}

// NewKafkaConsumer 创建新的Kafka消费者
func NewKafkaConsumer(cfg *config.KafkaConfig, dataStorage storage.MetricsStorage) *KafkaConsumer {
	if !cfg.Enabled || len(cfg.Brokers) == 0 {
		return &KafkaConsumer{
			config:      cfg,
			enabled:     false,
			stopCh:      make(chan struct{}),
			dataStorage: dataStorage,
		}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       10e3,        // 10KB
		MaxBytes:       10e6,        // 10MB
		MaxWait:        1 * time.Second,
		StartOffset:    kafka.LastOffset,
		CommitInterval: time.Second,
	})

	return &KafkaConsumer{
		config:      cfg,
		reader:      reader,
		enabled:     true,
		stopCh:      make(chan struct{}),
		dataStorage: dataStorage,
	}
}

// Start 启动消费者
func (c *KafkaConsumer) Start(ctx context.Context) error {
	if !c.enabled {
		log.Printf("Kafka消费者未启用，跳过启动")
		return nil
	}

	log.Printf("启动Kafka消费者，连接到: %v, 主题: %s, 组ID: %s", 
		c.config.Brokers, c.config.Topic, c.config.GroupID)

	c.wg.Add(1)
	go c.consumeLoop(ctx)

	return nil
}

// Stop 停止消费者
func (c *KafkaConsumer) Stop() error {
	if !c.enabled {
		return nil
	}

	close(c.stopCh)
	c.wg.Wait()

	if c.reader != nil {
		if err := c.reader.Close(); err != nil {
			return err
		}
	}

	return nil
}

// consumeLoop 消费循环
func (c *KafkaConsumer) consumeLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Kafka消费循环收到ctx.Done信号，正在退出")
			return
		case <-c.stopCh:
			log.Printf("Kafka消费循环收到停止信号，正在退出")
			return
		default:
			// 设置读取超时
			readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			m, err := c.reader.FetchMessage(readCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded || err == context.Canceled {
					// 超时或取消，继续循环
					continue
				}
				log.Printf("从Kafka读取消息失败: %v", err)
				time.Sleep(1 * time.Second) // 避免CPU空转
				continue
			}

			// 处理消息
			if err := c.processMessage(ctx, m); err != nil {
				log.Printf("处理Kafka消息失败: %v", err)
				continue
			}

			// 提交消息
			if err := c.reader.CommitMessages(ctx, m); err != nil {
				log.Printf("提交Kafka消息失败: %v", err)
			}
		}
	}
}

// processMessage 处理消息
func (c *KafkaConsumer) processMessage(ctx context.Context, m kafka.Message) error {
	var data models.MetricsData

	// 解析消息
	if err := json.Unmarshal(m.Value, &data); err != nil {
		return err
	}

	// 存储数据
	if err := c.dataStorage.StoreMetrics(ctx, data); err != nil {
		return err
	}

	log.Printf("成功处理来自Kafka的 %d 个指标，主机ID: %s", len(data.Metrics), data.HostID)
	return nil
}
