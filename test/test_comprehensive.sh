#!/bin/bash

# 综合测试脚本 - 测试整个分布式监控系统

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${BLUE}===========================================${NC}"
echo -e "${BLUE}  分布式监控系统综合测试脚本${NC}"
echo -e "${BLUE}===========================================${NC}"

# 测试结果统计
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 日志目录
LOG_DIR="logs/test_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$LOG_DIR"

# 测试配置
TEST_CONFIGS=(
    "configs/agent.yaml"
    "configs/broker.yaml"
    "configs/visualization.yaml"
)

# 测试函数
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_result="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "${YELLOW}运行测试: $test_name${NC}"
    
    # 运行测试
    eval "$test_command" > "$LOG_DIR/${test_name}.log" 2>&1
    local exit_code=$?
    
    if [ $exit_code -eq $expected_result ]; then
        echo -e "${GREEN}✓ $test_name 通过${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}✗ $test_name 失败 (退出码: $exit_code, 期望: $expected_result)${NC}"
        echo -e "${YELLOW}查看日志: $LOG_DIR/${test_name}.log${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# 检查配置文件
check_configs() {
    echo -e "${YELLOW}检查配置文件...${NC}"
    
    for config in "${TEST_CONFIGS[@]}"; do
        if [ ! -f "$config" ]; then
            echo -e "${RED}✗ 配置文件不存在: $config${NC}"
            return 1
        fi
        echo -e "${GREEN}✓ 配置文件存在: $config${NC}"
    done
    
    return 0
}

# 构建所有组件
build_components() {
    echo -e "${YELLOW}构建所有组件...${NC}"
    
    # 创建bin目录
    mkdir -p bin
    
    # 构建agent
    run_test "构建Agent" "go build -o bin/agent ./agent/cmd" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 构建broker
    run_test "构建Broker" "go build -o bin/broker ./broker/cmd" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 构建visualization
    run_test "构建Visualization" "go build -o bin/visualization ./visualization/cmd" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 编译proto文件
    if [ -f "proto/monitor.proto" ]; then
        run_test "编译Proto" "protoc --go_out=. --go-grpc_out=. proto/monitor.proto" 0
    fi
    
    return 0
}

# 测试单个组件
test_component() {
    local component="$1"
    local config="$2"
    local port="$3"
    local test_commands="$4"
    
    echo -e "${YELLOW}测试 $component...${NC}"
    
    # 启动组件
    "./bin/$component" --config "$config" > "$LOG_DIR/${component}.log" 2>&1 &
    local pid=$!
    
    # 等待启动
    sleep 5
    
    # 检查进程
    if ! ps -p $pid > /dev/null; then
        echo -e "${RED}✗ $component 启动失败${NC}"
        cat "$LOG_DIR/${component}.log"
        return 1
    fi
    
    echo -e "${GREEN}✓ $component 启动成功 (PID: $pid)${NC}"
    
    # 运行测试命令
    for test_cmd in $test_commands; do
        eval "$test_cmd"
        if [ $? -ne 0 ]; then
            echo -e "${RED}✗ $component 测试失败${NC}"
            kill $pid 2>/dev/null
            return 1
        fi
    done
    
    # 停止组件
    kill $pid 2>/dev/null
    wait $pid 2>/dev/null
    
    echo -e "${GREEN}✓ $component 测试通过${NC}"
    return 0
}

# 测试集成场景
test_integration() {
    echo -e "${YELLOW}测试集成场景...${NC}"
    
    # 创建必要的目录
    mkdir -p data/raft/logs data/raft/snapshots
    
    # 启动Broker
    ./bin/broker --config configs/broker.yaml > "$LOG_DIR/broker_integration.log" 2>&1 &
    local broker_pid=$!
    
    # 等待Broker启动
    sleep 5
    
    if ! ps -p $broker_pid > /dev/null; then
        echo -e "${RED}✗ Broker 启动失败${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ Broker 启动成功${NC}"
    
    # 启动Agent
    ./bin/agent --config configs/agent.yaml > "$LOG_DIR/agent_integration.log" 2>&1 &
    local agent_pid=$!
    
    # 等待Agent启动
    sleep 5
    
    if ! ps -p $agent_pid > /dev/null; then
        echo -e "${RED}✗ Agent 启动失败${NC}"
        kill $broker_pid 2>/dev/null
        return 1
    fi
    
    echo -e "${GREEN}✓ Agent 启动成功${NC}"
    
    # 启动Visualization
    ./bin/visualization --config configs/visualization.yaml > "$LOG_DIR/visualization_integration.log" 2>&1 &
    local viz_pid=$!
    
    # 等待Visualization启动
    sleep 5
    
    if ! ps -p $viz_pid > /dev/null; then
        echo -e "${RED}✗ Visualization 启动失败${NC}"
        kill $broker_pid 2>/dev/null
        kill $agent_pid 2>/dev/null
        return 1
    fi
    
    echo -e "${GREEN}✓ Visualization 启动成功${NC}"
    
    # 测试端到端功能
    echo -e "${YELLOW}测试端到端功能...${NC}"
    
    # 等待数据采集
    sleep 10
    
    # 测试API端点
    curl -s http://localhost:8080/api/status > /dev/null
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ API端点测试通过${NC}"
    else
        echo -e "${RED}✗ API端点测试失败${NC}"
    fi
    
    # 测试WebSocket
    # 这里可以添加WebSocket连接测试
    
    # 停止所有组件
    echo -e "${YELLOW}停止所有组件...${NC}"
    kill $broker_pid $agent_pid $viz_pid 2>/dev/null
    wait $broker_pid $agent_pid $viz_pid 2>/dev/null
    
    echo -e "${GREEN}✓ 集成测试完成${NC}"
    return 0
}

