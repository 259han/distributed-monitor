package collector

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
)

// CPUCollector CPU指标采集器
type CPUCollector struct {
	config    *config.Config
	stopCh    chan struct{}
	prevStats map[string]cpuStat
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
	return &CPUCollector{
		config:    cfg,
		stopCh:    make(chan struct{}),
		prevStats: make(map[string]cpuStat),
	}
}

// Start 启动采集器
func (c *CPUCollector) Start(ctx context.Context) error {
	// 初始化CPU状态
	_, err := c.readCPUStat()
	return err
}

// Stop 停止采集器
func (c *CPUCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集CPU指标
func (c *CPUCollector) Collect() ([]models.Metric, error) {
	var metrics []models.Metric

	// 读取CPU使用率
	cpuUsage, err := c.collectCPUUsage()
	if err != nil {
		return nil, err
	}
	metrics = append(metrics, models.Metric{
		Name:  "cpu.usage",
		Value: cpuUsage,
		Unit:  "%",
	})

	// 读取CPU负载
	loadAvg1, loadAvg5, loadAvg15, err := c.collectLoadAvg()
	if err != nil {
		return nil, err
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

	// 读取CPU温度（如果支持）
	temperature, err := c.collectCPUTemperature()
	if err == nil {
		metrics = append(metrics, models.Metric{
			Name:  "cpu.temperature",
			Value: temperature,
			Unit:  "°C",
		})
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

// collectCPUTemperature 采集CPU温度
func (c *CPUCollector) collectCPUTemperature() (float64, error) {
	// 尝试从thermal_zone读取温度
	files, err := ioutil.ReadDir("/sys/class/thermal")
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "thermal_zone") {
			typeFile := "/sys/class/thermal/" + file.Name() + "/type"
			tempFile := "/sys/class/thermal/" + file.Name() + "/temp"

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

	return 0, fmt.Errorf("CPU temperature not available")
} 