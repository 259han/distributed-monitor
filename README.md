# 分布式实时监控系统

## 系统架构

该系统由三个主要组件构成：

1. **数据采集代理（Agent）**：部署在每台被监控主机上，负责采集主机的各项指标数据，如CPU使用率、内存使用情况、网络流量、磁盘I/O等。
2. **分布式消息中转层（Broker）**：接收来自各个数据采集代理的数据，进行数据分片存储和管理，实现高可用和水平扩展。
3. **可视化分析端（Visualization）**：提供Web界面，展示实时监控数据和历史数据，支持告警配置和管理。

## 项目结构

```
monitor/
├── agent/               # 数据采集代理
│   ├── cmd/             # 命令行入口
│   └── internal/        # 内部实现
│       ├── algorithm/   # 算法实现
│       ├── collector/   # 数据采集器
│       ├── config/      # 配置管理
│       ├── models/      # 数据模型
│       └── service/     # 服务实现
├── broker/              # 分布式消息中转层
│   ├── cmd/             # 命令行入口
│   └── internal/        # 内部实现
│       ├── config/      # 配置管理
│       ├── hash/        # 一致性哈希
│       ├── models/      # 数据模型
│       ├── raft/        # Raft共识
│       ├── service/     # 服务实现
│       └── storage/     # 存储接口
├── visualization/       # 可视化分析端
│   ├── cmd/             # 命令行入口
│   ├── internal/        # 内部实现
│   │   ├── auth/        # 认证授权
│   │   ├── config/      # 配置管理
│   │   ├── models/      # 数据模型
│   │   ├── quic/        # QUIC协议
│   │   ├── radix/       # 基数树
│   │   ├── service/     # 服务实现
│   │   ├── topk/        # Top-K算法
│   │   └── websocket/   # WebSocket服务
│   └── static/          # 静态资源
├── proto/               # Protocol Buffers定义
├── configs/             # 配置文件
│   ├── agent.yaml       # 数据采集代理配置
│   ├── broker.yaml      # 分布式消息中转层配置
│   └── visualization.yaml # 可视化分析端配置
├── Makefile             # 构建脚本
├── go.mod               # Go模块定义
└── README.md            # 项目说明
```

## 开发环境

- Go 1.21+
- Redis 7.0+
- Protocol Buffers 3.21+
- 支持Linux系统（Ubuntu 22.04+推荐）

## 编译与运行

### 安装依赖

```bash
# 安装Protocol Buffers编译器
sudo apt install -y protobuf-compiler

# 安装Go的Protocol Buffers插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 安装Redis（如果尚未安装）
sudo apt install -y redis-server
```

### 生成Protocol Buffers代码

```bash
make proto
```

### 编译所有组件

```bash
make build
```

### 编译单个组件

```bash
make build-agent    # 编译数据采集代理
make build-broker   # 编译分布式消息中转层
make build-viz      # 编译可视化分析端
make build-c        # 编译C语言模块
make build-cpp      # 编译C++模块
```

### C/C++高性能模块

系统集成了多个C/C++高性能模块：

- **Ring Buffer**: 高性能循环缓冲区，用于数据采集代理的数据传输
- **Epoll服务器**: 基于Linux epoll的高性能网络服务器，支持高并发连接
- **Top-K算法**: 高性能Top-K查询算法，用于可视化分析端的数据分析

这些模块通过CGO与Go代码集成，提供卓越的性能表现。

### 运行组件

```bash
# 运行数据采集代理
make run-agent

# 运行分布式消息中转层
make run-broker

# 运行可视化分析端
make run-viz
```

### 清理构建产物

```bash
make clean
```

## 性能指标

- 数据采集代理：每秒可采集100+项指标，CPU占用<1%，内存占用<50MB
- 分布式消息中转层：单节点每秒可处理10000+数据点，支持水平扩展
- 可视化分析端：支持100+客户端并发连接，页面加载时间<1秒

## API文档

### 数据采集代理API

- `SendMetrics`：发送采集的指标数据到消息中转层
  - 请求：流式发送`MetricsData`对象
  - 响应：`MetricsResponse`对象，包含成功/失败状态

### 分布式消息中转层API

- `GetMetrics`：获取指定主机的指标数据
  - 请求：`MetricsRequest`对象，包含主机ID、时间范围
  - 响应：流式返回`MetricsData`对象

### 可视化分析端API

- `GET /api/status`：获取系统状态
- `WebSocket /ws`：实时数据推送
- `GET /api/metrics/{hostId}`：获取指定主机的指标数据

## 贡献指南

1. Fork本仓库
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'Add some amazing feature'`
4. 推送到分支：`git push origin feature/amazing-feature`
5. 提交Pull Request 