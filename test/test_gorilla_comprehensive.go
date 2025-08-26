package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/han-fei/monitor/pkg/compression"
	"github.com/han-fei/monitor/pkg/compression/gorilla"
)

// TestResult 测试结果结构
type TestResult struct {
	Name              string
	DataPoints        int
	OriginalSize      int64
	CompressedSize    int64
	CompressionRatio  float64
	CompressionTime   time.Duration
	DecompressionTime time.Duration
	DataIntegrity     bool
	ErrorRate         float64
	Throughput        float64 // MB/s
}

// generatePattern 生成不同模式的测试数据
func generatePattern(size int, pattern string, params map[string]float64) []compression.TimeSeriesPoint {
	data := make([]compression.TimeSeriesPoint, size)
	baseTime := time.Now().Unix()

	for i := 0; i < size; i++ {
		timestamp := baseTime + int64(i)
		var value float64

		switch pattern {
		case "constant":
			value = params["base"]

		case "linear":
			value = params["base"] + float64(i)*params["slope"]

		case "sine":
			amplitude := params["amplitude"]
			frequency := params["frequency"]
			offset := params["offset"]
			value = offset + amplitude*math.Sin(float64(i)*frequency)

		case "exponential":
			base := params["base"]
			growth := params["growth"]
			value = base * math.Pow(growth, float64(i)/100.0)

		case "random_walk":
			if i == 0 {
				value = params["start"]
			} else {
				change := (rand.Float64() - 0.5) * params["volatility"]
				value = data[i-1].Value + change
			}

		case "sawtooth":
			period := params["period"]
			amplitude := params["amplitude"]
			value = params["offset"] + amplitude*(float64(i%int(period))/period)

		case "spiky":
			base := params["base"]
			if i%int(params["spike_interval"]) == 0 {
				value = base + params["spike_height"]
			} else {
				value = base + (rand.Float64()-0.5)*params["noise"]
			}

		case "step":
			stepSize := params["step_size"]
			stepInterval := params["step_interval"]
			value = params["base"] + math.Floor(float64(i)/stepInterval)*stepSize

		case "noisy_sine":
			amplitude := params["amplitude"]
			frequency := params["frequency"]
			noise := params["noise"]
			offset := params["offset"]
			sineWave := amplitude * math.Sin(float64(i)*frequency)
			noiseValue := (rand.Float64() - 0.5) * noise
			value = offset + sineWave + noiseValue

		default:
			value = 100.0
		}

		data[i] = compression.TimeSeriesPoint{
			Timestamp: timestamp,
			Value:     value,
		}
	}

	return data
}

// runCompressionTest 执行压缩测试
func runCompressionTest(name string, data []compression.TimeSeriesPoint) TestResult {
	compressor := gorilla.NewGorillaCompressor()
	defer compressor.Close()

	originalSize := int64(len(data) * 16) // 每个点16字节

	// 压缩测试
	start := time.Now()
	compressed, err := compressor.Compress(data)
	compressionTime := time.Since(start)

	result := TestResult{
		Name:         name,
		DataPoints:   len(data),
		OriginalSize: originalSize,
	}

	if err != nil {
		fmt.Printf("❌ %s: 压缩失败 - %v\n", name, err)
		return result
	}

	result.CompressedSize = int64(len(compressed))
	result.CompressionRatio = float64(originalSize) / float64(len(compressed))
	result.CompressionTime = compressionTime
	result.Throughput = float64(originalSize) / compressionTime.Seconds() / (1024 * 1024) // MB/s

	// 解压缩测试
	start = time.Now()
	decompressed, err := compressor.Decompress(compressed)
	result.DecompressionTime = time.Since(start)

	if err != nil {
		fmt.Printf("❌ %s: 解压缩失败 - %v\n", name, err)
		return result
	}

	// 数据完整性验证
	result.DataIntegrity = true
	errorCount := 0
	totalError := 0.0

	if len(decompressed) != len(data) {
		result.DataIntegrity = false
		fmt.Printf("❌ %s: 数据长度不匹配 %d != %d\n", name, len(data), len(decompressed))
		return result
	}

	for i := 0; i < len(data); i++ {
		if data[i].Timestamp != decompressed[i].Timestamp {
			result.DataIntegrity = false
			errorCount++
		}

		valueDiff := math.Abs(data[i].Value - decompressed[i].Value)
		if valueDiff > 1e-10 {
			result.DataIntegrity = false
			errorCount++
			totalError += valueDiff
		}
	}

	if errorCount > 0 {
		result.ErrorRate = float64(errorCount) / float64(len(data)) * 100
	}

	return result
}

