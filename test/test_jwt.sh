#!/bin/bash

# 测试JWT认证的脚本

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 工作目录
WORK_DIR=$(dirname "$(realpath "$0")")
cd "$WORK_DIR/.." || exit 1

echo -e "${YELLOW}开始测试JWT认证...${NC}"

# 构建可视化分析端
echo -e "${YELLOW}构建可视化分析端...${NC}"
go build -o bin/visualization ./visualization/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建成功${NC}"

# 构建Broker
echo -e "${YELLOW}构建Broker...${NC}"
go build -o bin/broker ./broker/cmd
if [ $? -ne 0 ]; then
    echo -e "${RED}构建Broker失败${NC}"
    exit 1
fi
echo -e "${GREEN}构建Broker成功${NC}"

# 启动Broker
echo -e "${YELLOW}启动Broker...${NC}"
./bin/broker --config configs/broker.yaml > logs/broker_jwt_test.log 2>&1 &
BROKER_PID=$!

# 等待Broker启动
sleep 5

# 启动可视化分析端
echo -e "${YELLOW}启动可视化分析端...${NC}"
./bin/visualization --config configs/visualization.yaml > logs/visualization_jwt_test.log 2>&1 &
VIZ_PID=$!

# 等待服务启动
sleep 5

# 检查服务状态
if ps -p $VIZ_PID > /dev/null; then
    echo -e "${GREEN}可视化分析端已启动，PID: $VIZ_PID${NC}"
else
    echo -e "${RED}可视化分析端启动失败${NC}"
    cat logs/visualization_jwt_test.log
    exit 1
fi

# 测试JWT认证功能
echo -e "${YELLOW}测试JWT认证功能...${NC}"

# 测试1: 访问受保护的API（未认证）
echo -e "${YELLOW}测试1: 访问受保护的API（未认证）...${NC}"
response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/metrics)
if [ "$response" = "401" ]; then
    echo -e "${GREEN}✓ 未认证访问被拒绝${NC}"
else
    echo -e "${RED}✗ 未认证访问未被拒绝 (HTTP $response)${NC}"
fi

# 测试2: 用户登录
echo -e "${YELLOW}测试2: 用户登录...${NC}"
login_response=$(curl -s -X POST http://localhost:8080/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin123"}')

echo "登录响应: $login_response"

# 提取token
token=$(echo "$login_response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
if [ -n "$token" ]; then
    echo -e "${GREEN}✓ 登录成功，获取到token${NC}"
else
    echo -e "${RED}✗ 登录失败${NC}"
fi

# 测试3: 使用token访问受保护的API
if [ -n "$token" ]; then
    echo -e "${YELLOW}测试3: 使用token访问受保护的API...${NC}"
    response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/metrics \
        -H "Authorization: Bearer $token")
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✓ 认证访问成功${NC}"
    else
        echo -e "${RED}✗ 认证访问失败 (HTTP $response)${NC}"
    fi
fi

# 测试4: 访问用户信息
if [ -n "$token" ]; then
    echo -e "${YELLOW}测试4: 访问用户信息...${NC}"
    profile_response=$(curl -s http://localhost:8080/api/auth/profile \
        -H "Authorization: Bearer $token")
    echo "用户信息: $profile_response"
fi

# 测试5: 访问管理员API
if [ -n "$token" ]; then
    echo -e "${YELLOW}测试5: 访问管理员API...${NC}"
    response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/admin \
        -H "Authorization: Bearer $token")
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✓ 管理员访问成功${NC}"
    else
        echo -e "${RED}✗ 管理员访问失败 (HTTP $response)${NC}"
    fi
fi

# 测试6: 用户登出
if [ -n "$token" ]; then
    echo -e "${YELLOW}测试6: 用户登出...${NC}"
    logout_response=$(curl -s -X POST http://localhost:8080/api/auth/logout \
        -H "Authorization: Bearer $token")
    echo "登出响应: $logout_response"
fi

# 停止服务
echo -e "${YELLOW}停止服务...${NC}"
kill $VIZ_PID 2>/dev/null
kill $BROKER_PID 2>/dev/null
wait $VIZ_PID 2>/dev/null
wait $BROKER_PID 2>/dev/null

echo -e "${GREEN}JWT认证测试完成${NC}"
exit 0