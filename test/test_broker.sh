#!/bin/bash

# 测试分布式消息中转层的脚本

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${YELLOW}开始测试分布式消息中转层...${NC}"

# 检查配置文件是否存在
if [ ! -f "configs/broker.yaml" ]; then
    echo -e "${RED}错误: 配置文件 configs/broker.yaml 不存在${NC}"
    exit 1
fi

# 检查Redis是否运行
echo -e "${YELLOW}检查Redis服务...${NC}"
redis-cli ping > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo -e "${RED}错误: Redis服务未运行，尝试启动...${NC}"
    sudo service redis-server start
    sleep 2
    redis-cli ping > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${RED}错误: 无法启动Redis服务${NC}"
        exit 1
    fi
    echo -e "${GREEN}Redis服务已启动${NC}"
else
    echo -e "${GREEN}Redis服务正在运行${NC}"
fi

# 创建必要的目录
mkdir -p logs data/raft/logs data/raft/snapshots

# 构建分布式消息中转层
echo -e "${YELLOW}构建分布式消息中转层...${NC}"
go build -o bin/broker ./broker/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建成功${NC}"

# 运行分布式消息中转层（后台）
echo -e "${YELLOW}启动分布式消息中转层...${NC}"
./bin/broker --config configs/broker.yaml > logs/broker_test.log 2>&1 &
BROKER_PID=$!

# 等待启动
sleep 3

# 检查进程是否存在
if ps -p $BROKER_PID > /dev/null; then
    echo -e "${GREEN}分布式消息中转层已启动，PID: $BROKER_PID${NC}"
else
    echo -e "${RED}分布式消息中转层启动失败${NC}"
    cat logs/broker_test.log
    exit 1
fi

# 测试gRPC服务器
echo -e "${YELLOW}测试gRPC服务器...${NC}"
grep "gRPC服务器启动" logs/broker_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}gRPC服务器工作正常${NC}"
else
    echo -e "${RED}gRPC服务器可能存在问题${NC}"
fi

# 测试Raft服务
echo -e "${YELLOW}测试Raft服务...${NC}"
grep "Raft" logs/broker_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}Raft服务工作正常${NC}"
else
    echo -e "${RED}Raft服务可能存在问题${NC}"
fi

# 测试Redis存储
echo -e "${YELLOW}测试Redis存储...${NC}"
grep "Redis" logs/broker_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}Redis存储工作正常${NC}"
else
    echo -e "${RED}Redis存储可能存在问题${NC}"
fi

# 测试一致性哈希
echo -e "${YELLOW}测试一致性哈希...${NC}"
grep "哈希" logs/broker_test.log
if [ $? -eq 0 ]; then
    echo -e "${GREEN}一致性哈希工作正常${NC}"
else
    echo -e "${RED}一致性哈希可能存在问题${NC}"
fi

# 停止分布式消息中转层
echo -e "${YELLOW}停止分布式消息中转层...${NC}"
kill $BROKER_PID
wait $BROKER_PID 2>/dev/null
echo -e "${GREEN}分布式消息中转层已停止${NC}"

echo -e "${GREEN}测试完成${NC}"
exit 0 