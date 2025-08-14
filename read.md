# 分布式实时监控系统 - 功能详细说明

## 🎯 项目概述

这是一个**分布式实时监控系统**，用于监控多台服务器的系统性能指标，并提供实时数据可视化和分析功能。系统采用微服务架构，具有高可用性、可扩展性和高性能特点。

## 🏗️ 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   数据采集代理    │    │  分布式消息中转层  │    │  可视化分析端    │
│   (Agent)       │    │   (Broker)      │    │ (Visualization) │
│                 │    │                 │    │                 │
│ • CPU监控        │───▶│ • 数据接收       │───▶│ • Web界面       │
│ • 内存监控       │    │ • 数据存储       │    │ • 实时图表       │
│ • 网络监控       │    │ • 负载均衡       │    │ • 告警管理       │
│ • 磁盘监控       │    │ • 高可用         │    │ • 数据分析       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 📊 各组件详细功能

### 1. 数据采集代理 (Agent)

**作用：** 部署在被监控的每台服务器上，负责采集系统性能指标

**主要功能：**
- **CPU监控**：采集CPU使用率、负载、进程数等
- **内存监控**：采集内存使用率、可用内存、交换分区等
- **网络监控**：采集网络流量、连接数、带宽使用等
- **磁盘监控**：采集磁盘使用率、I/O性能、读写速度等
- **主机注册**：向Broker注册主机信息，保持心跳连接
- **算法管理**：运行各种性能分析算法

**运行命令：**
```bash
./bin/agent -config configs/agent.yaml
```

**实际作用：**
- 每5秒采集一次系统指标
- 将数据通过gRPC发送到Broker
- 支持QUIC协议（可选）进行高性能传输
- 占用资源极少（CPU<1%，内存<50MB）

### 2. 分布式消息中转层 (Broker)

**作用：** 系统的核心组件，负责接收、存储、转发监控数据

**主要功能：**
- **数据接收**：接收来自多个Agent的数据
- **数据存储**：使用Redis存储监控数据
- **负载均衡**：使用一致性哈希算法进行数据分片
- **高可用**：使用Raft共识算法实现集群高可用
- **主机管理**：管理所有注册的主机，监控其健康状态
- **数据查询**：为Visualization提供数据查询服务

**运行命令：**
```bash
./bin/broker -config configs/broker.yaml
```

**实际作用：**
- 接收并存储来自所有Agent的数据
- 支持水平扩展，可以部署多个Broker节点
- 提供gRPC API供Visualization查询数据
- 支持QUIC协议（可选）进行高性能通信
- 每秒可处理10000+数据点

### 3. 可视化分析端 (Visualization)

**作用：** 提供Web界面，展示监控数据和进行数据分析

**主要功能：**
- **Web界面**：提供直观的监控数据展示
- **实时图表**：通过WebSocket推送实时数据
- **历史数据**：展示历史监控数据和趋势分析
- **告警管理**：配置和管理告警规则
- **数据分析**：使用Top-K算法进行性能分析
- **用户认证**：JWT认证和权限管理
- **QUIC支持**：支持QUIC协议进行高性能数据传输

**运行命令：**
```bash
./bin/visualization -config configs/visualization.yaml
```

**实际作用：**
- 提供HTTP API（端口8080）
- 提供WebSocket服务（实时数据推送）
- 提供QUIC服务（端口8081，可选）
- 支持100+客户端并发连接
- 页面加载时间<1秒

## 🔧 技术特性

### 高性能模块
- **Ring Buffer**：C语言实现的高性能循环缓冲区
- **Epoll服务器**：基于Linux epoll的高并发网络服务器
- **Top-K算法**：C++实现的高性能Top-K查询算法

### 通信协议
- **gRPC**：主要的RPC通信协议
- **QUIC**：新增的高性能传输协议（可选）
- **WebSocket**：实时数据推送
- **HTTP**：Web界面和API接口

### 存储和共识
- **Redis**：高性能内存数据库，存储监控数据
- **Raft**：分布式共识算法，保证数据一致性
- **一致性哈希**：数据分片和负载均衡

## 🚀 使用场景

### 1. 单机监控
```bash
# 启动所有组件
./bin/broker -config configs/broker.yaml &
./bin/agent -config configs/agent.yaml &
./bin/visualization -config configs/visualization.yaml &
```

### 2. 多机监控
```bash
# 在每台被监控机器上运行Agent
./bin/agent -config configs/agent.yaml

# 在中心服务器上运行Broker和Visualization
./bin/broker -config configs/broker.yaml &
./bin/visualization -config configs/visualization.yaml &
```

### 3. 集群部署
```bash
# 部署多个Broker节点实现高可用
./bin/broker -config configs/broker1.yaml &
./bin/broker -config configs/broker2.yaml &
./bin/broker -config configs/broker3.yaml &
```

## 📈 监控指标

### 系统指标
- **CPU使用率**：用户态、系统态、空闲时间
- **内存使用率**：物理内存、虚拟内存、交换分区
- **网络流量**：入站/出站字节数、包数、错误数
- **磁盘使用率**：读写字节数、IOPS、延迟

### 性能指标
- **响应时间**：系统响应延迟
- **吞吐量**：每秒处理请求数
- **错误率**：系统错误和异常
- **资源利用率**：各种资源的使用情况

## 🔍 访问方式

### Web界面
- **地址**：http://localhost:8080
- **功能**：实时监控图表、历史数据、告警配置

### API接口
- **状态检查**：GET /api/status
- **指标数据**：GET /api/metrics/{hostId}
- **WebSocket**：ws://localhost:8080/ws

### QUIC接口（可选）
- **端口**：8081
- **协议**：QUIC over UDP
- **优势**：低延迟、高吞吐量

## 🛠️ 配置说明

### Agent配置 (configs/agent.yaml)
```yaml
# 采集间隔
collector:
  interval: 5s

# Broker连接
broker:
  address: "localhost:9093"

# QUIC配置（可选）
quic:
  enable: false
  server_addr: "localhost:8081"
```

### Broker配置 (configs/broker.yaml)
```yaml
# 节点信息
node:
  id: "broker-1"
  address: "localhost:9093"

# Redis存储
storage:
  redis:
    addr: "localhost:6379"

# 集群配置
cluster:
  endpoints:
    - "localhost:9093"
```

### Visualization配置 (configs/visualization.yaml)
```yaml
# HTTP服务
server:
  port: 8080

# QUIC服务（可选）
quic:
  enable: true
  port: 8081

# Broker连接
broker:
  address: "localhost:9093"
```

## 🎯 实际应用价值

1. **运维监控**：实时监控服务器性能，及时发现异常
2. **容量规划**：分析历史数据，预测资源需求
3. **故障诊断**：快速定位性能瓶颈和系统问题
4. **性能优化**：基于监控数据进行系统调优
5. **成本控制**：优化资源使用，降低运维成本

这个系统特别适合：
- 中小型企业的服务器监控
- 云原生应用的性能监控
- 微服务架构的系统监控
- 需要实时数据展示的场景 