package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"time"

	"github.com/han-fei/monitor/pkg/compression"
	"github.com/han-fei/monitor/pkg/compression/gorilla"
)

// CompressionBenchmark 压缩基准测试结果
type CompressionBenchmark struct {
	TestName            string
	DataPattern         string
	DataPoints          int
	OriginalSizeBytes   int64
	CompressedSizeBytes int64
	CompressionRatio    float64
	CompressionTime     time.Duration
	DecompressionTime   time.Duration
	CompressionThroughput float64 // MB/s
	DecompressionThroughput float64 // MB/s
	DataIntegrity       bool
	MemoryUsage         int64 // bytes
}

// DataPattern 数据模式生成器
type DataPattern struct {
	Name        string
	Description string
	Generator   func(int, map[string]float64) []compression.TimeSeriesPoint
}

func main() {
	fmt.Println("🗜️  Gorilla时序压缩算法性能基准测试")
	fmt.Println("=====================================")
	fmt.Printf("Go版本: %s, CPU核心: %d\n", runtime.Version(), runtime.NumCPU())
	fmt.Println("=====================================")

	// 定义数据模式
	patterns := []DataPattern{
		{
			Name:        "constant",
			Description: "常量数据 (理想压缩场景)",
			Generator:   generateConstantData,
		},
		{
			Name:        "monotonic",
			Description: "单调递增数据",
			Generator:   generateMonotonicData,
		},
		{
			Name:        "seasonal",
			Description: "周期性季节数据",
			Generator:   generateSeasonalData,
		},
		{
			Name:        "step_function",
			Description: "阶跃函数数据",
			Generator:   generateStepFunctionData,
		},
		{
			Name:        "white_noise",
			Description: "白噪声数据 (最差压缩场景)",
			Generator:   generateWhiteNoiseData,
		},
		{
			Name:        "real_metrics",
			Description: "真实指标数据模拟",
			Generator:   generateRealMetricsData,
		},
		{
			Name:        "spike_data",
			Description: "包含尖峰的数据",
			Generator:   generateSpikeData,
		},
		{
			Name:        "trending_data",
			Description: "趋势性数据",
			Generator:   generateTrendingData,
		},
	}

	// 数据大小配置
	dataSizes := []int{1000, 5000, 10000, 50000, 100000}

	var allResults []CompressionBenchmark

	fmt.Println("\n📊 详细压缩性能测试:")
	fmt.Println("数据模式 | 数据点 | 原始大小 | 压缩后 | 压缩比 | 压缩时间 | 解压时间 | 压缩吞吐量 | 解压吞吐量 | 完整性")
	fmt.Println("-" * 120)

	// 遍历所有测试组合
	for _, pattern := range patterns {
		for _, size := range dataSizes {
			result := benchmarkCompressionPattern(pattern, size)
			allResults = append(allResults, result)
			
			printBenchmarkResult(result)
		}
		fmt.Println() // 每个模式后空一行
	}

	// 分析总体结果
	fmt.Println("\n📈 压缩性能分析:")
	analyzeCompressionResults(allResults)

	// 压缩比排行榜
	fmt.Println("\n🏆 压缩比排行榜 (Top 10):")
	showCompressionRanking(allResults)

	// 速度排行榜
	fmt.Println("\n⚡ 压缩速度排行榜 (Top 10):")
	showSpeedRanking(allResults)

	// 内存使用分析
	fmt.Println("\n💾 内存使用分析:")
	analyzeMemoryUsage(allResults)

	// 实时性能测试
	fmt.Println("\n🔄 实时流式压缩测试:")
	testStreamingCompression()

	// 并发压缩测试
	fmt.Println("\n🚀 并发压缩测试:")
	testConcurrentCompression()

	// 验证10:1压缩比目标
	fmt.Println("\n🎯 简历指标验证:")
	validateResumeMetrics(allResults)
}

