# 分布式系统监控平台

一个基于 Go 语言开发的高性能分布式系统监控平台，支持实时指标采集、存储、分析和可视化。

## 🚀 项目特性

### 核心功能
- **实时监控**: 采集 CPU、内存、网络、磁盘等系统指标
- **分布式架构**: Agent-Broker-Visualization 三层架构，支持水平扩展
- **高性能存储**: 基于 Redis 的时序数据存储，支持 TTL 和自动过期
- **实时推送**: WebSocket/QUIC 实时数据推送到前端
- **高可用**: Raft 共识算法保证集群一致性和故障恢复

### 技术亮点
- **高性能组件**: C/C++ 实现的 Ring Buffer 和 Top-K 算法
- **智能分片**: 一致性哈希算法实现数据分片和负载均衡
- **多协议支持**: gRPC、WebSocket、QUIC 多种通信协议
- **模块化设计**: 清晰的包结构，便于扩展和维护

## 📋 系统要求

- **Go**: 1.19 或更高版本
- **Redis**: 6.0 或更高版本
- **系统**: Linux/macOS (推荐 Linux)
- **编译器**: GCC (用于 C/C++ 组件)

## 🛠️ 安装部署

### 1. 克隆项目

```bash
git clone https://github.com/your-username/monitor.git
cd monitor
```

### 2. 安装依赖

```bash
# 安装 Go 依赖
go mod tidy

# 安装 Redis (Ubuntu/Debian)
sudo apt-get install redis-server

# 或者使用 Docker
docker run -d -p 6379:6379 redis:6.2-alpine
```

### 3. 编译项目

```bash
# 编译所有组件 (包括C/C++模块)
make all

# 或者仅编译Go组件
make build

# 单独编译各组件
make build-agent    # 编译Agent
make build-broker   # 编译Broker
make build-viz      # 编译Visualization

# 编译C/C++模块
make build-c        # 编译Ring Buffer等C模块
make build-cpp      # 编译Top-K等C++模块
```

### 4. 配置文件

项目已包含默认配置文件，可直接使用或根据需要修改：

```bash
# 查看配置文件
ls configs/
# agent.yaml  broker.yaml  visualization.yaml

# 根据环境修改配置
vim configs/broker.yaml       # 修改 Redis 连接信息
vim configs/agent.yaml        # 修改 Agent 配置
vim configs/visualization.yaml # 修改可视化服务配置
```

## 🚀 快速开始

### 单机部署

#### 方式一：一键启动（推荐）

```bash
# 启动 Redis
redis-server &

# 编译并启动所有服务
make start

# 访问 Web 界面
# 浏览器访问 http://localhost:8080
```

#### 方式二：分步启动

```bash
# 1. 启动 Redis
redis-server

# 2. 编译项目
make build

# 3. 分别启动各组件（推荐开启多个终端）
make run-broker     # 启动Broker
make run-agent      # 启动Agent  
make run-viz        # 启动Visualization

# 4. 访问 Web 界面
# 浏览器访问 http://localhost:8080
```

#### 服务管理命令

```bash
# 查看服务状态
make status

# 查看实时日志
make logs

# 停止所有服务
make stop
```

### 集群部署

#### Broker 集群

```bash
# 节点 1
./bin/broker -config configs/broker-node1.yaml

# 节点 2  
./bin/broker -config configs/broker-node2.yaml

# 节点 3
./bin/broker -config configs/broker-node3.yaml
```

#### 多 Agent 部署

```bash
# 在各个主机上启动 Agent
./bin/agent -config configs/agent.yaml
```

## 📊 使用说明

### Web 界面功能

- **实时指标**: 查看 CPU、内存、网络、磁盘使用率
- **Top-K 分析**: CPU 使用率前 5 排行榜
- **历史数据**: 时间序列图表展示
- **详细指标**: 所有采集指标的原始数据

### API 接口

#### 获取指标数据
```bash
curl "http://localhost:8080/api/metrics?host=host-1&start=1640995200&end=1640995800"
```

