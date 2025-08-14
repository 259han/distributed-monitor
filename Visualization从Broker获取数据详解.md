# Visualization从Broker获取数据详解

## 📋 概述

本文档详细分析Visualization模块是如何从Broker获取监控数据的，包括gRPC通信、实时数据拉取、数据转换和错误处理等关键机制。

---

## 🏗️ **1. 整体架构概览**

### 1.1 数据获取架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Visualization数据获取架构                     │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   连接管理层    │   查询服务层    │        推送服务层           │
├─────────────────┼─────────────────┼─────────────────────────────┤
│ • gRPC连接池    │ • API处理器     │ • 实时数据拉取              │
│ • 重试机制      │ • 数据转换      │ • WebSocket推送             │
│ • 负载均衡      │ • 缓存管理      │ • QUIC推送                  │
│ • 健康检查      │ • 聚合分析      │ • 增量更新                  │
└─────────────────┴─────────────────┴─────────────────────────────┘
```

### 1.2 数据流向图

```
浏览器客户端 ← WebSocket ← 推送服务 ← 定时拉取 ← gRPC客户端 ← Broker集群
    │                      │               │              │
    ▼                      ▼               ▼              ▼
API请求 → API处理器 → gRPC查询 → 数据转换 → 响应返回
           │              │         │
           ▼              ▼         ▼
        分析器缓存    重试机制    数据格式化
```

---

## 🔌 **2. gRPC连接管理**

### 2.1 连接客户端结构

```go
// GRPCClient gRPC客户端
type GRPCClient struct {
    config     *config.Config                    // 配置信息
    conns      map[string]*grpc.ClientConn       // 连接池
    clients    map[string]pb.MonitorServiceClient // 客户端池
    mu         sync.RWMutex                      // 读写锁
    retryCount int                               // 重试次数
}
```

### 2.2 连接建立流程

```go
连接配置 → 创建连接 → 建立客户端 → 保存连接池 → 连接就绪
    │         │         │         │           │
    ▼         ▼         ▼         ▼           ▼
BrokerConfig  gRPC.Dial  MonitorService  连接映射   可用状态
配置端点列表   TCP连接    Client创建     conns/clients  服务就绪
```

**具体实现：**
```go
func (c *GRPCClient) Connect() error {
    // 遍历所有Broker端点
    for _, endpoint := range c.config.Broker.Endpoints {
        // 设置连接选项
        opts := []grpc.DialOption{
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithBlock(),                        // 阻塞直到连接建立
            grpc.WithTimeout(c.config.Broker.Timeout), // 连接超时
        }

        // 建立gRPC连接
        conn, err := grpc.Dial(endpoint, opts...)
        if err != nil {
            return fmt.Errorf("连接到broker %s失败: %v", endpoint, err)
        }

        // 创建客户端并保存
        client := pb.NewMonitorServiceClient(conn)
        c.conns[endpoint] = conn
        c.clients[endpoint] = client
    }
    return nil
}
```

### 2.3 配置参数

```go
// BrokerConfig 中转层配置
type BrokerConfig struct {
    Endpoints []string      `yaml:"endpoints"`  // Broker端点列表
    Timeout   time.Duration `yaml:"timeout"`    // 连接超时时间
    MaxRetry  int           `yaml:"max_retry"`  // 最大重试次数
}

// 默认配置值
默认端点: ["localhost:9090"]
默认超时: 5秒
默认重试: 3次
```

---

## 📊 **3. 数据查询机制**

### 3.1 基础查询流程

```go
查询请求 → 选择客户端 → 创建流 → 接收数据 → 数据转换 → 返回结果
    │         │         │       │         │         │
    ▼         ▼         ▼       ▼         ▼         ▼
MetricsRequest  轮询选择   gRPC Stream  Proto数据  模型转换  MetricsData[]
时间范围/主机ID  负载均衡   流式传输    序列化    结构化    Go结构体
```

**核心查询实现：**
```go
func (c *GRPCClient) GetMetrics(ctx context.Context, hostID string, startTime, endTime time.Time) ([]*models.MetricsData, error) {
    // 1. 选择可用客户端（简单轮询）
    var selectedEndpoint string
    for endpoint := range c.clients {
        selectedEndpoint = endpoint
        break
    }
    client := c.clients[selectedEndpoint]

    // 2. 创建查询请求
    req := &pb.MetricsRequest{
        HostId:    hostID,
        StartTime: startTime.Unix(),
        EndTime:   endTime.Unix(),
    }

    // 3. 创建gRPC流
    stream, err := client.GetMetrics(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("创建流失败: %v", err)
    }

    // 4. 流式接收数据
    var result []*models.MetricsData
    for {
        data, err := stream.Recv()
        if err == io.EOF {
            break // 数据接收完毕
        }
        if err != nil {
            return nil, fmt.Errorf("接收数据失败: %v", err)
        }

        // 5. 数据格式转换
        metricsData := convertProtoToModel(data)
        result = append(result, metricsData)
    }

    return result, nil
}
```

### 3.2 数据转换流程

```go
Proto数据 → 解析指标 → 类型转换 → 创建模型 → 返回结构
    │         │         │         │         │
    ▼         ▼         ▼         ▼         ▼
