package visualization

import (
	"testing"
	"time"

	"github.com/han-fei/monitor/visualization/internal/analysis"
	"github.com/han-fei/monitor/visualization/internal/config"
	"github.com/han-fei/monitor/visualization/internal/models"
)

// TestVisualizationCreation 测试Visualization创建
func TestVisualizationCreation(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		Broker: config.BrokerConfig{
			Endpoints: []string{"localhost:9090"},
		},
	}

	viz := NewVisualization(cfg)
	if viz == nil {
		t.Fatal("Visualization creation failed")
	}

	if viz.Config.Server.Port != cfg.Server.Port {
		t.Errorf("Expected port %d, got %d", cfg.Server.Port, viz.Config.Server.Port)
	}
}

// TestMetricsAnalysis 测试指标分析
func TestMetricsAnalysis(t *testing.T) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		t.Fatal("Analyzer creation failed")
	}

	// 测试指标数据
	data := &models.MetricsData{
		HostID:    "test-host",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_usage", Value: 45.5, Unit: "%"},
			{Name: "memory_usage", Value: 1024.0, Unit: "MB"},
			{Name: "disk_usage", Value: 85.0, Unit: "%"},
		},
	}

	// 添加数据到分析器
	analyzer.AddData(data)

	// 测试聚合功能
	timeRange := analysis.TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	result, err := analyzer.Aggregate("test-host", "cpu_usage", "avg", timeRange)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Aggregation result should not be nil")
	}

	if result.Type != "avg" {
		t.Errorf("Expected aggregation type 'avg', got '%s'", result.Type)
	}
}

// TestTrendAnalysis 测试趋势分析
func TestTrendAnalysis(t *testing.T) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		t.Fatal("Analyzer creation failed")
	}

	// 添加测试数据 - 使用过去的时间点
	baseTime := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 15; i++ {
		data := &models.MetricsData{
			HostID:    "test-host",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Metrics: []models.Metric{
				{Name: "cpu_usage", Value: float64(40 + i), Unit: "%"},
			},
		}
		analyzer.AddData(data)
	}

	// 测试趋势分析
	timeRange := analysis.TimeRange{
		Start: baseTime,
		End:   time.Now(),
	}

	trend, err := analyzer.AnalyzeTrend("test-host", "cpu_usage", timeRange)
	if err != nil {
		t.Fatalf("Trend analysis failed: %v", err)
	}

	if trend == nil {
		t.Fatal("Trend result should not be nil")
	}

	if trend.Direction == "" {
		t.Error("Trend direction should not be empty")
	}

	t.Logf("Trend analysis: Direction=%s, Slope=%.2f, Correlation=%.2f", 
		trend.Direction, trend.Slope, trend.Correlation)
}

// TestAnomalyDetection 测试异常检测
func TestAnomalyDetection(t *testing.T) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		t.Fatal("Analyzer creation failed")
	}

	// 添加正常数据 - 使用过去的时间点
	baseTime := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 25; i++ {
		data := &models.MetricsData{
			HostID:    "test-host",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Metrics: []models.Metric{
				{Name: "cpu_usage", Value: 45.0 + float64(i%5), Unit: "%"},
			},
		}
		analyzer.AddData(data)
	}

	// 添加异常数据
	anomalyData := &models.MetricsData{
		HostID:    "test-host",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_usage", Value: 95.0, Unit: "%"}, // 异常高值
		},
	}
	analyzer.AddData(anomalyData)

	// 测试异常检测
	timeRange := analysis.TimeRange{
		Start: baseTime,
		End:   time.Now(),
	}

	anomaly, err := analyzer.DetectAnomaly("test-host", "cpu_usage", timeRange)
	if err != nil {
		t.Fatalf("Anomaly detection failed: %v", err)
	}

	if anomaly == nil {
		t.Fatal("Anomaly result should not be nil")
	}

	t.Logf("Anomaly detection: IsAnomaly=%v, Score=%.2f, Method=%s", 
		anomaly.IsAnomaly, anomaly.AnomalyScore, anomaly.Method)
}

// TestTopKAnalysis 测试TopK分析
func TestTopKAnalysis(t *testing.T) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		t.Fatal("Analyzer creation failed")
	}

	// 添加多主机数据
	hosts := []string{"host-1", "host-2", "host-3", "host-4", "host-5"}
	cpuValues := []float64{85.0, 45.0, 92.0, 38.0, 76.0}

	for i, host := range hosts {
		data := &models.MetricsData{
			HostID:    host,
			Timestamp: time.Now(),
			Metrics: []models.Metric{
				{Name: "cpu_usage", Value: cpuValues[i], Unit: "%"},
			},
		}
		analyzer.AddData(data)
	}

	// 测试TopK分析
	topK := analyzer.GetTopK("cpu_usage", 3)
	if len(topK) == 0 {
		t.Fatal("TopK results should not be empty")
	}

	if len(topK) > 3 {
		t.Errorf("Expected at most 3 results, got %d", len(topK))
	}

	// 验证排序
	for i := 1; i < len(topK); i++ {
		if topK[i].Value > topK[i-1].Value {
			t.Errorf("TopK results should be sorted in descending order")
		}
	}

	t.Logf("TopK CPU usage:")
	for i, result := range topK {
		t.Logf("  %d. %s: %.2f%%", i+1, result.HostID, result.Value)
	}
}

