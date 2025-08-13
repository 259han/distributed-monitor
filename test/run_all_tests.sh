#!/bin/bash

# 运行所有测试的脚本

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${YELLOW}开始运行所有测试...${NC}"

# 创建必要的目录
mkdir -p logs

# 检查测试脚本是否存在
if [ ! -f "test/test_agent.sh" ] || [ ! -f "test/test_broker.sh" ] || [ ! -f "test/test_visualization.sh" ]; then
    echo -e "${RED}错误: 测试脚本不存在${NC}"
    exit 1
fi

# 确保测试脚本可执行
chmod +x test/test_agent.sh test/test_broker.sh test/test_visualization.sh

# 运行数据采集代理测试
echo -e "\n${YELLOW}运行数据采集代理测试...${NC}"
test/test_agent.sh
AGENT_TEST_RESULT=$?

# 运行分布式消息中转层测试
echo -e "\n${YELLOW}运行分布式消息中转层测试...${NC}"
test/test_broker.sh
BROKER_TEST_RESULT=$?

# 运行可视化分析端测试
echo -e "\n${YELLOW}运行可视化分析端测试...${NC}"
test/test_visualization.sh
VIZ_TEST_RESULT=$?

# 输出测试结果摘要
echo -e "\n${YELLOW}测试结果摘要:${NC}"
if [ $AGENT_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ 数据采集代理测试通过${NC}"
else
    echo -e "${RED}✗ 数据采集代理测试失败${NC}"
fi

if [ $BROKER_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ 分布式消息中转层测试通过${NC}"
else
    echo -e "${RED}✗ 分布式消息中转层测试失败${NC}"
fi

if [ $VIZ_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ 可视化分析端测试通过${NC}"
else
    echo -e "${RED}✗ 可视化分析端测试失败${NC}"
fi

# 计算总结果
TOTAL_RESULT=$((AGENT_TEST_RESULT + BROKER_TEST_RESULT + VIZ_TEST_RESULT))

if [ $TOTAL_RESULT -eq 0 ]; then
    echo -e "\n${GREEN}所有测试通过!${NC}"
    exit 0
else
    echo -e "\n${RED}测试失败! 请查看日志了解详情。${NC}"
    exit 1
fi 