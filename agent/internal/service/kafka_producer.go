package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/segmentio/kafka-go"
)

// KafkaProducer Kafka生产者
type KafkaProducer struct {
	config   *config.KafkaConfig
	writer   *kafka.Writer
	enabled  bool
}

// NewKafkaProducer 创建新的Kafka生产者
func NewKafkaProducer(cfg *config.KafkaConfig) *KafkaProducer {
	if !cfg.Enabled || len(cfg.Brokers) == 0 {
		return &KafkaProducer{
			config:  cfg,
			enabled: false,
		}
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		RequiredAcks: kafka.RequireOne,
	}

	return &KafkaProducer{
		config:  cfg,
		writer:  writer,
		enabled: true,
	}
}

// IsEnabled 检查Kafka生产者是否启用
func (p *KafkaProducer) IsEnabled() bool {
	return p.enabled
}

// SendMetrics 发送指标数据到Kafka
func (p *KafkaProducer) SendMetrics(ctx context.Context, metrics []models.MetricsData) error {
	if !p.enabled {
		return nil
	}

	messages := make([]kafka.Message, 0, len(metrics))

	for _, data := range metrics {
		// 使用配置的编码格式序列化数据
		var value []byte
		var err error

		if p.config.Encoding == "protobuf" {
			// 如果需要protobuf支持，可以在这里添加
			// 目前使用JSON作为默认格式
			value, err = json.Marshal(data)
		} else {
			// 默认使用JSON格式
			value, err = json.Marshal(data)
		}

		if err != nil {
			log.Printf("序列化数据失败: %v", err)
			continue
		}

		messages = append(messages, kafka.Message{
			Key:   []byte(data.HostID),
			Value: value,
			Time:  time.Now(),
		})
	}

	// 如果没有消息，直接返回
	if len(messages) == 0 {
		return nil
	}

	// 发送消息到Kafka
	return p.writer.WriteMessages(ctx, messages...)
}

// SendMetricsWithRetry 带重试的发送指标数据
func (p *KafkaProducer) SendMetricsWithRetry(ctx context.Context, metrics []models.MetricsData) error {
	if !p.enabled {
		return nil
	}

	var err error
	for i := 0; i < p.config.MaxRetry; i++ {
		err = p.SendMetrics(ctx, metrics)
		if err == nil {
			return nil
		}
		log.Printf("发送数据到Kafka失败 (尝试 %d/%d): %v", i+1, p.config.MaxRetry, err)
		// 指数退避重试
		time.Sleep(time.Duration(1<<uint(i)) * 100 * time.Millisecond)
	}
	return err
}

// Close 关闭Kafka生产者
func (p *KafkaProducer) Close() error {
	if !p.enabled || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
