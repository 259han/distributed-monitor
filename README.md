# Distributed System Monitoring Platform

[English](README.md) | [中文版本](README_zh.md)

A high-performance distributed system monitoring platform built with Go, supporting real-time metrics collection, storage, analysis, and visualization.

## 🚀 Features

### Core Functions
- **Real-time Monitoring**: Collect CPU, memory, network, disk and other system metrics
- **Distributed Architecture**: Agent-Broker-Visualization three-tier architecture with horizontal scaling
- **High-performance Storage**: Redis-based time-series data storage with TTL and auto-expiration
- **Real-time Push**: WebSocket/QUIC real-time data push to frontend
- **High Availability**: Raft consensus algorithm ensures cluster consistency and fault recovery

### Technical Highlights
- **Hybrid Architecture**: Go+C/C++ hybrid design, C-layer lock-free queue, C++ streaming Top-K algorithm
- **True Lock-free Queue**: C-layer CAS atomic operations, removed Go-layer mutex, Ring Buffer 2000 entries
- **Smart Sharding**: Consistent hashing algorithm for data sharding and load balancing
- **Multi-protocol Support**: gRPC(9093), WebSocket, QUIC(8081) multiple communication protocols

## 📋 System Requirements

- **Go**: 1.19 or higher
- **Redis**: 6.0 or higher
- **System**: Linux/macOS (Linux recommended)
- **Compiler**: GCC (for C/C++ components)

## 🛠️ Installation and Deployment

### 1. Clone Project

```bash
git clone https://github.com/your-username/monitor.git
cd monitor
```

### 2. Install Dependencies

```bash
# Install Go dependencies
go mod tidy

# Install Redis (Ubuntu/Debian)
sudo apt-get install redis-server

# Or use Docker
docker run -d -p 6379:6379 redis:6.2-alpine
```

### 3. Build Project

```bash
# Build all components (including C/C++ modules)
make all

# Or build Go components only
make build

# Build individual components
make build-agent    # Build Agent
make build-broker   # Build Broker
make build-viz      # Build Visualization

# Build C/C++ modules
make build-c        # Build Ring Buffer and other C modules
make build-cpp      # Build Top-K and other C++ modules
```

### 4. Configuration Files

The project includes default configuration files that can be used directly or modified as needed:

```bash
# View configuration files
ls configs/
# agent.yaml  broker.yaml  visualization.yaml

# Modify configurations for your environment
vim configs/broker.yaml       # Modify Redis connection info
vim configs/agent.yaml        # Modify Agent configuration
vim configs/visualization.yaml # Modify visualization service configuration
```

## 🚀 Quick Start

### Single Machine Deployment

#### Method 1: One-click Start (Recommended)

```bash
# Start Redis
redis-server &

# Build and start all services
make start

# Access Web Interface
# Browser: http://localhost:8080
```

#### Method 2: Step-by-step Start

```bash
# 1. Start Redis
redis-server

# 2. Build project
make build

# 3. Start components separately (recommended to open multiple terminals)
make run-broker     # Start Broker
make run-agent      # Start Agent  
make run-viz        # Start Visualization

# 4. Access Web Interface
# Browser: http://localhost:8080
```

#### Service Management Commands

```bash
# Check service status
make status

# View real-time logs
make logs

# Stop all services
make stop
```

### Cluster Deployment

#### Broker Cluster

```bash
# Modify node information in configuration files, then start multiple nodes
# Node 1 (modify configs/broker.yaml)
./bin/broker -config configs/broker.yaml

# Node 2 (need to copy and modify configuration)
cp configs/broker.yaml configs/broker-node2.yaml
# Modify node.id and node.address
./bin/broker -config configs/broker-node2.yaml
```

#### Multi-Agent Deployment

```bash
# Start Agent on each host (modify host_id)
./bin/agent -config configs/agent.yaml
```

## 📊 Usage Guide

### Web Interface Features

- **Real-time Metrics**: View CPU, memory, network, disk utilization
- **Top-K Analysis**: CPU usage top 5 leaderboard
- **Historical Data**: Time series chart display
- **Detailed Metrics**: Raw data of all collected metrics

### API Interfaces

