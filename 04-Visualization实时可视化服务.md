# Visualization实时可视化服务 - 数据展示与用户交互

## 📋 **Visualization概述**

### **功能定位**
Visualization是分布式监控系统的前端展示层，负责从Broker集群查询监控数据，提供HTTP API接口，通过WebSocket实时推送数据到前端，为用户提供直观、实时、交互式的监控数据可视化体验。

### **核心特性**
- **实时数据查询**: 从Broker集群获取历史和实时监控数据
- **WebSocket推送**: 毫秒级实时数据推送到前端界面
- **RESTful API**: 标准化API接口，支持第三方集成
- **数据分析**: 聚合分析、趋势分析、异常检测等功能

---

## 🏗️ **Visualization架构设计**

### **服务架构概览**

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Visualization服务架构                           │
├─────────────────┬─────────────────┬─────────────────┬───────────────┤
│   API服务层     │   数据查询层    │   实时推送层    │   分析处理层  │
├─────────────────┼─────────────────┼─────────────────┼───────────────┤
│ • HTTP服务器    │ • gRPC客户端    │ • WebSocket     │ • 数据聚合    │
│ • 路由管理      │ • 多Broker连接  │ • 实时推送      │ • 趋势分析    │
│ • 中间件        │ • 负载均衡      │ • 连接管理      │ • 异常检测    │
│ • 参数验证      │ • 重试机制      │ • 心跳检测      │ • Top-K算法   │
└─────────────────┴─────────────────┴─────────────────┴───────────────┘
                                │
                                ▼
                        ┌─────────────────┐
                        │   前端界面      │
                        │ • 实时图表      │
                        │ • 数据面板      │
                        │ • 告警管理      │
                        │ • 系统状态      │
                        └─────────────────┘
```

### **数据流向设计**

```
Broker查询 → 数据获取 → 格式转换 → 缓存存储 → API响应
     ↓          ↓          ↓          ↓          ↓
gRPC调用    结构化数据    前端格式    内存缓存   JSON返回
负载均衡    Go结构体     JavaScript  LRU缓存   HTTP响应

              ↓
         实时数据推送
              ↓
WebSocket广播 → 客户端连接 → 前端更新 → 图表渲染
     ↓              ↓             ↓          ↓
  消息分发        浏览器连接    数据更新   实时显示
  并发推送        事件驱动      状态管理   用户界面
```

---

## 🌐 **HTTP API服务**

### **API架构设计**

#### **路由注册管理**
```go
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
```

#### **API处理器架构**
```go
type APIHandler struct {
    analyzer   *analysis.Analyzer    // 数据分析器
    grpcClient *service.GRPCClient   // gRPC客户端
}
```

### **核心API接口**

#### **指标查询接口**
```
GET /api/metrics
参数:
  - host_id: 主机ID (可选)
  - start_time: 开始时间 (ISO8601格式)
  - end_time: 结束时间 (ISO8601格式)
  - metrics: 指标类型 (cpu,memory,network,disk)

响应:
{
  "success": true,
  "data": [
    {
      "host_id": "host-1",
      "timestamp": 1640995200,
      "metrics": [
        {"name": "cpu_usage", "value": 75.5, "unit": "%"},
        {"name": "memory_usage", "value": 60.2, "unit": "%"}
      ]
    }
  ]
}
```

#### **主机指标接口**
```
GET /api/metrics/host/{hostID}
参数:
  - start_time: 开始时间
  - end_time: 结束时间

功能: 获取指定主机的所有指标数据
```

#### **聚合分析接口**
```
POST /api/metrics/aggregate
请求体:
{
  "host_id": "host-1",
  "metric": "cpu_usage",
  "agg_type": "avg",
  "start_time": "2023-01-01T00:00:00Z",
  "end_time": "2023-01-02T00:00:00Z"
}

功能: 对指定时间范围的指标进行聚合计算
```

### **数据查询处理**

#### **查询参数处理**
```
参数解析 → 参数验证 → 默认值设置 → 时间范围检查 → 查询执行
    │         │         │           │             │
    ▼         ▼         ▼           ▼             ▼
URL参数    格式验证    默认1小时     合理性检查     gRPC调用
解析       类型检查    当前时间      范围限制       Broker查询
```

#### **查询优化策略**
- **参数缓存**: 相同查询参数结果缓存，提高响应速度
- **批量查询**: 多主机查询合并处理，减少网络往返
- **异步处理**: 大数据量查询异步处理，避免阻塞
- **分页支持**: 大结果集分页返回，控制内存使用

### **错误处理与响应**

#### **统一错误响应格式**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_PARAMS",
    "message": "Invalid time format",
    "details": "Time format should be ISO8601"
  }
}
```

#### **错误类型分类**
- **参数错误**: 400 Bad Request - 参数格式或值错误
- **认证错误**: 401 Unauthorized - 认证失败
- **权限错误**: 403 Forbidden - 权限不足
- **资源错误**: 404 Not Found - 资源不存在
- **服务错误**: 500 Internal Server Error - 服务内部错误

