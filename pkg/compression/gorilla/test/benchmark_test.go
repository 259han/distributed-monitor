package test

import (
	"math"
	"testing"
	"time"

	"github.com/han-fei/monitor/pkg/compression"
	"github.com/han-fei/monitor/pkg/compression/gorilla"
)

// 生成测试数据
func generateTestData(n int) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	points := make([]compression.TimeSeriesPoint, n)
	for i := 0; i < n; i++ {
		points[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     100.0 + 10.0*math.Sin(float64(i)*0.1),
		}
	}
	return points
}

// BenchmarkGorillaCompression 压缩性能基准测试
func BenchmarkGorillaCompression(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run("压缩-"+string(rune(size)), func(b *testing.B) {
			compressor := gorilla.NewGorillaCompressor()
			defer compressor.Close()
			
			data := generateTestData(size)
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				compressed, err := compressor.Compress(data)
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(compressed)))
			}
		})
	}
}

// BenchmarkGorillaDecompression 解压性能基准测试
func BenchmarkGorillaDecompression(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run("解压-"+string(rune(size)), func(b *testing.B) {
			compressor := gorilla.NewGorillaCompressor()
			defer compressor.Close()
			
			data := generateTestData(size)
			compressed, err := compressor.Compress(data)
			if err != nil {
				b.Fatal(err)
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := compressor.Decompress(compressed)
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(compressed)))
			}
		})
	}
}

// BenchmarkGorillaCompressionRatio 压缩比基准测试
func BenchmarkGorillaCompressionRatio(b *testing.B) {
	patterns := []struct {
		name     string
		generate func(int) []compression.TimeSeriesPoint
	}{
		{
			name: "常量数据",
			generate: func(n int) []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				points := make([]compression.TimeSeriesPoint, n)
				for i := 0; i < n; i++ {
					points[i] = compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     42.0,
					}
				}
				return points
			},
		},
		{
			name: "阶跃函数",
			generate: func(n int) []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				points := make([]compression.TimeSeriesPoint, n)
				for i := 0; i < n; i++ {
					value := 10.0
					if i >= n/2 {
						value = 20.0
					}
					points[i] = compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     value,
					}
				}
				return points
			},
		},
		{
			name: "正弦波",
			generate: func(n int) []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				points := make([]compression.TimeSeriesPoint, n)
				for i := 0; i < n; i++ {
					points[i] = compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     100.0 + 10.0*math.Sin(float64(i)*0.1),
					}
				}
				return points
			},
		},
		{
			name: "随机数据",
			generate: func(n int) []compression.TimeSeriesPoint {
				baseTime := time.Now().Unix()
				points := make([]compression.TimeSeriesPoint, n)
				for i := 0; i < n; i++ {
					points[i] = compression.TimeSeriesPoint{
						Timestamp: baseTime + int64(i),
						Value:     100.0 + float64(i%17)*0.5,
					}
				}
				return points
			},
		},
	}

	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()
	
	for _, pattern := range patterns {
		data := pattern.generate(1000)
		compressed, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
		
		originalSize := len(data) * 16 // 每个点16字节
		compressedSize := len(compressed)
		ratio := float64(originalSize) / float64(compressedSize)
		
		b.Logf("%s: 压缩比 %.2fx (原始: %d bytes, 压缩后: %d bytes)",
			pattern.name, ratio, originalSize, compressedSize)
	}
}

// BenchmarkGorillaConcurrency 并发性能基准测试
func BenchmarkGorillaConcurrency(b *testing.B) {
	if testing.Short() {
		b.Skip("跳过并发测试")
	}
	
	concurrencyLevels := []int{1, 4, 16, 32}
	
	for _, concurrency := range concurrencyLevels {
		b.Run("并发-"+string(rune(concurrency)), func(b *testing.B) {
			data := generateTestData(1000)
			
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				compressor := gorilla.NewGorillaCompressor()
				defer compressor.Close()
				
				for pb.Next() {
					compressed, err := compressor.Compress(data)
					if err != nil {
						b.Fatal(err)
					}
					
					_, err = compressor.Decompress(compressed)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
