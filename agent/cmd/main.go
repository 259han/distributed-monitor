package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/han-fei/monitor/agent/internal/algorithm"
	"github.com/han-fei/monitor/agent/internal/collector"
	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/registry"
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

	// 创建主机注册管理器
	hostRegistry := registry.NewHostRegistry(cfg)
	if cfg.HostRegistry.Enable {
		if err := hostRegistry.Start(); err != nil {
			log.Fatalf("启动主机注册管理器失败: %v", err)
		}
		defer hostRegistry.Stop()
	}

	// 创建算法管理器
	algorithmManager := algorithm.NewAlgorithmManager(cfg)
	if err := algorithmManager.Start(); err != nil {
		log.Fatalf("启动算法管理器失败: %v", err)
	}
	defer algorithmManager.Stop()

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

	// 启动后台任务
	go startBackgroundTasks(ctx, cfg, hostRegistry, algorithmManager, metricsCollector)

	fmt.Println("数据采集代理已启动...")
	printSystemInfo(cfg, hostRegistry, algorithmManager)

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

// startBackgroundTasks 启动后台任务
func startBackgroundTasks(ctx context.Context, cfg *config.Config, hostRegistry *registry.HostRegistry, algorithmManager *algorithm.AlgorithmManager, metricsCollector *collector.MetricsCollector) {
	// 启动主机状态监控
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 监控主机状态
				if cfg.HostRegistry.Enable {
					stats := hostRegistry.GetStats()
					log.Printf("主机注册统计: 总数=%d, 在线=%d, 离线=%d",
						stats["total_hosts"], stats["online_hosts"], stats["offline_hosts"])
				}
			}
		}
	}()

	// 启动算法统计监控
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 监控算法统计
				stats := algorithmManager.GetStats()
				log.Printf("算法统计: 滑动窗口=%d, 时间轮任务=%d",
					len(stats.SlidingWindowStats), stats.TimeWheelStats["task_count"])
			}
		}
	}()
}

// printSystemInfo 打印系统信息
func printSystemInfo(cfg *config.Config, hostRegistry *registry.HostRegistry, algorithmManager *algorithm.AlgorithmManager) {
	fmt.Println("=== 数据采集代理系统信息 ===")
	fmt.Printf("配置文件: %s\n", configFile)
	fmt.Printf("主机ID: %s\n", cfg.Agent.HostID)
	fmt.Printf("主机名: %s\n", cfg.Agent.Hostname)
	fmt.Printf("IP地址: %s\n", cfg.Agent.IP)

	if cfg.HostRegistry.Enable {
		localHost := hostRegistry.GetLocalHost()
		if localHost != nil {
			fmt.Printf("本地主机: %s (%s)\n", localHost.ID, localHost.IP)
		}
		fmt.Printf("主机注册: 已启用\n")
	} else {
		fmt.Printf("主机注册: 已禁用\n")
	}

	// 打印算法状态
	algorithmStats := algorithmManager.GetStats()
	fmt.Printf("滑动窗口数量: %d\n", len(algorithmStats.SlidingWindowStats))
	fmt.Printf("布隆过滤器: %s\n", map[bool]string{true: "启用", false: "禁用"}[cfg.Algorithms.BloomFilter.Enable])
	fmt.Printf("时间轮: %s\n", map[bool]string{true: "启用", false: "禁用"}[cfg.Algorithms.TimeWheel.Enable])
	fmt.Printf("前缀树: %s\n", map[bool]string{true: "启用", false: "禁用"}[cfg.Algorithms.PrefixTree.Enable])

	fmt.Println("=== 启动完成 ===")
}