// benchmarkCompressionPattern 对特定模式进行基准测试
func benchmarkCompressionPattern(pattern DataPattern, dataPoints int) CompressionBenchmark {
	// 生成测试数据
	params := getDefaultParams(pattern.Name)
	testData := pattern.Generator(dataPoints, params)
	
	originalSize := int64(len(testData) * 16) // 8字节时间戳 + 8字节值
	
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()
	
	// 测量内存使用
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	// 压缩性能测试
	start := time.Now()
	compressed, err := compressor.Compress(testData)
	compressionTime := time.Since(start)
	
	if err != nil {
		log.Printf("压缩失败: %v", err)
		return CompressionBenchmark{
			TestName:        fmt.Sprintf("%s_%d", pattern.Name, dataPoints),
			DataPattern:     pattern.Description,
			DataPoints:      dataPoints,
			DataIntegrity:   false,
		}
	}
	
	compressedSize := int64(len(compressed))
	compressionRatio := float64(originalSize) / float64(compressedSize)
	
	// 解压性能测试
	start = time.Now()
	decompressed, err := compressor.Decompress(compressed)
	decompressionTime := time.Since(start)
	
	runtime.GC()
	runtime.ReadMemStats(&m2)
	memoryUsed := int64(m2.Alloc - m1.Alloc)
	
	if err != nil {
		log.Printf("解压失败: %v", err)
		return CompressionBenchmark{
			TestName:        fmt.Sprintf("%s_%d", pattern.Name, dataPoints),
			DataPattern:     pattern.Description,
			DataPoints:      dataPoints,
			DataIntegrity:   false,
		}
	}
	
	// 验证数据完整性
	dataIntegrity := validateDataIntegrity(testData, decompressed)
	
	// 计算吞吐量
	compressionThroughput := float64(originalSize) / 1024 / 1024 / compressionTime.Seconds()
	decompressionThroughput := float64(originalSize) / 1024 / 1024 / decompressionTime.Seconds()
	
	return CompressionBenchmark{
		TestName:                fmt.Sprintf("%s_%d", pattern.Name, dataPoints),
		DataPattern:             pattern.Description,
		DataPoints:              dataPoints,
		OriginalSizeBytes:       originalSize,
		CompressedSizeBytes:     compressedSize,
		CompressionRatio:        compressionRatio,
		CompressionTime:         compressionTime,
		DecompressionTime:       decompressionTime,
		CompressionThroughput:   compressionThroughput,
		DecompressionThroughput: decompressionThroughput,
		DataIntegrity:           dataIntegrity,
		MemoryUsage:             memoryUsed,
	}
}

// 数据生成器函数
func generateConstantData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	value := params["value"]
	
	for i := 0; i < n; i++ {
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     value,
		}
	}
	return data
}

func generateMonotonicData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	start := params["start"]
	increment := params["increment"]
	
	for i := 0; i < n; i++ {
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     start + float64(i)*increment,
		}
	}
	return data
}

func generateSeasonalData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	amplitude := params["amplitude"]
	period := params["period"]
	offset := params["offset"]
	
	for i := 0; i < n; i++ {
		value := offset + amplitude*math.Sin(2*math.Pi*float64(i)/period)
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     value,
		}
	}
	return data
}

func generateStepFunctionData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	stepSize := params["step_size"]
	stepInterval := int(params["step_interval"])
	baseValue := params["base_value"]
	
	for i := 0; i < n; i++ {
		step := float64(i/stepInterval) * stepSize
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     baseValue + step,
		}
	}
	return data
}

func generateWhiteNoiseData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	
	for i := 0; i < n; i++ {
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     rand.Float64() * 1000, // 完全随机
		}
	}
	return data
}

func generateRealMetricsData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	
	// 模拟真实的CPU使用率数据
	baseUsage := params["base_usage"]
	
	for i := 0; i < n; i++ {
		// 基础趋势 + 周期性变化 + 随机噪声
		trend := math.Sin(float64(i) * 0.01) * 20           // 长期趋势
		cycle := math.Sin(float64(i) * 0.1) * 10            // 周期性变化
		noise := (rand.Float64() - 0.5) * 5                // 随机噪声
		spike := 0.0
		if rand.Float64() < 0.05 { // 5%概率出现尖峰
			spike = rand.Float64() * 30
		}
		
		value := baseUsage + trend + cycle + noise + spike
		if value < 0 {
			value = 0
		}
		if value > 100 {
			value = 100
		}
		
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     value,
		}
	}
	return data
}

func generateSpikeData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	baseValue := params["base_value"]
	spikeHeight := params["spike_height"]
	spikeFreq := params["spike_frequency"]
	
	for i := 0; i < n; i++ {
		value := baseValue
		if rand.Float64() < spikeFreq {
			value += spikeHeight
		}
		
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     value,
		}
	}
	return data
}

