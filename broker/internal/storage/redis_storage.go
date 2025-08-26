package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/redis/go-redis/v9"
)

// RedisStorage Redis存储实现
type RedisStorage struct {
	client    redis.UniversalClient
	keyPrefix string
	keyTTL    time.Duration
	config    RedisConfig
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addresses      []string      `json:"addresses"`
	Password       string        `json:"password"`
	DB             int           `json:"db"`
	PoolSize       int           `json:"pool_size"`
	KeyPrefix      string        `json:"key_prefix"`
	DefaultTTL     time.Duration `json:"default_ttl"`
	EnableCluster  bool          `json:"enable_cluster"`
	EnableSentinel bool          `json:"enable_sentinel"`
	SentinelAddrs  []string      `json:"sentinel_addrs"`
	SentinelMaster string        `json:"sentinel_master"`
	Address        string        `json:"address"`
}

// NewRedisStorage 创建Redis存储
func NewRedisStorage(config RedisConfig) (*RedisStorage, error) {
	var client redis.UniversalClient

	// 根据配置创建不同的Redis客户端
	if config.EnableSentinel && len(config.SentinelAddrs) > 0 {
		// 哨兵模式
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    config.SentinelMaster,
			SentinelAddrs: config.SentinelAddrs,
			Password:      config.Password,
			DB:            config.DB,
			PoolSize:      config.PoolSize,
		})
	} else if config.EnableCluster && len(config.Addresses) > 1 {
		// 集群模式
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.Addresses,
			Password:     config.Password,
			PoolSize:     config.PoolSize,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		})
	} else {
		// 单机模式
		addr := "localhost:6379"
		if config.Address != "" {
			addr = config.Address
		} else if len(config.Addresses) > 0 {
			addr = config.Addresses[0]
		}
		client = redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     config.Password,
			DB:           config.DB,
			PoolSize:     config.PoolSize,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		})
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	// 设置默认值
	if config.PoolSize == 0 {
		config.PoolSize = 10
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "monitor:"
	}

	return &RedisStorage{
		client:    client,
		keyPrefix: config.KeyPrefix,
		keyTTL:    config.DefaultTTL,
		config:    config,
	}, nil
}

// Close 关闭存储
func (s *RedisStorage) Close() error {
	return s.client.Close()
}

// Set 设置值
func (s *RedisStorage) Set(ctx context.Context, key string, value interface{}) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	// 设置键值
	return s.client.Set(ctx, s.formatKey(key), data, s.keyTTL).Err()
}

// Get 获取值
func (s *RedisStorage) Get(ctx context.Context, key string, value interface{}) error {
	// 获取值
	data, err := s.client.Get(ctx, s.formatKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("failed to get value: %v", err)
	}

	// 反序列化
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal value: %v", err)
	}
	return nil
}

// Delete 删除值
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.formatKey(key)).Err()
}

// Keys 获取匹配指定模式的键列表
func (s *RedisStorage) Keys(ctx context.Context, pattern string) ([]string, error) {
	return s.client.Keys(ctx, pattern).Result()
}

// Exists 检查键是否存在
func (s *RedisStorage) Exists(ctx context.Context, key string) (bool, error) {
	result, err := s.client.Exists(ctx, s.formatKey(key)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Pipeline 创建管道
func (s *RedisStorage) Pipeline() redis.Pipeliner {
	return s.client.Pipeline()
}

// formatKey 格式化键
func (s *RedisStorage) formatKey(key string) string {
	return s.keyPrefix + key
}

// GetKeyPrefix 获取键前缀
func (s *RedisStorage) GetKeyPrefix() string {
	return s.keyPrefix
}

// SetBatch 批量设置值
func (s *RedisStorage) SetBatch(ctx context.Context, items map[string]interface{}) error {
	// 创建管道
	pipe := s.client.Pipeline()

	// 添加命令
	for key, value := range items {
		// 序列化值
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}

		pipe.Set(ctx, s.formatKey(key), data, s.keyTTL)
	}

	// 执行命令
	_, err := pipe.Exec(ctx)
	return err
}

// GetBatch 批量获取值
func (s *RedisStorage) GetBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	// 创建管道
	pipe := s.client.Pipeline()

	// 添加命令
	for _, key := range keys {
		// 如果键已经包含前缀，则不再添加
		if strings.HasPrefix(key, s.keyPrefix) {
			pipe.Get(ctx, key)
		} else {
			pipe.Get(ctx, s.formatKey(key))
		}
	}

	// 执行命令
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	// 处理结果
	result := make(map[string][]byte)
	for i, cmd := range cmds {
		if cmd.Err() != nil && cmd.Err() != redis.Nil {
			return nil, cmd.Err()
		}

		if cmd.Err() == nil {
			val, err := cmd.(*redis.StringCmd).Bytes()
			if err != nil {
				return nil, err
			}
			result[keys[i]] = val
		}
	}

	return result, nil
}

