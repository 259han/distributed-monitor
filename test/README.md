# 测试说明

本目录包含分布式实时监控系统的测试脚本，用于验证各个组件的功能。

## 测试脚本

- `test_agent.sh`: 测试数据采集代理
- `test_broker.sh`: 测试分布式消息中转层
- `test_visualization.sh`: 测试可视化分析端
- `run_all_tests.sh`: 运行所有测试

## 运行测试

### 运行所有测试

```bash
./test/run_all_tests.sh
```

### 运行单个组件测试

```bash
# 测试数据采集代理
./test/test_agent.sh

# 测试分布式消息中转层
./test/test_broker.sh

# 测试可视化分析端
./test/test_visualization.sh
```

## 测试前提条件

1. **Redis服务**：分布式消息中转层测试需要Redis服务，如果未运行，测试脚本会尝试启动它。
2. **配置文件**：确保`configs`目录下有正确的配置文件：
   - `configs/agent.yaml`
   - `configs/broker.yaml`
   - `configs/visualization.yaml`
3. **静态文件**：可视化分析端测试需要`static/index.html`文件。

## 测试日志

所有测试日志保存在`logs`目录下：
- `logs/agent_test.log`
- `logs/broker_test.log`
- `logs/visualization_test.log`

## 注意事项

1. 测试脚本会自动创建必要的目录。
2. 每个测试脚本都会构建相应的组件，确保源代码可编译。
3. 测试过程中会启动组件进程，测试完成后会自动停止这些进程。
4. 如果测试失败，请查看相应的日志文件了解详情。
5. 确保端口未被占用（默认：代理8080，中转层9090，可视化端8080）。 