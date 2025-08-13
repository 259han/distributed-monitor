#!/bin/bash

echo "=== QUIC集成测试 ==="

# 启动broker
echo "1. 启动broker..."
./bin/broker -config configs/broker.yaml &
BROKER_PID=$!
sleep 3

# 启动visualization
echo "2. 启动visualization..."
./bin/visualization -config configs/visualization.yaml &
VIS_PID=$!
sleep 3

# 检查服务状态
echo "3. 检查服务状态..."
netstat -tlnp | grep -E "(8080|8081|9093)" || echo "端口检查失败"

# 测试QUIC连接
echo "4. 测试QUIC连接..."
timeout 5s bash -c "echo test | nc -u localhost 8081" 2>/dev/null || echo "QUIC端口测试（UDP）"

# 测试HTTP连接
echo "5. 测试HTTP连接..."
curl -s http://localhost:8080/api/status || echo "HTTP连接测试失败"

# 清理
echo "6. 清理进程..."
kill $VIS_PID 2>/dev/null
kill $BROKER_PID 2>/dev/null
sleep 2

echo "=== 测试完成 ==="