// DeleteBatch 批量删除值
func (s *RedisStorage) DeleteBatch(ctx context.Context, keys []string) error {
	// 格式化键
	formattedKeys := make([]string, len(keys))
	for i, key := range keys {
		formattedKeys[i] = s.formatKey(key)
	}

	// 删除键
	return s.client.Del(ctx, formattedKeys...).Err()
}

// SetTTL 设置过期时间
func (s *RedisStorage) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, s.formatKey(key), ttl).Err()
}

// GetTTL 获取过期时间
func (s *RedisStorage) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return s.client.TTL(ctx, s.formatKey(key)).Result()
}

// Increment 增加计数器
func (s *RedisStorage) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	if delta == 1 {
		return s.client.Incr(ctx, s.formatKey(key)).Result()
	}
	return s.client.IncrBy(ctx, s.formatKey(key), delta).Result()
}

// SetWithTTL 设置值并指定TTL
func (s *RedisStorage) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	// 设置键值
	return s.client.Set(ctx, s.formatKey(key), data, ttl).Err()
}

// HashSet 设置哈希字段
func (s *RedisStorage) HashSet(ctx context.Context, key string, field string, value interface{}) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	// 设置哈希字段
	return s.client.HSet(ctx, s.formatKey(key), field, data).Err()
}

// HashGet 获取哈希字段
func (s *RedisStorage) HashGet(ctx context.Context, key string, field string, value interface{}) error {
	// 获取哈希字段
	data, err := s.client.HGet(ctx, s.formatKey(key), field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("field not found: %s", field)
		}
		return fmt.Errorf("failed to get hash field: %v", err)
	}

	// 反序列化
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal value: %v", err)
	}
	return nil
}

// HashGetAll 获取所有哈希字段
func (s *RedisStorage) HashGetAll(ctx context.Context, key string) (map[string]string, error) {
	return s.client.HGetAll(ctx, s.formatKey(key)).Result()
}

// ListPush 列表推入
func (s *RedisStorage) ListPush(ctx context.Context, key string, value interface{}) error {
	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	// 推入列表
	return s.client.LPush(ctx, s.formatKey(key), data).Err()
}

// ListPop 列表弹出
func (s *RedisStorage) ListPop(ctx context.Context, key string, value interface{}) error {
	// 弹出列表
	data, err := s.client.RPop(ctx, s.formatKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("list is empty")
		}
		return fmt.Errorf("failed to pop from list: %v", err)
	}

	// 反序列化
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("failed to unmarshal value: %v", err)
	}
	return nil
}

// ListRange 获取列表范围
func (s *RedisStorage) ListRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return s.client.LRange(ctx, s.formatKey(key), start, stop).Result()
}

// SetAdd 集合添加
func (s *RedisStorage) SetAdd(ctx context.Context, key string, member interface{}) error {
	// 序列化成员
	data, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("failed to marshal member: %v", err)
	}

	// 添加到集合
	return s.client.SAdd(ctx, s.formatKey(key), data).Err()
}

// SetMembers 获取集合成员
func (s *RedisStorage) SetMembers(ctx context.Context, key string) ([]string, error) {
	return s.client.SMembers(ctx, s.formatKey(key)).Result()
}

// Publish 发布消息
func (s *RedisStorage) Publish(ctx context.Context, channel string, message interface{}) error {
	// 序列化消息
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// 发布消息
	return s.client.Publish(ctx, s.formatKey(channel), data).Err()
}

// Subscribe 订阅消息
func (s *RedisStorage) Subscribe(ctx context.Context, channels []string) <-chan *redis.Message {
	pubsub := s.client.Subscribe(ctx, channels...)
	return pubsub.Channel()
}

