.PHONY: all build clean proto test run-agent run-broker run-viz build-agent build-broker build-viz build-c build-cpp test-c-cpp

all: build build-c build-cpp

build: build-agent build-broker build-viz

proto:
	@echo "生成Protocol Buffers代码..."
	@mkdir -p github.com/han-fei/monitor/proto
	@protoc --go_out=. --go-grpc_out=. proto.bak/monitor.proto

build-agent: proto build-c
	@echo "构建数据采集代理..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -tags cgo -ldflags "-extldflags '-L./bin/lib -lringbuffer'" -o bin/agent ./agent/cmd

build-agent-integrated: proto build-c
	@echo "构建集成版数据采集代理..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -tags cgo -o bin/agent-integrated ./agent/cmd

build-broker: proto
	@echo "构建分布式消息中转层..."
	@mkdir -p bin
	@go build -o bin/broker ./broker/cmd

build-viz: proto build-cpp
	@echo "构建可视化分析端..."
	@mkdir -p bin
	@export LD_LIBRARY_PATH=./bin/lib:$$LD_LIBRARY_PATH && go build -tags cgo -o bin/visualization ./visualization/cmd

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
	@echo "编译libevent服务器(跳过，需要libevent-dev)..."
	@echo "# 跳过libevent编译，需要安装libevent-dev"

build-cpp:
	@echo "构建C++模块..."
	@mkdir -p bin/cpp bin/lib
	@echo "编译Top-K模块..."
	@g++ -Wall -Wextra -O2 -std=c++11 -pthread -DCOMPILE_TEST -o bin/cpp/topk_test ./visualization/internal/cpp/topk_shared.cpp ./visualization/internal/cpp/topk_test.cpp -lrt
	@echo "编译C++ Top-K共享库..."
	@g++ -shared -fPIC -Wall -Wextra -O2 -std=c++11 -pthread -o bin/lib/libtopk.so ./visualization/internal/cpp/topk_shared.cpp ./visualization/internal/cpp/topk_c_wrapper.cpp -lrt

test-c-cpp:
	@echo "测试C/C++模块..."
	@./test/test_c_cpp_modules.sh

clean:
	@echo "清理构建产物..."
	@rm -rf bin/
	@rm -rf proto/github.com/
	@find . -name "*.pb.go" -delete
	@find . -name "*.o" -delete 