// printTestResult 打印测试结果
func printTestResult(result TestResult) {
	status := "✅"
	if !result.DataIntegrity {
		status = "❌"
	}

	fmt.Printf("%s %-25s | %6d点 | %8.2fx | %8.2f MB/s | %8.2fµs | %8.2fµs\n",
		status,
		result.Name,
		result.DataPoints,
		result.CompressionRatio,
		result.Throughput,
		float64(result.CompressionTime.Nanoseconds())/1000.0,
		float64(result.DecompressionTime.Nanoseconds())/1000.0)
}

func main() {
	fmt.Println("🔬 Gorilla压缩算法详细性能测试")
	fmt.Println("==================================================================================")

	rand.Seed(time.Now().UnixNano())

	var allResults []TestResult

	// 1. 基础数据模式测试
	fmt.Println("\n📊 1. 基础数据模式测试")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	testCases := []struct {
		name    string
		size    int
		pattern string
		params  map[string]float64
	}{
		{"常量数据", 1000, "constant", map[string]float64{"base": 100.0}},
		{"线性增长", 1000, "linear", map[string]float64{"base": 100.0, "slope": 0.1}},
		{"正弦波", 1000, "sine", map[string]float64{"amplitude": 10.0, "frequency": 0.1, "offset": 100.0}},
		{"随机游走", 1000, "random_walk", map[string]float64{"start": 100.0, "volatility": 2.0}},
		{"锯齿波", 1000, "sawtooth", map[string]float64{"period": 50.0, "amplitude": 20.0, "offset": 100.0}},
		{"尖峰数据", 1000, "spiky", map[string]float64{"base": 100.0, "spike_interval": 100.0, "spike_height": 50.0, "noise": 1.0}},
		{"阶跃函数", 1000, "step", map[string]float64{"base": 100.0, "step_size": 10.0, "step_interval": 100.0}},
		{"噪声正弦", 1000, "noisy_sine", map[string]float64{"amplitude": 10.0, "frequency": 0.05, "noise": 2.0, "offset": 100.0}},
	}

	for _, tc := range testCases {
		data := generatePattern(tc.size, tc.pattern, tc.params)
		result := runCompressionTest(tc.name, data)
		printTestResult(result)
		allResults = append(allResults, result)
	}

	// 2. 数据规模测试
	fmt.Println("\n📈 2. 数据规模压缩测试")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	dataSizes := []int{10, 100, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
	for _, size := range dataSizes {
		name := fmt.Sprintf("正弦波-%d点", size)
		data := generatePattern(size, "sine", map[string]float64{
			"amplitude": 10.0, "frequency": 0.1, "offset": 100.0,
		})
		result := runCompressionTest(name, data)
		printTestResult(result)
		allResults = append(allResults, result)
	}

	// 3. 极端情况测试
	fmt.Println("\n⚠️  3. 极端情况测试")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	extremeCases := []struct {
		name    string
		size    int
		pattern string
		params  map[string]float64
	}{
		{"单点数据", 1, "constant", map[string]float64{"base": 100.0}},
		{"最小数据集", 2, "linear", map[string]float64{"base": 100.0, "slope": 1.0}},
		{"高频振荡", 1000, "sine", map[string]float64{"amplitude": 100.0, "frequency": 1.0, "offset": 0.0}},
		{"指数增长", 500, "exponential", map[string]float64{"base": 1.0, "growth": 1.01}},
		{"极高噪声", 1000, "noisy_sine", map[string]float64{"amplitude": 1.0, "frequency": 0.1, "noise": 100.0, "offset": 100.0}},
	}

	for _, tc := range extremeCases {
		data := generatePattern(tc.size, tc.pattern, tc.params)
		result := runCompressionTest(tc.name, data)
		printTestResult(result)
		allResults = append(allResults, result)
	}

	// 4. 监控场景模拟
	fmt.Println("\n🖥️  4. 真实监控场景模拟")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	// CPU使用率模拟（0-100%，相对稳定）
	cpuData := generatePattern(1000, "noisy_sine", map[string]float64{
		"amplitude": 15.0, "frequency": 0.02, "noise": 5.0, "offset": 35.0,
	})
	result := runCompressionTest("CPU使用率模拟", cpuData)
	printTestResult(result)
	allResults = append(allResults, result)

	// 内存使用率模拟（缓慢增长趋势）
	memData := generatePattern(1000, "linear", map[string]float64{
		"base": 60.0, "slope": 0.01,
	})
	// 添加小幅波动
	for i := range memData {
		memData[i].Value += (rand.Float64() - 0.5) * 3.0
	}
	result = runCompressionTest("内存使用率模拟", memData)
	printTestResult(result)
	allResults = append(allResults, result)

	// 网络流量模拟（突发性）
	netData := generatePattern(1000, "spiky", map[string]float64{
		"base": 10.0, "spike_interval": 50.0, "spike_height": 90.0, "noise": 5.0,
	})
	result = runCompressionTest("网络流量模拟", netData)
	printTestResult(result)
	allResults = append(allResults, result)

	// 5. 并发压缩测试
	fmt.Println("\n🔄 5. 并发压缩测试")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	// 测试不同并发级别
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32, 64, 128, 256}

	for _, numGoroutines := range concurrencyLevels {
		results := make(chan TestResult, numGoroutines)

		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				name := fmt.Sprintf("并发%d线程-%d", numGoroutines, id)
				// 每个goroutine处理不同的数据片段，避免缓存影响
				data := generatePattern(1000, "sine", map[string]float64{
					"amplitude": 10.0 + float64(id)*0.1,
					"frequency": 0.1 + float64(id)*0.001,
					"offset":    100.0 + float64(id),
				})
				result := runCompressionTest(name, data)
				results <- result
			}(i)
		}

		var concurrentResults []TestResult
		for i := 0; i < numGoroutines; i++ {
			result := <-results
			concurrentResults = append(concurrentResults, result)
		}
		totalTime := time.Since(start)

		// 计算并发性能统计
		var avgRatio, avgThroughput float64
		successCount := 0
		for _, r := range concurrentResults {
			if r.DataIntegrity {
				avgRatio += r.CompressionRatio
				avgThroughput += r.Throughput
				successCount++
			}
		}

		if successCount > 0 {
			avgRatio /= float64(successCount)
			avgThroughput /= float64(successCount)

			// 计算总吞吐量
			totalThroughput := avgThroughput * float64(numGoroutines)

			fmt.Printf("✅ %-25s | %6d点 | %8.2fx | %8.2f MB/s | %8.2fms | 成功率%.0f%%\n",
				fmt.Sprintf("并发测试-%d线程", numGoroutines),
				1000,
				avgRatio,
				totalThroughput,
				float64(totalTime.Nanoseconds())/1000000.0,
				float64(successCount)/float64(numGoroutines)*100)
		}
	}

	// 6. 极限压力测试
	fmt.Println("\n💥 6. 极限压力测试")
	fmt.Println("状态 | 测试名称                  | 数据点 | 压缩比   | 吞吐量     | 压缩时间 | 解压时间")
	fmt.Println("----------------------------------------------------------------------------------------")

	// 超大数据量测试
	megaSizes := []int{2000000, 5000000, 10000000} // 200万、500万、1000万点
	for _, size := range megaSizes {
		name := fmt.Sprintf("超大数据-%d点", size)
		fmt.Printf("🔄 正在生成%d点数据...\n", size)

		// 分批生成数据以避免内存爆炸
		data := make([]compression.TimeSeriesPoint, size)
		baseTime := time.Now().Unix()
		batchSize := 100000

		for i := 0; i < size; i += batchSize {
			end := i + batchSize
			if end > size {
				end = size
			}

			for j := i; j < end; j++ {
				timestamp := baseTime + int64(j)
				value := 100.0 + 10.0*math.Sin(float64(j)*0.001) + (rand.Float64()-0.5)*2.0
				data[j] = compression.TimeSeriesPoint{
					Timestamp: timestamp,
					Value:     value,
				}
			}
		}

		fmt.Printf("🗜️  开始压缩%d点数据...\n", size)
		result := runCompressionTest(name, data)
		printTestResult(result)
		allResults = append(allResults, result)

		// 强制垃圾回收
		data = nil
		rand.Seed(time.Now().UnixNano()) // 重新设置随机种子
	}

	// 内存压力测试
	fmt.Println("\n🧠 内存压力测试")
	memoryTestSizes := []int{100000, 500000, 1000000}
	for _, size := range memoryTestSizes {
		name := fmt.Sprintf("内存压力-%d点", size)

		// 创建多个压缩器同时工作
		numCompressors := 4
		results := make(chan TestResult, numCompressors)

		start := time.Now()
		for i := 0; i < numCompressors; i++ {
			go func(id int) {
				data := generatePattern(size, "noisy_sine", map[string]float64{
					"amplitude": 10.0 + float64(id),
					"frequency": 0.01 + float64(id)*0.001,
					"noise":     5.0,
					"offset":    100.0 + float64(id)*10,
				})
				testName := fmt.Sprintf("%s-压缩器%d", name, id)
				result := runCompressionTest(testName, data)
				results <- result
			}(i)
		}

		var memResults []TestResult
		for i := 0; i < numCompressors; i++ {
			result := <-results
			memResults = append(memResults, result)
		}
		totalTime := time.Since(start)

		// 计算平均性能
		var avgRatio, avgThroughput float64
		successCount := 0
		for _, r := range memResults {
			if r.DataIntegrity {
				avgRatio += r.CompressionRatio
				avgThroughput += r.Throughput
				successCount++
			}
		}

		if successCount > 0 {
			avgRatio /= float64(successCount)
			avgThroughput /= float64(successCount)

			fmt.Printf("✅ %-25s | %6d点 | %8.2fx | %8.2f MB/s | %8.2fms | %d压缩器\n",
				name, size, avgRatio, avgThroughput*float64(numCompressors),
				float64(totalTime.Nanoseconds())/1000000.0, numCompressors)
		}
	}

	// 持续运行测试
	fmt.Println("\n⏱️  持续运行稳定性测试")
	stressData := generatePattern(50000, "sine", map[string]float64{
		"amplitude": 10.0, "frequency": 0.1, "offset": 100.0,
	})

	iterations := 100
	var stressResults []TestResult
	fmt.Printf("🔄 运行%d次连续压缩测试...\n", iterations)

	stressStart := time.Now()
	for i := 0; i < iterations; i++ {
		name := fmt.Sprintf("稳定性测试-%d", i+1)
		result := runCompressionTest(name, stressData)
		stressResults = append(stressResults, result)

		if (i+1)%20 == 0 {
			fmt.Printf("   完成 %d/%d 次测试\n", i+1, iterations)
		}
	}
	stressTotal := time.Since(stressStart)

	// 统计稳定性结果
	var stressRatio, stressThroughput float64
	stressSuccess := 0
	for _, r := range stressResults {
		if r.DataIntegrity {
			stressRatio += r.CompressionRatio
			stressThroughput += r.Throughput
			stressSuccess++
		}
	}

	if stressSuccess > 0 {
		stressRatio /= float64(stressSuccess)
		stressThroughput /= float64(stressSuccess)

		fmt.Printf("✅ %-25s | %6d点 | %8.2fx | %8.2f MB/s | %8.2fs | 成功率%.1f%%\n",
			"稳定性测试汇总", 50000, stressRatio, stressThroughput,
			stressTotal.Seconds(), float64(stressSuccess)/float64(iterations)*100)
	}

	// 7. 综合统计分析
	fmt.Println("\n📊 7. 综合统计分析")
	fmt.Println("==================================================")

	if len(allResults) > 0 {
		// 按压缩比排序
		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].CompressionRatio > allResults[j].CompressionRatio
		})

		var totalRatio, totalThroughput float64
		var successCount int
		maxRatio := allResults[0].CompressionRatio
		minRatio := allResults[len(allResults)-1].CompressionRatio

		for _, result := range allResults {
			if result.DataIntegrity {
				successCount++
				totalRatio += result.CompressionRatio
				totalThroughput += result.Throughput
			}
		}

		avgRatio := totalRatio / float64(successCount)
		avgThroughput := totalThroughput / float64(successCount)

		fmt.Printf("📈 测试总数: %d\n", len(allResults))
		fmt.Printf("✅ 成功率: %.1f%% (%d/%d)\n", float64(successCount)/float64(len(allResults))*100, successCount, len(allResults))
		fmt.Printf("🏆 最佳压缩比: %.2fx (%s)\n", maxRatio, allResults[0].Name)
		fmt.Printf("📊 平均压缩比: %.2fx\n", avgRatio)
		fmt.Printf("📉 最低压缩比: %.2fx (%s)\n", minRatio, allResults[len(allResults)-1].Name)
		fmt.Printf("⚡ 平均吞吐量: %.2f MB/s\n", avgThroughput)

		// 压缩比分布统计
		fmt.Println("\n📊 压缩比分布:")
		ranges := []struct {
			min, max float64
			name     string
		}{
			{10.0, math.Inf(1), "优秀(≥10x)"},
			{5.0, 10.0, "良好(5-10x)"},
			{2.0, 5.0, "一般(2-5x)"},
			{1.0, 2.0, "较差(1-2x)"},
			{0.0, 1.0, "失败(<1x)"},
		}

		for _, r := range ranges {
			count := 0
			for _, result := range allResults {
				if result.CompressionRatio >= r.min && result.CompressionRatio < r.max {
					count++
				}
			}
			percentage := float64(count) / float64(len(allResults)) * 100
			fmt.Printf("   %s: %d个 (%.1f%%)\n", r.name, count, percentage)
		}
	}

	// 8. 获取全局统计
	fmt.Println("\n🌍 8. 全局压缩统计")
	globalStats := gorilla.GetCompressionStats()
	fmt.Printf("算法: %s\n", globalStats.Algorithm)
	fmt.Printf("总处理: %d bytes → %d bytes\n", globalStats.OriginalSize, globalStats.CompressedSize)
	fmt.Printf("全局压缩比: %.2fx\n", globalStats.CompressionRatio)

	fmt.Println("\n🎉 详细测试完成!")
	if len(allResults) > 0 {
		// 重新计算总体平均值用于最终汇总
		var finalRatio float64
		finalSuccessCount := 0
		for _, result := range allResults {
			if result.DataIntegrity {
				finalRatio += result.CompressionRatio
				finalSuccessCount++
			}
		}
		if finalSuccessCount > 0 {
			finalRatio /= float64(finalSuccessCount)
			fmt.Printf("💾 存储节省: %.1f%% (压缩比 %.2fx)\n", (1.0-1.0/finalRatio)*100, finalRatio)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