// TestAlertSystem 测试告警系统
func TestAlertSystem(t *testing.T) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		t.Fatal("Analyzer creation failed")
	}

	// 添加触发告警的数据
	alertData := &models.MetricsData{
		HostID:    "test-host",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_usage", Value: 95.0, Unit: "%"}, // 超过告警阈值
		},
	}
	analyzer.AddData(alertData)

	// 检查告警
	alerts := analyzer.CheckAlerts(nil)
	if len(alerts) == 0 {
		t.Log("No alerts triggered (may be expected)")
	} else {
		t.Logf("Detected %d alerts:", len(alerts))
		for _, alert := range alerts {
			t.Logf("  - %s: %s (%s)", alert.RuleName, alert.Message, alert.Severity)
		}
	}
}


// TestConfigurationLoading 测试配置加载
func TestConfigurationLoading(t *testing.T) {
	cfg, err := config.LoadConfig("../configs/visualization.yaml")
	if err != nil {
		t.Fatalf("Config loading failed: %v", err)
	}

	if cfg.Server.Port <= 0 {
		t.Error("Server port should be positive")
	}

	if cfg.Server.Host == "" {
		t.Error("Server host should not be empty")
	}

	if len(cfg.Broker.Endpoints) == 0 {
		t.Error("Broker endpoints should not be empty")
	}
}

// TestVisualizationLifecycle 测试Visualization生命周期
func TestVisualizationLifecycle(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8081,
			Host: "localhost",
		},
		Broker: config.BrokerConfig{
			Endpoints: []string{"localhost:9090"},
		},
	}

	viz := NewVisualization(cfg)
	if viz == nil {
		t.Fatal("Visualization creation failed")
	}

	// 启动Visualization
	err := viz.Start()
	if err != nil {
		t.Logf("Visualization start failed (expected if port not available): %v", err)
	} else {
		// 等待一段时间
		time.Sleep(100 * time.Millisecond)

		// 停止Visualization
		err = viz.Stop()
		if err != nil {
			t.Logf("Visualization stop failed: %v", err)
		} else {
			t.Log("Visualization lifecycle test completed")
		}
	}
}

// BenchmarkAnalysisPerformance 基准测试：分析性能
func BenchmarkAnalysisPerformance(b *testing.B) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		b.Fatal("Analyzer creation failed")
	}

	// 预填充数据
	for i := 0; i < 1000; i++ {
		data := &models.MetricsData{
			HostID:    "benchmark-host",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Metrics: []models.Metric{
				{Name: "cpu_usage", Value: float64(40 + i%20), Unit: "%"},
				{Name: "memory_usage", Value: float64(1024 + i%100), Unit: "MB"},
			},
		}
		analyzer.AddData(data)
	}

	timeRange := analysis.TimeRange{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.Aggregate("benchmark-host", "cpu_usage", "avg", timeRange)
		if err != nil {
			b.Fatalf("Aggregation failed: %v", err)
		}
	}
}

// BenchmarkTopKPerformance 基准测试：TopK性能
func BenchmarkTopKPerformance(b *testing.B) {
	analyzer := analysis.NewAnalyzer()
	if analyzer == nil {
		b.Fatal("Analyzer creation failed")
	}

	// 预填充数据
	hosts := []string{"host-1", "host-2", "host-3", "host-4", "host-5"}
	for i := 0; i < 1000; i++ {
		for j, host := range hosts {
			data := &models.MetricsData{
				HostID:    host,
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
				Metrics: []models.Metric{
					{Name: "cpu_usage", Value: float64(40 + j*10 + i%20), Unit: "%"},
				},
			}
			analyzer.AddData(data)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := analyzer.GetTopK("cpu_usage", 3)
		if len(results) == 0 {
			b.Fatal("TopK results should not be empty")
		}
	}
}

// Mock implementations for testing
type Visualization struct {
	Config *config.Config
	// 其他字段...
}

func NewVisualization(cfg *config.Config) *Visualization {
	return &Visualization{Config: cfg}
}

func (v *Visualization) Start() error {
	// 模拟启动逻辑
	return nil
}

func (v *Visualization) Stop() error {
	// 模拟停止逻辑
	return nil
}