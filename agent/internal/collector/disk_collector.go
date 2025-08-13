package collector

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
)

// DiskCollector 磁盘指标采集器
type DiskCollector struct {
	config *config.Config
	stopCh chan struct{}
}

// NewDiskCollector 创建磁盘采集器
func NewDiskCollector(cfg *config.Config) *DiskCollector {
	return &DiskCollector{
		config: cfg,
		stopCh: make(chan struct{}),
	}
}

// Start 启动采集器
func (c *DiskCollector) Start(ctx context.Context) error {
	return nil
}

// Stop 停止采集器
func (c *DiskCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集磁盘指标
func (c *DiskCollector) Collect() ([]models.Metric, error) {
	var metrics []models.Metric

	// 获取磁盘使用情况
	diskUsage, err := c.getDiskUsage()
	if err != nil {
		return nil, err
	}

	for _, disk := range diskUsage {
		// 总容量
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.total.%s", disk.MountPoint),
			Value: float64(disk.Total),
			Unit:  "B",
		})

		// 已使用
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.used.%s", disk.MountPoint),
			Value: float64(disk.Used),
			Unit:  "B",
		})

		// 可用
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.free.%s", disk.MountPoint),
			Value: float64(disk.Free),
			Unit:  "B",
		})

		// 使用率
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.usage.%s", disk.MountPoint),
			Value: disk.UsageRate,
			Unit:  "%",
		})
	}

	// 获取磁盘IO统计
	diskStats, err := c.getDiskStats()
	if err != nil {
		return nil, err
	}

	for device, stat := range diskStats {
		// 读操作次数
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.reads.%s", device),
			Value: float64(stat.ReadOps),
			Unit:  "ops",
		})

		// 写操作次数
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.writes.%s", device),
			Value: float64(stat.WriteOps),
			Unit:  "ops",
		})

		// 读取字节数
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.read_bytes.%s", device),
			Value: float64(stat.ReadBytes),
			Unit:  "B",
		})

		// 写入字节数
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.write_bytes.%s", device),
			Value: float64(stat.WriteBytes),
			Unit:  "B",
		})

		// IO时间
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("disk.io_time.%s", device),
			Value: float64(stat.IOTime),
			Unit:  "ms",
		})
	}

	return metrics, nil
}

// DiskUsage 磁盘使用情况
type DiskUsage struct {
	Device     string  // 设备名称
	MountPoint string  // 挂载点
	Total      uint64  // 总容量（字节）
	Used       uint64  // 已使用（字节）
	Free       uint64  // 空闲（字节）
	UsageRate  float64 // 使用率（百分比）
}

// getDiskUsage 获取磁盘使用情况
func (c *DiskCollector) getDiskUsage() ([]DiskUsage, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var result []DiskUsage
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// 跳过一些特殊的文件系统
		if fsType == "proc" || fsType == "sysfs" || fsType == "devpts" ||
			fsType == "tmpfs" || fsType == "devtmpfs" || strings.HasPrefix(mountPoint, "/sys") ||
			strings.HasPrefix(mountPoint, "/proc") || strings.HasPrefix(mountPoint, "/dev") {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &stat); err != nil {
			continue
		}

		// 计算磁盘空间
		blockSize := uint64(stat.Bsize)
		total := blockSize * stat.Blocks
		free := blockSize * stat.Bfree
		available := blockSize * stat.Bavail
		used := total - free

		// 计算使用率
		var usageRate float64
		if total > 0 {
			usageRate = float64(used) / float64(total) * 100
		}

		result = append(result, DiskUsage{
			Device:     device,
			MountPoint: mountPoint,
			Total:      total,
			Used:       used,
			Free:       available,
			UsageRate:  usageRate,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// DiskStat 磁盘IO统计
type DiskStat struct {
	ReadOps    uint64 // 读操作次数
	WriteOps   uint64 // 写操作次数
	ReadBytes  uint64 // 读取字节数
	WriteBytes uint64 // 写入字节数
	IOTime     uint64 // IO时间（毫秒）
}

// getDiskStats 获取磁盘IO统计
func (c *DiskCollector) getDiskStats() (map[string]DiskStat, error) {
	data, err := ioutil.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	result := make(map[string]DiskStat)

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		// 设备名称
		device := fields[2]

		// 跳过循环设备和分区
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "ram") ||
			strings.Contains(device, "dm-") {
			continue
		}

		// 解析统计数据
		readOps, _ := strconv.ParseUint(fields[3], 10, 64)
		writeOps, _ := strconv.ParseUint(fields[7], 10, 64)

		// 扇区大小通常为512字节
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		readBytes := readSectors * 512
		writeBytes := writeSectors * 512

		// IO时间（毫秒）
		ioTime, _ := strconv.ParseUint(fields[12], 10, 64)

		result[device] = DiskStat{
			ReadOps:    readOps,
			WriteOps:   writeOps,
			ReadBytes:  readBytes,
			WriteBytes: writeBytes,
			IOTime:     ioTime,
		}
	}

	return result, nil
}