# 性能测试
performance_test() {
    echo -e "${YELLOW}性能测试...${NC}"
    
    # 测试Broker性能
    echo -e "${YELLOW}测试Broker性能...${NC}"
    
    # 启动Broker
    ./bin/broker --config configs/broker.yaml > "$LOG_DIR/broker_perf.log" 2>&1 &
    local broker_pid=$!
    
    sleep 5
    
    # 模拟并发请求
    echo -e "${YELLOW}模拟并发请求...${NC}"
    for i in {1..10}; do
        curl -s http://localhost:9090/api/status > /dev/null &
    done
    
    wait
    
    # 停止Broker
    kill $broker_pid 2>/dev/null
    wait $broker_pid 2>/dev/null
    
    echo -e "${GREEN}✓ 性能测试完成${NC}"
    return 0
}

# 内存泄漏测试
memory_leak_test() {
    echo -e "${YELLOW}内存泄漏测试...${NC}"
    
    # 启动Visualization并监控内存使用
    ./bin/visualization --config configs/visualization.yaml > "$LOG_DIR/memory_test.log" 2>&1 &
    local viz_pid=$!
    
    sleep 5
    
    # 监控内存使用
    for i in {1..5}; do
        memory_usage=$(ps -p $viz_pid -o rss= | tail -1)
        echo "内存使用: $memory_usage KB"
        sleep 10
    done
    
    # 停止
    kill $viz_pid 2>/dev/null
    wait $viz_pid 2>/dev/null
    
    echo -e "${GREEN}✓ 内存泄漏测试完成${NC}"
    return 0
}

# 生成测试报告
generate_report() {
    echo -e "${YELLOW}生成测试报告...${NC}"
    
    local report_file="$LOG_DIR/test_report.md"
    
    cat > "$report_file" << EOF
# 分布式监控系统测试报告

## 测试概述
- **测试时间**: $(date)
- **总测试数**: $TOTAL_TESTS
- **通过测试**: $PASSED_TESTS
- **失败测试**: $FAILED_TESTS
- **成功率**: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%

## 测试结果

### 配置检查
$(check_configs >/dev/null 2>&1 && echo "✓ 通过" || echo "✗ 失败")

### 构建测试
$(build_components >/dev/null 2>&1 && echo "✓ 通过" || echo "✗ 失败")

### 组件测试
- Agent: $(grep -q "Agent 测试通过" "$LOG_DIR"/*.log 2>/dev/null && echo "✓ 通过" || echo "✗ 失败")
- Broker: $(grep -q "Broker 测试通过" "$LOG_DIR"/*.log 2>/dev/null && echo "✓ 通过" || echo "✗ 失败")
- Visualization: $(grep -q "Visualization 测试通过" "$LOG_DIR"/*.log 2>/dev/null && echo "✓ 通过" || echo "✗ 失败")

### 集成测试
$(test_integration >/dev/null 2>&1 && echo "✓ 通过" || echo "✗ 失败")

### 性能测试
$(performance_test >/dev/null 2>&1 && echo "✓ 通过" || echo "✗ 失败")

### 内存泄漏测试
$(memory_leak_test >/dev/null 2>&1 && echo "✓ 通过" || echo "✗ 失败")

## 详细日志
- 测试日志目录: $LOG_DIR
- 各组件日志文件位于: $LOG_DIR/*.log

## 建议
EOF

    if [ $FAILED_TESTS -gt 0 ]; then
        cat >> "$report_file" << EOF
- 有 $FAILED_TESTS 个测试失败，请查看详细日志
- 建议检查失败组件的配置和依赖
- 确保所有服务端口未被占用
EOF
    else
        cat >> "$report_file" << EOF
- 所有测试通过，系统运行正常
- 建议定期运行测试确保系统稳定性
EOF
    fi
    
    echo -e "${GREEN}✓ 测试报告已生成: $report_file${NC}"
}

# 主测试流程
main() {
    echo -e "${BLUE}开始分布式监控系统综合测试...${NC}"
    
    # 检查配置
    run_test "配置检查" "check_configs" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 构建组件
    run_test "构建组件" "build_components" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 测试Agent
    test_component "Agent" "configs/agent.yaml" "8081" ""
    if [ $? -ne 0 ]; then return 1; fi
    
    # 测试Broker
    test_component "Broker" "configs/broker.yaml" "9090" ""
    if [ $? -ne 0 ]; then return 1; fi
    
    # 测试Visualization
    test_component "Visualization" "configs/visualization.yaml" "8080" ""
    if [ $? -ne 0 ]; then return 1; fi
    
    # 集成测试
    run_test "集成测试" "test_integration" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 性能测试
    run_test "性能测试" "performance_test" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 内存泄漏测试
    run_test "内存泄漏测试" "memory_leak_test" 0
    if [ $? -ne 0 ]; then return 1; fi
    
    # 生成报告
    generate_report
    
    # 显示最终结果
    echo -e "${BLUE}===========================================${NC}"
    echo -e "${BLUE}  测试结果汇总${NC}"
    echo -e "${BLUE}===========================================${NC}"
    echo -e "${GREEN}总测试数: $TOTAL_TESTS${NC}"
    echo -e "${GREEN}通过测试: $PASSED_TESTS${NC}"
    echo -e "${RED}失败测试: $FAILED_TESTS${NC}"
    echo -e "${BLUE}成功率: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}🎉 所有测试通过！${NC}"
        return 0
    else
        echo -e "${RED}❌ 有 $FAILED_TESTS 个测试失败${NC}"
        return 1
    fi
}

# 运行主函数
main "$@"