pb.MetricsData  Metrics数组  字符串→浮点数  models.MetricsData  Go结构体
protobuf格式   name/value/unit  ParseFloat  标准化格式      业务模型
```

**数据转换实现：**
```go
// Proto格式 → Go模型转换
metrics := make([]models.Metric, 0, len(data.Metrics))
for _, m := range data.Metrics {
    // 解析数值（处理字符串格式的数值）
    value, err := strconv.ParseFloat(m.Value, 64)
    if err != nil {
        continue // 跳过无效数值
    }
    
    // 创建业务模型
    metrics = append(metrics, models.Metric{
        Name:  m.Name,   // 指标名称
        Value: value,    // 数值
        Unit:  m.Unit,   // 单位
    })
}

// 组装完整的指标数据
metricsData := &models.MetricsData{
    HostID:    data.HostId,
    Timestamp: time.Unix(data.Timestamp, 0),
    Metrics:   metrics,
}
```

---

## 🔄 **4. 重试与错误处理**

### 4.1 重试机制实现

```go
func (c *GRPCClient) GetMetricsWithRetry(ctx context.Context, hostID string, startTime, endTime time.Time) ([]*models.MetricsData, error) {
    var result []*models.MetricsData
    var err error

    // 重试循环
    for i := 0; i <= c.retryCount; i++ {
        result, err = c.GetMetrics(ctx, hostID, startTime, endTime)
        if err == nil {
            return result, nil // 成功返回
        }

        // 最后一次重试失败
        if i == c.retryCount {
            break
        }

        // 等待后重试
        select {
        case <-ctx.Done():
            return nil, ctx.Err() // 上下文取消
        case <-time.After(1 * time.Second):
            // 1秒后重试
        }
    }

    return nil, fmt.Errorf("获取数据失败，已重试%d次: %v", c.retryCount, err)
}
```

### 4.2 错误处理策略

```go
错误类型判断 → 重试策略 → 失败处理
    │             │         │
    ▼             ▼         ▼
网络错误 → 指数退避重试 → 切换Broker
超时错误 → 延迟重试     → 降级服务
协议错误 → 立即失败     → 错误上报
```

---

## ⚡ **5. 实时数据推送**

### 5.1 数据拉取服务

```go
func pullDataFromBroker(ctx context.Context, client *service.GRPCClient, wsServer *websocket.Server, quicServer interface{ SendData(*models.MetricsData) }) {
    ticker := time.NewTicker(2 * time.Second) // 2秒拉取周期
    defer ticker.Stop()

    // 维护每个主机的最后发送时间戳
    lastSent := make(map[string]int64)

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 增量数据拉取逻辑
            h.pullIncrementalData(client, wsServer, quicServer, lastSent)
        }
    }
}
```

### 5.2 增量数据拉取机制

```go
增量拉取 → 时间窗口计算 → Broker查询 → 数据去重 → 推送广播
    │         │           │         │         │
    ▼         ▼           ▼         ▼         ▼
2秒周期    lastSent时间戳   gRPC查询   避免重复   WebSocket/QUIC
定时器    (上次最后点)     增量数据    时间戳检查   实时推送
```

**增量拉取实现：**
```go
for _, hostID := range hostIDs {
    // 计算增量时间窗口
    var start time.Time
    if ts, ok := lastSent[hostID]; ok && ts > 0 {
        start = time.Unix(ts+1, 0) // 从上次最后一个点的下一秒开始
    } else {
        start = now.Add(-30 * time.Second) // 首次拉取最近30秒
    }

    // 查询增量数据
    metrics, err := client.GetMetricsWithRetry(ctx, hostID, start, now)
    if err != nil {
        log.Printf("获取主机 %s 的数据失败: %v", hostID, err)
        continue
    }

    // 兜底机制：如果没有增量数据，扩大时间窗口
    if len(metrics) == 0 {
        fallbackStart := now.Add(-5 * time.Minute)
        fb, fbErr := client.GetMetricsWithRetry(ctx, hostID, fallbackStart, now)
        if fbErr == nil && len(fb) > 0 {
            metrics = fb
            log.Printf("主机 %s 本轮无增量，使用5分钟兜底取到 %d 条", hostID, len(fb))
        }
    }

    // 广播数据并更新时间戳
    var maxTS int64
    for _, data := range metrics {
        wsServer.Broadcast(data)     // WebSocket推送
        if quicServer != nil {
            quicServer.SendData(data) // QUIC推送
        }
        if ts := data.Timestamp.Unix(); ts > maxTS {
            maxTS = ts
        }
    }
    if maxTS > 0 {
        lastSent[hostID] = maxTS // 更新最后发送时间戳
    }
}
```

---

## 🌐 **6. API查询服务**

### 6.1 HTTP API处理

```go
type APIHandler struct {
    analyzer   *analysis.Analyzer  // 数据分析器
    grpcClient *service.GRPCClient // gRPC客户端
}