---

## 📡 **实时数据推送**

### **WebSocket服务架构**

#### **WebSocket服务器设计**
```go
type Server struct {
    config     *config.Config      // 配置
    clients    map[*Client]bool    // 客户端连接池
    register   chan *Client        // 注册通道
    unregister chan *Client        // 注销通道
    broadcast  chan []byte         // 广播通道
    upgrader   websocket.Upgrader  // WebSocket升级器
}
```

#### **客户端连接管理**
```go
type Client struct {
    server *Server           // 服务器引用
    conn   *websocket.Conn   // WebSocket连接
    send   chan []byte       // 发送缓冲通道
    mu     sync.Mutex        // 连接锁
}
```

### **实时数据拉取**

#### **增量数据拉取机制**
```go
func pullDataFromBroker(ctx context.Context, client *service.GRPCClient, wsServer *websocket.Server) {
    ticker := time.NewTicker(2 * time.Second)  // 2秒拉取周期
    lastSent := make(map[string]int64)         // 每个主机的最后时间戳
    
    for {
        select {
        case <-ticker.C:
            for _, hostID := range hostIDs {
                // 计算增量时间窗口
                var start time.Time
                if ts, ok := lastSent[hostID]; ok && ts > 0 {
                    start = time.Unix(ts+1, 0)  // 避免重复数据
                } else {
                    start = time.Now().Add(-30 * time.Second)
                }
                
                // 拉取增量数据
                metrics, err := client.GetMetricsWithRetry(ctx, hostID, start, time.Now())
                if err == nil {
                    // 广播到所有WebSocket连接
                    for _, data := range metrics {
                        wsServer.Broadcast(data)
                        lastSent[hostID] = data.Timestamp.Unix()
                    }
                }
            }
        }
    }
}
```

#### **数据去重策略**
- **时间戳管理**: 记录每个主机最后发送的数据时间戳
- **增量拉取**: 只拉取上次时间戳之后的新数据
- **兜底机制**: 无增量数据时扩大时间窗口重新拉取
- **重复检测**: 检测重复数据点，避免重复推送

### **消息广播机制**

#### **数据格式转换**
```go
func (s *Server) Broadcast(data *models.MetricsData) {
    // 1. 数据格式转换（兼容前端）
    var convertedMetrics []map[string]interface{}
    for _, metric := range data.Metrics {
        // 指标名称映射
        var frontendName string
        switch {
        case strings.Contains(strings.ToLower(metric.Name), "cpu"):
            frontendName = "cpu_usage"
        case strings.Contains(strings.ToLower(metric.Name), "memory"):
            frontendName = "memory_usage"
        default:
            frontendName = metric.Name
        }

        convertedMetrics = append(convertedMetrics, map[string]interface{}{
            "name":  frontendName,
            "value": metric.Value,
            "unit":  metric.Unit,
        })
    }

    // 2. 构造广播消息
    message := map[string]interface{}{
        "type":      "metrics_update",
        "host_id":   data.HostID,
        "timestamp": data.Timestamp.Unix(),
        "metrics":   convertedMetrics,
    }

    // 3. JSON序列化并广播
    jsonData, _ := json.Marshal(message)
    s.broadcast <- jsonData
}
```

#### **连接状态管理**
```
客户端连接 → 注册管理 → 心跳检测 → 消息分发 → 连接清理
    │           │         │         │         │
    ▼           ▼         ▼         ▼         ▼
WebSocket     连接池     定期ping   消息推送   资源回收
握手成功      添加连接   保持活跃   并发发送   关闭连接
```

### **连接优化与容错**

#### **心跳机制**
- **Ping间隔**: 30秒发送Ping消息检测连接状态
- **Pong超时**: 60秒未收到Pong响应则关闭连接
- **自动重连**: 客户端检测到连接断开自动重新连接
- **连接池清理**: 定期清理僵尸连接，释放资源

#### **背压控制**
- **发送缓冲**: 每个连接设置发送缓冲区，避免阻塞
- **慢消费者**: 检测慢消费者，必要时主动断开连接
- **内存控制**: 限制总连接数和缓冲区大小，防止OOM
- **优雅降级**: 高负载时降低推送频率或数据精度

---

## 📊 **数据分析与聚合**

### **分析器架构**

#### **分析器设计**
```go
type Analyzer struct {
    dataCache   map[string][]*models.MetricsData  // 数据缓存
    aggregators map[string]Aggregator              // 聚合器映射
    cacheMutex  sync.RWMutex                      // 缓存锁
    config      *AnalysisConfig                   // 分析配置
}
```

#### **聚合器接口**
```go
type Aggregator interface {
    Aggregate(data []*models.MetricsData, timeRange TimeRange) (*AggregationResult, error)
    GetType() string
    GetName() string
}
```

