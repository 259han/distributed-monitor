package interfaces

import "context"

// Storage 定义了存储层的接口
type Storage interface {
	// Set 设置键值对
	Set(ctx context.Context, key string, value interface{}) error

	// Get 获取键对应的值
	Get(ctx context.Context, key string, value interface{}) error

	// Delete 删除键值对
	Delete(ctx context.Context, key string) error

	// List 列出指定前缀的所有键
	List(ctx context.Context, prefix string) ([]string, error)

	// Close 关闭存储连接
	Close() error
}