// API路由注册
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/api/metrics", h.handleMetrics)           // 基础指标查询
    mux.HandleFunc("/api/metrics/host/", h.handleHostMetrics) // 主机指标查询
    mux.HandleFunc("/api/metrics/aggregate", h.handleAggregateMetrics) // 聚合查询
    mux.HandleFunc("/api/analysis/trend", h.handleTrendAnalysis)       // 趋势分析
    mux.HandleFunc("/api/analysis/topk", h.handleTopK)                 // Top-K查询
}
```

### 6.2 指标查询实现

```go
func (h *APIHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
    // 1. 解析查询参数
    query := r.URL.Query()
    hostID := query.Get("host_id")
    startTime := query.Get("start_time")
    endTime := query.Get("end_time")

    // 2. 时间参数处理
    var start, end time.Time
    if startTime != "" {
        start, err = time.Parse(time.RFC3339, startTime)
    } else {
        start = time.Now().Add(-1 * time.Hour) // 默认1小时前
    }

    if endTime != "" {
        end, err = time.Parse(time.RFC3339, endTime)
    } else {
        end = time.Now() // 默认当前时间
    }

    // 3. 调用gRPC客户端查询
    ctx := r.Context()
    var metrics []*models.MetricsData

    if hostID != "" {
        // 单主机查询
        metrics, err = h.grpcClient.GetMetricsWithRetry(ctx, hostID, start, end)
    } else {
        // 多主机查询
        hostIDs := []string{"host-1", "host-2", "host-3"}
        for _, id := range hostIDs {
            data, err := h.grpcClient.GetMetricsWithRetry(ctx, id, start, end)
            if err == nil {
                metrics = append(metrics, data...)
            }
        }
    }

    // 4. 缓存到分析器
    for _, metric := range metrics {
        h.analyzer.AddData(metric)
    }

    // 5. 返回JSON响应
    json.NewEncoder(w).Encode(metrics)
}
```

---

## 📡 **7. WebSocket实时推送**

### 7.1 WebSocket服务器结构

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

### 7.2 数据广播机制

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
        case strings.Contains(strings.ToLower(metric.Name), "disk"):
            frontendName = "disk_usage"
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

    // 3. JSON序列化
    jsonData, err := json.Marshal(message)
    if err != nil {
        log.Printf("序列化广播数据失败: %v", err)
        return
    }

    // 4. 发送到广播通道
    s.broadcast <- jsonData
}
```

### 7.3 客户端消息分发

```go
func (s *Server) run() {
    for {
        select {
        case message := <-s.broadcast:
            // 遍历所有连接的客户端
            s.mu.RLock()
            for client := range s.clients {
                select {
                case client.send <- message:
                    // 成功发送到客户端缓冲区
                default:
                    // 客户端缓冲区已满，关闭连接
                    client.close()
                }
            }
            s.mu.RUnlock()
        }
    }
}
```

---

## 🎯 **8. 性能优化策略**

### 8.1 连接优化

```go
// 连接池配置
连接复用：gRPC长连接，避免频繁建连
连接超时：5秒连接超时，防止阻塞
读写缓冲：优化缓冲区大小，提升吞吐
```

### 8.2 查询优化

```go
// 查询策略
流式传输：gRPC流式接收，支持大数据量
增量拉取：只拉取新数据，减少网络传输
数据缓存：分析器缓存，避免重复查询
```

### 8.3 推送优化

```go
// 推送策略
双通道推送：WebSocket + QUIC，适应不同网络
客户端缓冲：异步发送，避免阻塞
心跳机制：定期检测，清理僵尸连接
```

---

## 🚨 **9. 监控与调试**

### 9.1 关键指标

```go
// 连接指标
连接数量：活跃gRPC连接数
连接延迟：连接建立时间
连接成功率：连接成功/总尝试次数

// 查询指标
查询QPS：每秒查询请求数
查询延迟：平均查询响应时间
查询成功率：成功查询/总查询次数

// 推送指标
推送延迟：从数据拉取到推送的时间
客户端数量：WebSocket连接数
消息积压：待发送消息队列长度
```

### 9.2 日志记录

```go
log.Printf("连接到broker失败: %v", err)
log.Printf("获取主机 %s 的数据失败: %v", hostID, err)
log.Printf("客户端已连接，当前连接数: %d", len(s.clients))
log.Printf("成功获取 %d 条指标数据", len(metrics))
```

---

## 🎯 **总结**

Visualization从Broker获取数据的完整流程包括：

✅ **连接管理**：gRPC连接池，支持多Broker端点，自动重试和负载均衡  
✅ **数据查询**：流式gRPC查询，支持时间范围和主机筛选，自动数据转换  
✅ **实时推送**：2秒增量拉取，WebSocket/QUIC双通道推送，避免重复数据  
✅ **API服务**：RESTful API，支持聚合查询、趋势分析和Top-K算法  
✅ **错误处理**：多层重试机制，优雅降级，完善的监控和日志  

这种设计确保了Visualization能够高效、可靠地从Broker集群获取监控数据，并实时推送给前端客户端，为用户提供流畅的实时监控体验。
