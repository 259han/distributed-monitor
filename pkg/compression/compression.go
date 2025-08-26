package compression

import (
	"time"
)

// TimeSeriesPoint 表示时间序列中的一个数据点
type TimeSeriesPoint struct {
	Timestamp int64   // Unix时间戳
	Value     float64 // 数据点的值
}

// Compressor 定义了时间序列压缩器的接口
type Compressor interface {
	// Compress 压缩时间序列数据
	Compress(points []TimeSeriesPoint) ([]byte, error)

	// Decompress 解压缩时间序列数据
	Decompress(data []byte) ([]TimeSeriesPoint, error)

	// Close 关闭压缩器并释放资源
	Close() error
}

// CompressionStats 保存压缩统计信息
type CompressionStats struct {
	TotalRequests      int64     // 总请求数
	CompressedRequests int64     // 实际压缩的请求数
	OriginalBytes      int64     // 原始数据大小（字节）
	CompressedBytes    int64     // 压缩后数据大小（字节）
	AverageRatio       float64   // 平均压缩比
	TotalSavings       int64     // 总节省空间（字节）
	LastUpdated        time.Time // 最后更新时间
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled           bool    // 是否启用压缩
	MinDataPoints     int     // 最小数据点数量，低于此值不压缩
	CompressionRatio  float64 // 最小压缩比，低于此值不保存压缩结果
	BatchSize         int     // 批处理大小
	EnableStats       bool    // 是否启用统计
	TimeWindowMinutes int     // 时间窗口（分钟）
}

// DefaultCompressionConfig 返回默认压缩配置
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Enabled:           true,
		MinDataPoints:     10,
		CompressionRatio:  1.2,
		BatchSize:         100,
		EnableStats:       true,
		TimeWindowMinutes: 60,
	}
}
