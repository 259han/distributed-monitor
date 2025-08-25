// Package interfaces 定义了系统中的核心接口
package interfaces

import "context"

// Metric 表示一个监控指标
type Metric struct {
	Name      string
	Value     float64
	Labels    map[string]string
	Timestamp int64
}

// Collector 定义了数据收集器的接口
type Collector interface {
	// Start 启动收集器
	Start(ctx context.Context) error

	// Stop 停止收集器
	Stop() error

	// Collect 收集监控指标
	Collect() ([]Metric, error)
}
