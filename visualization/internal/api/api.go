package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/han-fei/monitor/visualization/internal/analysis"
	"github.com/han-fei/monitor/visualization/internal/models"
	"github.com/han-fei/monitor/visualization/internal/service"
)

// APIHandler API处理器
type APIHandler struct {
	analyzer   *analysis.Analyzer
	grpcClient *service.GRPCClient
}

// NewAPIHandler 创建API处理器
func NewAPIHandler(analyzer *analysis.Analyzer, grpcClient *service.GRPCClient) *APIHandler {
	return &APIHandler{
		analyzer:   analyzer,
		grpcClient: grpcClient,
	}
}

// RegisterRoutes 注册API路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	// 指标数据相关
	mux.HandleFunc("/api/metrics", h.handleMetrics)
	mux.HandleFunc("/api/metrics/host/", h.handleHostMetrics)
	mux.HandleFunc("/api/metrics/aggregate", h.handleAggregateMetrics)
	
	// 分析相关
	mux.HandleFunc("/api/analysis/trend", h.handleTrendAnalysis)
	mux.HandleFunc("/api/analysis/anomaly", h.handleAnomalyDetection)
	mux.HandleFunc("/api/analysis/topk", h.handleTopK)
	
	// 告警相关
	mux.HandleFunc("/api/alerts", h.handleAlerts)
	mux.HandleFunc("/api/alerts/rules", h.handleAlertRules)
	
	// 系统状态
	mux.HandleFunc("/api/system/status", h.handleSystemStatus)
	mux.HandleFunc("/api/system/stats", h.handleSystemStats)
}

