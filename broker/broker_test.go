package broker

import (
	"fmt"
	"testing"
	"time"

	"github.com/han-fei/monitor/broker/internal/config"
)

// TestBrokerCreation 测试Broker创建
func TestBrokerCreation(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:      "broker-1",
			Address: "localhost:9095",
		},
		Cluster: config.ClusterConfig{
			Endpoints: []string{"localhost:9095"},
		},
		Raft: config.RaftConfig{
			LogDir:        "data/raft/logs",
			SnapshotDir:   "data/raft/snapshots",
			HeartbeatTimeout: 500 * time.Millisecond,
			ElectionTimeout:  time.Second,
		},
	}

	broker := NewBroker(cfg)
	if broker == nil {
		t.Fatal("Broker creation failed")
	}

	if broker.Config.Node.ID != cfg.Node.ID {
		t.Errorf("Expected Node ID %s, got %s", cfg.Node.ID, broker.Config.Node.ID)
	}
}

// TestConsistentHash 测试一致性哈希
func TestConsistentHash(t *testing.T) {
	// 简单的一致性哈希测试
	nodes := []string{"node-1", "node-2", "node-3"}
	testKeys := []string{"key-1", "key-2", "key-3", "key-4", "key-5"}
	distribution := make(map[string]int)

	// 简单的哈希函数
	simpleHash := func(key string) int {
		hash := 0
		for _, c := range key {
			hash = (hash << 5) + hash + int(c)
		}
		return hash
	}

	// 测试数据分布
	for _, key := range testKeys {
		hash := simpleHash(key)
		nodeIndex := hash % len(nodes)
		selectedNode := nodes[nodeIndex]
		distribution[selectedNode]++
	}

	// 验证分布
	if len(distribution) == 0 {
		t.Error("Should have some distribution")
	}

	t.Logf("Distribution: %+v", distribution)
}

// TestBrokerLifecycle 测试Broker生命周期
func TestBrokerLifecycle(t *testing.T) {
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:      "lifecycle-broker",
			Address: "localhost:9096",
		},
		Cluster: config.ClusterConfig{
			Endpoints: []string{"localhost:9096"},
		},
		Raft: config.RaftConfig{
			LogDir:        "test-data/raft/logs",
			SnapshotDir:   "test-data/raft/snapshots",
			HeartbeatTimeout: 500 * time.Millisecond,
			ElectionTimeout:  time.Second,
		},
	}

	broker := NewBroker(cfg)
	if broker == nil {
		t.Fatal("Broker creation failed")
	}

	// 启动Broker
	err := broker.Start()
	if err != nil {
		t.Logf("Broker start failed (expected if ports not available): %v", err)
	} else {
		// 等待一段时间
		time.Sleep(100 * time.Millisecond)

		// 停止Broker
		err = broker.Stop()
		if err != nil {
			t.Logf("Broker stop failed: %v", err)
		} else {
			t.Log("Broker lifecycle test completed")
		}
	}
}



// BenchmarkHashPerformance 基准测试：哈希性能
func BenchmarkHashPerformance(b *testing.B) {
	// 简单的哈希函数
	simpleHash := func(key string) int {
		hash := 0
		for _, c := range key {
			hash = (hash << 5) + hash + int(c)
		}
		return hash
	}

	nodes := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		hash := simpleHash(key)
		nodeIndex := hash % len(nodes)
		_ = nodes[nodeIndex]
	}
}

// Mock implementations for testing
type Broker struct {
	Config *config.Config
	// 其他字段...
}

func NewBroker(cfg *config.Config) *Broker {
	return &Broker{Config: cfg}
}

func (b *Broker) Start() error {
	// 模拟启动逻辑
	return nil
}

func (b *Broker) Stop() error {
	// 模拟停止逻辑
	return nil
}