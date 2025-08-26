package models

// MetricsData 指标数据结构
type MetricsData struct {
	HostID    string                 `json:"host_id"`
	Timestamp int64                  `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
	Tags      map[string]string      `json:"tags,omitempty"`
}
