---
AIGC:
  ContentProducer: '001191110102MAD55U9H0F10002'
  ContentPropagator: '001191110102MAD55U9H0F10002'
  Label: '1'
  ProduceID: 'b3f8985d-989c-46ea-aca1-94d5ad0e21c1'
  PropagateID: 'b3f8985d-989c-46ea-aca1-94d5ad0e21c1'
  ReservedCode1: 'fee6e24d-e160-4ca7-83f5-aa4e943e334a'
  ReservedCode2: 'fee6e24d-e160-4ca7-83f5-aa4e943e334a'
---

# 监控指标说明

> 本文档记录所有写入 VictoriaMetrics 的指标及其在前端图表中的 PromQL 表达式。
> 新增图表时请同步更新本文档。

## 数据流

```
JMX Exporter / AdminClient API
        │
        ▼
SyncWorker (每 30 秒采集)
        │
        ├── collectPerBrokerMetrics()  → JMX 原始指标 → 写入 VM (per-broker 粒度)
        └── collectAndWriteMetrics()   → AdminClient 元数据 → 写入 VM (per-partition / per-group)
        │
        ▼
VictoriaMetrics (时序存储)
        │
        ▼
前端 PromQL 查询 → ECharts 渲染
```

**原则**：JMX 指标统一以 per-broker 粒度写入 VM，集群级聚合由前端 PromQL (`sum()` / `max()`) 完成。

---

## 一、JMX Exporter 指标（per-broker 写入）

> 采集函数：`collectPerBrokerMetrics`
> 每个 Broker 独立的 JMX Exporter HTTP 端点，由 `MultiJMXClient.FetchAllBrokerRawMetrics` 并行抓取。

### 1.1 网络字节

| VM 指标名 | 类型 | JMX 原始指标名 | 说明 |
|---|---|---|---|
| `kafka_broker_bytes_in_total` | 计数器 | `kafka_server_brokertopicmetrics_bytesin_total` | 字节流入累计值 |
| `kafka_broker_bytes_out_total` | 计数器 | `kafka_server_brokertopicmetrics_bytesout_total` | 字节流出累计值 |

### 1.2 消息吞吐

| VM 指标名 | 类型 | JMX 原始指标名 | 说明 |
|---|---|---|---|
| `kafka_broker_messages_in_total` | 计数器 | `kafka_server_brokertopicmetrics_messagesin_total` | 消息流入累计值 |

### 1.3 请求指标

| VM 指标名 | 类型 | JMX 原始指标名 | 说明 |
|---|---|---|---|
| `kafka_broker_produce_requests_total` | 计数器 | `kafka_server_brokertopicmetrics_totalproducerequests_total` | 生产请求累计值 |
| `kafka_broker_fetch_requests_total` | 计数器 | `kafka_server_brokertopicmetrics_totalfetchrequests_total` | 消费请求累计值 |
| `kafka_broker_request_queue_size` | Gauge | `kafka_network_requestchannel_requestqueuesize` | 请求队列大小 |

### 1.4 请求延迟（P99）

| VM 指标名 | 类型 | JMX 原始指标名 | 额外标签 | 说明 |
|---|---|---|---|---|
| `kafka_broker_request_latency_ms` | Gauge | `kafka_network_requestmetrics_totaltimems` | `quantile="0.99"`, `request` | 请求总延迟 P99（ms） |

`request` 标签取值：
- `Produce` — 生产请求
- `FetchConsumer` — 消费请求
- `FetchFollower` — 副本同步请求

### 1.5 副本状态

| VM 指标名 | 类型 | JMX 原始指标名 | 说明 |
|---|---|---|---|
| `kafka_broker_replica_max_lag` | Gauge | `kafka_server_replicafetchermanager_maxlag` | 副本同步最大 Lag |
| `kafka_broker_under_replicated_partitions` | Gauge | `kafka_server_replicamanager_underreplicatedpartitions` | 未同步分区数 |

### 1.6 Controller 状态

