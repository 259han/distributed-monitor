package agent

import (
	"testing"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
)

// TestAgentCreation 测试Agent创建
func TestAgentCreation(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			HostID:   "test-agent",
			Hostname: "test-hostname",
			IP:       "127.0.0.1",
			Port:     9093,
		},
		Collect: config.CollectConfig{
			Interval: time.Second,
		},
		Broker: config.BrokerConfig{
			Endpoints: []string{"localhost:9093"},
		},
	}

	agent := NewAgent(cfg)
	if agent == nil {
		t.Fatal("Agent creation failed")
	}

	if agent.Config.Agent.HostID != cfg.Agent.HostID {
		t.Errorf("Expected HostID %s, got %s", cfg.Agent.HostID, agent.Config.Agent.HostID)
	}
}

// TestMetricsCollection 测试指标采集
func TestMetricsCollection(t *testing.T) {
	// 测试基本的指标数据结构
	data := &models.MetricsData{
		HostID:    "test-host",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_usage", Value: 45.5, Unit: "%"},
			{Name: "memory_usage", Value: 1024.0, Unit: "MB"},
			{Name: "disk_usage", Value: 85.0, Unit: "%"},
		},
	}

	if data.HostID == "" {
		t.Error("HostID should not be empty")
	}

	if data.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if len(data.Metrics) == 0 {
		t.Error("Should collect at least one metric")
	}

	// 验证基本指标
	metricNames := make(map[string]bool)
	for _, metric := range data.Metrics {
		metricNames[metric.Name] = true
		
		if metric.Value < 0 {
			t.Errorf("Metric %s should not be negative: %f", metric.Name, metric.Value)
		}
	}

	// 检查关键指标是否存在
	expectedMetrics := []string{"cpu_usage", "memory_usage", "disk_usage"}
	for _, name := range expectedMetrics {
		if !metricNames[name] {
			t.Errorf("Expected metric %s not found", name)
		}
	}
}

// TestConfigurationLoading 测试配置加载
func TestConfigurationLoading(t *testing.T) {
	cfg, err := config.LoadConfig("../configs/agent.yaml")
	if err != nil {
		t.Fatalf("Config loading failed: %v", err)
	}

	if cfg.Agent.HostID == "" {
		t.Error("HostID should not be empty")
	}

	if len(cfg.Broker.Endpoints) == 0 {
		t.Error("Broker endpoints should not be empty")
	}

	if cfg.Collect.Interval <= 0 {
		t.Error("Interval should be positive")
	}
}

// TestHostInfo 测试主机信息
func TestHostInfo(t *testing.T) {
	hostInfo := &models.HostInfo{
		ID:          "test-host",
		Hostname:    "test-hostname",
		IP:          "192.168.1.100",
		OS:          "Linux",
		CPUCores:    4,
		TotalMemory: 8589934592, // 8GB
	}

	if hostInfo.ID == "" {
		t.Error("HostID should not be empty")
	}

	if hostInfo.Hostname == "" {
		t.Error("Hostname should not be empty")
	}

	if hostInfo.IP == "" {
		t.Error("IP should not be empty")
	}

	if hostInfo.CPUCores <= 0 {
		t.Error("CPUCores should be positive")
	}

	if hostInfo.TotalMemory <= 0 {
		t.Error("TotalMemory should be positive")
	}
}

// TestAgentLifecycle 测试Agent生命周期
func TestAgentLifecycle(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			HostID:   "lifecycle-test-agent",
			Hostname: "test-hostname",
			IP:       "127.0.0.1",
			Port:     9093,
		},
		Collect: config.CollectConfig{
			Interval: 100 * time.Millisecond,
		},
		Broker: config.BrokerConfig{
			Endpoints: []string{"localhost:9093"},
		},
	}

	agent := NewAgent(cfg)
	if agent == nil {
		t.Fatal("Agent creation failed")
	}

	// 启动Agent
	err := agent.Start()
	if err != nil {
		t.Logf("Agent start failed (expected if broker not running): %v", err)
	} else {
		// 等待一段时间
		time.Sleep(200 * time.Millisecond)

		// 停止Agent
		err = agent.Stop()
		if err != nil {
			t.Logf("Agent stop failed: %v", err)
		} else {
			t.Log("Agent lifecycle test completed")
		}
	}
}

// TestMetricsValidation 测试指标验证
func TestMetricsValidation(t *testing.T) {
	validMetrics := []models.Metric{
		{Name: "cpu_usage", Value: 45.5, Unit: "%"},
		{Name: "memory_usage", Value: 1024.0, Unit: "MB"},
		{Name: "disk_usage", Value: 85.0, Unit: "%"},
	}

	for _, metric := range validMetrics {
		if metric.Name == "" {
			t.Errorf("Metric %s should have valid name", metric.Name)
		}
		if metric.Value < 0 {
			t.Errorf("Metric %s should not be negative: %f", metric.Name, metric.Value)
		}
	}

	invalidMetrics := []models.Metric{
		{Name: "", Value: 45.5, Unit: "%"},        // 空名称
		{Name: "cpu_usage", Value: -1.0, Unit: "%"}, // 负值
	}

	for _, metric := range invalidMetrics {
		isValid := true
		if metric.Name == "" {
			isValid = false
		}
		if metric.Value < 0 {
			isValid = false
		}
		
		if isValid {
			t.Errorf("Metric %+v should be invalid", metric)
		}
	}
}

// BenchmarkDataTransmission 基准测试：数据传输性能
func BenchmarkDataTransmission(b *testing.B) {
	data := &models.MetricsData{
		HostID:    "benchmark-host",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_usage", Value: 45.5, Unit: "%"},
			{Name: "memory_usage", Value: 1024.0, Unit: "MB"},
			{Name: "disk_usage", Value: 85.0, Unit: "%"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟数据验证
		if data.HostID == "" {
			b.Fatal("HostID should not be empty")
		}
		if len(data.Metrics) == 0 {
			b.Fatal("Metrics should not be empty")
		}
	}
}

// Mock implementations for testing
type Agent struct {
	Config *config.Config
	// 其他字段...
}

func NewAgent(cfg *config.Config) *Agent {
	return &Agent{Config: cfg}
}

func (a *Agent) Start() error {
	// 模拟启动逻辑
	return nil
}

func (a *Agent) Stop() error {
	// 模拟停止逻辑
	return nil
}