#### Top-K 分析
```bash
curl "http://localhost:8080/api/analysis/topk?metric=cpu_usage&k=5"
```

#### 系统状态
```bash
curl "http://localhost:8080/api/status"
```

### 配置说明

#### Agent 配置 (configs/agent.yaml)

```yaml
host:
  id: "host-1"
  name: "Web Server 1"

collect:
  interval: 5s
  metrics:
    - cpu
    - memory
    - network
    - disk

broker:
  address: "localhost:9090"
  timeout: 10s
```

#### Broker 配置 (configs/broker.yaml)

```yaml
node:
  id: "broker-1"
  address: "localhost:9090"

storage:
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    key_prefix: "monitor:"

raft:
  log_dir: "./data/raft/logs"
  snapshot_dir: "./data/raft/snapshots"
```

#### Visualization 配置 (configs/visualization.yaml)

```yaml
server:
  address: ":8080"
  
broker:
  addresses:
    - "localhost:9090"

websocket:
  enable: true
  
quic:
  enable: false
```

## 🏗️ 架构说明

### 组件架构

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Agent    │───▶│   Broker    │───▶│Visualization│
│   (采集)    │    │  (存储/路由) │    │  (展示)     │
└─────────────┘    └─────────────┘    └─────────────┘
                          │
                          ▼
                   ┌─────────────┐
                   │    Redis    │
                   │   (存储)    │
                   └─────────────┘
```

### 数据流

1. **Agent** 周期性采集系统指标
2. **Broker** 接收数据并存储到 Redis
3. **Visualization** 从 Broker 查询数据
4. **WebSocket** 实时推送到前端界面

### 目录结构

```
monitor/
├── agent/              # Agent 组件
├── broker/             # Broker 组件  
├── visualization/      # Visualization 组件
├── pkg/                # 共享包
│   ├── storage/        # 存储抽象层
│   ├── hash/           # 一致性哈希
│   ├── queue/          # 消息队列
│   └── algorithm/      # 通用算法
├── proto/              # gRPC 协议定义
├── configs/            # 配置文件
└── static/             # 前端静态资源
```

## 🔧 开发指南

### 本地开发

```bash
# 查看所有可用命令
make help

# 启动开发环境（清理+编译）
make dev

# 运行测试
make test

# 测试C/C++模块
make test-c-cpp

# 代码格式化
make fmt

# 代码检查（需要安装golangci-lint）
make lint

# 清理构建产物
make clean
```

### 测试 Redis 连接

```bash
go run test_redis.go
```

### 添加新指标

1. 在 `agent/internal/collector/` 添加新的采集器
2. 在 `proto/monitor.proto` 中定义新的指标类型
3. 更新前端展示逻辑

## 📈 性能特性

- **高吞吐**: C 实现的 Ring Buffer 支持高频数据采集
- **低延迟**: Top-K 算法毫秒级响应时间
- **可扩展**: 一致性哈希支持节点动态增减
- **高可用**: Raft 共识保证 99.9% 可用性

## 🔍 监控指标

### 系统指标
- **CPU**: 使用率、负载均衡
- **内存**: 使用量、缓存、交换分区
- **网络**: 流量、包数、错误率
- **磁盘**: 使用率、读写速度、IOPS

### 应用指标
- **连接数**: 活跃连接、连接池状态
- **响应时间**: API 响应延迟分布
- **错误率**: 4xx/5xx 错误统计

## 🛡️ 安全说明

- JWT 认证（可配置开启/关闭）
- Redis 连接密码保护
- gRPC TLS 加密（可选）
- 配置文件敏感信息保护

## 📝 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 🔖 版本历史

- **v1.0.0** - 初始版本，基础监控功能
- **v1.1.0** - 增加 Top-K 分析功能
- **v1.2.0** - 支持集群部署和高可用

---

⭐ 如果这个项目对你有帮助，请给个 Star！
