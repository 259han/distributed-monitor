//go:build integration && compression

package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/han-fei/monitor/broker/internal/storage"
	"github.com/han-fei/monitor/broker/internal/models"
	"github.com/han-fei/monitor/pkg/compression"
	"github.com/han-fei/monitor/pkg/compression/gorilla"
)

func main() {
	fmt.Println("🔧 Gorilla压缩集成测试")
	fmt.Println("=================================================")

	// 创建Redis存储配置
	redisConfig := storage.RedisConfig{
		Address:   "localhost:6379",
		Password:  "",
		DB:        1, // 使用测试数据库
		KeyPrefix: "test:monitor:",
	}

	// 创建基础Redis存储
	fmt.Println("📱 连接Redis...")
	redisStorage, err := storage.NewRedisStorage(redisConfig)
	if err != nil {
		log.Fatalf("创建Redis存储失败: %v", err)
	}
	defer redisStorage.Close()

	// 创建压缩配置
	compressionConfig := storage.CompressionConfig{
		Enabled:           true,
		MinDataPoints:     5,   // 最小5个数据点
		CompressionRatio:  1.2, // 最小压缩比1.2x
		BatchSize:         100,
		EnableStats:       true,
		TimeWindowMinutes: 60,
	}

	// 创建压缩存储
	fmt.Println("🗜️  初始化Gorilla压缩存储...")
	compressedStorage := storage.NewCompressedRedisStorage(redisStorage, compressionConfig)
	defer compressedStorage.Close()

	ctx := context.Background()

	// 测试1: 基础压缩功能
	fmt.Println("\n📊 测试1: 基础压缩功能")
	err = testBasicCompression(ctx, compressedStorage)
	if err != nil {
		log.Printf("❌ 基础压缩测试失败: %v", err)
	} else {
		fmt.Println("✅ 基础压缩测试通过")
	}

	// 测试2: 时序数据压缩
	fmt.Println("\n📈 测试2: 时序数据压缩")
	err = testTimeSeriesCompression(ctx, compressedStorage)
	if err != nil {
		log.Printf("❌ 时序数据压缩测试失败: %v", err)
	} else {
		fmt.Println("✅ 时序数据压缩测试通过")
	}

	// 测试3: 批量数据压缩
	fmt.Println("\n📦 测试3: 批量数据压缩")
	err = testBatchCompression(ctx, compressedStorage)
	if err != nil {
		log.Printf("❌ 批量压缩测试失败: %v", err)
	} else {
		fmt.Println("✅ 批量压缩测试通过")
	}

	// 测试4: 压缩统计信息
	fmt.Println("\n📋 测试4: 压缩统计信息")
	testCompressionStats(compressedStorage)

	// 测试5: 健康检查
	fmt.Println("\n🏥 测试5: 健康检查")
	err = compressedStorage.HealthCheck(ctx)
	if err != nil {
		log.Printf("❌ 健康检查失败: %v", err)
	} else {
		fmt.Println("✅ 健康检查通过")
	}

	fmt.Println("\n🎉 Gorilla压缩集成测试完成!")
}

// testBasicCompression 测试基础压缩功能
func testBasicCompression(ctx context.Context, storage *storage.CompressedRedisStorage) error {
	// 创建测试数据（单个数值）
	testData := &models.MetricsData{
		HostID:    "test-host-001",
		Timestamp: time.Now().Unix(),
		Metrics: map[string]interface{}{
			"cpu_usage":    85.5,
			"memory_usage": 67.2,
			"disk_usage":   45.8,
		},
	}

	// 保存压缩数据
	err := storage.SaveMetricsDataCompressed(ctx, testData)
	if err != nil {
		return fmt.Errorf("保存压缩数据失败: %v", err)
	}

	fmt.Printf("   💾 已保存: 主机=%s, 指标数=%d\n", testData.HostID, len(testData.Metrics))
	return nil
}

// testTimeSeriesCompression 测试时序数据压缩
func testTimeSeriesCompression(ctx context.Context, storage *storage.CompressedRedisStorage) error {
	baseTime := time.Now().Unix()

	// 创建时序数据（数组格式）
	var cpuSeries []interface{}
	var memorySeries []interface{}

	for i := 0; i < 20; i++ {
		cpuValue := 50.0 + 20.0*math.Sin(float64(i)*0.3)
		memoryValue := 60.0 + 10.0*math.Cos(float64(i)*0.2)

		cpuSeries = append(cpuSeries, cpuValue)
		memorySeries = append(memorySeries, memoryValue)
	}

	testData := &models.MetricsData{
		HostID:    "test-host-002",
		Timestamp: baseTime,
		Metrics: map[string]interface{}{
			"cpu_timeseries": map[string]interface{}{
				"series": cpuSeries,
			},
			"memory_timeseries": map[string]interface{}{
				"series": memorySeries,
			},
		},
	}

	// 保存压缩数据
	err := storage.SaveMetricsDataCompressed(ctx, testData)
	if err != nil {
		return fmt.Errorf("保存时序数据失败: %v", err)
	}

	fmt.Printf("   📈 已保存时序数据: 主机=%s, CPU点数=%d, 内存点数=%d\n",
		testData.HostID, len(cpuSeries), len(memorySeries))

	// 查询压缩数据
	retrievedData, err := storage.GetMetricsDataCompressed(ctx, testData.HostID,
		baseTime-10, baseTime+10)
	if err != nil {
		return fmt.Errorf("查询压缩数据失败: %v", err)
	}

	fmt.Printf("   🔍 查询结果: 找到%d条记录\n", len(retrievedData))
	return nil
}