| VM 指标名 | 类型 | JMX 原始指标名 | 说明 |
|---|---|---|---|
| `kafka_broker_active_controller` | Gauge | `kafka_controller_kafkacontroller_activecontrollercount` | 是否为 Controller（1/0） |
| `kafka_broker_offline_partitions` | Gauge | `kafka_controller_kafkacontroller_offlinepartitionscount` | 离线分区数 |

### 1.7 公共标签

所有 per-broker 指标携带以下标签：

| 标签 | 说明 |
|---|---|
| `cluster_id` | 集群 ID |
| `cluster_name` | 集群名称 |
| `broker_id` | Broker ID |
| `broker_host` | Broker 主机名 |

---

## 二、AdminClient 指标

> 采集函数：`collectAndWriteMetrics`
> 通过 sarama AdminClient API 获取集群元数据、分区详情、消费者组信息。

### 2.1 集群元数据

| VM 指标名 | 类型 | 说明 |
|---|---|---|
| `kafka_broker_count` | Gauge | Broker 数量 |
| `kafka_topic_count` | Gauge | Topic 数量 |
| `kafka_consumer_group_count` | Gauge | 消费者组数量 |
| `kafka_broker_info` | Info (值=1) | Broker 信息（含 broker_id/host/port/rack 标签） |

### 2.2 Topic 分区指标（per-partition）

| VM 指标名 | 类型 | 说明 |
|---|---|---|
| `kafka_topic_partitions` | Gauge | Topic 分区数（per-topic） |
| `kafka_topic_partition_replicas` | Gauge | 分区副本数 |
| `kafka_topic_partition_in_sync_replica` | Gauge | ISR 数量 |
| `kafka_topic_partition_leader_is_preferred` | Gauge | 是否首选 Leader（1/0） |
| `kafka_topic_partition_under_replicated_partition` | Gauge | 是否未同步（1/0） |
| `kafka_topic_partition_current_offset` | Gauge | LogEndOffset |
| `kafka_topic_partition_oldest_offset` | Gauge | LogStartOffset |

标签：`cluster_id`, `cluster_name`, `topic`, `partition`

### 2.3 消费者组指标（per-group / per-partition）

| VM 指标名 | 类型 | 说明 |
|---|---|---|
| `kafka_consumergroup_members` | Gauge | 消费组成员数 |
| `kafka_consumergroup_lag_sum` | Gauge | 消费组 Topic 级 Lag 汇总 |
| `kafka_consumergroup_lag` | Gauge | 消费组分区级 Lag |
| `kafka_consumergroup_current_offset` | Gauge | 消费组分区当前 Offset |
| `kafka_consumergroup_lag_seconds` | Gauge | 消费组分区 Lag 时间（秒） |
| `kafka_total_lag` | Gauge | 集群总 Lag |

标签：`cluster_id`, `cluster_name`, `consumergroup`, `topic`, `partition`（按级别选用）

### 2.4 计算指标（per-broker 写入）

> 由 SyncWorker 从分区详情的 Leader/Replicas 字段统计得出。

| VM 指标名 | 类型 | 说明 |
|---|---|---|
| `kafka_broker_leader_count` | Gauge | 该 Broker 担任 Leader 的分区数 |
| `kafka_broker_replica_count` | Gauge | 该 Broker 拥有的副本总数 |

---

## 三、前端 PromQL 表达式

### 3.1 集群概览 Tab

#### 统计卡片（即时查询）

| 卡片 | PromQL |
|---|---|
| 分区总数 | `sum(kafka_topic_partitions{cluster_id="X",topic!~"__.*"})` |
| 消费组数量 | `count(kafka_consumergroup_members{cluster_id="X",consumergroup!~"__.*"})` |
| 消费组成员总数 | `sum(kafka_consumergroup_members{cluster_id="X",consumergroup!~"__.*"})` |
| ISR 总数 | `sum(kafka_topic_partition_in_sync_replica{cluster_id="X"})` |
| 非首选 Leader | `count(kafka_topic_partition_leader_is_preferred{cluster_id="X"}<1)` |

#### 趋势图（范围查询）