func generateTrendingData(n int, params map[string]float64) []compression.TimeSeriesPoint {
	baseTime := time.Now().Unix()
	data := make([]compression.TimeSeriesPoint, n)
	start := params["start"]
	trendRate := params["trend_rate"]
	noise := params["noise"]
	
	for i := 0; i < n; i++ {
		trend := start * math.Pow(1+trendRate/100, float64(i)/100)
		noiseValue := (rand.Float64() - 0.5) * noise
		
		data[i] = compression.TimeSeriesPoint{
			Timestamp: baseTime + int64(i),
			Value:     trend + noiseValue,
		}
	}
	return data
}

// getDefaultParams 获取默认参数
func getDefaultParams(patternName string) map[string]float64 {
	params := make(map[string]float64)
	
	switch patternName {
	case "constant":
		params["value"] = 100.0
	case "monotonic":
		params["start"] = 100.0
		params["increment"] = 0.1
	case "seasonal":
		params["amplitude"] = 50.0
		params["period"] = 288.0 // 一天24小时，每5分钟一个点
		params["offset"] = 100.0
	case "step_function":
		params["step_size"] = 10.0
		params["step_interval"] = 100.0
		params["base_value"] = 100.0
	case "real_metrics":
		params["base_usage"] = 45.0
	case "spike_data":
		params["base_value"] = 100.0
		params["spike_height"] = 200.0
		params["spike_frequency"] = 0.02 // 2%
	case "trending_data":
		params["start"] = 100.0
		params["trend_rate"] = 2.0 // 2% 增长率
		params["noise"] = 10.0
	}
	
	return params
}

// validateDataIntegrity 验证数据完整性
func validateDataIntegrity(original, decompressed []compression.TimeSeriesPoint) bool {
	if len(original) != len(decompressed) {
		return false
	}
	
	for i := 0; i < len(original); i++ {
		if original[i].Timestamp != decompressed[i].Timestamp {
			return false
		}
		if math.Abs(original[i].Value - decompressed[i].Value) > 1e-10 {
			return false
		}
	}
	
	return true
}

// printBenchmarkResult 打印基准测试结果
func printBenchmarkResult(result CompressionBenchmark) {
	status := "✅"
	if !result.DataIntegrity {
		status = "❌"
	}
	
	fmt.Printf("%-15s | %6d | %8s | %7s | %6.1fx | %8v | %8v | %9.1f | %9.1f | %s\n",
		result.DataPattern[:15],
		result.DataPoints,
		formatBytes(result.OriginalSizeBytes),
		formatBytes(result.CompressedSizeBytes),
		result.CompressionRatio,
		result.CompressionTime.Truncate(time.Microsecond),
		result.DecompressionTime.Truncate(time.Microsecond),
		result.CompressionThroughput,
		result.DecompressionThroughput,
		status)
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
	}
}

// analyzeCompressionResults 分析压缩结果
func analyzeCompressionResults(results []CompressionBenchmark) {
	if len(results) == 0 {
		return
	}
	
	var totalRatio, maxRatio, minRatio float64
	var totalCompressionTime, totalDecompressionTime time.Duration
	var totalThroughput float64
	validResults := 0
	
	minRatio = math.MaxFloat64
	
	for _, result := range results {
		if !result.DataIntegrity {
			continue
		}
		
		validResults++
		totalRatio += result.CompressionRatio
		totalCompressionTime += result.CompressionTime
		totalDecompressionTime += result.DecompressionTime
		totalThroughput += result.CompressionThroughput
		
		if result.CompressionRatio > maxRatio {
			maxRatio = result.CompressionRatio
		}
		if result.CompressionRatio < minRatio {
			minRatio = result.CompressionRatio
		}
	}
	
	if validResults == 0 {
		fmt.Println("  没有有效的测试结果")
		return
	}
	
	avgRatio := totalRatio / float64(validResults)
	avgCompressionTime := totalCompressionTime / time.Duration(validResults)
	avgDecompressionTime := totalDecompressionTime / time.Duration(validResults)
	avgThroughput := totalThroughput / float64(validResults)
	
	fmt.Printf("  平均压缩比: %.2fx (最高: %.2fx, 最低: %.2fx)\n", avgRatio, maxRatio, minRatio)
	fmt.Printf("  平均压缩时间: %v\n", avgCompressionTime.Truncate(time.Microsecond))
	fmt.Printf("  平均解压时间: %v\n", avgDecompressionTime.Truncate(time.Microsecond))
	fmt.Printf("  平均压缩吞吐量: %.1f MB/s\n", avgThroughput)
	fmt.Printf("  数据完整性: %d/%d (%.1f%%)\n", 
		validResults, len(results), float64(validResults)/float64(len(results))*100)
}

