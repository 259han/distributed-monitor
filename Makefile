.PHONY: all build clean proto test run-agent run-broker run-viz build-agent build-broker build-viz build-c build-cpp test-c-cpp dev fmt lint help

all: build build-c build-cpp

build: build-agent build-broker build-viz

proto:
	@echo "生成Protocol Buffers代码..."
	@mkdir -p github.com/han-fei/monitor/proto
	@protoc --go_out=. --go-grpc_out=. proto.bak/monitor.proto

build-agent: proto build-c
	@echo "构建数据采集代理..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -buildvcs=false -tags cgo -ldflags "-extldflags '-L./bin/lib -lringbuffer'" -o bin/agent ./agent/cmd

build-agent-integrated: proto build-c
	@echo "构建集成版数据采集代理..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -buildvcs=false -tags cgo -o bin/agent-integrated ./agent/cmd

build-broker: proto
	@echo "构建分布式消息中转层..."
	@mkdir -p bin
	@go build -buildvcs=false -o bin/broker ./broker/cmd

build-viz: proto build-cpp
	@echo "构建可视化分析端..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -buildvcs=false -tags cgo -ldflags "-extldflags '-L./bin/lib -lstreaming_topk -lstdc++ -lpthread'" -o bin/visualization ./visualization/cmd

run-agent:
	@echo "运行数据采集代理..."
	@mkdir -p logs
	@go run ./agent/cmd/main.go --config configs/agent.yaml

run-broker:
	@echo "运行分布式消息中转层..."
	@mkdir -p logs data/raft/logs data/raft/snapshots
	@go run ./broker/cmd/main.go --config configs/broker.yaml

run-viz:
	@echo "运行可视化分析端..."
	@mkdir -p logs static
	@go run ./visualization/cmd/main.go --config configs/visualization.yaml

test:
	@echo "运行测试..."
	@go test -v ./...

# C/C++模块构建
build-c:
	@echo "构建C语言模块..."
	@mkdir -p bin/c bin/lib
	@echo "编译epoll服务器..."
	@gcc -Wall -Wextra -O2 -pthread -DCOMPILE_STANDALONE -o bin/c/epoll_server ./agent/internal/c/epoll_server.c ./agent/internal/c/epoll_main.c
	@echo "编译ring buffer测试..."
	@gcc -Wall -Wextra -O2 -pthread -DCOMPILE_TEST -o bin/c/ring_buffer_test ./agent/internal/c/ring_buffer.c ./agent/internal/c/ring_buffer_test.c
	@echo "编译C ring buffer共享库..."
	@gcc -shared -fPIC -Wall -Wextra -O2 -pthread -o bin/lib/libringbuffer.so ./agent/internal/c/ring_buffer.c
	
build-cpp:
	@echo "构建C++模块..."
	@mkdir -p bin/cpp bin/lib
	@echo "编译流式Top-K模块..."
	@cd ./visualization/internal/cpp && make all
	@cp ./visualization/internal/cpp/libstreaming_topk.* ./bin/lib/
	@echo "C++流式Top-K模块构建完成"

test-c-cpp:
	@echo "测试C/C++模块..."
	@./test/test_c_cpp_modules.sh

clean:
	@echo "清理构建产物..."
	@rm -rf bin/
	@rm -rf proto/github.com/
	@find . -name "*.pb.go" -delete
	@find . -name "*.o" -delete

# 开发相关命令
dev: clean build
	@echo "开发环境准备完成"

fmt:
	@echo "格式化Go代码..."
	@go fmt ./...
	@echo "代码格式化完成"

lint:
	@echo "代码静态检查..."
	@which golangci-lint > /dev/null || (echo "请安装 golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@golangci-lint run ./...
	@echo "代码检查完成"

# 快速启动完整环境
start: build
	@echo "启动完整监控环境..."
	@echo "请确保Redis已启动 (redis-server)"
	@echo "启动Broker..."
	@nohup ./bin/broker -config configs/broker.yaml > logs/broker.log 2>&1 &
	@sleep 2
	@echo "启动Agent..."
	@nohup ./bin/agent -config configs/agent.yaml > logs/agent.log 2>&1 &
	@sleep 2
	@echo "启动Visualization..."
	@nohup ./bin/visualization -config configs/visualization.yaml > logs/visualization.log 2>&1 &
	@echo "服务启动完成! 访问 http://localhost:8080"

# 停止所有服务
stop:
	@echo "停止所有服务..."
	@pkill -f "bin/broker" || true
	@pkill -f "bin/agent" || true  
	@pkill -f "bin/visualization" || true
	@echo "服务已停止"

# 查看服务状态
status:
	@echo "服务运行状态:"
	@ps aux | grep -E "(bin/broker|bin/agent|bin/visualization)" | grep -v grep || echo "没有运行的服务"

# 查看日志
logs:
	@echo "查看日志 (按Ctrl+C退出):"
	@tail -f logs/*.log

help:
	@echo "可用命令:"
	@echo "  build          - 编译所有组件"
	@echo "  build-agent    - 编译Agent"
	@echo "  build-broker   - 编译Broker"  
	@echo "  build-viz      - 编译Visualization"
	@echo "  build-c        - 编译C模块"
	@echo "  build-cpp      - 编译C++模块"
	@echo "  run-agent      - 运行Agent (前台)"
	@echo "  run-broker     - 运行Broker (前台)"
	@echo "  run-viz        - 运行Visualization (前台)"
	@echo "  start          - 启动完整环境 (后台)"
	@echo "  stop           - 停止所有服务"
	@echo "  status         - 查看服务状态"
	@echo "  logs           - 查看实时日志"
	@echo "  test           - 运行测试"
	@echo "  test-c-cpp     - 测试C/C++模块"
	@echo "  dev            - 开发环境准备"
	@echo "  fmt            - 格式化代码"
	@echo "  lint           - 代码检查"
	@echo "  clean          - 清理构建产物"
	@echo "  help           - 显示帮助信息" 