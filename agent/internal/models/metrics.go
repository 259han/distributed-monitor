package models

import (
	"time"
)

// Metric 表示单个监控指标
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// MetricsData 表示一组监控数据
type MetricsData struct {
	HostID    string    `json:"host_id"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   []Metric  `json:"metrics"`
}

// CPUMetric CPU指标
type CPUMetric struct {
	Usage       float64 `json:"usage"`       // CPU使用率百分比
	Temperature float64 `json:"temperature"` // CPU温度（摄氏度）
	LoadAvg1    float64 `json:"load_avg_1"`  // 1分钟平均负载
	LoadAvg5    float64 `json:"load_avg_5"`  // 5分钟平均负载
	LoadAvg15   float64 `json:"load_avg_15"` // 15分钟平均负载
}

// MemoryMetric 内存指标
type MemoryMetric struct {
	Total     uint64  `json:"total"`     // 总内存（字节）
	Used      uint64  `json:"used"`      // 已使用内存（字节）
	Free      uint64  `json:"free"`      // 空闲内存（字节）
	UsageRate float64 `json:"usageRate"` // 使用率百分比
}

// NetworkMetric 网络指标
type NetworkMetric struct {
	Interface   string  `json:"interface"`   // 网络接口名称
	RxBytes     uint64  `json:"rx_bytes"`    // 接收字节数
	TxBytes     uint64  `json:"tx_bytes"`    // 发送字节数
	RxPackets   uint64  `json:"rx_packets"`  // 接收包数
	TxPackets   uint64  `json:"tx_packets"`  // 发送包数
	RxErrors    uint64  `json:"rx_errors"`   // 接收错误数
	TxErrors    uint64  `json:"tx_errors"`   // 发送错误数
	RxDropped   uint64  `json:"rx_dropped"`  // 接收丢包数
	TxDropped   uint64  `json:"tx_dropped"`  // 发送丢包数
	Bandwidth   float64 `json:"bandwidth"`   // 带宽使用率（Mbps）
	Connections uint64  `json:"connections"` // 连接数
}

// DiskMetric 磁盘指标
type DiskMetric struct {
	Device     string  `json:"device"`     // 设备名称
	MountPoint string  `json:"mountPoint"` // 挂载点
	Total      uint64  `json:"total"`      // 总容量（字节）
	Used       uint64  `json:"used"`       // 已使用（字节）
	Free       uint64  `json:"free"`       // 空闲（字节）
	UsageRate  float64 `json:"usageRate"`  // 使用率百分比
	ReadOps    uint64  `json:"readOps"`    // 读操作次数
	WriteOps   uint64  `json:"writeOps"`   // 写操作次数
	ReadBytes  uint64  `json:"readBytes"`  // 读取字节数
	WriteBytes uint64  `json:"writeBytes"` // 写入字节数
	IOTime     uint64  `json:"ioTime"`     // IO时间（毫秒）
}

// HostInfo 主机信息
type HostInfo struct {
	ID          string `json:"id"`          // 主机唯一标识
	Hostname    string `json:"hostname"`    // 主机名
	IP          string `json:"ip"`          // IP地址
	OS          string `json:"os"`          // 操作系统
	CPUCores    int    `json:"cpuCores"`    // CPU核心数
	TotalMemory uint64 `json:"totalMemory"` // 总内存（字节）
} 