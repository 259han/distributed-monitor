package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/han-fei/monitor/visualization/internal/analysis"
	"github.com/han-fei/monitor/visualization/internal/models"
)

func main() {
	fmt.Println("=== SIMD聚合器集成测试 ===")

	// 生成测试数据
	testData := generateTestMetrics(1000, 50) // 1000个时间点，每个50个指标
	fmt.Printf("生成测试数据: %d个MetricsData，约%d个指标\n", 
		len(testData), len(testData)*50)

	// 创建分析器
	analyzer := analysis.NewAnalyzer()

	// 测试SIMD vs 原生聚合器性能
	fmt.Println("\n--- 性能对比测试 ---")
	performanceComparison(analyzer, testData)

	// 测试功能正确性
	fmt.Println("\n--- 功能正确性测试 ---")
	functionalityTest(analyzer, testData)

	// 测试SIMD特有功能
	fmt.Println("\n--- SIMD特有功能测试 ---")
	simdSpecificTests(analyzer, testData)

	fmt.Println("\n🎉 SIMD聚合器集成测试完成！")
}

// generateTestMetrics 生成测试数据
func generateTestMetrics(count int, metricsPerData int) []*models.MetricsData {
	rand.Seed(time.Now().UnixNano())
	
	data := make([]*models.MetricsData, count)
	
	for i := 0; i < count; i++ {
		metrics := make([]models.Metric, metricsPerData)
		
		for j := 0; j < metricsPerData; j++ {
			// 生成不同类型的指标值
			var value float64
			switch j % 4 {
			case 0: // CPU使用率 (0-100)
				value = rand.Float64() * 100
			case 1: // 内存使用量 (MB)
				value = rand.Float64() * 8192
			case 2: // 网络流量 (KB/s)
				value = rand.Float64() * 1024
			case 3: // 磁盘使用率 (0-100)
				value = rand.Float64() * 100
			}
			
			metrics[j] = models.Metric{
				Name:  fmt.Sprintf("metric_%d", j),
				Value: value,
				Unit:  "unit",
			}
		}
		
		data[i] = &models.MetricsData{
			HostID:    "test-host",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Metrics:   metrics,
		}
	}
	
	return data
}

// performanceComparison 性能对比测试
func performanceComparison(analyzer *analysis.Analyzer, testData []*models.MetricsData) {
	iterations := 100

	tests := []struct {
		name        string
		originalType string
		simdType     string
	}{
		{"平均值计算", "avg", "simd_avg"},
		{"求和计算", "sum", "simd_sum"},
		{"最小值计算", "min", "simd_min"},
		{"最大值计算", "max", "simd_max"},
		{"标准差计算", "stddev", "simd_stddev"},
		{"P95百分位数", "p95", "simd_p95"},
	}

	for _, test := range tests {
		fmt.Printf("\n%s:\n", test.name)

		// 测试原生实现
		start := time.Now()
		var originalResult interface{}
		for i := 0; i < iterations; i++ {
			if aggregator := analyzer.GetAggregator(test.originalType); aggregator != nil {
				originalResult = aggregator.Aggregate(testData)
			}
		}
		originalTime := time.Since(start)

		// 测试SIMD实现
		start = time.Now()
		var simdResult interface{}
		for i := 0; i < iterations; i++ {
			if aggregator := analyzer.GetAggregator(test.simdType); aggregator != nil {
				simdResult = aggregator.Aggregate(testData)
			}
		}
		simdTime := time.Since(start)

		// 计算加速比
		speedup := float64(originalTime) / float64(simdTime)

		fmt.Printf("  原生实现: %v (结果: %.6f)\n", originalTime, getFloatValue(originalResult))
		fmt.Printf("  SIMD实现: %v (结果: %.6f)\n", simdTime, getFloatValue(simdResult))
		fmt.Printf("  加速比: %.2fx\n", speedup)

		if speedup >= 2.0 {
			fmt.Printf("  ✅ 优秀 (>= 2x)\n")
		} else if speedup >= 1.2 {
			fmt.Printf("  ✅ 良好 (>= 1.2x)\n")
		} else if speedup >= 1.0 {
			fmt.Printf("  ⚠️  边际改善\n")
		} else {
			fmt.Printf("  ❌ 性能倒退\n")
		}
	}
}

