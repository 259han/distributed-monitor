package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/han-fei/monitor/broker/internal/config"
	"github.com/han-fei/monitor/broker/internal/host"
)

func mainHostManager() {
	var configFile string
	flag.StringVar(&configFile, "config", "configs/broker.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动主机管理器
	if cfg.HostManagement.Enabled {
		managerConfig := &host.ManagerConfig{
			RegistryConfig: &host.RegistryConfig{
				HeartbeatInterval:   cfg.HostManagement.Registry.HeartbeatInterval,
				HealthCheckInterval: cfg.HostManagement.Registry.HealthCheckInterval,
				OfflineTimeout:      cfg.HostManagement.Registry.OfflineTimeout,
				CleanupInterval:     cfg.HostManagement.Registry.CleanupInterval,
			},
			DiscoveryConfig: &host.DiscoveryConfig{
				ListenPort:       cfg.HostManagement.Discovery.ListenPort,
				ScanInterval:     cfg.HostManagement.Discovery.ScanInterval,
				ScanTimeout:      cfg.HostManagement.Discovery.ScanTimeout,
				EnableMulticast:  cfg.HostManagement.Discovery.EnableMulticast,
				EnableHTTP:       cfg.HostManagement.Discovery.EnableHTTP,
				EnableFileWatch:  cfg.HostManagement.Discovery.EnableFileWatch,
				NetworkRanges:    cfg.HostManagement.Discovery.NetworkRanges,
				DiscoveryTypes:   cfg.HostManagement.Discovery.DiscoveryTypes,
				ServicePortRange: cfg.HostManagement.Discovery.ServicePortRange,
			},
			HealthConfig: &host.HealthConfig{
				CheckInterval:   cfg.HostManagement.Health.CheckInterval,
				Timeout:         cfg.HostManagement.Health.Timeout,
				MaxRetries:      cfg.HostManagement.Health.MaxRetries,
				EnableTCP:       cfg.HostManagement.Health.EnableTCP,
				EnableHTTP:      cfg.HostManagement.Health.EnableHTTP,
				EnableCustom:    cfg.HostManagement.Health.EnableCustom,
				TCPPorts:        cfg.HostManagement.Health.TCPPorts,
				HTTPPaths:       cfg.HostManagement.Health.HTTPPaths,
				AlertThresholds: cfg.HostManagement.Health.AlertThresholds,
			},
			AutoRemove:    cfg.HostManagement.AutoRemoveCfg.Enabled,
			RemoveTimeout: cfg.HostManagement.AutoRemoveCfg.UnhealthyTimeout,
			MaxHosts:      cfg.HostManagement.MaxHosts,
			EnableScaling: cfg.HostManagement.EnableScaling,
		}

		hostManager := host.NewHostManager(managerConfig)
		if err := hostManager.Start(ctx); err != nil {
			log.Fatalf("启动主机管理器失败: %v", err)
		}
		defer hostManager.Stop()

		log.Println("动态主机管理系统已启动")
		log.Printf("配置: 启用=%v, 自动移除=%v, 扩缩容=%v, 最大主机数=%d",
			cfg.HostManagement.Enabled,
			cfg.HostManagement.AutoRemoveCfg.Enabled,
			cfg.HostManagement.EnableScaling,
			cfg.HostManagement.MaxHosts)

		// 订阅主机管理事件
		eventChan := hostManager.Subscribe("main")
		go func() {
			for event := range eventChan {
				log.Printf("主机管理事件: %+v", event)
			}
		}()
		defer hostManager.Unsubscribe("main")
	} else {
		log.Println("动态主机管理已禁用")
	}

	// 等待信号
	<-sigChan
	log.Println("正在关闭动态主机管理系统...")
	cancel()

	log.Println("动态主机管理系统已关闭")
}
