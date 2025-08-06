package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/han-fei/monitor/visualization/internal/models"
)

// Analyzer 数据分析器
type Analyzer struct {
	dataCache    map[string][]*models.MetricsData
	cacheMutex   sync.RWMutex
	aggregators  map[string]Aggregator
	alertRules   map[string]*AlertRule
	topkAnalyzer *TopKAnalyzer
}

// Aggregator 聚合器接口
type Aggregator interface {
	Aggregate(data []*models.MetricsData) interface{}
	GetType() string
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Metric      string                 `json:"metric"`
	Condition   string                 `json:"condition"`
	Threshold   float64                `json:"threshold"`
	Duration    time.Duration          `json:"duration"`
	Severity    string                 `json:"severity"`
	Enabled     bool                   `json:"enabled"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Alert 告警
type Alert struct {
	ID          string                 `json:"id"`
	RuleID      string                 `json:"rule_id"`
	RuleName    string                 `json:"rule_name"`
	HostID      string                 `json:"host_id"`
	Metric      string                 `json:"metric"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold"`
	Severity    string                 `json:"severity"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AggregationResult 聚合结果
type AggregationResult struct {
	Type        string      `json:"type"`
	TimeRange   TimeRange   `json:"time_range"`
	HostID      string      `json:"host_id,omitempty"`
	Metric      string      `json:"metric,omitempty"`
	Data        interface{} `json:"data"`
	Timestamp   time.Time   `json:"timestamp"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	Direction    string  `json:"direction"`    // "up", "down", "stable"
	Slope        float64 `json:"slope"`        // 斜率
	Correlation  float64 `json:"correlation"`  // 相关系数
	Prediction   float64 `json:"prediction"`   // 预测值
	Confidence   float64 `json:"confidence"`   // 置信度
}

// AnomalyDetection 异常检测结果
type AnomalyDetection struct {
	IsAnomaly    bool    `json:"is_anomaly"`
	AnomalyScore float64 `json:"anomaly_score"`
	Threshold    float64 `json:"threshold"`
	Method       string  `json:"method"`
	Description  string  `json:"description"`
}

// NewAnalyzer 创建数据分析器
func NewAnalyzer() *Analyzer {
	analyzer := &Analyzer{
		dataCache:   make(map[string][]*models.MetricsData),
		aggregators: make(map[string]Aggregator),
		alertRules:  make(map[string]*AlertRule),
		topkAnalyzer: NewTopKAnalyzer(),
	}

	// 注册默认聚合器
	analyzer.RegisterAggregator(&AvgAggregator{})
	analyzer.RegisterAggregator(&SumAggregator{})
	analyzer.RegisterAggregator(&MinAggregator{})
	analyzer.RegisterAggregator(&MaxAggregator{})
	analyzer.RegisterAggregator(&CountAggregator{})
	analyzer.RegisterAggregator(&RateAggregator{})
	analyzer.RegisterAggregator(&PercentileAggregator{Percentile: 95})
	analyzer.RegisterAggregator(&StdDevAggregator{})

	// 注册默认告警规则
	analyzer.RegisterDefaultAlertRules()

	return analyzer
}

// AddData 添加数据到缓存
func (a *Analyzer) AddData(data *models.MetricsData) {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()

	hostID := data.HostID
	a.dataCache[hostID] = append(a.dataCache[hostID], data)

	// 保持缓存大小，防止内存泄漏
	maxCacheSize := 1000
	if len(a.dataCache[hostID]) > maxCacheSize {
		a.dataCache[hostID] = a.dataCache[hostID][len(a.dataCache[hostID])-maxCacheSize:]
	}

	// 更新TopK分析器
	a.topkAnalyzer.AddData(data)
}

// GetCacheData 获取缓存数据
func (a *Analyzer) GetCacheData(hostID string) []*models.MetricsData {
	a.cacheMutex.RLock()
	defer a.cacheMutex.RUnlock()

	return a.dataCache[hostID]
}

