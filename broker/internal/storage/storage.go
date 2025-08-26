package storage

import (
	"context"
	"github.com/han-fei/monitor/broker/internal/models"
)

// MetricsStorage 指标存储接口
type MetricsStorage interface {
	// StoreMetrics 存储指标数据
	StoreMetrics(ctx context.Context, data models.MetricsData) error

	// Close 关闭存储连接
	Close() error
}
