package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/han-fei/monitor/agent/internal/algorithm"
	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/broker/internal/hash"
	"github.com/han-fei/monitor/broker/internal/queue"
	"github.com/han-fei/monitor/broker/internal/storage"
	"github.com/han-fei/monitor/internal/utils"
)

// TestSuite 测试套件
type TestSuite struct {
	Name        string
	Setup       func() error
	Teardown    func() error
	TestCases   []TestCase
}

// TestCase 测试用例
type TestCase struct {
	Name     string
	TestFunc func(*testing.T) error
}

// TestRunner 测试运行器
type TestRunner struct {
	suites      []*TestSuite
	results     []*TestResult
	currentDir  string
	logFile     *os.File
	errorLog    *os.File
}

// TestResult 测试结果
type TestResult struct {
	SuiteName   string
	TestCase    string
	Passed      bool
	Error       error
	Duration    time.Duration
	LogFile     string
}

// NewTestRunner 创建新的测试运行器
func NewTestRunner() (*TestRunner, error) {
	// 创建日志目录
	logDir := "logs/tests"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}
	
	// 创建日志文件
	timestamp := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logDir, fmt.Sprintf("test_%s.log", timestamp))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}
	
	// 创建错误日志文件
	errorLogPath := filepath.Join(logDir, fmt.Sprintf("error_%s.log", timestamp))
	errorLogFile, err := os.Create(errorLogPath)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("创建错误日志文件失败: %v", err)
	}
	
	runner := &TestRunner{
		suites:     make([]*TestSuite, 0),
		results:    make([]*TestResult, 0),
		currentDir: "",
		logFile:    logFile,
		errorLog:   errorLogFile,
	}
	
	return runner, nil
}

// AddSuite 添加测试套件
func (tr *TestRunner) AddSuite(suite *TestSuite) {
	tr.suites = append(tr.suites, suite)
}

// Run 运行所有测试
func (tr *TestRunner) Run() []*TestResult {
	for _, suite := range tr.suites {
		tr.runSuite(suite)
	}
	
	return tr.results
}

// runSuite 运行测试套件
func (tr *TestRunner) runSuite(suite *TestSuite) {
	log.Printf("运行测试套件: %s", suite.Name)
	
	// 设置
	if suite.Setup != nil {
		if err := suite.Setup(); err != nil {
			log.Printf("测试套件 %s 设置失败: %v", suite.Name, err)
			return
		}
	}
	
	// 运行测试用例
	for _, testCase := range suite.TestCases {
		result := tr.runTestCase(suite, testCase)
		tr.results = append(tr.results, result)
	}
	
	// 清理
	if suite.Teardown != nil {
		if err := suite.Teardown(); err != nil {
			log.Printf("测试套件 %s 清理失败: %v", suite.Name, err)
		}
	}
}

// runTestCase 运行测试用例
func (tr *TestRunner) runTestCase(suite *TestSuite, testCase *TestCase) *TestResult {
	result := &TestResult{
		SuiteName: suite.Name,
		TestCase:  testCase.Name,
		Passed:    false,
		Duration:  0,
	}
	
	startTime := time.Now()
	
	// 创建测试上下文
	test := &testing.T{}
	
	// 运行测试函数
	err := testCase.TestFunc(test)
	
	result.Duration = time.Since(startTime)
	result.Passed = err == nil
	result.Error = err
	
	if err != nil {
		log.Printf("测试失败: %s.%s - %v", suite.Name, testCase.Name, err)
		// 记录错误到错误日志
		fmt.Fprintf(tr.errorLog, "测试失败: %s.%s - %v\n", suite.Name, testCase.Name, err)
		fmt.Fprintf(tr.errorLog, "堆栈信息:\n%s\n", getStackTrace())
	} else {
		log.Printf("测试通过: %s.%s", suite.Name, testCase.Name)
	}
	
	// 记录结果到日志
	fmt.Fprintf(tr.logFile, "测试: %s.%s - %v - 耗时: %v\n", 
		suite.Name, testCase.Name, result.Passed, result.Duration)
	
	return result
}

// Close 关闭测试运行器
func (tr *TestRunner) Close() error {
	if tr.logFile != nil {
		tr.logFile.Close()
	}
	if tr.errorLog != nil {
		tr.errorLog.Close()
	}
	return nil
}

