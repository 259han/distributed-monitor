package collector

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/han-fei/monitor/pkg/interfaces"
)

// MemoryCollector 内存指标采集器
type MemoryCollector struct {
	config        *config.Config
	stopCh        chan struct{}
	mu            sync.RWMutex
	cacheDuration time.Duration
	lastCollect   time.Time
	cachedMetrics []models.Metric
	enableVMStat  bool
	enableNuma    bool
	numaNodes     []string
	vmstatFields  []string
}

// NewMemoryCollector 创建内存采集器
func NewMemoryCollector(cfg *config.Config) *MemoryCollector {
	c := &MemoryCollector{
		config:        cfg,
		stopCh:        make(chan struct{}),
		cacheDuration: 2 * time.Second,
		lastCollect:   time.Time{},
		cachedMetrics: make([]models.Metric, 0),
		enableVMStat:  true,
		enableNuma:    true,
		numaNodes:     make([]string, 0),
		vmstatFields: []string{
			"nr_free_pages", "nr_inactive_anon", "nr_active_anon",
			"nr_inactive_file", "nr_active_file", "nr_unevictable",
			"nr_slab_reclaimable", "nr_slab_unreclaimable", "nr_page_table_pages",
			"nr_dirty", "nr_writeback", "nr_anon_pages", "nr_mapped",
			"nr_shmem", "nr_slab", "nr_kernel_stack", "nr_pages_scanned",
			"workingset_refault", "nr_vmscan_write", "nr_vmscan_immediate_reclaim",
		},
	}

	// 检测NUMA节点
	c.detectNUMANodes()

	// 从配置中读取设置
	if cfg.Collect.CacheDuration > 0 {
		c.cacheDuration = cfg.Collect.CacheDuration
	}
	c.enableVMStat = cfg.Collect.EnableVMStat
	c.enableNuma = cfg.Collect.EnableNUMA

	return c
}

// detectNUMANodes 检测NUMA节点
func (c *MemoryCollector) detectNUMANodes() {
	numaPath := "/sys/devices/system/node"
	if _, err := os.Stat(numaPath); os.IsNotExist(err) {
		return
	}

	files, err := ioutil.ReadDir(numaPath)
	if err != nil {
		return
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "node") {
			c.numaNodes = append(c.numaNodes, file.Name())
		}
	}
}

// Start 启动采集器
func (c *MemoryCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 启动后台协程更新缓存
	go c.cacheUpdateLoop(ctx)

	return nil
}

// cacheUpdateLoop 缓存更新循环
func (c *MemoryCollector) cacheUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cacheDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.updateCache()
		}
	}
}

// updateCache 更新缓存
func (c *MemoryCollector) updateCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics, err := c.collectAllMetrics()
	if err != nil {
		return
	}

	c.cachedMetrics = metrics
	c.lastCollect = time.Now()
}

// Stop 停止采集器
func (c *MemoryCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集内存指标
func (c *MemoryCollector) Collect() ([]interfaces.Metric, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 如果缓存未过期，直接返回缓存数据
	if time.Since(c.lastCollect) < c.cacheDuration && len(c.cachedMetrics) > 0 {
		return ConvertMetrics(c.cachedMetrics), nil
	}

	// 否则重新采集
	metrics, err := c.collectAllMetrics()
	if err != nil {
		return nil, err
	}

	return ConvertMetrics(metrics), nil
}

// collectAllMetrics 采集所有内存指标
func (c *MemoryCollector) collectAllMetrics() ([]models.Metric, error) {
	var metrics []models.Metric

	// 读取内存信息
	memInfo, err := c.readMemInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to read memory info: %v", err)
	}

	// 基本内存指标
	if err := c.collectBasicMetrics(memInfo, &metrics); err != nil {
		return nil, err
	}

	// 高级内存指标
	if err := c.collectAdvancedMetrics(&metrics); err != nil {
		return nil, err
	}

	// NUMA内存指标
	if c.enableNuma && len(c.numaNodes) > 0 {
		if err := c.collectNUMAMetrics(&metrics); err != nil {
			return nil, err
		}
	}

	// 虚拟内存统计
	if c.enableVMStat {
		if err := c.collectVMStatMetrics(&metrics); err != nil {
			return nil, err
		}
	}

	return metrics, nil
}

