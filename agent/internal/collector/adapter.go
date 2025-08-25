package collector

import (
	"github.com/han-fei/monitor/agent/internal/models"
	"github.com/han-fei/monitor/pkg/interfaces"
)

// ConvertMetrics 将内部 Metric 转换为接口 Metric
func ConvertMetrics(internalMetrics []models.Metric) []interfaces.Metric {
	result := make([]interfaces.Metric, len(internalMetrics))
	for i, m := range internalMetrics {
		result[i] = interfaces.Metric{
			Name:  m.Name,
			Value: m.Value,
			Labels: map[string]string{
				"unit": m.Unit,
			},
			Timestamp: 0, // 这里可以根据需要设置时间戳
		}
	}
	return result
}

// ConvertBackMetrics 将接口 Metric 转换为内部 Metric
func ConvertBackMetrics(interfaceMetrics []interfaces.Metric) []models.Metric {
	result := make([]models.Metric, len(interfaceMetrics))
	for i, m := range interfaceMetrics {
		unit := ""
		if m.Labels != nil {
			if u, ok := m.Labels["unit"]; ok {
				unit = u
			}
		}

		result[i] = models.Metric{
			Name:  m.Name,
			Value: m.Value,
			Unit:  unit,
		}
	}
	return result
}
