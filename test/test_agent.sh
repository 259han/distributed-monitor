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
mkdir -p logs

# 构建数据采集代理
echo -e "${YELLOW}构建数据采集代理...${NC}"
go build -o bin/agent ./agent/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建成功${NC}"

# 运行数据采集代理（后台）
echo -e "${YELLOW}启动数据采集代理...${NC}"
./bin/agent --config configs/agent.yaml > logs/agent_test.log 2>&1 &
AGENT_PID=$!

# 等待启动
sleep 2

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
sleep 5
grep "cpu" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}CPU采集器工作正常${NC}"
else
    echo -e "${RED}CPU采集器可能存在问题${NC}"
fi

# 测试内存采集器
echo -e "${YELLOW}测试内存采集器...${NC}"
grep "memory" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}内存采集器工作正常${NC}"
else
    echo -e "${RED}内存采集器可能存在问题${NC}"
fi

# 测试网络采集器
echo -e "${YELLOW}测试网络采集器...${NC}"
grep "network" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}网络采集器工作正常${NC}"
else
    echo -e "${RED}网络采集器可能存在问题${NC}"
fi

# 测试磁盘采集器
echo -e "${YELLOW}测试磁盘采集器...${NC}"
grep "disk" logs/agent_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}磁盘采集器工作正常${NC}"
else
    echo -e "${RED}磁盘采集器可能存在问题${NC}"
fi

# 停止数据采集代理
echo -e "${YELLOW}停止数据采集代理...${NC}"
kill $AGENT_PID
wait $AGENT_PID 2>/dev/null
echo -e "${GREEN}数据采集代理已停止${NC}"

echo -e "${GREEN}测试完成${NC}"
exit 0 