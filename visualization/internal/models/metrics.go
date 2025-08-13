package models

import (
	"time"
)

// Metric 单个指标
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// MetricsData 指标数据
type MetricsData struct {
	HostID    string    `json:"host_id"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   []Metric  `json:"metrics"`
}

// CPUMetric CPU指标
type CPUMetric struct {
	Usage       float64 `json:"usage"`       // CPU使用率
	Temperature float64 `json:"temperature"` // CPU温度
	LoadAvg1    float64 `json:"load_avg_1"`  // 1分钟平均负载
	LoadAvg5    float64 `json:"load_avg_5"`  // 5分钟平均负载
	LoadAvg15   float64 `json:"load_avg_15"` // 15分钟平均负载
}

// MemoryMetric 内存指标
type MemoryMetric struct {
	Total     float64 `json:"total"`      // 总内存
	Used      float64 `json:"used"`       // 已使用内存
	Free      float64 `json:"free"`       // 空闲内存
	Available float64 `json:"available"`  // 可用内存
	Buffers   float64 `json:"buffers"`    // 缓冲区
	Cached    float64 `json:"cached"`     // 缓存
	SwapTotal float64 `json:"swap_total"` // 总交换空间
	SwapUsed  float64 `json:"swap_used"`  // 已使用交换空间
	SwapFree  float64 `json:"swap_free"`  // 空闲交换空间
}

// NetworkMetric 网络指标
type NetworkMetric struct {
	Interface      string  `json:"interface"`       // 网络接口
	RxBytes        float64 `json:"rx_bytes"`        // 接收字节数
	TxBytes        float64 `json:"tx_bytes"`        // 发送字节数
	RxPackets      float64 `json:"rx_packets"`      // 接收数据包数
	TxPackets      float64 `json:"tx_packets"`      // 发送数据包数
	RxErrors       float64 `json:"rx_errors"`       // 接收错误数
	TxErrors       float64 `json:"tx_errors"`       // 发送错误数
	RxDropped      float64 `json:"rx_dropped"`      // 接收丢弃数
	TxDropped      float64 `json:"tx_dropped"`      // 发送丢弃数
	RxBytesRate    float64 `json:"rx_bytes_rate"`   // 接收速率
	TxBytesRate    float64 `json:"tx_bytes_rate"`   // 发送速率
	TcpConnections int     `json:"tcp_connections"` // TCP连接数
}

// DiskMetric 磁盘指标
type DiskMetric struct {
	Device       string  `json:"device"`        // 设备名
	MountPoint   string  `json:"mount_point"`   // 挂载点
	Total        float64 `json:"total"`         // 总空间
	Used         float64 `json:"used"`          // 已使用空间
	Free         float64 `json:"free"`          // 空闲空间
	UsageRate    float64 `json:"usage_rate"`    // 使用率
	ReadOps      float64 `json:"read_ops"`      // 读操作次数
	WriteOps     float64 `json:"write_ops"`     // 写操作次数
	ReadBytes    float64 `json:"read_bytes"`    // 读取字节数
	WriteBytes   float64 `json:"write_bytes"`   // 写入字节数
	ReadTime     float64 `json:"read_time"`     // 读取时间
	WriteTime    float64 `json:"write_time"`    // 写入时间
	IoTime       float64 `json:"io_time"`       // I/O时间
	WeightedTime float64 `json:"weighted_time"` // 加权I/O时间
}

// HostInfo 主机信息
type HostInfo struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}