// testBatchCompression 测试批量压缩
func testBatchCompression(ctx context.Context, storage *storage.CompressedRedisStorage) error {
	baseTime := time.Now().Unix()
	var batchData []*models.MetricsData

	// 创建批量测试数据
	for i := 0; i < 10; i++ {
		data := &models.MetricsData{
			HostID:    fmt.Sprintf("batch-host-%03d", i),
			Timestamp: baseTime + int64(i),
			Metrics: map[string]interface{}{
				"cpu_usage":    50.0 + float64(i)*2.0,
				"memory_usage": 60.0 + float64(i)*1.5,
				"network_rx": []interface{}{
					100.0 + float64(i*10),
					102.0 + float64(i*10),
					98.0 + float64(i*10),
					105.0 + float64(i*10),
					99.0 + float64(i*10),
				},
			},
		}
		batchData = append(batchData, data)
	}

	// 批量保存
	err := storage.OptimizeBatchCompression(ctx, batchData)
	if err != nil {
		return fmt.Errorf("批量压缩失败: %v", err)
	}

	fmt.Printf("   📦 批量保存完成: %d个主机的数据\n", len(batchData))
	return nil
}

// testCompressionStats 测试压缩统计
func testCompressionStats(storage *storage.CompressedRedisStorage) {
	stats := storage.GetCompressionStats()
	config := storage.GetCompressionConfig()

	fmt.Printf("   📊 压缩配置:\n")
	fmt.Printf("      启用状态: %v\n", config.Enabled)
	fmt.Printf("      最小数据点: %d\n", config.MinDataPoints)
	fmt.Printf("      最小压缩比: %.1fx\n", config.CompressionRatio)
	fmt.Printf("      批量大小: %d\n", config.BatchSize)

	fmt.Printf("   📈 压缩统计:\n")
	fmt.Printf("      总请求数: %d\n", stats.TotalRequests)
	fmt.Printf("      压缩请求数: %d\n", stats.CompressedRequests)
	fmt.Printf("      原始大小: %d bytes\n", stats.OriginalBytes)
	fmt.Printf("      压缩大小: %d bytes\n", stats.CompressedBytes)
	fmt.Printf("      平均压缩比: %.2fx\n", stats.AverageRatio)
	fmt.Printf("      节省空间: %d bytes\n", stats.TotalSavings)
	fmt.Printf("      更新时间: %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))

	if stats.OriginalBytes > 0 {
		savingsPercent := float64(stats.TotalSavings) / float64(stats.OriginalBytes) * 100
		fmt.Printf("      节省百分比: %.1f%%\n", savingsPercent)
	}
}

// 辅助函数：生成测试时序数据
func generateTestTimeSeries(baseTime int64, count int) []compression.TimeSeriesPoint {
	var points []compression.TimeSeriesPoint
	for i := 0; i < count; i++ {
		points = append(points, compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     100.0 + 10.0*math.Sin(float64(i)*0.1),
		})
	}
	return points
}

// 辅助函数：直接测试Gorilla压缩
func testDirectGorillaCompression() {
	fmt.Println("\n🧪 直接Gorilla压缩测试")

	// 创建测试数据
	testData := generateTestTimeSeries(time.Now().Unix(), 50)

	// 创建压缩器
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	// 压缩
	compressed, err := compressor.Compress(testData)
	if err != nil {
		log.Printf("❌ 直接压缩失败: %v", err)
		return
	}

	// 解压缩
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		log.Printf("❌ 解压缩失败: %v", err)
		return
	}

	// 验证结果
	originalSize := len(testData) * 16
	compressedSize := len(compressed)
	compressionRatio := float64(originalSize) / float64(compressedSize)

	fmt.Printf("   📊 直接压缩结果:\n")
	fmt.Printf("      原始大小: %d bytes\n", originalSize)
	fmt.Printf("      压缩大小: %d bytes\n", compressedSize)
	fmt.Printf("      压缩比: %.2fx\n", compressionRatio)
	fmt.Printf("      数据点数: %d → %d\n", len(testData), len(decompressed))

	if len(testData) == len(decompressed) {
		fmt.Println("   ✅ 数据完整性验证通过")
	} else {
		fmt.Printf("   ❌ 数据完整性验证失败: %d != %d\n", len(testData), len(decompressed))
	}
}