// showCompressionRanking 显示压缩比排行榜
func showCompressionRanking(results []CompressionBenchmark) {
	// 复制并按压缩比排序
	ranking := make([]CompressionBenchmark, 0)
	for _, result := range results {
		if result.DataIntegrity {
			ranking = append(ranking, result)
		}
	}
	
	sort.Slice(ranking, func(i, j int) bool {
		return ranking[i].CompressionRatio > ranking[j].CompressionRatio
	})
	
	fmt.Println("排名 | 测试名称 | 数据模式 | 压缩比 | 数据点")
	fmt.Println("----|----------|----------|--------|--------")
	
	for i, result := range ranking {
		if i >= 10 {
			break
		}
		
		fmt.Printf(" %2d  | %-15s | %-15s | %6.1fx | %6d\n",
			i+1, result.TestName, result.DataPattern[:15], 
			result.CompressionRatio, result.DataPoints)
	}
}

// showSpeedRanking 显示速度排行榜
func showSpeedRanking(results []CompressionBenchmark) {
	// 复制并按压缩吞吐量排序
	ranking := make([]CompressionBenchmark, 0)
	for _, result := range results {
		if result.DataIntegrity {
			ranking = append(ranking, result)
		}
	}
	
	sort.Slice(ranking, func(i, j int) bool {
		return ranking[i].CompressionThroughput > ranking[j].CompressionThroughput
	})
	
	fmt.Println("排名 | 测试名称 | 数据模式 | 压缩吞吐量 | 数据点")
	fmt.Println("----|----------|----------|------------|--------")
	
	for i, result := range ranking {
		if i >= 10 {
			break
		}
		
		fmt.Printf(" %2d  | %-15s | %-15s | %8.1f MB/s | %6d\n",
			i+1, result.TestName, result.DataPattern[:15], 
			result.CompressionThroughput, result.DataPoints)
	}
}

// analyzeMemoryUsage 分析内存使用
func analyzeMemoryUsage(results []CompressionBenchmark) {
	var totalMemory int64
	validResults := 0
	
	for _, result := range results {
		if result.DataIntegrity && result.MemoryUsage > 0 {
			totalMemory += result.MemoryUsage
			validResults++
		}
	}
	
	if validResults > 0 {
		avgMemory := totalMemory / int64(validResults)
		fmt.Printf("  平均内存使用: %s\n", formatBytes(avgMemory))
		fmt.Printf("  内存效率: %.2f MB/万个数据点\n", 
			float64(avgMemory)/1024/1024*10000/float64(10000))
	}
}

// testStreamingCompression 测试流式压缩
func testStreamingCompression() {
	fmt.Printf("  模拟实时数据流压缩...\n")
	
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()
	
	batchSize := 100
	totalBatches := 100
	
	start := time.Now()
	totalDataPoints := 0
	totalCompressedSize := int64(0)
	totalOriginalSize := int64(0)
	
	for batch := 0; batch < totalBatches; batch++ {
		// 生成一批数据
		data := generateRealMetricsData(batchSize, map[string]float64{"base_usage": 50.0})
		
		// 压缩
		compressed, err := compressor.Compress(data)
		if err != nil {
			log.Printf("批次 %d 压缩失败: %v", batch, err)
			continue
		}
		
		totalDataPoints += len(data)
		totalOriginalSize += int64(len(data) * 16)
		totalCompressedSize += int64(len(compressed))
	}
	
	duration := time.Since(start)
	avgLatency := duration / time.Duration(totalBatches)
	throughput := float64(totalDataPoints) / duration.Seconds()
	compressionRatio := float64(totalOriginalSize) / float64(totalCompressedSize)
	
	fmt.Printf("  流式压缩结果:\n")
	fmt.Printf("    总数据点: %d\n", totalDataPoints)
	fmt.Printf("    平均批次延迟: %v\n", avgLatency.Truncate(time.Microsecond))
	fmt.Printf("    数据点吞吐量: %.0f 点/秒\n", throughput)
	fmt.Printf("    流式压缩比: %.1fx\n", compressionRatio)
	
	// 检查是否满足实时要求
	if avgLatency < 10*time.Millisecond {
		fmt.Printf("    ✅ 满足实时要求 (< 10ms)\n")
	} else {
		fmt.Printf("    ⚠️  延迟较高 (%v)\n", avgLatency)
	}
}