// collectBasicMetrics 采集基本内存指标
func (c *MemoryCollector) collectBasicMetrics(memInfo map[string]uint64, metrics *[]models.Metric) error {
	// 总内存
	if total, ok := memInfo["MemTotal"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.total",
			Value: float64(total),
			Unit:  "KB",
		})
	}

	// 空闲内存
	if free, ok := memInfo["MemFree"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.free",
			Value: float64(free),
			Unit:  "KB",
		})
	}

	// 可用内存（包括缓存和缓冲区）
	if available, ok := memInfo["MemAvailable"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.available",
			Value: float64(available),
			Unit:  "KB",
		})
	}

	// 已使用内存
	if total, ok := memInfo["MemTotal"]; ok {
		if available, ok := memInfo["MemAvailable"]; ok {
			used := total - available
			*metrics = append(*metrics, models.Metric{
				Name:  "memory.used",
				Value: float64(used),
				Unit:  "KB",
			})

			// 使用率
			usageRate := float64(used) / float64(total) * 100
			*metrics = append(*metrics, models.Metric{
				Name:  "memory.usage",
				Value: usageRate,
				Unit:  "%",
			})
		}
	}

	// 缓冲区
	if buffers, ok := memInfo["Buffers"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.buffers",
			Value: float64(buffers),
			Unit:  "KB",
		})
	}

	// 缓存
	if cached, ok := memInfo["Cached"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.cached",
			Value: float64(cached),
			Unit:  "KB",
		})
	}

	// Swap总量
	if swapTotal, ok := memInfo["SwapTotal"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.swap_total",
			Value: float64(swapTotal),
			Unit:  "KB",
		})
	}

	// Swap空闲
	if swapFree, ok := memInfo["SwapFree"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.swap_free",
			Value: float64(swapFree),
			Unit:  "KB",
		})
	}

	// Swap使用量
	if swapTotal, ok := memInfo["SwapTotal"]; ok {
		if swapFree, ok := memInfo["SwapFree"]; ok {
			swapUsed := swapTotal - swapFree
			*metrics = append(*metrics, models.Metric{
				Name:  "memory.swap_used",
				Value: float64(swapUsed),
				Unit:  "KB",
			})

			// Swap使用率
			if swapTotal > 0 {
				swapUsageRate := float64(swapUsed) / float64(swapTotal) * 100
				*metrics = append(*metrics, models.Metric{
					Name:  "memory.swap_usage",
					Value: swapUsageRate,
					Unit:  "%",
				})
			}
		}
	}

	return nil
}

// collectAdvancedMetrics 采集高级内存指标
func (c *MemoryCollector) collectAdvancedMetrics(metrics *[]models.Metric) error {
	// 读取内存映射信息
	if err := c.collectMemoryMappings(metrics); err != nil {
		return err
	}

	// 读取内存压力信息
	if err := c.collectMemoryPressure(metrics); err != nil {
		return err
	}

	// 读取内存页面信息
	if err := c.collectPageInfo(metrics); err != nil {
		return err
	}

	// 读取内存交换信息
	if err := c.collectSwapInfo(metrics); err != nil {
		return err
	}

	return nil
}

// collectMemoryMappings 采集内存映射信息
func (c *MemoryCollector) collectMemoryMappings(metrics *[]models.Metric) error {
	// 读取/proc/meminfo中的额外信息
	memInfo, err := c.readMemInfo()
	if err != nil {
		return err
	}

	// 内存映射
	if maps, ok := memInfo["Mapped"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.mapped",
			Value: float64(maps),
			Unit:  "KB",
		})
	}

	// 共享内存
	if shmem, ok := memInfo["Shmem"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.shared",
			Value: float64(shmem),
			Unit:  "KB",
		})
	}

	// 临时内存
	if tmpfs, ok := memInfo["Tmpfs"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.tmpfs",
			Value: float64(tmpfs),
			Unit:  "KB",
		})
	}

	// SLAB内存
	if slab, ok := memInfo["Slab"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.slab",
			Value: float64(slab),
			Unit:  "KB",
		})
	}

	// 可回收SLAB
	if slabReclaimable, ok := memInfo["SReclaimable"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.slab_reclaimable",
			Value: float64(slabReclaimable),
			Unit:  "KB",
		})
	}

	// 不可回收SLAB
	if slabUnreclaimable, ok := memInfo["SUnreclaim"]; ok {
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.slab_unreclaimable",
			Value: float64(slabUnreclaimable),
			Unit:  "KB",
		})
	}

	return nil
}

