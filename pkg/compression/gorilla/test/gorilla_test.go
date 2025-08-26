package test

import (
	"math"
	"testing"
	"time"

	"github.com/han-fei/monitor/pkg/compression"
	"github.com/han-fei/monitor/pkg/compression/gorilla"
	"github.com/stretchr/testify/assert"
)

// TestGorillaBasicCompression 测试基本的压缩和解压功能
func TestGorillaBasicCompression(t *testing.T) {
	// 创建压缩器
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	// 创建测试数据
	baseTime := time.Now().Unix()
	var points []compression.TimeSeriesPoint
	for i := 0; i < 100; i++ {
		points = append(points, compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     100.0 + 10.0*math.Sin(float64(i)*0.1),
		})
	}

	// 压缩
	compressed, err := compressor.Compress(points)
	assert.NoError(t, err)
	assert.NotNil(t, compressed)

	// 解压
	decompressed, err := compressor.Decompress(compressed)
	assert.NoError(t, err)
	assert.Equal(t, len(points), len(decompressed))

	// 验证数据完整性
	for i := 0; i < len(points); i++ {
		assert.Equal(t, points[i].Timestamp, decompressed[i].Timestamp)
		assert.Equal(t, points[i].Value, decompressed[i].Value)
	}
}

// TestGorillaEmptyData 测试空数据处理
func TestGorillaEmptyData(t *testing.T) {
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	// 空数据点
	_, err := compressor.Compress([]compression.TimeSeriesPoint{})
	assert.Error(t, err)

	// 空压缩数据
	_, err = compressor.Decompress([]byte{})
	assert.Error(t, err)
}

// TestGorillaDataPatterns 测试不同数据模式
func TestGorillaDataPatterns(t *testing.T) {
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	testCases := []struct {
		name     string
		generate func() []compression.TimeSeriesPoint
	}{
		{
			name: "常量数据",
			generate: func() []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				var points []compression.TimeSeriesPoint
				for i := 0; i < 100; i++ {
					points = append(points, compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     42.0, // 常量值
					})
				}
				return points
			},
		},
		{
			name: "阶跃函数",
			generate: func() []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				var points []compression.TimeSeriesPoint
				for i := 0; i < 100; i++ {
					value := 10.0
					if i >= 50 {
						value = 20.0
					}
					points = append(points, compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     value,
					})
				}
				return points
			},
		},
		{
			name: "正弦波",
			generate: func() []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				var points []compression.TimeSeriesPoint
				for i := 0; i < 100; i++ {
					points = append(points, compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     100.0 + 10.0*math.Sin(float64(i)*0.1),
					})
				}
				return points
			},
		},
		{
			name: "随机数据",
			generate: func() []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				var points []compression.TimeSeriesPoint
				for i := 0; i < 100; i++ {
					points = append(points, compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     100.0 + float64(i%17)*0.5,
					})
				}
				return points
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := tc.generate()

			// 压缩
			compressed, err := compressor.Compress(points)
			assert.NoError(t, err)

			// 计算压缩比
			originalSize := len(points) * 16 // 每个点16字节 (8字节时间戳 + 8字节值)
			compressedSize := len(compressed)
			compressionRatio := float64(originalSize) / float64(compressedSize)

			t.Logf("压缩比: %.2fx (原始: %d bytes, 压缩后: %d bytes)",
				compressionRatio, originalSize, compressedSize)

			// 解压
			decompressed, err := compressor.Decompress(compressed)
			assert.NoError(t, err)
			assert.Equal(t, len(points), len(decompressed))

			// 验证数据
			for i := 0; i < len(points); i++ {
				assert.Equal(t, points[i].Timestamp, decompressed[i].Timestamp)
				assert.Equal(t, points[i].Value, decompressed[i].Value)
			}
		})
	}
}

// TestGorillaLargeDataset 测试大数据集
func TestGorillaLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大数据集测试")
	}

	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	// 创建大数据集
	baseTime := time.Now().Unix()
	var points []compression.TimeSeriesPoint
	for i := 0; i < 10000; i++ {
		points = append(points, compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     100.0 + 10.0*math.Sin(float64(i)*0.01),
		})
	}

	// 压缩
	compressed, err := compressor.Compress(points)
	assert.NoError(t, err)

	// 解压
	decompressed, err := compressor.Decompress(compressed)
	assert.NoError(t, err)
	assert.Equal(t, len(points), len(decompressed))

	// 验证部分数据点
	for i := 0; i < len(points); i += 100 {
		assert.Equal(t, points[i].Timestamp, decompressed[i].Timestamp)
		assert.Equal(t, points[i].Value, decompressed[i].Value)
	}
}
