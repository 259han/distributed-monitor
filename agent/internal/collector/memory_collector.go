package collector

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
)

// MemoryCollector 内存指标采集器
type MemoryCollector struct {
	config *config.Config
	stopCh chan struct{}
}

// NewMemoryCollector 创建内存采集器
func NewMemoryCollector(cfg *config.Config) *MemoryCollector {
	return &MemoryCollector{
		config: cfg,
		stopCh: make(chan struct{}),
	}
}

// Start 启动采集器
func (c *MemoryCollector) Start(ctx context.Context) error {
	return nil
}

// Stop 停止采集器
func (c *MemoryCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集内存指标
func (c *MemoryCollector) Collect() ([]models.Metric, error) {
	var metrics []models.Metric

	// 读取内存信息
	memInfo, err := c.readMemInfo()
	if err != nil {
		return nil, err
	}

	// 总内存
	if total, ok := memInfo["MemTotal"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.total",
			Value: float64(total),
			Unit:  "KB",
		})
	}

	// 空闲内存
	if free, ok := memInfo["MemFree"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.free",
			Value: float64(free),
			Unit:  "KB",
		})
	}

	// 可用内存（包括缓存和缓冲区）
	if available, ok := memInfo["MemAvailable"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.available",
			Value: float64(available),
			Unit:  "KB",
		})
	}

	// 已使用内存
	if total, ok := memInfo["MemTotal"]; ok {
		if available, ok := memInfo["MemAvailable"]; ok {
			used := total - available
			metrics = append(metrics, models.Metric{
				Name:  "memory.used",
				Value: float64(used),
				Unit:  "KB",
			})

			// 使用率
			usageRate := float64(used) / float64(total) * 100
			metrics = append(metrics, models.Metric{
				Name:  "memory.usage",
				Value: usageRate,
				Unit:  "%",
			})
		}
	}

	// 缓冲区
	if buffers, ok := memInfo["Buffers"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.buffers",
			Value: float64(buffers),
			Unit:  "KB",
		})
	}

	// 缓存
	if cached, ok := memInfo["Cached"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.cached",
			Value: float64(cached),
			Unit:  "KB",
		})
	}

	// Swap总量
	if swapTotal, ok := memInfo["SwapTotal"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.swap_total",
			Value: float64(swapTotal),
			Unit:  "KB",
		})
	}

	// Swap空闲
	if swapFree, ok := memInfo["SwapFree"]; ok {
		metrics = append(metrics, models.Metric{
			Name:  "memory.swap_free",
			Value: float64(swapFree),
			Unit:  "KB",
		})
	}

	// Swap使用量
	if swapTotal, ok := memInfo["SwapTotal"]; ok {
		if swapFree, ok := memInfo["SwapFree"]; ok {
			swapUsed := swapTotal - swapFree
			metrics = append(metrics, models.Metric{
				Name:  "memory.swap_used",
				Value: float64(swapUsed),
				Unit:  "KB",
			})

			// Swap使用率
			if swapTotal > 0 {
				swapUsageRate := float64(swapUsed) / float64(swapTotal) * 100
				metrics = append(metrics, models.Metric{
					Name:  "memory.swap_usage",
					Value: swapUsageRate,
					Unit:  "%",
				})
			}
		}
	}

	return metrics, nil
}

// readMemInfo 读取内存信息
func (c *MemoryCollector) readMemInfo() (map[string]uint64, error) {
	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	memInfo := make(map[string]uint64)

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// 移除冒号
		key := strings.TrimSuffix(parts[0], ":")

		// 解析值
		value, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}

		memInfo[key] = value
	}

	if len(memInfo) == 0 {
		return nil, fmt.Errorf("no memory information found")
	}

	return memInfo, nil
}
