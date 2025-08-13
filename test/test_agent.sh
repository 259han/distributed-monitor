#!/bin/bash

# 测试数据采集代理的脚本

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${YELLOW}开始测试数据采集代理...${NC}"

# 检查配置文件是否存在
if [ ! -f "configs/agent.yaml" ]; then
    echo -e "${RED}错误: 配置文件 configs/agent.yaml 不存在${NC}"
    exit 1
fi

# 创建必要的目录
mkdir -p logs data/raft/logs data/raft/snapshots

# 构建数据采集代理
echo -e "${YELLOW}构建数据采集代理...${NC}"
go build -o bin/agent ./agent/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建成功${NC}"

# 构建Broker（Agent需要连接到Broker）
echo -e "${YELLOW}构建Broker...${NC}"
go build -o bin/broker ./broker/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建Broker失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建Broker成功${NC}"

# 启动Broker（后台）
echo -e "${YELLOW}启动Broker...${NC}"
./bin/broker --config configs/broker.yaml > logs/broker_for_agent_test.log 2>&1 &
BROKER_PID=$!

# 等待Broker启动
sleep 5

# 检查Broker进程是否存在
if ps -p $BROKER_PID > /dev/null; then
    echo -e "${GREEN}Broker已启动，PID: $BROKER_PID${NC}"
else
    echo -e "${RED}Broker启动失败${NC}"
    cat logs/broker_for_agent_test.log
    exit 1
fi

# 运行数据采集代理（后台）
echo -e "${YELLOW}启动数据采集代理...${NC}"
./bin/agent --config configs/agent.yaml > logs/agent_test.log 2>&1 &
AGENT_PID=$!

# 等待启动
sleep 5

# 检查进程是否存在
if ps -p $AGENT_PID > /dev/null; then
    echo -e "${GREEN}数据采集代理已启动，PID: $AGENT_PID${NC}"
else
    echo -e "${RED}数据采集代理启动失败${NC}"
    cat logs/agent_test.log
    exit 1
fi

# 测试CPU采集器
echo -e "${YELLOW}测试CPU采集器...${NC}"
sleep 8
grep "成功采集cpu指标" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}CPU采集器工作正常${NC}"
else
    echo -e "${RED}CPU采集器可能存在问题${NC}"
fi

# 测试内存采集器
echo -e "${YELLOW}测试内存采集器...${NC}"
grep "成功采集memory指标" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}内存采集器工作正常${NC}"
else
    echo -e "${RED}内存采集器可能存在问题${NC}"
fi

# 测试网络采集器
echo -e "${YELLOW}测试网络采集器...${NC}"
grep "成功采集network指标" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}网络采集器工作正常${NC}"
else
    echo -e "${RED}网络采集器可能存在问题${NC}"
fi

# 测试磁盘采集器
echo -e "${YELLOW}测试磁盘采集器...${NC}"
grep "成功采集disk指标" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}磁盘采集器工作正常${NC}"
else
    echo -e "${RED}磁盘采集器可能存在问题${NC}"
fi

# 停止数据采集代理
echo -e "${YELLOW}停止数据采集代理...${NC}"
kill $AGENT_PID 2>/dev/null
wait $AGENT_PID 2>/dev/null
echo -e "${GREEN}数据采集代理已停止${NC}"

# 停止Broker
echo -e "${YELLOW}停止Broker...${NC}"
kill $BROKER_PID 2>/dev/null
wait $BROKER_PID 2>/dev/null
echo -e "${GREEN}Broker已停止${NC}"

echo -e "${GREEN}测试完成${NC}"
exit 0 