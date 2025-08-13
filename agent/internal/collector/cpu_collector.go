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
)

// CPUCollector CPU指标采集器
type CPUCollector struct {
	config        *config.Config
	stopCh        chan struct{}
	prevStats     map[string]cpuStat
	mu            sync.RWMutex
	cacheDuration time.Duration
	lastCollect   time.Time
	cachedMetrics []models.Metric
	coreCount     int
	enablePerCore bool
	tempPath      string
}

// cpuStat CPU状态
type cpuStat struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
	total   uint64
}

// NewCPUCollector 创建CPU采集器
func NewCPUCollector(cfg *config.Config) *CPUCollector {
	c := &CPUCollector{
		config:        cfg,
		stopCh:        make(chan struct{}),
		prevStats:     make(map[string]cpuStat),
		cacheDuration: 1 * time.Second,
		lastCollect:   time.Time{},
		cachedMetrics: make([]models.Metric, 0),
		enablePerCore: true,
		tempPath:      "/sys/class/thermal",
	}

	// 获取CPU核心数
	c.coreCount = c.getCoreCount()

	// 从配置中读取设置
	if cfg.Collect.CacheDuration > 0 {
		c.cacheDuration = cfg.Collect.CacheDuration
	}
	c.enablePerCore = cfg.Collect.EnablePerCore

	return c
}

// getCoreCount 获取CPU核心数
func (c *CPUCollector) getCoreCount() int {
	data, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1
	}

	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}

	if count == 0 {
		count = 1
	}
	return count
}

// Start 启动采集器
func (c *CPUCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 初始化CPU状态
	_, err := c.readCPUStat()
	if err != nil {
		return fmt.Errorf("failed to initialize CPU stats: %v", err)
	}

	// 启动后台协程更新缓存
	go c.cacheUpdateLoop(ctx)

	return nil
}

// cacheUpdateLoop 缓存更新循环
func (c *CPUCollector) cacheUpdateLoop(ctx context.Context) {
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
func (c *CPUCollector) updateCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics, err := c.collectAllMetrics()
	if err != nil {
		return
	}

	c.cachedMetrics = metrics
	c.lastCollect = time.Now()
}

// collectAllMetrics 采集所有CPU指标
func (c *CPUCollector) collectAllMetrics() ([]models.Metric, error) {
	var metrics []models.Metric

	// 读取CPU使用率
	cpuUsage, err := c.collectCPUUsage()
	if err != nil {
		return nil, fmt.Errorf("failed to collect CPU usage: %v", err)
	}
	metrics = append(metrics, models.Metric{
		Name:  "cpu.usage",
		Value: cpuUsage,
		Unit:  "%",
	})

	// 读取每个核心的使用率
	if c.enablePerCore {
		coreUsages, err := c.collectPerCoreUsage()
		if err == nil {
			for core, usage := range coreUsages {
				metrics = append(metrics, models.Metric{
					Name:  fmt.Sprintf("cpu.core_%d.usage", core),
					Value: usage,
					Unit:  "%",
				})
			}
		}
	}

	// 读取CPU负载
	loadAvg1, loadAvg5, loadAvg15, err := c.collectLoadAvg()
	if err != nil {
		return nil, fmt.Errorf("failed to collect load average: %v", err)
	}
	metrics = append(metrics, models.Metric{
		Name:  "cpu.load_avg_1",
		Value: loadAvg1,
		Unit:  "",
	})
	metrics = append(metrics, models.Metric{
		Name:  "cpu.load_avg_5",
		Value: loadAvg5,
		Unit:  "",
	})
	metrics = append(metrics, models.Metric{
		Name:  "cpu.load_avg_15",
		Value: loadAvg15,
		Unit:  "",
	})

	// 读取CPU频率
	frequency, err := c.collectCPUFrequency()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.frequency",
			Value: frequency,
			Unit:  "MHz",
		})
	}

	// 读取CPU上下文切换
	ctxSwitches, err := c.collectContextSwitches()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.context_switches",
			Value: ctxSwitches,
			Unit:  "",
		})
	}

	// 读取CPU中断
	interrupts, err := c.collectInterrupts()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.interrupts",
			Value: interrupts,
			Unit:  "",
		})
	}

	// 读取CPU温度（如果支持）
	temperature, err := c.collectCPUTemperature()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.temperature",
			Value: temperature,
			Unit:  "°C",
		})
	}

	// 读取进程信息
	processes, threads, err := c.collectProcessInfo()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.processes",
			Value: processes,
			Unit:  "",
		})
		metrics = append(metrics, models.Metric{
			Name:  "cpu.threads",
			Value: threads,
			Unit:  "",
		})
	}

	return metrics, nil
}