// ClearCache 清除缓存
func (a *Analyzer) ClearCache() {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()

	a.dataCache = make(map[string][]*models.MetricsData)
}

// RegisterAggregator 注册聚合器
func (a *Analyzer) RegisterAggregator(aggregator Aggregator) {
	a.aggregators[aggregator.GetType()] = aggregator
}

// GetAggregator 获取聚合器
func (a *Analyzer) GetAggregator(aggType string) (Aggregator, bool) {
	aggregator, exists := a.aggregators[aggType]
	return aggregator, exists
}

// Aggregate 聚合数据
func (a *Analyzer) Aggregate(hostID, metricName, aggType string, timeRange TimeRange) (*AggregationResult, error) {
	// 获取数据
	data := a.GetDataInRange(hostID, timeRange)
	if len(data) == 0 {
		return nil, fmt.Errorf("no data found for host %s in time range", hostID)
	}

	// 获取聚合器
	aggregator, exists := a.aggregators[aggType]
	if !exists {
		return nil, fmt.Errorf("aggregator type %s not found", aggType)
	}

	// 过滤指标数据
	filteredData := a.filterMetricData(data, metricName)
	if len(filteredData) == 0 {
		return nil, fmt.Errorf("no data found for metric %s", metricName)
	}

	// 执行聚合
	result := aggregator.Aggregate(filteredData)

	return &AggregationResult{
		Type:      aggType,
		TimeRange: timeRange,
		HostID:    hostID,
		Metric:    metricName,
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}

// GetDataInRange 获取时间范围内的数据
func (a *Analyzer) GetDataInRange(hostID string, timeRange TimeRange) []*models.MetricsData {
	a.cacheMutex.RLock()
	defer a.cacheMutex.RUnlock()

	var result []*models.MetricsData
	for _, data := range a.dataCache[hostID] {
		if data.Timestamp.After(timeRange.Start) && data.Timestamp.Before(timeRange.End) {
			result = append(result, data)
		}
	}

	return result
}

// filterMetricData 过滤指标数据
func (a *Analyzer) filterMetricData(data []*models.MetricsData, metricName string) []*models.MetricsData {
	var result []*models.MetricsData
	for _, d := range data {
		for _, metric := range d.Metrics {
			if metric.Name == metricName {
				result = append(result, d)
				break
			}
		}
	}
	return result
}

// AnalyzeTrend 分析趋势
func (a *Analyzer) AnalyzeTrend(hostID, metricName string, timeRange TimeRange) (*TrendAnalysis, error) {
	data := a.GetDataInRange(hostID, timeRange)
	if len(data) < 2 {
		return nil, fmt.Errorf("insufficient data for trend analysis")
	}

	filteredData := a.filterMetricData(data, metricName)
	if len(filteredData) < 2 {
		return nil, fmt.Errorf("insufficient data for metric %s", metricName)
	}

	// 提取时间序列数据
	var xValues []float64
	var yValues []float64
	_ = filteredData[0].Timestamp.Unix() // baseTime for potential future use

	for i, d := range filteredData {
		xValues = append(xValues, float64(i))
		for _, metric := range d.Metrics {
			if metric.Name == metricName {
				yValues = append(yValues, metric.Value)
				break
			}
		}
	}

	// 计算线性回归
	slope, correlation := a.linearRegression(xValues, yValues)

	// 确定趋势方向
	direction := "stable"
	if math.Abs(slope) > 0.1 {
		if slope > 0 {
			direction = "up"
		} else {
			direction = "down"
		}
	}

	// 简单预测
	prediction := yValues[len(yValues)-1] + slope*float64(len(yValues))

	return &TrendAnalysis{
		Direction:   direction,
		Slope:       slope,
		Correlation: correlation,
		Prediction:  prediction,
		Confidence:  math.Abs(correlation) * 100,
	}, nil
}

// linearRegression 线性回归
func (a *Analyzer) linearRegression(x, y []float64) (slope, correlation float64) {
	n := float64(len(x))
	if n < 2 {
		return 0, 0
	}

	// 计算均值
	sumX, sumY := 0.0, 0.0
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX, meanY := sumX/n, sumY/n

	// 计算斜率和相关系数
	var sumXY, sumX2, sumY2 float64
	for i := range x {
		sumXY += (x[i] - meanX) * (y[i] - meanY)
		sumX2 += (x[i] - meanX) * (x[i] - meanX)
		sumY2 += (y[i] - meanY) * (y[i] - meanY)
	}

	if sumX2 == 0 {
		return 0, 0
	}

	slope = sumXY / sumX2
	
	if sumY2 == 0 {
		correlation = 1.0
	} else {
		correlation = sumXY / math.Sqrt(sumX2*sumY2)
	}

	return slope, correlation
}

// DetectAnomaly 检测异常
func (a *Analyzer) DetectAnomaly(hostID, metricName string, timeRange TimeRange) (*AnomalyDetection, error) {
	data := a.GetDataInRange(hostID, timeRange)
	if len(data) < 10 {
		return nil, fmt.Errorf("insufficient data for anomaly detection")
	}

	filteredData := a.filterMetricData(data, metricName)
	if len(filteredData) < 10 {
		return nil, fmt.Errorf("insufficient data for metric %s", metricName)
	}

	// 提取数值
	var values []float64
	for _, d := range filteredData {
		for _, metric := range d.Metrics {
			if metric.Name == metricName {
				values = append(values, metric.Value)
				break
			}
		}
	}

	// 使用Z-score方法检测异常
	mean, stdDev := a.calculateMeanStdDev(values)
	if stdDev == 0 {
		return &AnomalyDetection{
			IsAnomaly:    false,
			AnomalyScore: 0,
			Threshold:    3.0,
			Method:       "z-score",
			Description:  "No variation in data",
		}, nil
	}

	// 检测最后一个数据点
	lastValue := values[len(values)-1]
	zScore := math.Abs(lastValue-mean) / stdDev
	threshold := 3.0

	isAnomaly := zScore > threshold

	return &AnomalyDetection{
		IsAnomaly:    isAnomaly,
		AnomalyScore: zScore,
		Threshold:    threshold,
		Method:       "z-score",
		Description:  fmt.Sprintf("Z-score: %.2f", zScore),
	}, nil
}

// calculateMeanStdDev 计算均值和标准差
func (a *Analyzer) calculateMeanStdDev(values []float64) (mean, stdDev float64) {
	n := float64(len(values))
	if n == 0 {
		return 0, 0
	}

	// 计算均值
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / n

	// 计算标准差
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= n
	stdDev = math.Sqrt(variance)

	return mean, stdDev
}

// CheckAlerts 检查告警
func (a *Analyzer) CheckAlerts(ctx context.Context) []*Alert {
	var alerts []*Alert

	for _, rule := range a.alertRules {
		if !rule.Enabled {
			continue
		}

		// 对每个主机检查告警
		a.cacheMutex.RLock()
		for hostID, data := range a.dataCache {
			if len(data) == 0 {
				continue
			}

			// 获取最近的指标值
			latestData := data[len(data)-1]
			for _, metric := range latestData.Metrics {
				if metric.Name == rule.Metric {
					if a.evaluateCondition(metric.Value, rule.Condition, rule.Threshold) {
						alert := &Alert{
							ID:        fmt.Sprintf("%s-%s-%d", rule.ID, hostID, time.Now().Unix()),
							RuleID:    rule.ID,
							RuleName:  rule.Name,
							HostID:    hostID,
							Metric:    rule.Metric,
							Value:     metric.Value,
							Threshold: rule.Threshold,
							Severity:  rule.Severity,
							Message:   fmt.Sprintf("%s %s %.2f (threshold: %.2f)", rule.Metric, rule.Condition, metric.Value, rule.Threshold),
							Timestamp: time.Now(),
							Status:    "active",
							Metadata:  rule.Metadata,
						}
						alerts = append(alerts, alert)
					}
				}
			}
		}
		a.cacheMutex.RUnlock()
	}

	return alerts
}

// evaluateCondition 评估条件
func (a *Analyzer) evaluateCondition(value float64, condition string, threshold float64) bool {
	switch condition {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

// RegisterAlertRule 注册告警规则
func (a *Analyzer) RegisterAlertRule(rule *AlertRule) {
	a.alertRules[rule.ID] = rule
}

// GetAlertRules 获取告警规则
func (a *Analyzer) GetAlertRules() map[string]*AlertRule {
	return a.alertRules
}

// RegisterDefaultAlertRules 注册默认告警规则
func (a *Analyzer) RegisterDefaultAlertRules() {
	defaultRules := []*AlertRule{
		{
			ID:        "cpu-high",
			Name:      "CPU使用率过高",
			Metric:    "cpu_usage",
			Condition: ">",
			Threshold: 80.0,
			Severity:  "warning",
			Enabled:   true,
		},
		{
			ID:        "cpu-critical",
			Name:      "CPU使用率严重过高",
			Metric:    "cpu_usage",
			Condition: ">",
			Threshold: 95.0,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			ID:        "memory-high",
			Name:      "内存使用率过高",
			Metric:    "memory_usage",
			Condition: ">",
			Threshold: 85.0,
			Severity:  "warning",
			Enabled:   true,
		},
		{
			ID:        "memory-critical",
			Name:      "内存使用率严重过高",
			Metric:    "memory_usage",
			Condition: ">",
			Threshold: 95.0,
			Severity:  "critical",
			Enabled:   true,
		},
		{
			ID:        "disk-high",
			Name:      "磁盘使用率过高",
			Metric:    "disk_usage",
			Condition: ">",
			Threshold: 90.0,
			Severity:  "warning",
			Enabled:   true,
		},
	}

	for _, rule := range defaultRules {
		a.RegisterAlertRule(rule)
	}
}

// GetTopK 获取TopK结果
func (a *Analyzer) GetTopK(metricName string, k int) []*TopKResult {
	return a.topkAnalyzer.GetTopK(metricName, k)
}

// GetStatistics 获取统计信息
func (a *Analyzer) GetStatistics(hostID string) map[string]interface{} {
	a.cacheMutex.RLock()
	defer a.cacheMutex.RUnlock()

	stats := make(map[string]interface{})
	
	if data, exists := a.dataCache[hostID]; exists {
		stats["total_data_points"] = len(data)
		stats["latest_timestamp"] = data[len(data)-1].Timestamp
		stats["earliest_timestamp"] = data[0].Timestamp
		
		// 计算各指标的统计信息
		metricStats := make(map[string]interface{})
		for _, d := range data {
			for _, metric := range d.Metrics {
				if _, exists := metricStats[metric.Name]; !exists {
					metricStats[metric.Name] = map[string]interface{}{
						"count": 0,
						"min":   math.MaxFloat64,
						"max":   -math.MaxFloat64,
						"sum":   0.0,
					}
				}
				
				stat := metricStats[metric.Name].(map[string]interface{})
				stat["count"] = stat["count"].(int) + 1
				stat["min"] = math.Min(stat["min"].(float64), metric.Value)
				stat["max"] = math.Max(stat["max"].(float64), metric.Value)
				stat["sum"] = stat["sum"].(float64) + metric.Value
			}
		}
		
		// 计算平均值
		for _, stat := range metricStats {
			statMap := stat.(map[string]interface{})
			if count := statMap["count"].(int); count > 0 {
				statMap["avg"] = statMap["sum"].(float64) / float64(count)
			}
		}
		
		stats["metrics"] = metricStats
	}
	
	return stats
}

// ToJSON 转换为JSON
func (a *Analyzer) ToJSON() (string, error) {
	data := map[string]interface{}{
		"cache_size":    len(a.dataCache),
		"aggregators":   len(a.aggregators),
		"alert_rules":   len(a.alertRules),
		"statistics":   a.GetStatistics("all"),
	}
	
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}