// GetStats 获取Redis统计信息
func (s *RedisStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := s.client.Info(ctx).Result()
	if err != nil {
		return nil, err
	}

	// 简单的信息解析
	stats := make(map[string]interface{})
	stats["info"] = info

	// 获取连接池状态
	poolStats := s.client.PoolStats()
	stats["pool_stats"] = poolStats

	return stats, nil
}

// Health 健康检查
func (s *RedisStorage) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// SaveMetricsData 保存指标数据
func (s *RedisStorage) SaveMetricsData(ctx context.Context, data *models.MetricsData) error {
	// 构建键，确保使用正确的键前缀格式
	// 使用统一的键格式：monitor:metrics:hostID:timestamp
	key := fmt.Sprintf("metrics:%s:%d", data.HostID, data.Timestamp)
	formattedKey := s.formatKey(key)

	// 使用哈希结构存储指标数据
	pipe := s.client.Pipeline()

	// 存储基本信息
	pipe.HSet(ctx, formattedKey, "host_id", data.HostID)
	pipe.HSet(ctx, formattedKey, "timestamp", data.Timestamp)

	// 存储指标
	if data.Metrics != nil {
		for k, v := range data.Metrics {
			metricData, _ := json.Marshal(v)
			pipe.HSet(ctx, formattedKey, fmt.Sprintf("metric:%s", k), metricData)
		}
	}

	// 存储标签
	if data.Tags != nil {
		for k, v := range data.Tags {
			pipe.HSet(ctx, formattedKey, fmt.Sprintf("tag:%s", k), v)
		}
	}

	// 设置过期时间（默认7天）
	pipe.Expire(ctx, formattedKey, 7*24*time.Hour)

	// 添加到时间序列索引
	indexKey := fmt.Sprintf("index:metrics:%s", data.HostID)
	formattedIndexKey := s.formatKey(indexKey)
	pipe.ZAdd(ctx, formattedIndexKey, redis.Z{
		Score:  float64(data.Timestamp),
		Member: key, // 注意：这里使用未格式化的key作为成员，因为GetMetricsDataRange会使用它
	})

	// 设置索引过期时间
	pipe.Expire(ctx, formattedIndexKey, 7*24*time.Hour)

	// 执行管道命令，并捕获可能的错误
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Redis管道执行失败: %v", err)
		// 如果错误是由于事务已关闭导致的，尝试重新执行
		if strings.Contains(err.Error(), "tx closed") {
			log.Printf("警告: 事务已关闭，尝试重新执行: %v", err)

			// 创建新的管道并重试
			pipe = s.client.Pipeline()

			// 重新添加所有命令
			pipe.HSet(ctx, formattedKey, "host_id", data.HostID)
			pipe.HSet(ctx, formattedKey, "timestamp", data.Timestamp)

			// 重新存储指标
			if data.Metrics != nil {
				for k, v := range data.Metrics {
					metricData, _ := json.Marshal(v)
					pipe.HSet(ctx, formattedKey, fmt.Sprintf("metric:%s", k), metricData)
				}
			}

			// 重新存储标签
			if data.Tags != nil {
				for k, v := range data.Tags {
					pipe.HSet(ctx, formattedKey, fmt.Sprintf("tag:%s", k), v)
				}
			}

			// 重新设置过期时间
			pipe.Expire(ctx, formattedKey, 7*24*time.Hour)

			// 重新添加到时间序列索引
			pipe.ZAdd(ctx, formattedIndexKey, redis.Z{
				Score:  float64(data.Timestamp),
				Member: key,
			})

			// 重新设置索引过期时间
			pipe.Expire(ctx, formattedIndexKey, 7*24*time.Hour)

			// 执行重试
			_, retryErr := pipe.Exec(ctx)
			if retryErr != nil {
				log.Printf("重试保存指标数据失败: %v", retryErr)
				return retryErr
			}

			log.Printf("重试保存指标数据成功，键: %s", formattedKey)
			return nil
		}
		return err
	}
	// 成功日志（调试）：记录键与指标数量
	log.Printf("保存指标数据到Redis成功: key=%s metrics=%d", formattedKey, len(data.Metrics))
	return nil
}