// Stop 停止采集器
func (c *CPUCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集CPU指标
func (c *CPUCollector) Collect() ([]models.Metric, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 如果缓存未过期，直接返回缓存数据
	if time.Since(c.lastCollect) < c.cacheDuration && len(c.cachedMetrics) > 0 {
		return c.cachedMetrics, nil
	}

	// 否则重新采集
	metrics, err := c.collectAllMetrics()
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// readCPUStat 读取CPU状态
func (c *CPUCollector) readCPUStat() (map[string]cpuStat, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	stats := make(map[string]cpuStat)

	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		cpuName := fields[0]
		user, _ := strconv.ParseUint(fields[1], 10, 64)
		nice, _ := strconv.ParseUint(fields[2], 10, 64)
		system, _ := strconv.ParseUint(fields[3], 10, 64)
		idle, _ := strconv.ParseUint(fields[4], 10, 64)
		iowait, _ := strconv.ParseUint(fields[5], 10, 64)
		irq, _ := strconv.ParseUint(fields[6], 10, 64)
		softirq, _ := strconv.ParseUint(fields[7], 10, 64)
		steal := uint64(0)
		if len(fields) > 8 {
			steal, _ = strconv.ParseUint(fields[8], 10, 64)
		}

		total := user + nice + system + idle + iowait + irq + softirq + steal

		stats[cpuName] = cpuStat{
			user:    user,
			nice:    nice,
			system:  system,
			idle:    idle,
			iowait:  iowait,
			irq:     irq,
			softirq: softirq,
			steal:   steal,
			total:   total,
		}
	}

	return stats, nil
}

// collectCPUUsage 采集CPU使用率
func (c *CPUCollector) collectCPUUsage() (float64, error) {
	currentStats, err := c.readCPUStat()
	if err != nil {
		return 0, err
	}

	if len(c.prevStats) == 0 {
		c.prevStats = currentStats
		time.Sleep(100 * time.Millisecond)
		return c.collectCPUUsage()
	}

	prevStat, ok := c.prevStats["cpu"]
	if !ok {
		return 0, fmt.Errorf("previous CPU stat not found")
	}

	currentStat, ok := currentStats["cpu"]
	if !ok {
		return 0, fmt.Errorf("current CPU stat not found")
	}

	totalDiff := currentStat.total - prevStat.total
	if totalDiff == 0 {
		return 0, nil
	}

	idleDiff := currentStat.idle - prevStat.idle
	usagePercent := 100.0 * (1.0 - float64(idleDiff)/float64(totalDiff))

	// 更新上一次的状态
	c.prevStats = currentStats

	return usagePercent, nil
}

// collectLoadAvg 采集负载均值
func (c *CPUCollector) collectLoadAvg() (float64, float64, float64, error) {
	data, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

// collectPerCoreUsage 采集每个CPU核心的使用率
func (c *CPUCollector) collectPerCoreUsage() (map[int]float64, error) {
	currentStats, err := c.readCPUStat()
	if err != nil {
		return nil, err
	}

	if len(c.prevStats) == 0 {
		c.prevStats = currentStats
		time.Sleep(100 * time.Millisecond)
		return c.collectPerCoreUsage()
	}

	coreUsages := make(map[int]float64)

	for i := 0; i < c.coreCount; i++ {
		coreName := fmt.Sprintf("cpu%d", i)
		prevStat, ok := c.prevStats[coreName]
		if !ok {
			continue
		}

		currentStat, ok := currentStats[coreName]
		if !ok {
			continue
		}

		totalDiff := currentStat.total - prevStat.total
		if totalDiff == 0 {
			continue
		}

		idleDiff := currentStat.idle - prevStat.idle
		usagePercent := 100.0 * (1.0 - float64(idleDiff)/float64(totalDiff))
		coreUsages[i] = usagePercent
	}

	c.prevStats = currentStats
	return coreUsages, nil
}

// collectCPUFrequency 采集CPU频率
func (c *CPUCollector) collectCPUFrequency() (float64, error) {
	// 尝试从cpufreq读取频率
	for i := 0; i < c.coreCount; i++ {
		freqFile := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i)
		data, err := ioutil.ReadFile(freqFile)
		if err != nil {
			continue
		}

		freq, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}

		// 频率以KHz为单位，转换为MHz
		return freq / 1000.0, nil
	}

	return 0, fmt.Errorf("CPU frequency not available")
}

// collectContextSwitches 采集上下文切换次数
func (c *CPUCollector) collectContextSwitches() (float64, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ctxt") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				value, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return 0, err
				}
				return value, nil
			}
		}
	}

	return 0, fmt.Errorf("context switches not available")
}

// collectInterrupts 采集中断次数
func (c *CPUCollector) collectInterrupts() (float64, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "intr") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				value, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return 0, err
				}
				return value, nil
			}
		}
	}

	return 0, fmt.Errorf("interrupts not available")
}

// collectProcessInfo 采集进程和线程信息
func (c *CPUCollector) collectProcessInfo() (float64, float64, error) {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	var processes, threads float64

	for _, line := range lines {
		if strings.HasPrefix(line, "processes") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				processes, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					processes = 0
				}
			}
		} else if strings.HasPrefix(line, "procs_running") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				threads, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					threads = 0
				}
			}
		}
	}

	return processes, threads, nil
}

// collectCPUTemperature 采集CPU温度
func (c *CPUCollector) collectCPUTemperature() (float64, error) {
	// 尝试从thermal_zone读取温度
	files, err := ioutil.ReadDir(c.tempPath)
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "thermal_zone") {
			typeFile := filepath.Join(c.tempPath, file.Name(), "type")
			tempFile := filepath.Join(c.tempPath, file.Name(), "temp")

			// 读取类型
			typeData, err := ioutil.ReadFile(typeFile)
			if err != nil {
				continue
			}

			// 如果是CPU相关的传感器
			if strings.Contains(strings.ToLower(string(typeData)), "cpu") {
				tempData, err := ioutil.ReadFile(tempFile)
				if err != nil {
					continue
				}

				temp, err := strconv.ParseInt(strings.TrimSpace(string(tempData)), 10, 64)
				if err != nil {
					continue
				}

				// 温度通常以毫摄氏度为单位，需要转换为摄氏度
				return float64(temp) / 1000.0, nil
			}
		}
	}

	// 尝试其他温度传感器路径
	tempPaths := []string{
		"/sys/class/hwmon/hwmon0/temp1_input",
		"/sys/class/hwmon/hwmon1/temp1_input",
		"/sys/class/hwmon/hwmon2/temp1_input",
		"/sys/devices/platform/coretemp.0/temp1_input",
	}

	for _, path := range tempPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				continue
			}

			temp, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			if err != nil {
				continue
			}

			return float64(temp) / 1000.0, nil
		}
	}

	return 0, fmt.Errorf("CPU temperature not available")
}
