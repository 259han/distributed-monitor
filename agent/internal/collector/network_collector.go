package collector

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/han-fei/monitor/agent/internal/config"
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/han-fei/monitor/pkg/algorithm"
	"github.com/han-fei/monitor/pkg/interfaces"
)

// NetworkCollector 网络指标采集器
type NetworkCollector struct {
	config         *config.Config
	stopCh         chan struct{}
	prevStats      map[string]NetworkStat
	prevTime       time.Time
	slidingWindows map[string]*algorithm.SlidingWindow
}

// NetworkStat 网络接口统计
type NetworkStat struct {
	Interface   string // 接口名称
	RxBytes     uint64 // 接收字节数
	TxBytes     uint64 // 发送字节数
	RxPackets   uint64 // 接收包数
	TxPackets   uint64 // 发送包数
	RxErrors    uint64 // 接收错误数
	TxErrors    uint64 // 发送错误数
	RxDropped   uint64 // 接收丢包数
	TxDropped   uint64 // 发送丢包数
	Connections uint64 // 连接数
}

// NewNetworkCollector 创建网络采集器
func NewNetworkCollector(cfg *config.Config) *NetworkCollector {
	return &NetworkCollector{
		config:         cfg,
		stopCh:         make(chan struct{}),
		prevStats:      make(map[string]NetworkStat),
		prevTime:       time.Now(),
		slidingWindows: make(map[string]*algorithm.SlidingWindow),
	}
}

// Start 启动采集器
func (c *NetworkCollector) Start(ctx context.Context) error {
	// 初始化网络统计
	stats, err := c.readNetworkStats()
	if err != nil {
		return err
	}

	c.prevStats = stats
	c.prevTime = time.Now()

	// 为每个接口创建滑动窗口
	for iface := range stats {
		c.slidingWindows[iface+"_rx"] = algorithm.NewSlidingWindow(10, 60*time.Second)
		c.slidingWindows[iface+"_tx"] = algorithm.NewSlidingWindow(10, 60*time.Second)
	}

	return nil
}

// Stop 停止采集器
func (c *NetworkCollector) Stop() error {
	close(c.stopCh)
	return nil
}

