#!/bin/bash

# 测试可视化分析端的脚本

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${YELLOW}开始测试可视化分析端...${NC}"

# 检查配置文件是否存在
if [ ! -f "configs/visualization.yaml" ]; then
    echo -e "${RED}错误: 配置文件 configs/visualization.yaml 不存在${NC}"
    exit 1
fi

# 检查静态文件是否存在
if [ ! -f "static/index.html" ]; then
    echo -e "${YELLOW}警告: 静态文件 static/index.html 不存在，创建目录...${NC}"
    mkdir -p static
fi

# 创建必要的目录
mkdir -p logs data/raft/logs data/raft/snapshots static

# 构建可视化分析端
echo -e "${YELLOW}构建可视化分析端...${NC}"
go build -o bin/visualization ./visualization/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建成功${NC}"

# 构建Broker（Visualization需要连接到Broker）
echo -e "${YELLOW}构建Broker...${NC}"
go build -o bin/broker ./broker/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建Broker失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建Broker成功${NC}"

# 启动Broker（后台）
echo -e "${YELLOW}启动Broker...${NC}"
./bin/broker --config configs/broker.yaml > logs/broker_for_viz_test.log 2>&1 &
BROKER_PID=$!

# 等待Broker启动
sleep 5

# 检查Broker进程是否存在
if ps -p $BROKER_PID > /dev/null; then
    echo -e "${GREEN}Broker已启动，PID: $BROKER_PID${NC}"
else
    echo -e "${RED}Broker启动失败${NC}"
    cat logs/broker_for_viz_test.log
    exit 1
fi

# 运行可视化分析端（后台）
echo -e "${YELLOW}启动可视化分析端...${NC}"
./bin/visualization --config configs/visualization.yaml > logs/visualization_test.log 2>&1 &
VIZ_PID=$!

# 等待启动
sleep 5

# 检查进程是否存在
if ps -p $VIZ_PID > /dev/null; then
    echo -e "${GREEN}可视化分析端已启动，PID: $VIZ_PID${NC}"
else
    echo -e "${RED}可视化分析端启动失败${NC}"
    cat logs/visualization_test.log
    exit 1
fi

# 测试HTTP服务器
echo -e "${YELLOW}测试HTTP服务器...${NC}"
grep "HTTP服务器启动" logs/visualization_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}HTTP服务器工作正常${NC}"
else
    echo -e "${RED}HTTP服务器可能存在问题${NC}"
fi

# 测试WebSocket服务
echo -e "${YELLOW}测试WebSocket服务...${NC}"
grep "WebSocket" logs/visualization_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}WebSocket服务工作正常${NC}"
else
    echo -e "${RED}WebSocket服务可能存在问题${NC}"
fi

# 测试API端点
echo -e "${YELLOW}测试API端点...${NC}"
curl -s http://localhost:8080/api/status > /dev/null
if [ $? -eq 0 ]; then
    echo -e "${GREEN}API端点工作正常${NC}"
else
    echo -e "${RED}API端点可能存在问题${NC}"
fi

# 测试静态文件服务
echo -e "${YELLOW}测试静态文件服务...${NC}"
curl -s http://localhost:8080/ > /dev/null
if [ $? -eq 0 ]; then
    echo -e "${GREEN}静态文件服务工作正常${NC}"
else
    echo -e "${RED}静态文件服务可能存在问题${NC}"
fi

# 测试基数树
echo -e "${YELLOW}测试基数树...${NC}"
grep "基数树" logs/visualization_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}基数树工作正常${NC}"
else
    echo -e "${RED}基数树可能存在问题${NC}"
fi

# 测试QUIC服务（如果启用）
echo -e "${YELLOW}测试QUIC服务...${NC}"
grep "QUIC服务器" logs/visualization_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}QUIC服务工作正常${NC}"
else
    echo -e "${YELLOW}QUIC服务未启用或可能存在问题${NC}"
fi

# 停止可视化分析端
echo -e "${YELLOW}停止可视化分析端...${NC}"
kill $VIZ_PID 2>/dev/null
wait $VIZ_PID 2>/dev/null
echo -e "${GREEN}可视化分析端已停止${NC}"

# 停止Broker
echo -e "${YELLOW}停止Broker...${NC}"
kill $BROKER_PID 2>/dev/null
wait $BROKER_PID 2>/dev/null
echo -e "${GREEN}Broker已停止${NC}"

echo -e "${GREEN}测试完成${NC}"
exit 0 