// collectMemoryPressure 采集内存压力信息
func (c *MemoryCollector) collectMemoryPressure(metrics *[]models.Metric) error {
	// 读取内存压力信息
	pressureFiles := []string{
		"/proc/pressure/memory",
		"/proc/meminfo",
	}

	for _, file := range pressureFiles {
		if _, err := os.Stat(file); err != nil {
			continue
		}

		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}

		// 解析内存压力信息
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "some") || strings.Contains(line, "full") {
				// 解析压力值
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if strings.Contains(fields[0], "some") {
						if value, err := strconv.ParseFloat(fields[1], 64); err == nil {
							*metrics = append(*metrics, models.Metric{
								Name:  "memory.pressure_some",
								Value: value,
								Unit:  "%",
							})
						}
					} else if strings.Contains(fields[0], "full") {
						if value, err := strconv.ParseFloat(fields[1], 64); err == nil {
							*metrics = append(*metrics, models.Metric{
								Name:  "memory.pressure_full",
								Value: value,
								Unit:  "%",
							})
						}
					}
				}
			}
		}
		break
	}

	return nil
}

// collectPageInfo 采集内存页面信息
func (c *MemoryCollector) collectPageInfo(metrics *[]models.Metric) error {
	// 读取/proc/buddyinfo获取页面信息
	if _, err := os.Stat("/proc/buddyinfo"); err != nil {
		return nil
	}

	data, err := ioutil.ReadFile("/proc/buddyinfo")
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var totalFreePages uint64

	for _, line := range lines {
		if strings.Contains(line, "Node") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if i > 2 { // 跳过Node和zone字段
					if value, err := strconv.ParseUint(field, 10, 64); err == nil {
						totalFreePages += value
					}
				}
			}
		}
	}

	if totalFreePages > 0 {
		// 假设页面大小为4KB
		pageSizeKB := float64(totalFreePages) * 4
		*metrics = append(*metrics, models.Metric{
			Name:  "memory.free_pages",
			Value: pageSizeKB,
			Unit:  "KB",
		})
	}

	return nil
}

// collectSwapInfo 采集交换信息
func (c *MemoryCollector) collectSwapInfo(metrics *[]models.Metric) error {
	// 读取/proc/vmstat获取交换统计
	if _, err := os.Stat("/proc/vmstat"); err != nil {
		return nil
	}

	data, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	swapFields := map[string]string{
		"nr_swap_pages": "memory.swap_pages",
		"pswpin":        "memory.swap_in",
		"pswpout":       "memory.swap_out",
		"swap_faults":   "memory.swap_faults",
	}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if metricName, ok := swapFields[fields[0]]; ok {
				if value, err := strconv.ParseFloat(fields[1], 64); err == nil {
					*metrics = append(*metrics, models.Metric{
						Name:  metricName,
						Value: value,
						Unit:  "",
					})
				}
			}
		}
	}

	return nil
}

// collectNUMAMetrics 采集NUMA内存指标
func (c *MemoryCollector) collectNUMAMetrics(metrics *[]models.Metric) error {
	for _, node := range c.numaNodes {
		nodePath := filepath.Join("/sys/devices/system/node", node)

		// 读取NUMA节点内存信息
		meminfoFile := filepath.Join(nodePath, "meminfo")
		if _, err := os.Stat(meminfoFile); err != nil {
			continue
		}

		data, err := ioutil.ReadFile(meminfoFile)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		nodeNum := strings.TrimPrefix(node, "node")

		for _, line := range lines {
			if strings.Contains(line, "MemTotal") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					if value, err := strconv.ParseFloat(fields[3], 64); err == nil {
						*metrics = append(*metrics, models.Metric{
							Name:  fmt.Sprintf("memory.numa_%s.total", nodeNum),
							Value: value,
							Unit:  "KB",
						})
					}
				}
			} else if strings.Contains(line, "MemFree") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					if value, err := strconv.ParseFloat(fields[3], 64); err == nil {
						*metrics = append(*metrics, models.Metric{
							Name:  fmt.Sprintf("memory.numa_%s.free", nodeNum),
							Value: value,
							Unit:  "KB",
						})
					}
				}
			}
		}
	}

	return nil
}

// collectVMStatMetrics 采集虚拟内存统计指标
func (c *MemoryCollector) collectVMStatMetrics(metrics *[]models.Metric) error {
	if _, err := os.Stat("/proc/vmstat"); err != nil {
		return nil
	}

	data, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			fieldName := fields[0]

			// 检查是否是我们需要的字段
			for _, wantedField := range c.vmstatFields {
				if fieldName == wantedField {
					if value, err := strconv.ParseFloat(fields[1], 64); err == nil {
						*metrics = append(*metrics, models.Metric{
							Name:  fmt.Sprintf("memory.vmstat_%s", fieldName),
							Value: value,
							Unit:  "",
						})
					}
					break
				}
			}
		}
	}

	return nil
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