#### Get Metrics Data
```bash
curl "http://localhost:8080/api/metrics?host=host-1&start=1640995200&end=1640995800"
```

#### Top-K Analysis
```bash
curl "http://localhost:8080/api/analysis/topk?metric=cpu_usage&k=5"
```

#### System Status
```bash
curl "http://localhost:8080/api/status"
```

### Configuration Guide

#### Agent Configuration (configs/agent.yaml)

```yaml
agent:
  host_id: "host-1"
  hostname: "localhost"
  ip: "127.0.0.1"

collect:
  interval: 1s
  batch_size: 100
  metrics:
    - cpu
    - memory
    - network
    - disk

broker:
  endpoints:
    - "localhost:9093"
  timeout: 5s
```

#### Broker Configuration (configs/broker.yaml)

```yaml
node:
  id: "broker-1"
  address: "localhost:9095"

grpc:
  port: 9093

storage:
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
    key_prefix: "monitor:"

raft:
  log_dir: "data/raft/logs"
  snapshot_dir: "data/raft/snapshots"
```

#### Visualization Configuration (configs/visualization.yaml)

```yaml
server:
  host: "localhost"
  port: 8080
  
broker:
  endpoints:
    - "localhost:9093"

websocket:
  enable: true
  
quic:
  enable: true
  port: 8081
```

## 🏗️ Architecture Overview

### Component Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Agent    │───▶│   Broker    │───▶│Visualization│
│ (Collection)│    │(Storage/Route)│   │ (Display)   │
└─────────────┘    └─────────────┘    └─────────────┘
                          │
                          ▼
                   ┌─────────────┐
                   │    Redis    │
                   │  (Storage)  │
                   └─────────────┘
```

### Data Flow

1. **Agent** periodically collects system metrics
2. **Broker** receives data and stores it in Redis
3. **Visualization** queries data from Broker
4. **WebSocket** pushes real-time data to frontend interface

### Directory Structure

```
monitor/
├── agent/              # Agent component
├── broker/             # Broker component  
├── visualization/      # Visualization component
├── pkg/                # Shared packages
│   ├── storage/        # Storage abstraction layer
│   ├── hash/           # Consistent hashing
│   ├── queue/          # Message queue
│   └── algorithm/      # Common algorithms
├── proto/              # gRPC protocol definitions
├── configs/            # Configuration files
└── static/             # Frontend static resources
```

## 🔧 Development Guide

### Local Development

```bash
# View all available commands
make help

# Start development environment (clean+build)
make dev

# Run tests
make test

# Test C/C++ modules
make test-c-cpp

# Code formatting
make fmt

# Code linting (requires golangci-lint)
make lint

# Clean build artifacts
make clean
```

### Test Connections

```bash
# Test Redis connection
redis-cli ping

# Run project tests
make test
```

### Adding New Metrics

1. Add new collector in `agent/internal/collector/`
2. Define new metric types in `proto/monitor.proto`
3. Update frontend display logic

## 📈 Performance Features

- **High-frequency Collection**: 1-second interval collection, batch processing of 100 entries, Ring Buffer 2000 entries
- **Lock-free Design**: C-layer CAS atomic operations, true lock-free queue, avoiding lock contention
- **Distributed Consistency**: Raft protocol cluster consensus, consistent hashing data sharding
- **Real-time Transmission**: gRPC streaming transmission, WebSocket/QUIC real-time push

## 🔍 Monitoring Metrics

### System Metrics
- **CPU**: Usage rate, load balancing
- **Memory**: Usage, cache, swap partition
- **Network**: Traffic, packet count, error rate
- **Disk**: Usage rate, read/write speed, IOPS

### Application Metrics
- **Connections**: Active connections, connection pool status
- **Response Time**: API response latency distribution
- **Error Rate**: 4xx/5xx error statistics

## 🛡️ Security Notes

- JWT authentication (configurable enable/disable)
- Redis connection password protection
- gRPC TLS encryption (optional)
- Configuration file sensitive information protection

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.


## 🔖 Version History

- **v1.0.0** - Initial version, basic monitoring features
- **v1.1.0** - Added Top-K analysis features
- **v1.2.0** - Support for cluster deployment and high availability

---

⭐ If this project helps you, please give it a Star!