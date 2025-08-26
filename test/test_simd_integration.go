package main

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/han-fei/monitor/pkg/compute/simd"
)

func main() {
	fmt.Println("=== SIMD Go Integration Test ===")

	// 测试CPU支持
	fmt.Printf("AVX2 Support: %v\n", simd.SupportsAVX2())

	// 准备测试数据
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	fmt.Printf("Test data: %v\n", values)

	// 测试基础统计
	fmt.Println("\n--- Basic Statistics Test ---")
	result, err := simd.CalculateBasicStats(values)
	if err != nil {
		log.Fatalf("Basic stats failed: %v", err)
	}

	fmt.Printf("Sum: %.2f\n", result.Sum)
	fmt.Printf("Mean: %.2f\n", result.Mean)
	fmt.Printf("Min: %.2f\n", result.Min)
	fmt.Printf("Max: %.2f\n", result.Max)
	fmt.Printf("StdDev: %.2f\n", result.StdDev)

	// 验证结果
	expectedSum := 55.0
	expectedMean := 5.5
	if math.Abs(result.Sum-expectedSum) > 1e-9 || math.Abs(result.Mean-expectedMean) > 1e-9 {
		log.Fatalf("Results incorrect: got sum=%.2f mean=%.2f, want sum=%.2f mean=%.2f",
			result.Sum, result.Mean, expectedSum, expectedMean)
	}
	fmt.Println("✅ Basic statistics test PASSED")

	// 测试百分位数
	fmt.Println("\n--- Percentiles Test ---")
	percentiles := []float64{50.0, 90.0, 95.0, 99.0}
	pResults, err := simd.CalculatePercentiles(values, percentiles)
	if err != nil {
		log.Fatalf("Percentiles failed: %v", err)
	}

	for i, p := range percentiles {
		fmt.Printf("P%.0f: %.2f\n", p, pResults[i])
	}
	fmt.Println("✅ Percentiles test PASSED")

	// 测试百分比转换
	fmt.Println("\n--- Percentage Conversion Test ---")
	rawValues := []uint64{25, 50, 75, 100}
	maxValue := uint64(100)

	percentages, err := simd.BatchConvertToPercentage(rawValues, maxValue)
	if err != nil {
		log.Fatalf("Percentage conversion failed: %v", err)
	}

	for i, raw := range rawValues {
		fmt.Printf("%d -> %.1f%%\n", raw, percentages[i])
	}
	fmt.Println("✅ Percentage conversion test PASSED")

	// 测试数据验证
	fmt.Println("\n--- Data Validation Test ---")
	testValues := []float64{1.0, 2.0, math.NaN(), 4.0, math.Inf(1), 6.0}
	validCount, err := simd.ValidateMetrics(testValues)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Printf("Valid metrics: %d out of %d\n", validCount, len(testValues))
	if validCount != 4 {
		log.Fatalf("Expected 4 valid metrics, got %d", validCount)
	}
	fmt.Println("✅ Data validation test PASSED")

	// 性能测试
	fmt.Println("\n--- Performance Test ---")
	performanceTest()

	fmt.Println("\n🎉 All SIMD integration tests PASSED!")
}

func performanceTest() {
	// 生成大量测试数据
	size := 100000
	largeData := make([]float64, size)
	for i := 0; i < size; i++ {
		largeData[i] = float64(i) * 0.1
	}

	iterations := 100

	// SIMD性能测试
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := simd.CalculateBasicStats(largeData)
		if err != nil {
			log.Fatalf("SIMD performance test failed: %v", err)
		}
	}
	simdDuration := time.Since(start)

	// 标量性能测试（作为对比）
	start = time.Now()
	for i := 0; i < iterations; i++ {
		scalarBasicStats(largeData)
	}
	scalarDuration := time.Since(start)

	speedup := float64(scalarDuration) / float64(simdDuration)

	fmt.Printf("Data size: %d elements\n", size)
	fmt.Printf("Iterations: %d\n", iterations)
	fmt.Printf("SIMD time: %v\n", simdDuration)
	fmt.Printf("Scalar time: %v\n", scalarDuration)
	fmt.Printf("Speedup: %.2fx\n", speedup)

	if speedup >= 2.0 {
		fmt.Printf("✅ EXCELLENT performance (%.2fx speedup)\n", speedup)
	} else if speedup >= 1.2 {
		fmt.Printf("✅ GOOD performance (%.2fx speedup)\n", speedup)
	} else if speedup >= 1.0 {
		fmt.Printf("⚠️  MARGINAL performance (%.2fx speedup)\n", speedup)
	} else {
		fmt.Printf("❌ SLOWER than scalar (%.2fx)\n", speedup)
	}
}

// 标量实现作为性能对比基准
func scalarBasicStats(values []float64) (sum, mean, min, max, stddev float64) {
	if len(values) == 0 {
		return
	}

	sum = 0
	min = values[0]
	max = values[0]

	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	mean = sum / float64(len(values))

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	stddev = math.Sqrt(variance)

	return
}
