package analysis

import (
	"fmt"
	"math"
	"sort"

	"github.com/han-fei/monitor/visualization/internal/models"
)

// AvgAggregator 平均值聚合器
type AvgAggregator struct{}

func (a *AvgAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) == 0 {
		return 0.0
	}

	var sum float64
	var count int

	for _, d := range data {
		for _, metric := range d.Metrics {
			sum += metric.Value
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return sum / float64(count)
}

func (a *AvgAggregator) GetType() string {
	return "avg"
}

// SumAggregator 求和聚合器
type SumAggregator struct{}

func (s *SumAggregator) Aggregate(data []*models.MetricsData) interface{} {
	var sum float64

	for _, d := range data {
		for _, metric := range d.Metrics {
			sum += metric.Value
		}
	}

	return sum
}

func (s *SumAggregator) GetType() string {
	return "sum"
}

// MinAggregator 最小值聚合器
type MinAggregator struct{}

func (m *MinAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) == 0 {
		return 0.0
	}

	min := math.MaxFloat64

	for _, d := range data {
		for _, metric := range d.Metrics {
			if metric.Value < min {
				min = metric.Value
			}
		}
	}

	if min == math.MaxFloat64 {
		return 0.0
	}

	return min
}

func (m *MinAggregator) GetType() string {
	return "min"
}

// MaxAggregator 最大值聚合器
type MaxAggregator struct{}

func (m *MaxAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) == 0 {
		return 0.0
	}

	max := -math.MaxFloat64

	for _, d := range data {
		for _, metric := range d.Metrics {
			if metric.Value > max {
				max = metric.Value
			}
		}
	}

	if max == -math.MaxFloat64 {
		return 0.0
	}

	return max
}

func (m *MaxAggregator) GetType() string {
	return "max"
}

// CountAggregator 计数聚合器
type CountAggregator struct{}

func (c *CountAggregator) Aggregate(data []*models.MetricsData) interface{} {
	var count int

	for _, d := range data {
		count += len(d.Metrics)
	}

	return count
}

func (c *CountAggregator) GetType() string {
	return "count"
}

// RateAggregator 速率聚合器
type RateAggregator struct{}

func (r *RateAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) < 2 {
		return 0.0
	}

	// 按时间排序
	sort.Slice(data, func(i, j int) bool {
		return data[i].Timestamp.Before(data[j].Timestamp)
	})

	// 计算时间差
	timeDiff := data[len(data)-1].Timestamp.Sub(data[0].Timestamp).Seconds()
	if timeDiff <= 0 {
		return 0.0
	}

	// 计算值的变化
	var totalValue float64
	var prevValues map[string]float64

	for i, d := range data {
		if i == 0 {
			// 记录初始值
			prevValues = make(map[string]float64)
			for _, metric := range d.Metrics {
				prevValues[metric.Name] = metric.Value
			}
			continue
		}

		// 计算变化量
		currentValues := make(map[string]float64)
		for _, metric := range d.Metrics {
			currentValues[metric.Name] = metric.Value
		}

		for name, current := range currentValues {
			if prev, exists := prevValues[name]; exists {
				if current > prev {
					totalValue += current - prev
				}
			}
		}

		// 更新前值
		prevValues = currentValues
	}

	return totalValue / timeDiff
}

func (r *RateAggregator) GetType() string {
	return "rate"
}

// PercentileAggregator 百分位数聚合器
type PercentileAggregator struct {
	Percentile float64
}

func (p *PercentileAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) == 0 {
		return 0.0
	}

	// 提取所有值
	var values []float64
	for _, d := range data {
		for _, metric := range d.Metrics {
			values = append(values, metric.Value)
		}
	}

	if len(values) == 0 {
		return 0.0
	}

	// 排序
	sort.Float64s(values)

	// 计算百分位数
	index := p.Percentile / 100.0 * float64(len(values)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return values[lower]
	}

	// 线性插值
	weight := index - float64(lower)
	return values[lower]*(1-weight) + values[upper]*weight
}

func (p *PercentileAggregator) GetType() string {
	return fmt.Sprintf("p%.0f", p.Percentile)
}

// StdDevAggregator 标准差聚合器
type StdDevAggregator struct{}

func (s *StdDevAggregator) Aggregate(data []*models.MetricsData) interface{} {
	if len(data) == 0 {
		return 0.0
	}

	// 计算平均值
	var sum float64
	var count int

	for _, d := range data {
		for _, metric := range d.Metrics {
			sum += metric.Value
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	mean := sum / float64(count)

	// 计算方差
	var variance float64
	for _, d := range data {
		for _, metric := range d.Metrics {
			diff := metric.Value - mean
			variance += diff * diff
		}
	}

	variance /= float64(count)

	// 返回标准差
	return math.Sqrt(variance)
}

func (s *StdDevAggregator) GetType() string {
	return "stddev"
}