### **聚合分析功能**

#### **基础聚合计算**
- **平均值**: 计算指定时间范围内指标的平均值
- **最大值**: 获取时间范围内的最大指标值
- **最小值**: 获取时间范围内的最小指标值
- **总和**: 累计指标值总和（适用于流量等）
- **计数**: 统计数据点数量

#### **聚合计算流程**
```
数据输入 → 时间过滤 → 指标分组 → 聚合计算 → 结果格式化
    │         │         │         │           │
    ▼         ▼         ▼         ▼           ▼
原始数据   时间范围   按指标分类   统计函数    结构化结果
多个数据点  过滤器    分组Map     数学计算    JSON格式
```

### **趋势分析**

#### **趋势计算算法**
```go
func (a *Analyzer) CalculateTrend(data []*models.MetricsData, metric string) (*TrendResult, error) {
    // 1. 数据预处理
    values := extractMetricValues(data, metric)
    timestamps := extractTimestamps(data)
    
    // 2. 线性回归计算
    slope, intercept := linearRegression(timestamps, values)
    
    // 3. 趋势判断
    trend := "stable"
    if slope > 0.1 {
        trend = "increasing"
    } else if slope < -0.1 {
        trend = "decreasing"
    }
    
    return &TrendResult{
        Metric:    metric,
        Trend:     trend,
        Slope:     slope,
        Intercept: intercept,
    }, nil
}
```

#### **趋势分析类型**
- **上升趋势**: 指标值呈现持续上升态势
- **下降趋势**: 指标值呈现持续下降态势
- **稳定趋势**: 指标值在合理范围内波动
- **周期性趋势**: 指标值呈现周期性变化模式

### **异常检测**

#### **异常检测算法**
- **统计异常**: 基于均值和标准差的2σ/3σ准则
- **阈值异常**: 超过预设阈值的异常值检测
- **趋势异常**: 指标变化趋势的异常检测
- **周期异常**: 相对于历史同期的异常检测

#### **异常检测流程**
```
历史数据 → 基线计算 → 实时比较 → 异常判定 → 异常标记
    │         │         │         │         │
    ▼         ▼         ▼         ▼         ▼
过去30天   统计基线   当前数值   阈值比较   异常级别
统计分析   均值/标准差  实时数据   偏差计算   告警等级
```

---

## 🔍 **gRPC客户端服务**

### **客户端连接管理**

#### **多Broker连接**
```go
type GRPCClient struct {
    config     *config.Config
    conns      map[string]*grpc.ClientConn        // Broker连接池
    clients    map[string]pb.MonitorServiceClient // 客户端池
    mu         sync.RWMutex                       // 读写锁
    retryCount int                                // 重试次数
}
```

#### **负载均衡策略**
- **轮询算法**: 依次选择不同的Broker节点
- **健康检查**: 只选择健康的Broker节点
- **故障切换**: 节点故障时自动切换到其他节点
- **权重分配**: 根据节点性能分配不同权重

### **查询请求处理**

#### **带重试的查询机制**
```go
func (c *GRPCClient) GetMetricsWithRetry(ctx context.Context, hostID string, start, end time.Time) ([]*models.MetricsData, error) {
    for i := 0; i <= c.retryCount; i++ {
        result, err := c.GetMetrics(ctx, hostID, start, end)
        if err == nil {
            return result, nil
        }
        
        // 失败则切换到下一个Broker
        c.nextBroker()
        
        // 等待后重试
        if i < c.retryCount {
            time.Sleep(time.Duration(i+1) * time.Second)
        }
    }
    return nil, fmt.Errorf("all brokers failed after %d retries", c.retryCount)
}
```

#### **查询优化策略**
- **连接复用**: 复用gRPC连接，减少建连开销
- **并发查询**: 多主机查询并发执行，提高效率
- **结果缓存**: 热点查询结果缓存，减少重复查询
- **超时控制**: 设置合理的查询超时时间

### **数据格式转换**

#### **Protobuf到Go结构转换**
```go
func convertProtoToModel(data *pb.MetricsData) *models.MetricsData {
    metrics := make([]models.Metric, 0, len(data.Metrics))
    for _, m := range data.Metrics {
        value, err := strconv.ParseFloat(m.Value, 64)
        if err != nil {
            continue // 跳过无效数值
        }
        metrics = append(metrics, models.Metric{
            Name:  m.Name,
            Value: value,
            Unit:  m.Unit,
        })
    }

    return &models.MetricsData{
        HostID:    data.HostId,
        Timestamp: time.Unix(data.Timestamp, 0),
        Metrics:   metrics,
    }
}
```

Visualization作为分布式监控系统的用户接口层，通过高效的数据查询、实时的WebSocket推送和友好的Web界面，为用户提供了优秀的监控数据可视化体验，是整个监控系统的重要组成部分。