| 图表 | PromQL |
|---|---|
| 集群生产速率 | `sum(rate(kafka_topic_partition_current_offset{cluster_id="X",topic!~"__.*"}[30s]))` |
| 集群消费速率 | `sum(rate(kafka_consumergroup_current_offset{cluster_id="X"}[30s]))` |
| 消费者组总 Lag | `sum(kafka_consumergroup_lag_sum{cluster_id="X"})` |
| 字节流入速率 | `sum(rate(kafka_broker_bytes_in_total{cluster_id="X"}[30s]))` |
| 字节流出速率 | `sum(rate(kafka_broker_bytes_out_total{cluster_id="X"}[30s]))` |

### 3.2 Broker 监控 Tab

#### Broker 总览表格（后端 API：`/api/v1/metrics/broker-overview/:id`）

后端从 VM 查询以下指标并聚合：

| 数据项 | VM 查询（即时值） |
|---|---|
| Broker 列表 | `kafka_broker_info{cluster_id="X"}` |
| Leader 数 | `kafka_broker_leader_count{cluster_id="X"}` |
| Replica 数 | `kafka_broker_replica_count{cluster_id="X"}` |
| Controller | `kafka_broker_active_controller{cluster_id="X"}` |
| Leader% | Go 层计算：`leader_count / replica_count * 100` |

#### 图表（范围查询）

| 图表 | 选全部 Broker | 选单个 Broker |
|---|---|---|
| 生产请求延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="X",request="Produce"}` | `kafka_broker_request_latency_ms{cluster_id="X",request="Produce",broker_id="Y"}` |
| 消费请求延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="X",request="FetchConsumer"}` | `kafka_broker_request_latency_ms{cluster_id="X",request="FetchConsumer",broker_id="Y"}` |
| 副本同步延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="X",request="FetchFollower"}` | `kafka_broker_request_latency_ms{cluster_id="X",request="FetchFollower",broker_id="Y"}` |
| 副本同步 Lag | `kafka_broker_replica_max_lag{cluster_id="X"}` (取各 Broker 最大值) | `kafka_broker_replica_max_lag{cluster_id="X",broker_id="Y"}` |
| 字节流入速率 | `rate(kafka_broker_bytes_in_total{cluster_id="X"}[30s])` (多线) | `rate(kafka_broker_bytes_in_total{cluster_id="X",broker_id="Y"}[30s])` |
| 字节流出速率 | `rate(kafka_broker_bytes_out_total{cluster_id="X"}[30s])` (多线) | `rate(kafka_broker_bytes_out_total{cluster_id="X",broker_id="Y"}[30s])` |

### 3.3 Topic 监控 Tab

| 图表 | PromQL |
|---|---|
| Topic 生产速率（按分区） | `rate(kafka_topic_partition_current_offset{cluster_id="X",topic="T"}[30s])` |
| 消费组消费速率（按分区） | `rate(kafka_consumergroup_current_offset{cluster_id="X",topic="T",consumergroup="G"}[30s])` |
| 消费组 Lag（按分区） | `kafka_consumergroup_lag{cluster_id="X",topic="T",consumergroup="G"}` |

---

## 四、命名规范

| 类别 | 前缀 | 示例 |
|---|---|---|
| Broker 级 JMX 指标 | `kafka_broker_` | `kafka_broker_bytes_in_total` |
| Partition 级 AdminClient 指标 | `kafka_topic_partition_` | `kafka_topic_partition_replicas` |
| ConsumerGroup 级指标 | `kafka_consumergroup_` | `kafka_consumergroup_lag` |
| 集群元数据 | `kafka_` | `kafka_broker_count`, `kafka_topic_count` |
| 计数器 | 后缀 `_total` | `kafka_broker_bytes_in_total` |
| Gauge | 无特殊后缀 | `kafka_broker_replica_max_lag` |

---

## 五、变更记录

| 日期 | 变更内容 |
|---|---|
| 2026-05-09 | Broker Tab 上线：新增 per-broker JMX 指标写入，JMX 指标从集群聚合改为 per-broker 粒度，集群级由 PromQL 聚合 |

> AI生成