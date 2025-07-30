.PHONY: all build clean proto test run-agent run-broker run-viz build-agent build-broker build-viz

# 默认目标
all: build

# 构建所有组件
build: build-agent build-broker build-viz

# 生成Protocol Buffers代码
proto:
	@echo "生成Protocol Buffers代码..."
	@mkdir -p proto/github.com/han-fei/monitor/proto
	@protoc --go_out=. --go-grpc_out=. proto/monitor.proto

# 构建数据采集代理
build-agent: proto
	@echo "构建数据采集代理..."
	@mkdir -p bin
	@go build -o bin/agent ./agent/cmd

# 构建分布式消息中转层
build-broker: proto
	@echo "构建分布式消息中转层..."
	@mkdir -p bin
	@go build -o bin/broker ./broker/cmd

# 构建可视化分析端
build-viz: proto
	@echo "构建可视化分析端..."
	@mkdir -p bin
	@go build -o bin/visualization ./visualization/cmd

# 运行数据采集代理
run-agent:
	@echo "运行数据采集代理..."
	@mkdir -p logs
	@go run ./agent/cmd/main.go --config configs/agent.yaml

# 运行分布式消息中转层
run-broker:
	@echo "运行分布式消息中转层..."
	@mkdir -p logs data/raft/logs data/raft/snapshots
	@go run ./broker/cmd/main.go --config configs/broker.yaml

# 运行可视化分析端
run-viz:
	@echo "运行可视化分析端..."
	@mkdir -p logs static
	@go run ./visualization/cmd/main.go --config configs/visualization.yaml

# 运行测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf bin/
	@rm -rf proto/github.com/
	@find . -name "*.pb.go" -delete 