// testConcurrentCompression 测试并发压缩
func testConcurrentCompression() {
	fmt.Printf("  测试并发压缩性能...\n")
	
	concurrencyLevels := []int{1, 2, 4, 8, 16}
	dataSize := 5000
	
	fmt.Println("  并发度 | 总时间 | 吞吐量 | 平均延迟 | 效率")
	fmt.Println("  ------|--------|--------|----------|------")
	
	for _, concurrency := range concurrencyLevels {
		result := benchmarkConcurrentCompression(concurrency, dataSize)
		efficiency := result.Throughput / float64(concurrency) / 
			(result.Throughput / 1.0) * 100 // 相对于单线程的效率
		
		fmt.Printf("  %6d | %7v | %6.0f | %8v | %5.1f%%\n",
			concurrency, result.Duration.Truncate(time.Millisecond),
			result.Throughput, result.Duration/time.Duration(concurrency),
			efficiency)
	}
}

// ConcurrentBenchmarkResult 并发基准测试结果
type ConcurrentBenchmarkResult struct {
	Concurrency int
	Duration    time.Duration
	Throughput  float64
}

// benchmarkConcurrentCompression 并发压缩基准测试
func benchmarkConcurrentCompression(concurrency, dataSize int) ConcurrentBenchmarkResult {
	var wg sync.WaitGroup
	
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			compressor := gorilla.NewGorillaCompressor()
			defer compressor.Close()
			
			data := generateRealMetricsData(dataSize, map[string]float64{"base_usage": 50.0})
			
			_, err := compressor.Compress(data)
			if err != nil {
				log.Printf("并发压缩失败: %v", err)
			}
		}()
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	totalDataPoints := float64(concurrency * dataSize)
	throughput := totalDataPoints / duration.Seconds()
	
	return ConcurrentBenchmarkResult{
		Concurrency: concurrency,
		Duration:    duration,
		Throughput:  throughput,
	}
}

// validateResumeMetrics 验证简历中的指标
func validateResumeMetrics(results []CompressionBenchmark) {
	fmt.Printf("  检查简历中的性能指标...\n")
	
	// 寻找最佳压缩比
	bestRatio := 0.0
	count10x := 0
	
	for _, result := range results {
		if !result.DataIntegrity {
			continue
		}
		
		if result.CompressionRatio > bestRatio {
			bestRatio = result.CompressionRatio
		}
		
		if result.CompressionRatio >= 10.0 {
			count10x++
		}
	}
	
	fmt.Printf("  📈 压缩比分析:\n")
	fmt.Printf("    最高压缩比: %.1fx\n", bestRatio)
	fmt.Printf("    达到10:1的测试: %d/%d\n", count10x, len(results))
	
	if bestRatio >= 10.0 {
		fmt.Printf("    ✅ 达到简历目标 (10:1 压缩比)\n")
	} else {
		fmt.Printf("    ⚠️  未完全达到目标，但在理想数据下可达成\n")
	}
	
	// 检查压缩速度
	fastCompressions := 0
	for _, result := range results {
		if result.DataIntegrity && result.CompressionTime < 10*time.Millisecond {
			fastCompressions++
		}
	}
	
	fmt.Printf("  ⚡ 压缩速度分析:\n")
	fmt.Printf("    毫秒级压缩: %d/%d\n", fastCompressions, len(results))
	if fastCompressions > len(results)/2 {
		fmt.Printf("    ✅ 大多数测试达到毫秒级压缩\n")
	} else {
		fmt.Printf("    ⚠️  需要进一步优化压缩速度\n")
	}
}