// getStackTrace 获取堆栈信息
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// AlgorithmTests 算法测试套件
func AlgorithmTests() *TestSuite {
	return &TestSuite{
		Name: "AlgorithmTests",
		Setup: func() error {
			log.Println("设置算法测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理算法测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestSlidingWindow",
				TestFunc: func(t *testing.T) error {
					window := algorithm.NewAdaptiveSlidingWindow(100, time.Hour, true)
					
					// 测试添加数据
					for i := 0; i < 50; i++ {
						window.Add(float64(i))
					}
					
					// 测试统计
					stats := window.GetStats()
					if stats["count"] != 50 {
						return fmt.Errorf("期望50个数据点，实际获得%d", int(stats["count"]))
					}
					
					// 测试平均值
					avg := window.Average()
					expectedAvg := 24.5 // (0+49)/2
					if avg != expectedAvg {
						return fmt.Errorf("期望平均值%.1f，实际获得%.1f", expectedAvg, avg)
					}
					
					return nil
				},
			},
			{
				Name: "TestTimeWheel",
				TestFunc: func(t *testing.T) error {
					timeWheel := algorithm.NewTimeWheel(time.Second, 60)
					
					// 创建测试任务
					task := &TestTask{executed: false}
					
					// 添加任务
					timeWheel.AddTask(time.Second, "test", task)
					
					// 启动时间轮
					timeWheel.Start()
					
					// 等待任务执行
					time.Sleep(2 * time.Second)
					
					// 检查任务是否执行
					if !task.executed {
						return fmt.Errorf("任务未执行")
					}
					
					timeWheel.Stop()
					return nil
				},
			},
			{
				Name: "TestBloomFilter",
				TestFunc: func(t *testing.T) error {
					bloomFilter := algorithm.NewBloomFilter(1000, 3)
					
					// 添加元素
					testItems := []string{"item1", "item2", "item3"}
					for _, item := range testItems {
						bloomFilter.AddString(item)
					}
					
					// 测试存在的元素
					for _, item := range testItems {
						if !bloomFilter.TestString(item) {
							return fmt.Errorf("元素 %s 应该存在", item)
						}
					}
					
					// 测试不存在的元素
					if bloomFilter.TestString("nonexistent") {
						return fmt.Errorf("不存在的元素不应该被检测到")
					}
					
					return nil
				},
			},
		},
	}
}

// TestTask 测试任务
type TestTask struct {
	executed bool
}

func (t *TestTask) Execute() {
	t.executed = true
}

// HashTests 哈希测试套件
func HashTests() *TestSuite {
	return &TestSuite{
		Name: "HashTests",
		Setup: func() error {
			log.Println("设置哈希测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理哈希测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestConsistentHash",
				TestFunc: func(t *testing.T) error {
					hashRing := hash.NewConsistentHash(3, nil)
					
					// 添加节点
					nodes := []string{"node1", "node2", "node3"}
					hashRing.Add(nodes...)
					
					// 测试数据分布
					distribution := make(map[string]int)
					for i := 0; i < 1000; i++ {
						key := fmt.Sprintf("key%d", i)
						node, err := hashRing.Get(key)
						if err != nil {
							return fmt.Errorf("获取节点失败: %v", err)
						}
						distribution[node]++
					}
					
					// 检查分布是否合理
					if len(distribution) != len(nodes) {
						return fmt.Errorf("期望分布到%d个节点，实际分布到%d个", len(nodes), len(distribution))
					}
					
					// 检查每个节点都有数据
					for _, node := range nodes {
						if distribution[node] == 0 {
							return fmt.Errorf("节点 %s 没有数据", node)
						}
					}
					
					return nil
				},
			},
		},
	}
}

// QueueTests 队列测试套件
func QueueTests() *TestSuite {
	return &TestSuite{
		Name: "QueueTests",
		Setup: func() error {
			log.Println("设置队列测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理队列测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestMessageQueue",
				TestFunc: func(t *testing.T) error {
					// 创建Redis存储（模拟）
					redisConfig := storage.RedisConfig{
						Address: "localhost:6379",
						DB:      1,
					}
					
					redisStorage, err := storage.NewRedisStorage(redisConfig)
					if err != nil {
						log.Printf("Redis连接失败，跳过队列测试: %v", err)
						return nil // 跳过测试而不是失败
					}
					
					messageQueue := queue.NewRedisMessageQueue(redisStorage, "test", time.Hour)
					
					// 创建测试消息
					message := &queue.Message{
						ID:        "test1",
						Topic:     "test",
						Payload:   []byte("test payload"),
						Timestamp: time.Now(),
					}
					
					// 发布消息
					ctx := context.Background()
					err = messageQueue.Publish(ctx, "test", message)
					if err != nil {
						return fmt.Errorf("发布消息失败: %v", err)
					}
					
					// 获取统计信息
					stats, err := messageQueue.GetStats(ctx)
					if err != nil {
						return fmt.Errorf("获取统计信息失败: %v", err)
					}
					
					if stats.TotalMessages != 1 {
						return fmt.Errorf("期望1条消息，实际获得%d条", stats.TotalMessages)
					}
					
					return nil
				},
			},
		},
	}
}