// Collect 采集网络指标
func (c *NetworkCollector) Collect() ([]interfaces.Metric, error) {
	var metrics []models.Metric

	// 读取网络统计
	currentStats, err := c.readNetworkStats()
	if err != nil {
		return nil, err
	}

	// 当前时间
	now := time.Now()
	// 时间差（秒）
	timeDiff := now.Sub(c.prevTime).Seconds()

	if timeDiff <= 0 {
		timeDiff = 1
	}

	// 计算每秒速率
	for iface, stat := range currentStats {
		prevStat, ok := c.prevStats[iface]
		if !ok {
			continue
		}

		// 计算带宽（字节/秒）
		rxBandwidth := float64(stat.RxBytes-prevStat.RxBytes) / timeDiff
		txBandwidth := float64(stat.TxBytes-prevStat.TxBytes) / timeDiff

		// 更新滑动窗口
		if window, ok := c.slidingWindows[iface+"_rx"]; ok {
			window.Add(rxBandwidth)
		}
		if window, ok := c.slidingWindows[iface+"_tx"]; ok {
			window.Add(txBandwidth)
		}

		// 获取平均带宽
		var rxAvgBandwidth, txAvgBandwidth float64
		if window, ok := c.slidingWindows[iface+"_rx"]; ok {
			rxAvgBandwidth = window.Average()
		} else {
			rxAvgBandwidth = rxBandwidth
		}
		if window, ok := c.slidingWindows[iface+"_tx"]; ok {
			txAvgBandwidth = window.Average()
		} else {
			txAvgBandwidth = txBandwidth
		}

		// 接收带宽（字节/秒）
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.rx_bytes.%s", iface),
			Value: rxBandwidth,
			Unit:  "B/s",
		})

		// 发送带宽（字节/秒）
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.tx_bytes.%s", iface),
			Value: txBandwidth,
			Unit:  "B/s",
		})

		// 平均接收带宽（字节/秒）
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.rx_bytes_avg.%s", iface),
			Value: rxAvgBandwidth,
			Unit:  "B/s",
		})

		// 平均发送带宽（字节/秒）
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.tx_bytes_avg.%s", iface),
			Value: txAvgBandwidth,
			Unit:  "B/s",
		})

		// 接收包速率（包/秒）
		rxPacketRate := float64(stat.RxPackets-prevStat.RxPackets) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.rx_packets.%s", iface),
			Value: rxPacketRate,
			Unit:  "pps",
		})

		// 发送包速率（包/秒）
		txPacketRate := float64(stat.TxPackets-prevStat.TxPackets) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.tx_packets.%s", iface),
			Value: txPacketRate,
			Unit:  "pps",
		})

		// 接收错误率（错误/秒）
		rxErrorRate := float64(stat.RxErrors-prevStat.RxErrors) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.rx_errors.%s", iface),
			Value: rxErrorRate,
			Unit:  "errors/s",
		})

		// 发送错误率（错误/秒）
		txErrorRate := float64(stat.TxErrors-prevStat.TxErrors) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.tx_errors.%s", iface),
			Value: txErrorRate,
			Unit:  "errors/s",
		})

		// 接收丢包率（丢包/秒）
		rxDropRate := float64(stat.RxDropped-prevStat.RxDropped) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.rx_dropped.%s", iface),
			Value: rxDropRate,
			Unit:  "drops/s",
		})

		// 发送丢包率（丢包/秒）
		txDropRate := float64(stat.TxDropped-prevStat.TxDropped) / timeDiff
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.tx_dropped.%s", iface),
			Value: txDropRate,
			Unit:  "drops/s",
		})

		// 连接数
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.connections.%s", iface),
			Value: float64(stat.Connections),
			Unit:  "conn",
		})

		// 总带宽（接收+发送）
		totalBandwidth := rxBandwidth + txBandwidth
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.bandwidth.%s", iface),
			Value: totalBandwidth,
			Unit:  "B/s",
		})

		// 总平均带宽
		totalAvgBandwidth := rxAvgBandwidth + txAvgBandwidth
		metrics = append(metrics, models.Metric{
			Name:  fmt.Sprintf("network.bandwidth_avg.%s", iface),
			Value: totalAvgBandwidth,
			Unit:  "B/s",
		})
	}

	// 更新上一次的统计和时间
	c.prevStats = currentStats
	c.prevTime = now

	return ConvertMetrics(metrics), nil
}

// readNetworkStats 读取网络统计
func (c *NetworkCollector) readNetworkStats() (map[string]NetworkStat, error) {
	// 读取/proc/net/dev获取网络接口统计
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]NetworkStat)
	scanner := bufio.NewScanner(file)

	// 跳过前两行（标题行）
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(strings.Replace(line, ":", " ", 1))
		if len(fields) < 17 {
			continue
		}

		iface := fields[0]

		// 跳过lo接口
		if iface == "lo" {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[2], 10, 64)
		rxErrors, _ := strconv.ParseUint(fields[3], 10, 64)
		rxDropped, _ := strconv.ParseUint(fields[4], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[9], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[10], 10, 64)
		txErrors, _ := strconv.ParseUint(fields[11], 10, 64)
		txDropped, _ := strconv.ParseUint(fields[12], 10, 64)

		// 获取连接数
		connections, err := c.getConnectionCount(iface)
		if err != nil {
			connections = 0
		}

		result[iface] = NetworkStat{
			Interface:   iface,
			RxBytes:     rxBytes,
			TxBytes:     txBytes,
			RxPackets:   rxPackets,
			TxPackets:   txPackets,
			RxErrors:    rxErrors,
			TxErrors:    txErrors,
			RxDropped:   rxDropped,
			TxDropped:   txDropped,
			Connections: connections,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// getConnectionCount 获取指定接口的连接数
func (c *NetworkCollector) getConnectionCount(iface string) (uint64, error) {
	// 读取/proc/net/sockstat获取TCP连接数
	data, err := ioutil.ReadFile("/proc/net/sockstat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "TCP:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[1] == "inuse" {
				count, err := strconv.ParseUint(fields[2], 10, 64)
				if err != nil {
					return 0, err
				}
				return count, nil
			}
		}
	}

	return 0, fmt.Errorf("no TCP connection information found")
}