// StoreMetrics 实现 MetricsStorage 接口
func (s *RedisStorage) StoreMetrics(ctx context.Context, data models.MetricsData) error {
	return s.SaveMetricsData(ctx, &data)
}

// GetMetricsData 获取指标数据
func (s *RedisStorage) GetMetricsData(ctx context.Context, hostID string, timestamp int64) (*models.MetricsData, error) {
	// 使用统一的键格式：monitor:metrics:hostID:timestamp
	key := fmt.Sprintf("metrics:%s:%d", hostID, timestamp)
	formattedKey := s.formatKey(key)

	// 获取哈希所有字段
	result, err := s.client.HGetAll(ctx, formattedKey).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("metrics data not found")
	}

	// 解析数据
	data := &models.MetricsData{
		HostID:    result["host_id"],
		Timestamp: timestamp,
		Metrics:   make(map[string]interface{}),
		Tags:      make(map[string]string),
	}

	// 解析指标
	for k, v := range result {
		if strings.HasPrefix(k, "metric:") {
			metricKey := strings.TrimPrefix(k, "metric:")
			var metricValue interface{}
			if err := json.Unmarshal([]byte(v), &metricValue); err == nil {
				data.Metrics[metricKey] = metricValue
			}
		} else if strings.HasPrefix(k, "tag:") {
			tagKey := strings.TrimPrefix(k, "tag:")
			data.Tags[tagKey] = v
		}
	}

	return data, nil
}

// GetMetricsDataRange 获取时间范围内的指标数据
func (s *RedisStorage) GetMetricsDataRange(ctx context.Context, hostID string, start, end int64) ([]*models.MetricsData, error) {
	// 使用统一的键格式：monitor:index:metrics:hostID
	indexKey := fmt.Sprintf("index:metrics:%s", hostID)
	formattedIndexKey := s.formatKey(indexKey)

	// 获取时间范围内的键
	zRangeCmd := s.client.ZRangeByScore(ctx, formattedIndexKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", start),
		Max: fmt.Sprintf("%d", end),
	})

	keys, err := zRangeCmd.Result()
	if err != nil {
		return nil, err
	}

	// 批量获取数据
	var metrics []*models.MetricsData
	for _, key := range keys {
		// 解析时间戳
		parts := strings.Split(key, ":")
		if len(parts) >= 3 {
			if timestamp, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
				if data, err := s.GetMetricsData(ctx, hostID, timestamp); err == nil {
					metrics = append(metrics, data)
				}
			}
		}
	}

	return metrics, nil
}

// DeleteMetricsData 删除指标数据
func (s *RedisStorage) DeleteMetricsData(ctx context.Context, hostID string, timestamp int64) error {
	key := fmt.Sprintf("metrics:%s:%d", hostID, timestamp)
	indexKey := fmt.Sprintf("index:metrics:%s", hostID)

	// 删除数据和索引
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.formatKey(key))
	pipe.ZRem(ctx, s.formatKey(indexKey), key)

	_, err := pipe.Exec(ctx)
	return err
}

// AggregateMetrics 聚合指标数据
func (s *RedisStorage) AggregateMetrics(ctx context.Context, hostID string, start, end int64, interval time.Duration) (map[string]interface{}, error) {
	// 获取时间范围内的数据
	metrics, err := s.GetMetricsDataRange(ctx, hostID, start, end)
	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return map[string]interface{}{}, nil
	}

	// 简单聚合计算
	aggregated := make(map[string]interface{})

	// 按指标名称聚合
	metricValues := make(map[string][]float64)
	for _, data := range metrics {
		for metricName, metricValue := range data.Metrics {
			if value, ok := metricValue.(float64); ok {
				metricValues[metricName] = append(metricValues[metricName], value)
			}
		}
	}

	// 计算统计值
	for metricName, values := range metricValues {
		if len(values) > 0 {
			sum := 0.0
			min := values[0]
			max := values[0]

			for _, v := range values {
				sum += v
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}

			avg := sum / float64(len(values))

			aggregated[metricName] = map[string]interface{}{
				"count": len(values),
				"sum":   sum,
				"avg":   avg,
				"min":   min,
				"max":   max,
			}
		}
	}

	return aggregated, nil
}