// handleMetrics 处理指标数据请求
func (h *APIHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getMetrics(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHostMetrics 处理主机指标数据请求
func (h *APIHandler) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 从URL中提取hostID
	hostID := r.URL.Path[len("/api/metrics/host/"):]
	if hostID == "" {
		http.Error(w, "Host ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getHostMetrics(w, r, hostID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAggregateMetrics 处理聚合指标请求
func (h *APIHandler) handleAggregateMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		h.aggregateMetrics(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTrendAnalysis 处理趋势分析请求
func (h *APIHandler) handleTrendAnalysis(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		h.trendAnalysis(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAnomalyDetection 处理异常检测请求
func (h *APIHandler) handleAnomalyDetection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		h.anomalyDetection(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTopK 处理TopK请求
func (h *APIHandler) handleTopK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getTopK(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAlerts 处理告警请求
func (h *APIHandler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getAlerts(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAlertRules 处理告警规则请求
func (h *APIHandler) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getAlertRules(w, r)
	case http.MethodPost:
		h.createAlertRule(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSystemStatus 处理系统状态请求
func (h *APIHandler) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getSystemStatus(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSystemStats 处理系统统计请求
func (h *APIHandler) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.getSystemStats(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// 以下是具体的处理函数

func (h *APIHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	query := r.URL.Query()
	hostID := query.Get("host_id")
	startTime := query.Get("start_time")
	endTime := query.Get("end_time")

	// 解析时间
	var start, end time.Time
	var err error

	if startTime != "" {
		start, err = time.Parse(time.RFC3339, startTime)
		if err != nil {
			http.Error(w, "Invalid start time format", http.StatusBadRequest)
			return
		}
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}

	if endTime != "" {
		end, err = time.Parse(time.RFC3339, endTime)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}
	} else {
		end = time.Now()
	}

	// 获取数据
	ctx := r.Context()
	var metrics []*models.MetricsData
	var err2 error

	if hostID != "" {
		metrics, err2 = h.grpcClient.GetMetricsWithRetry(ctx, hostID, start, end)
	} else {
		// 获取所有主机的数据
		hostIDs := []string{"host-1", "host-2", "host-3"}
		for _, id := range hostIDs {
			data, err := h.grpcClient.GetMetricsWithRetry(ctx, id, start, end)
			if err == nil {
				metrics = append(metrics, data...)
			}
		}
	}

	if err2 != nil {
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}

	// 添加到分析器缓存
	for _, metric := range metrics {
		h.analyzer.AddData(metric)
	}

	json.NewEncoder(w).Encode(metrics)
}

func (h *APIHandler) getHostMetrics(w http.ResponseWriter, r *http.Request, hostID string) {
	// 获取查询参数
	query := r.URL.Query()
	startTime := query.Get("start_time")
	endTime := query.Get("end_time")

	// 解析时间
	var start, end time.Time
	var err error

	if startTime != "" {
		start, err = time.Parse(time.RFC3339, startTime)
		if err != nil {
			http.Error(w, "Invalid start time format", http.StatusBadRequest)
			return
		}
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}

	if endTime != "" {
		end, err = time.Parse(time.RFC3339, endTime)
		if err != nil {
			http.Error(w, "Invalid end time format", http.StatusBadRequest)
			return
		}
	} else {
		end = time.Now()
	}

	// 获取数据
	ctx := r.Context()
	metrics, err := h.grpcClient.GetMetricsWithRetry(ctx, hostID, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 添加到分析器缓存
	for _, metric := range metrics {
		h.analyzer.AddData(metric)
	}

	json.NewEncoder(w).Encode(metrics)
}

func (h *APIHandler) aggregateMetrics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID    string `json:"host_id"`
		Metric    string `json:"metric"`
		AggType   string `json:"agg_type"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 解析时间
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// 执行聚合
	timeRange := analysis.TimeRange{Start: start, End: end}
	result, err := h.analyzer.Aggregate(req.HostID, req.Metric, req.AggType, timeRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (h *APIHandler) trendAnalysis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID    string `json:"host_id"`
		Metric    string `json:"metric"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 解析时间
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// 执行趋势分析
	timeRange := analysis.TimeRange{Start: start, End: end}
	result, err := h.analyzer.AnalyzeTrend(req.HostID, req.Metric, timeRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (h *APIHandler) anomalyDetection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID    string `json:"host_id"`
		Metric    string `json:"metric"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 解析时间
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		http.Error(w, "Invalid start time format", http.StatusBadRequest)
		return
	}

	end, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		http.Error(w, "Invalid end time format", http.StatusBadRequest)
		return
	}

	// 执行异常检测
	timeRange := analysis.TimeRange{Start: start, End: end}
	result, err := h.analyzer.DetectAnomaly(req.HostID, req.Metric, timeRange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (h *APIHandler) getTopK(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	metric := query.Get("metric")
	kStr := query.Get("k")

	if metric == "" {
		http.Error(w, "Metric parameter required", http.StatusBadRequest)
		return
	}

	k := 10 // 默认值
	if kStr != "" {
		var err error
		k, err = strconv.Atoi(kStr)
		if err != nil {
			http.Error(w, "Invalid k parameter", http.StatusBadRequest)
			return
		}
	}

	// 获取TopK结果
	results := h.analyzer.GetTopK(metric, k)
	json.NewEncoder(w).Encode(results)
}

func (h *APIHandler) getAlerts(w http.ResponseWriter, r *http.Request) {
	// 检查告警
	ctx := r.Context()
	alerts := h.analyzer.CheckAlerts(ctx)
	json.NewEncoder(w).Encode(alerts)
}

func (h *APIHandler) getAlertRules(w http.ResponseWriter, r *http.Request) {
	rules := h.analyzer.GetAlertRules()
	json.NewEncoder(w).Encode(rules)
}

func (h *APIHandler) createAlertRule(w http.ResponseWriter, r *http.Request) {
	var rule analysis.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.analyzer.RegisterAlertRule(&rule)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

func (h *APIHandler) getSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version": "1.0.0",
		"components": map[string]string{
			"analyzer": "healthy",
			"grpc_client": "healthy",
			"websocket": "healthy",
		},
	}

	json.NewEncoder(w).Encode(status)
}

func (h *APIHandler) getSystemStats(w http.ResponseWriter, r *http.Request) {
	stats := h.analyzer.GetStatistics("all")
	json.NewEncoder(w).Encode(stats)
}