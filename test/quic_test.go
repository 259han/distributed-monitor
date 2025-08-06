package main

import (
	"context"
	"log"
	"testing"
	"time"

	quicpkg "github.com/han-fei/monitor/pkg/quic"
	pb "github.com/han-fei/monitor/proto"
)

func TestQUICServerClient(t *testing.T) {
	// 创建数据处理器
	handler := quicpkg.NewDefaultDataHandler()
	receivedMetrics := make(chan *pb.MetricsData, 1)

	handler.SetMetricsHandler(func(ctx context.Context, metrics *pb.MetricsData) error {
		receivedMetrics <- metrics
		return nil
	})

	// 创建服务器配置
	serverConfig := &quicpkg.Config{
		Host:               "localhost",
		Port:               8083, // 使用不同端口避免冲突
		MaxStreamsPerConn:  100,
		KeepAlivePeriod:    30 * time.Second,
		MaxIdleTimeout:     60 * time.Second,
		HandshakeTimeout:   10 * time.Second,
		BufferSize:         256,
		MaxMessageSize:     1024 * 1024, // 1MB
		MaxRetries:         3,
		RetryInterval:      time.Second,
		InsecureSkipVerify: true,
		NextProtos:         []string{"monitor-quic/1.0"},
	}

	// 创建服务器
	server, err := quicpkg.NewServer(serverConfig, handler)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端配置
	clientConfig := &quicpkg.Config{
		MaxStreamsPerConn:  100,
		KeepAlivePeriod:    30 * time.Second,
		MaxIdleTimeout:     60 * time.Second,
		HandshakeTimeout:   10 * time.Second,
		BufferSize:         256,
		MaxMessageSize:     1024 * 1024, // 1MB
		MaxRetries:         3,
		RetryInterval:      time.Second,
		InsecureSkipVerify: true,
		NextProtos:         []string{"monitor-quic/1.0"},
	}

	// 创建客户端
	client := quicpkg.NewClient(clientConfig, nil)

	// 连接到服务器
	if err := client.Connect("localhost:8083"); err != nil {
		t.Fatalf("连接服务器失败: %v", err)
	}
	defer client.Disconnect()

	// 发送测试数据
	testMetrics := &pb.MetricsData{
		HostId:    "test-host",
		Timestamp: time.Now().Unix(),
		Metrics: []*pb.Metric{
			{Name: "cpu_usage", Value: "75.5", Unit: "%"},
			{Name: "memory_usage", Value: "60.2", Unit: "%"},
			{Name: "disk_usage", Value: "45.8", Unit: "%"},
			{Name: "network_in", Value: "1024", Unit: "bytes"},
			{Name: "network_out", Value: "512", Unit: "bytes"},
		},
	}

	if err := client.SendMetrics(testMetrics); err != nil {
		t.Fatalf("发送指标数据失败: %v", err)
	}

	// 等待接收数据
	select {
	case received := <-receivedMetrics:
		if received.HostId != testMetrics.HostId {
			t.Errorf("HostID不匹配: 期望 %s, 得到 %s", testMetrics.HostId, received.HostId)
		}
		if len(received.Metrics) != len(testMetrics.Metrics) {
			t.Errorf("指标数量不匹配: 期望 %d, 得到 %d", len(testMetrics.Metrics), len(received.Metrics))
		}
		log.Printf("测试成功: 收到数据 HostID=%s, 指标数量=%d", received.HostId, len(received.Metrics))
	case <-time.After(5 * time.Second):
		t.Fatal("超时: 未收到数据")
	}
}

func TestQUICHeartbeat(t *testing.T) {
	// 创建数据处理器
	handler := quicpkg.NewDefaultDataHandler()
	receivedHeartbeat := make(chan string, 1)

	handler.SetHeartbeatHandler(func(ctx context.Context, hostID string) error {
		receivedHeartbeat <- hostID
		return nil
	})

	// 创建服务器配置
	serverConfig := &quicpkg.Config{
		Host:               "localhost",
		Port:               8084, // 使用不同端口
		MaxStreamsPerConn:  100,
		KeepAlivePeriod:    30 * time.Second,
		MaxIdleTimeout:     60 * time.Second,
		HandshakeTimeout:   10 * time.Second,
		BufferSize:         256,
		MaxMessageSize:     1024 * 1024,
		MaxRetries:         3,
		RetryInterval:      time.Second,
		InsecureSkipVerify: true,
		NextProtos:         []string{"monitor-quic/1.0"},
	}

	// 创建服务器
	server, err := quicpkg.NewServer(serverConfig, handler)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端配置
	clientConfig := &quicpkg.Config{
		MaxStreamsPerConn:  100,
		KeepAlivePeriod:    30 * time.Second,
		MaxIdleTimeout:     60 * time.Second,
		HandshakeTimeout:   10 * time.Second,
		BufferSize:         256,
		MaxMessageSize:     1024 * 1024,
		MaxRetries:         3,
		RetryInterval:      time.Second,
		InsecureSkipVerify: true,
		NextProtos:         []string{"monitor-quic/1.0"},
	}

	// 创建客户端
	client := quicpkg.NewClient(clientConfig, nil)

	// 连接到服务器
	if err := client.Connect("localhost:8084"); err != nil {
		t.Fatalf("连接服务器失败: %v", err)
	}
	defer client.Disconnect()

	// 发送心跳
	testHostID := "test-heartbeat-host"
	if err := client.SendHeartbeat(testHostID); err != nil {
		t.Fatalf("发送心跳失败: %v", err)
	}

	// 等待接收心跳
	select {
	case received := <-receivedHeartbeat:
		if received != testHostID {
			t.Errorf("心跳HostID不匹配: 期望 %s, 得到 %s", testHostID, received)
		}
		log.Printf("心跳测试成功: 收到心跳 HostID=%s", received)
	case <-time.After(5 * time.Second):
		t.Fatal("超时: 未收到心跳")
	}
}
