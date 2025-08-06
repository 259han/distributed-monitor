package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/han-fei/monitor/pkg/storage"
)

// Message 消息结构
type Message struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Payload   []byte                 `json:"payload"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Retries   int                    `json:"retries"`
}

// MessageQueue 消息队列接口
type MessageQueue interface {
	Publish(ctx context.Context, topic string, message *Message) error
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	Ack(ctx context.Context, messageID string) error
	Nack(ctx context.Context, messageID string) error
	GetStats(ctx context.Context) (*QueueStats, error)
}

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, message *Message) error

// QueueStats 队列统计信息
type QueueStats struct {
	TotalMessages     int64                  `json:"total_messages"`
	PendingMessages   int64                  `json:"pending_messages"`
	ProcessedMessages int64                  `json:"processed_messages"`
	FailedMessages    int64                  `json:"failed_messages"`
	TopicStats        map[string]*TopicStats `json:"topic_stats"`
}

// TopicStats 主题统计信息
type TopicStats struct {
	MessageCount    int64     `json:"message_count"`
	LastMessageTime time.Time `json:"last_message_time"`
	ConsumerCount   int       `json:"consumer_count"`
}

// RedisMessageQueue Redis实现的消息队列
type RedisMessageQueue struct {
	storage     *storage.RedisStorage
	prefix      string
	ttl         time.Duration
	stats       *QueueStats
	mu          sync.RWMutex
	subscribers map[string][]MessageHandler
}

// NewRedisMessageQueue 创建Redis消息队列
func NewRedisMessageQueue(storage *storage.RedisStorage, prefix string, ttl time.Duration) *RedisMessageQueue {
	return &RedisMessageQueue{
		storage: storage,
		prefix:  prefix,
		ttl:     ttl,
		stats: &QueueStats{
			TopicStats: make(map[string]*TopicStats),
		},
		subscribers: make(map[string][]MessageHandler),
	}
}

// Publish 发布消息
func (q *RedisMessageQueue) Publish(ctx context.Context, topic string, message *Message) error {
	// 设置消息ID和时间戳
	if message.ID == "" {
		message.ID = fmt.Sprintf("%s:%d", topic, time.Now().UnixNano())
	}
	message.Timestamp = time.Now()
	message.Topic = topic

	// 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	// 存储消息
	messageKey := fmt.Sprintf("%s:message:%s", q.prefix, message.ID)
	if err := q.storage.SetWithTTL(ctx, messageKey, data, q.ttl); err != nil {
		return fmt.Errorf("存储消息失败: %v", err)
	}

	// 添加到主题队列
	queueKey := fmt.Sprintf("%s:queue:%s", q.prefix, topic)
	if err := q.storage.ListPush(ctx, queueKey, message.ID); err != nil {
		return fmt.Errorf("添加到队列失败: %v", err)
	}

	// 更新统计
	q.mu.Lock()
	q.stats.TotalMessages++
	q.stats.PendingMessages++
	if topicStat := q.stats.TopicStats[topic]; topicStat != nil {
		topicStat.MessageCount++
		topicStat.LastMessageTime = time.Now()
	} else {
		q.stats.TopicStats[topic] = &TopicStats{
			MessageCount:    1,
			LastMessageTime: time.Now(),
			ConsumerCount:   len(q.subscribers[topic]),
		}
	}
	q.mu.Unlock()

	// 通知订阅者
	go q.notifySubscribers(ctx, topic, message)

	return nil
}

// Subscribe 订阅主题
func (q *RedisMessageQueue) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 添加订阅者
	q.subscribers[topic] = append(q.subscribers[topic], handler)

	// 更新统计
	if topicStat := q.stats.TopicStats[topic]; topicStat != nil {
		topicStat.ConsumerCount = len(q.subscribers[topic])
	} else {
		q.stats.TopicStats[topic] = &TopicStats{
			ConsumerCount: 1,
		}
	}

	// 启动消费者
	go q.consumeMessages(ctx, topic, handler)

	return nil
}

// Ack 确认消息处理完成
func (q *RedisMessageQueue) Ack(ctx context.Context, messageID string) error {
	// 从处理中队列移除
	processingKey := fmt.Sprintf("%s:processing:%s", q.prefix, messageID)
	if err := q.storage.Delete(ctx, processingKey); err != nil {
		return fmt.Errorf("删除处理中消息失败: %v", err)
	}

	// 更新统计
	q.mu.Lock()
	q.stats.PendingMessages--
	q.stats.ProcessedMessages++
	q.mu.Unlock()

	return nil
}

// Nack 消息处理失败
func (q *RedisMessageQueue) Nack(ctx context.Context, messageID string) error {
	// 获取消息
	processingKey := fmt.Sprintf("%s:processing:%s", q.prefix, messageID)
	var message Message
	if err := q.storage.ListPop(ctx, processingKey, &message); err != nil {
		return fmt.Errorf("获取处理中消息失败: %v", err)
	}

	// 增加重试次数
	message.Retries++

	// 如果重试次数过多，移到死信队列
	if message.Retries > 3 {
		deadLetterKey := fmt.Sprintf("%s:dead:%s", q.prefix, message.Topic)
		if err := q.storage.ListPush(ctx, deadLetterKey, &message); err != nil {
			return fmt.Errorf("添加到死信队列失败: %v", err)
		}

		q.mu.Lock()
		q.stats.PendingMessages--
		q.stats.FailedMessages++
		q.mu.Unlock()
		return nil
	}

	// 重新加入队列
	queueKey := fmt.Sprintf("%s:queue:%s", q.prefix, message.Topic)
	if err := q.storage.ListPush(ctx, queueKey, &message); err != nil {
		return fmt.Errorf("重新加入队列失败: %v", err)
	}

	return nil
}

// GetStats 获取队列统计信息
func (q *RedisMessageQueue) GetStats(ctx context.Context) (*QueueStats, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// 返回统计信息的副本
	stats := *q.stats
	stats.TopicStats = make(map[string]*TopicStats)
	for k, v := range q.stats.TopicStats {
		topicCopy := *v
		stats.TopicStats[k] = &topicCopy
	}

	return &stats, nil
}

// consumeMessages 消费消息
func (q *RedisMessageQueue) consumeMessages(ctx context.Context, topic string, handler MessageHandler) {
	queueKey := fmt.Sprintf("%s:queue:%s", q.prefix, topic)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 从队列获取消息ID
			var messageID string
			if err := q.storage.ListPop(ctx, queueKey, &messageID); err != nil {
				continue
			}

			if messageID == "" {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// 获取完整消息
			messageKey := fmt.Sprintf("%s:message:%s", q.prefix, messageID)
			var message Message
			if err := q.storage.Get(ctx, messageKey, &message); err != nil {
				log.Printf("获取消息失败: %v", err)
				continue
			}

			// 移动到处理中队列
			processingKey := fmt.Sprintf("%s:processing:%s", q.prefix, messageID)
			if err := q.storage.ListPush(ctx, processingKey, &message); err != nil {
				log.Printf("移动到处理中队列失败: %v", err)
				continue
			}

			// 删除原消息
			if err := q.storage.Delete(ctx, messageKey); err != nil {
				log.Printf("删除原消息失败: %v", err)
			}

			// 处理消息
			if err := handler(ctx, &message); err != nil {
				log.Printf("处理消息失败: %v", err)
				// 处理失败，重新入队
				if err := q.Nack(ctx, messageID); err != nil {
					log.Printf("重新入队失败: %v", err)
				}
			} else {
				// 处理成功，确认
				if err := q.Ack(ctx, messageID); err != nil {
					log.Printf("确认消息失败: %v", err)
				}
			}
		}
	}
}

// notifySubscribers 通知订阅者
func (q *RedisMessageQueue) notifySubscribers(ctx context.Context, topic string, message *Message) {
	q.mu.RLock()
	handlers := q.subscribers[topic]
	q.mu.RUnlock()

	for _, handler := range handlers {
		go func(h MessageHandler) {
			if err := h(ctx, message); err != nil {
				log.Printf("订阅者处理消息失败: %v", err)
			}
		}(handler)
	}
}

// StreamProcessor 流处理器
type StreamProcessor struct {
	queue      MessageQueue
	processors map[string]StreamHandler
	mu         sync.RWMutex
}

// StreamHandler 流处理函数
type StreamHandler func(ctx context.Context, message *Message) error

// NewStreamProcessor 创建流处理器
func NewStreamProcessor(queue MessageQueue) *StreamProcessor {
	return &StreamProcessor{
		queue:      queue,
		processors: make(map[string]StreamHandler),
	}
}

// RegisterProcessor 注册处理器
func (sp *StreamProcessor) RegisterProcessor(topic string, handler StreamHandler) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.processors[topic] = handler

	// 订阅主题
	return sp.queue.Subscribe(context.Background(), topic, sp.wrapHandler(handler))
}

// wrapHandler 包装处理器函数
func (sp *StreamProcessor) wrapHandler(handler StreamHandler) MessageHandler {
	return func(ctx context.Context, message *Message) error {
		return handler(ctx, message)
	}
}

// Process 处理流数据
func (sp *StreamProcessor) Process(ctx context.Context, topic string, data []byte, metadata map[string]interface{}) error {
	message := &Message{
		Topic:    topic,
		Payload:  data,
		Metadata: metadata,
	}

	return sp.queue.Publish(ctx, topic, message)
}

// GetStats 获取处理器统计信息
func (sp *StreamProcessor) GetStats(ctx context.Context) (*QueueStats, error) {
	return sp.queue.GetStats(ctx)
}