// functionalityTest 功能正确性测试
func functionalityTest(analyzer *analysis.Analyzer, testData []*models.MetricsData) {
	// 使用小数据集进行精确验证
	smallData := testData[:10]

	tests := []struct {
		name         string
		originalType string
		simdType     string
	}{
		{"平均值", "avg", "simd_avg"},
		{"求和", "sum", "simd_sum"},
		{"最小值", "min", "simd_min"},
		{"最大值", "max", "simd_max"},
		{"标准差", "stddev", "simd_stddev"},
	}

	for _, test := range tests {
		original := analyzer.GetAggregator(test.originalType)
		simd := analyzer.GetAggregator(test.simdType)

		if original == nil || simd == nil {
			fmt.Printf("  %s: ❌ 聚合器未找到\n", test.name)
			continue
		}

		originalResult := getFloatValue(original.Aggregate(smallData))
		simdResult := getFloatValue(simd.Aggregate(smallData))

		diff := abs(originalResult - simdResult)
		tolerance := 1e-6

		if diff <= tolerance {
			fmt.Printf("  %s: ✅ 通过 (差异: %e)\n", test.name, diff)
		} else {
			fmt.Printf("  %s: ❌ 失败 (差异: %e, 原生: %f, SIMD: %f)\n", 
				test.name, diff, originalResult, simdResult)
		}
	}
}

// simdSpecificTests 测试SIMD特有功能
func simdSpecificTests(analyzer *analysis.Analyzer, testData []*models.MetricsData) {
	// 测试多统计指标聚合器
	fmt.Println("多统计指标聚合器:")
	multiStats := analyzer.GetAggregator("simd_multi_stats")
	if multiStats != nil {
		start := time.Now()
		result := multiStats.Aggregate(testData)
		duration := time.Since(start)

		fmt.Printf("  执行时间: %v\n", duration)
		if resultMap, ok := result.(map[string]interface{}); ok {
			fmt.Printf("  结果: %+v\n", resultMap)
			fmt.Printf("  ✅ 一次性计算多个统计指标\n")
		}
	}

	// 测试数据验证聚合器
	fmt.Println("\n数据验证聚合器:")
	validation := analyzer.GetAggregator("simd_validation")
	if validation != nil {
		start := time.Now()
		result := validation.Aggregate(testData)
		duration := time.Since(start)

		fmt.Printf("  执行时间: %v\n", duration)
		if resultMap, ok := result.(map[string]interface{}); ok {
			fmt.Printf("  验证结果: %+v\n", resultMap)
			fmt.Printf("  ✅ 快速数据验证完成\n")
		}
	}

	// 测试全统计指标聚合器
	fmt.Println("\n全统计指标聚合器:")
	allStats := analyzer.GetAggregator("simd_all")
	if allStats != nil {
		start := time.Now()
		result := allStats.Aggregate(testData)
		duration := time.Since(start)

		fmt.Printf("  执行时间: %v\n", duration)
		if resultMap, ok := result.(map[string]interface{}); ok {
			fmt.Printf("  统计结果: %+v\n", resultMap)
			fmt.Printf("  ✅ 一次性获得所有基础统计信息\n")
		}
	}
}

// getFloatValue 从interface{}中提取float64值
func getFloatValue(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}

// abs 绝对值函数
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// 扩展Analyzer以支持GetAggregator方法
func init() {
	// 为了测试需要，我们需要添加一个方法来获取聚合器
	// 这个应该在实际代码中添加到analyzer.go中
}
