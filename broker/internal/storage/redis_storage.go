package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage Redis存储实现
type RedisStorage struct {
	client    *redis.Client
	keyPrefix string
	keyTTL    time.Duration
}

// RedisConfig Redis配置
type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

// NewRedisStorage 创建Redis存储
func NewRedisStorage(config RedisConfig) (*RedisStorage, error) {
	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address, // 目前只使用第一个地址
		Password: config.Password,
		DB:       config.DB,
		PoolSize: 10, // 默认池大小
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStorage{
		client:    client,
		keyPrefix: "", // 默认无前缀
		keyTTL:    0,  // 默认无过期时间
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
		return err
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
		return err
	}

	// 反序列化
	return json.Unmarshal(data, value)
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
		pipe.Get(ctx, s.formatKey(key))
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
