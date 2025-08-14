package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/han-fei/monitor/visualization/internal/cpp"
)

func main() {
	fmt.Println("=== C++ Streaming Top-K 测试 ===")

	// 1. 测试基本功能
	testBasicFunctionality()

	// 2. 测试性能
	testPerformance()

	// 3. 测试管理器模式
	testManagerMode()
}

func testBasicFunctionality() {
	fmt.Println("\n--- 基本功能测试 ---")

	// 创建处理器
	processor := cpp.NewStreamingTopKProcessor(5, 3600)
	if processor == nil {
		log.Fatal("Failed to create processor")
	}
	defer processor.Close()

	// 添加测试数据
	testData := []struct {
		hostID string
		value  float64
	}{
		{"host1", 100.5},
		{"host2", 200.3},
		{"host3", 150.7},
		{"host4", 300.1},
		{"host5", 250.9},
		{"host6", 180.2}, // 这个应该被丢弃（流式处理）
		{"host7", 350.8}, // 这个应该替换最小的
	}

	for _, data := range testData {
		err := processor.ProcessStream(data.hostID, data.value)
		if err != nil {
			log.Printf("Error processing %s: %v", data.hostID, err)
		}
	}

	// 获取Top-K结果
	results := processor.GetTopK()
	fmt.Printf("Top-5 结果 (共 %d 个):\n", len(results))
	for i, item := range results {
		fmt.Printf("  %d. Host: %s, Value: %.2f, Timestamp: %d\n",
			i+1, item.HostID, item.Value, item.Timestamp)
	}

	// 获取统计信息
	stats := processor.GetStats()
	fmt.Printf("\n统计信息:\n")
	fmt.Printf("  总更新次数: %v\n", stats["total_updates"])
	fmt.Printf("  当前元素数: %v\n", stats["current_items"])
	fmt.Printf("  K值: %v\n", stats["k_value"])
	fmt.Printf("  TTL: %v秒\n", stats["ttl_seconds"])
}

func testPerformance() {
	fmt.Println("\n--- 性能测试 ---")

	processor := cpp.NewStreamingTopKProcessor(10, 3600)
	if processor == nil {
		log.Fatal("Failed to create processor")
	}
	defer processor.Close()

	// 生成随机数据
	rand.Seed(time.Now().UnixNano())
	
	start := time.Now()
	count := 10000

	for i := 0; i < count; i++ {
		hostID := fmt.Sprintf("host%d", rand.Intn(1000))
		value := rand.Float64() * 1000
		
		err := processor.ProcessStream(hostID, value)
		if err != nil {
			log.Printf("Error at iteration %d: %v", i, err)
		}
	}

	duration := time.Since(start)
	fmt.Printf("处理 %d 个数据项耗时: %v\n", count, duration)
	fmt.Printf("平均每个操作: %v\n", duration/time.Duration(count))
	fmt.Printf("吞吐量: %.0f ops/sec\n", float64(count)/duration.Seconds())

	// 获取结果
	results := processor.GetTopK()
	fmt.Printf("Top-10 结果:\n")
	for i, item := range results {
		fmt.Printf("  %d. %s: %.2f\n", i+1, item.HostID, item.Value)
	}
}

func testManagerMode() {
	fmt.Println("\n--- 管理器模式测试 ---")

	// 获取管理器实例
	manager := cpp.GetStreamingTopKManagerInstance()
	
	// 初始化
	err := manager.Initialize(10, 3600)
	if err != nil {
		log.Fatal("Failed to initialize manager:", err)
	}
	defer manager.Shutdown()

	// 创建多个处理器
	_, err = manager.CreateProcessor("cpu_usage", 5, 3600)
	if err != nil {
		log.Fatal("Failed to create CPU processor:", err)
	}

	_, err = manager.CreateProcessor("memory_usage", 5, 3600)
	if err != nil {
		log.Fatal("Failed to create memory processor:", err)
	}

	// 添加数据
	manager.AddMetric("cpu_usage", "host1", 85.5)
	manager.AddMetric("memory_usage", "host1", 75.2)
	manager.AddMetric("cpu_usage", "host2", 92.3)
	manager.AddMetric("memory_usage", "host2", 88.7)
	manager.AddMetric("cpu_usage", "host3", 78.9)
	manager.AddMetric("memory_usage", "host3", 95.1)

	// 获取结果
	cpuResults := manager.GetTopK("cpu_usage")
	memResults := manager.GetTopK("memory_usage")

	fmt.Println("CPU 使用率 Top-5:")
	for i, item := range cpuResults {
		fmt.Printf("  %d. %s: %.1f%%\n", i+1, item.HostID, item.Value)
	}

	fmt.Println("\n内存使用率 Top-5:")
	for i, item := range memResults {
		fmt.Printf("  %d. %s: %.1f%%\n", i+1, item.HostID, item.Value)
	}

	// 获取系统统计信息
	systemStats := manager.GetSystemStats()
	fmt.Printf("\n系统统计:\n")
	fmt.Printf("  总处理器数: %v\n", systemStats["total_processors"])
	fmt.Printf("  已初始化: %v\n", systemStats["initialized"])
	fmt.Printf("  运行中: %v\n", systemStats["running"])
	fmt.Printf("  总更新次数: %v\n", systemStats["total_updates"])
	fmt.Printf("  总元素数: %v\n", systemStats["total_items"])

	// 获取所有处理器名称
	processorNames := manager.GetProcessorNames()
	fmt.Printf("  处理器列表: %v\n", processorNames)
}

func testBatchProcessing() {
	fmt.Println("\n--- 批量处理测试 ---")

	processor := cpp.NewStreamingTopKProcessor(5, 3600)
	if processor == nil {
		log.Fatal("Failed to create processor")
	}
	defer processor.Close()

	// 准备批量数据
	batchItems := []struct {
		HostID string
		Value  float64
	}{
		{"host1", 100.5},
		{"host2", 200.3},
		{"host3", 150.7},
		{"host4", 300.1},
		{"host5", 250.9},
		{"host6", 180.2},
		{"host7", 350.8},
	}

	// 批量处理
	err := processor.ProcessStreamBatch(batchItems)
	if err != nil {
		log.Fatal("Batch processing failed:", err)
	}

	// 获取结果
	results := processor.GetTopK()
	fmt.Printf("批量处理后的 Top-5 结果:\n")
	for i, item := range results {
		fmt.Printf("  %d. %s: %.2f\n", i+1, item.HostID, item.Value)
	}
}
