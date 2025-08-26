# Kafka 集成指南

本文档介绍如何在分布式监控系统中启用和配置 Kafka 作为双重缓冲层。

## 功能概述

Kafka 集成为系统提供了以下功能：

1. **双重缓冲**：在 Agent 内部无锁队列之外，提供第二级缓冲
2. **可靠性保证**：即使 Broker 暂时不可用，数据也不会丢失
3. **异步处理**：解耦数据采集和处理流程，提高系统稳定性
4. **水平扩展**：支持多个 Broker 消费同一主题的数据，实现负载均衡

## 配置说明

### Agent 配置

在 `configs/agent.yaml` 中添加以下配置：

```yaml
# Kafka配置
kafka:
  enabled: true  # 是否启用Kafka
  brokers:        # Kafka服务器地址列表
    - "localhost:9092"
  topic: "metrics"  # 主题名称
  batch_size: 100   # 批处理大小
  batch_timeout: 1s  # 批处理超时时间
  encoding: "json"   # 编码格式 (json 或 protobuf)
  max_retry: 3       # 最大重试次数
```

### Broker 配置

在 `configs/broker.yaml` 中添加以下配置：

```yaml
# Kafka配置
kafka:
  enabled: true  # 是否启用Kafka
  brokers:        # Kafka服务器地址列表
    - "localhost:9092"
  topic: "metrics"  # 主题名称
  group_id: "broker-group"  # 消费者组ID
  batch_size: 100   # 批处理大小
  batch_timeout: 1s  # 批处理超时时间
  max_retry: 3       # 最大重试次数
```

## 部署要求

1. **Kafka 集群**：需要部署 Kafka 集群，并确保 Agent 和 Broker 都能访问
2. **依赖库**：系统使用 `github.com/segmentio/kafka-go` 作为 Kafka 客户端库

## 数据流程

启用 Kafka 后，数据流程如下：

```
Agent 采集器 -> 无锁队列 -> Kafka -> Broker -> 存储系统
```

如果 Kafka 不可用，系统会自动回退到直接通过 gRPC 发送数据：

```
Agent 采集器 -> 无锁队列 -> gRPC -> Broker -> 存储系统
```

## 故障处理

1. **Kafka 连接失败**：系统会自动回退到直接使用 gRPC 发送数据
2. **数据序列化失败**：错误会被记录，但不会中断采集流程
3. **消费者组冲突**：确保每个 Broker 实例使用唯一的 `group_id` 或共享同一个 `group_id` 以实现负载均衡

## 性能调优

1. **批处理大小**：调整 `batch_size` 参数可以优化吞吐量和延迟
2. **批处理超时**：调整 `batch_timeout` 参数可以平衡实时性和效率
3. **Kafka 分区**：根据系统规模，合理设置主题的分区数量

## 监控指标

启用 Kafka 后，系统会记录以下指标：

1. 发送到 Kafka 的消息数量
2. Kafka 发送失败的次数
3. 从 Kafka 消费的消息数量
4. 消费处理失败的次数

这些指标可以通过系统的监控接口获取。