// UtilsTests 工具测试套件
func UtilsTests() *TestSuite {
	return &TestSuite{
		Name: "UtilsTests",
		Setup: func() error {
			log.Println("设置工具测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理工具测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestCircuitBreaker",
				TestFunc: func(t *testing.T) error {
					cb := utils.NewCircuitBreaker(3, time.Second)
					
					// 测试正常操作
					err := cb.Execute(func() error {
						return nil
					})
					if err != nil {
						return fmt.Errorf("熔断器应该允许正常操作")
					}
					
					// 测试失败操作
					for i := 0; i < 3; i++ {
						err = cb.Execute(func() error {
							return fmt.Errorf("测试错误")
						})
						if err == nil {
							return fmt.Errorf("应该返回错误")
						}
					}
					
					// 检查熔断器是否打开
					if cb.GetState() != "open" {
						return fmt.Errorf("熔断器应该打开")
					}
					
					return nil
				},
			},
			{
				Name: "TestRateLimiter",
				TestFunc: func(t *testing.T) error {
					limiter := utils.NewRateLimiter(10, 10)
					
					// 测试速率限制
					allowed := 0
					for i := 0; i < 15; i++ {
						if limiter.Allow() {
							allowed++
						}
						time.Sleep(10 * time.Millisecond)
					}
					
					if allowed > 10 {
						return fmt.Errorf("速率限制器应该限制在10次以内")
					}
					
					return nil
				},
			},
			{
				Name: "TestRetryPolicy",
				TestFunc: func(t *testing.T) error {
					policy := utils.NewRetryPolicy(3, time.Millisecond, time.Millisecond, 2.0)
					
					attempts := 0
					err := policy.ExecuteWithRetry(func() error {
						attempts++
						if attempts < 3 {
							return fmt.Errorf("临时错误")
						}
						return nil
					})
					
					if err != nil {
						return fmt.Errorf("重试应该成功")
					}
					
					if attempts != 3 {
						return fmt.Errorf("期望3次尝试，实际%d次", attempts)
					}
					
					return nil
				},
			},
		},
	}
}

// ConfigTests 配置测试套件
func ConfigTests() *TestSuite {
	return &TestSuite{
		Name: "ConfigTests",
		Setup: func() error {
			log.Println("设置配置测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理配置测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestConfigLoading",
				TestFunc: func(t *testing.T) error {
					// 测试配置文件加载
					cfg, err := config.LoadConfig("configs/agent.yaml")
					if err != nil {
						return fmt.Errorf("加载配置失败: %v", err)
					}
					
					// 检查配置值
					if cfg.Agent.HostID == "" {
						return fmt.Errorf("HostID不应该为空")
					}
					
					if cfg.Collect.Interval <= 0 {
						return fmt.Errorf("采集间隔应该大于0")
					}
					
					return nil
				},
			},
		},
	}
}

// IntegrationTests 集成测试套件
func IntegrationTests() *TestSuite {
	return &TestSuite{
		Name: "IntegrationTests",
		Setup: func() error {
			log.Println("设置集成测试环境")
			return nil
		},
		Teardown: func() error {
			log.Println("清理集成测试环境")
			return nil
		},
		TestCases: []TestCase{
			{
				Name: "TestAlgorithmManager",
				TestFunc: func(t *testing.T) error {
					// 加载配置
					cfg, err := config.LoadConfig("configs/agent.yaml")
					if err != nil {
						return fmt.Errorf("加载配置失败: %v", err)
					}
					
					// 创建算法管理器
					manager := algorithm.NewAlgorithmManager(cfg)
					
					// 启动管理器
					err = manager.Start()
					if err != nil {
						return fmt.Errorf("启动算法管理器失败: %v", err)
					}
					
					// 测试滑动窗口
					err = manager.AddSlidingWindowValue("cpu", 50.0)
					if err != nil {
						return fmt.Errorf("添加滑动窗口值失败: %v", err)
					}
					
					// 测试布隆过滤器
					manager.AddToBloomFilter("test_host")
					if !manager.TestBloomFilter("test_host") {
						return fmt.Errorf("布隆过滤器应该包含test_host")
					}
					
					// 获取统计信息
					stats := manager.GetStats()
					if stats == nil {
						return fmt.Errorf("统计信息不应该为空")
					}
					
					// 停止管理器
					manager.Stop()
					
					return nil
				},
			},
		},
	}
}

// RunAllTests 运行所有测试
func RunAllTests() error {
	runner, err := NewTestRunner()
	if err != nil {
		return fmt.Errorf("创建测试运行器失败: %v", err)
	}
	defer runner.Close()
	
	// 添加测试套件
	runner.AddSuite(AlgorithmTests())
	runner.AddSuite(HashTests())
	runner.AddSuite(QueueTests())
	runner.AddSuite(UtilsTests())
	runner.AddSuite(ConfigTests())
	runner.AddSuite(IntegrationTests())
	
	// 运行测试
	results := runner.Run()
	
	// 统计结果
	total := len(results)
	passed := 0
	failed := 0
	
	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}
	
	// 输出结果
	fmt.Printf("\n测试结果汇总:\n")
	fmt.Printf("总测试数: %d\n", total)
	fmt.Printf("通过: %d\n", passed)
	fmt.Printf("失败: %d\n", failed)
	fmt.Printf("成功率: %.1f%%\n", float64(passed)/float64(total)*100)
	
	if failed > 0 {
		fmt.Printf("\n失败的测试:\n")
		for _, result := range results {
			if !result.Passed {
				fmt.Printf("- %s.%s: %v\n", result.SuiteName, result.TestCase, result.Error)
			}
		}
		return fmt.Errorf("有%d个测试失败", failed)
	}
	
	fmt.Printf("\n🎉 所有测试通过！\n")
	return nil
}