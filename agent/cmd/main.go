package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/han-fei/monitor/agent/internal/collector"
	"github.com/han-fei/monitor/agent/internal/config"
)

var (
	configFile string
)

func init() {
	flag.StringVar(&configFile, "config", "configs/agent.yaml", "配置文件路径")
}

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建指标采集器
	metricsCollector := collector.NewMetricsCollector(cfg)

	// 注册CPU采集器
	cpuCollector := collector.NewCPUCollector(cfg)
	metricsCollector.RegisterCollector("cpu", cpuCollector)

	// 注册内存采集器
	memCollector := collector.NewMemoryCollector(cfg)
	metricsCollector.RegisterCollector("memory", memCollector)

	// 注册网络采集器
	netCollector := collector.NewNetworkCollector(cfg)
	metricsCollector.RegisterCollector("network", netCollector)

	// 注册磁盘采集器
	diskCollector := collector.NewDiskCollector(cfg)
	metricsCollector.RegisterCollector("disk", diskCollector)

	// 启动采集器
	if err := metricsCollector.Start(ctx); err != nil {
		log.Fatalf("启动采集器失败: %v", err)
	}

	fmt.Println("数据采集代理已启动...")

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("正在关闭数据采集代理...")

	// 停止采集器
	if err := metricsCollector.Stop(); err != nil {
		log.Fatalf("停止采集器失败: %v", err)
	}

	fmt.Println("数据采集代理已关闭")
}
