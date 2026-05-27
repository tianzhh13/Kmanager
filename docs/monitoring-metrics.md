---
AIGC:
  ContentProducer: '001191110102MAD55U9H0F10002'
  ContentPropagator: '001191110102MAD55U9H0F10002'
  Label: '1'
  ProduceID: '467ba21e-4238-4cc7-8e5c-7e005f7f4131'
  PropagateID: '467ba21e-4238-4cc7-8e5c-7e005f7f4131'
  ReservedCode1: 'b3956b4b-8552-478d-9177-ef57ceb261ce'
  ReservedCode2: 'b3956b4b-8552-478d-9177-ef57ceb261ce'
---

# 监控指标说明

> 本文档记录所有写入 VictoriaMetrics 的指标及其在前端图表中的 PromQL 表达式。
> 新增图表时请同步更新本文档。
>
> **统计**：127 个 VM 指标 / 89 个前端图表查询（17 Stat + 9 集群趋势 + 43 Broker 趋势 + 10 Broker Stat + 10 Topic）

## 数据流

```
JMX Exporter / AdminClient API
        │
        ▼
SyncWorker (每 30 秒采集)
        │
        ├── collectAdminMetrics()   → AdminClient 元数据 → 写入 VM
        ├── collectJMXMetrics()     → JMX 原始指标 → 写入 VM (per-broker 粒度)
        └── collectJMXMetrics()     → 从分区详情计算 → 写入 VM (per-broker 粒度)
        │
        ▼
VictoriaMetrics (时序存储)
        │
        ▼
前端 PromQL 查询 → ECharts 渲染
```

**采集架构**：`collectCluster()` 并行调用 `collectAdminMetrics()` + `collectJMXMetrics()`，合并后一次性写入 VM。

**原则**：JMX 指标统一以 per-broker 粒度写入 VM，集群级聚合由前端 PromQL (`sum()` / `max()`) 完成。

---

## 一、AdminClient 指标（17 项）

> 采集函数：`collectAdminMetrics`
> 采集来源：Kafka AdminClient API + GetTopicPartitionOffsets
> 公共标签：`cluster_id`, `cluster_name`

### 1.1 集群元数据

| # | VM 指标名 | 类型 | 采集来源 (sarama API) | 标签 | 说明 |
|---|-----------|------|----------------------|------|------|
| 1 | `kafka_broker_count` | Gauge | `DescribeCluster()` → `len(Brokers)` | cluster_id, cluster_name | 集群 Broker 数量 |
| 2 | `kafka_topic_count` | Gauge | `ListTopics()` → `len(topics)` | cluster_id, cluster_name | 集群 Topic 数量 |
| 3 | `kafka_consumer_group_count` | Gauge | `ListConsumerGroups()` → `len(groups)` | cluster_id, cluster_name | 消费者组数量 |
| 4 | `kafka_broker_info` | Info (值=1) | `DescribeCluster()` → Broker.ID / Addr / Rack | cluster_id, cluster_name, broker_id, broker_host, broker_port, broker_rack(可选) | Broker 元信息 |

### 1.2 Topic 分区数

| # | VM 指标名 | 类型 | 采集来源 (sarama API) | 标签 | 说明 |
|---|-----------|------|----------------------|------|------|
| 5 | `kafka_topic_partitions` | Gauge | `ListTopics()` → `TopicDetail.NumPartitions` | cluster_id, cluster_name, topic | Topic 分区总数 |

### 1.3 分区详情指标（per-partition）

> 标签：`cluster_id`, `cluster_name`, `topic`, `partition`

| # | VM 指标名 | 类型 | 采集来源 (sarama API) | 说明 |
|---|-----------|------|----------------------|------|
| 6 | `kafka_topic_partition_replicas` | Gauge | `DescribeTopics(nil)` → `len(partition.Replicas)` | 分区副本数 |
| 7 | `kafka_topic_partition_in_sync_replica` | Gauge | `DescribeTopics(nil)` → `len(partition.ISR)` | 分区 ISR 数量 |
| 8 | `kafka_topic_partition_leader_is_preferred` | Gauge (0/1) | `DescribeTopics(nil)` → `Leader == Replicas[0]` | 是否首选 Leader |
| 9 | `kafka_topic_partition_under_replicated_partition` | Gauge (0/1) | `DescribeTopics(nil)` → `len(ISR) < len(Replicas)` | 是否未同步副本 |
| 10 | `kafka_topic_partition_current_offset` | Gauge | `Client.GetOffset(topic, partition, OffsetNewest)` | LogEndOffset |
| 11 | `kafka_topic_partition_oldest_offset` | Gauge | `Client.GetOffset(topic, partition, OffsetOldest)` | LogStartOffset |

### 1.4 消费者组指标

| # | VM 指标名 | 类型 | 采集来源 (sarama API) | 标签 | 说明 |
|---|-----------|------|----------------------|------|------|
| 12 | `kafka_consumergroup_members` | Gauge | `DescribeConsumerGroups()` → `len(desc.Members)` | cluster_id, cluster_name, consumergroup | 消费组成员数（排除 `__` 开头） |
| 13 | `kafka_consumergroup_lag_sum` | Gauge | `ListConsumerGroupOffsets()` → 分区 Lag 累加 | cluster_id, cluster_name, consumergroup, topic | 消费组按 Topic 汇总 Lag |
| 14 | `kafka_consumergroup_lag` | Gauge | `ListConsumerGroupOffsets()` → `endOffset - currentOffset` | cluster_id, cluster_name, consumergroup, topic, partition | 消费组分区级 Lag |
| 15 | `kafka_consumergroup_current_offset` | Gauge | `ListConsumerGroupOffsets()` → 提交 offset | cluster_id, cluster_name, consumergroup, topic, partition | 消费组分区当前 Offset |
| 16 | `kafka_consumergroup_lag_seconds` | Gauge | `Client.Leader()+Fetch()` → `now - msgTimestamp` | cluster_id, cluster_name, consumergroup, topic, partition | 消费组分区 Lag 时间（秒），仅 >=0 时写入 |
| 17 | `kafka_total_lag` | Gauge | 所有消费组 `TotalLag` 累加 | cluster_id, cluster_name | 集群总消费延迟 |

---

## 二、JMX 指标 — 流量与消息（10 项）

> 采集函数：`collectJMXMetrics`
> 采集来源：JMX Exporter `/metrics` HTTP 端点
> 公共标签：`cluster_id`, `cluster_name`, `broker_id`, `broker_host`
> 映射模式：`dualNameMappings`（新旧双 JMX 名称兼容）/ `directMappings`

| # | VM 指标名 | 类型 | JMX 原始名（新版） | JMX 原始名（旧版） | 说明 |
|---|-----------|------|-------------------|-------------------|------|
| 18 | `kafka_broker_bytes_in_total` | Counter | `kafka_server_BrokerTopicMetrics_BytesInPersec` | `kafka_server_brokertopicmetrics_bytesin_total` | 字节流入总量 |
| 19 | `kafka_broker_bytes_out_total` | Counter | `kafka_server_BrokerTopicMetrics_BytesOutPersec` | `kafka_server_brokertopicmetrics_bytesout_total` | 字节流出总量 |
| 20 | `kafka_broker_messages_in_total` | Counter | `kafka_server_BrokerTopicMetrics_MessagesInPersec` | `kafka_server_brokertopicmetrics_messagesin_total` | 消息流入总量 |
| 21 | `kafka_broker_produce_requests_total` | Counter | `kafka_server_BrokerTopicMetrics_TotalProduceRequestsPersec` | `kafka_server_brokertopicmetrics_totalproducerequests_total` | 生产请求总数 |
| 22 | `kafka_broker_fetch_requests_total` | Counter | `kafka_server_BrokerTopicMetrics_TotalFetchRequestsPersec` | `kafka_server_brokertopicmetrics_totalfetchrequests_total` | 拉取请求总数 |
| 23 | `kafka_broker_replication_bytes_in_total` | Counter | `kafka_server_brokertopicmetrics_replicationbytesin_total` | 副本同步流入字节 |
| 24 | `kafka_broker_replication_bytes_out_total` | Counter | `kafka_server_brokertopicmetrics_replicationbytesout_total` | 副本同步流出字节 |
| 25 | `kafka_broker_reassignment_bytes_in_total` | Counter | `kafka_server_brokertopicmetrics_reassignmentbytesin_total` | 分区迁移流入字节 |
| 26 | `kafka_broker_reassignment_bytes_out_total` | Counter | `kafka_server_brokertopicmetrics_reassignmentbytesout_total` | 分区迁移流出字节 |
| 27 | `kafka_broker_bytes_rejected_total` | Counter | `kafka_server_brokertopicmetrics_bytesrejected_total` | 拒绝字节总数 |

---

## 三、JMX 指标 — 请求错误与失败（12 项）

> 映射模式：`brokerTopicMappings` / `requestErrorMappings`（带 request + error 双标签）

| # | VM 指标名 | 类型 | JMX 原始名 | 额外标签 | 说明 |
|---|-----------|------|-----------|---------|------|
| 28 | `kafka_broker_failed_produce_requests_total` | Counter | `kafka_server_brokertopicmetrics_failedproducerequests_total` | — | 失败生产请求总数 |
| 29 | `kafka_broker_failed_fetch_requests_total` | Counter | `kafka_server_brokertopicmetrics_failedfetchrequests_total` | — | 失败拉取请求总数 |
| 30 | `kafka_broker_produce_message_conversions_total` | Counter | `kafka_server_brokertopicmetrics_producemessageconversions_total` | — | 生产消息转换总数 |
| 31 | `kafka_broker_fetch_message_conversions_total` | Counter | `kafka_server_brokertopicmetrics_fetchmessageconversions_total` | — | 拉取消息转换总数 |
| 32 | `kafka_broker_invalid_magic_number_records_total` | Counter | `kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total` | — | 无效 Magic Number 记录 |
| 33 | `kafka_broker_invalid_message_crc_records_total` | Counter | `kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total` | — | 无效 CRC 记录 |
| 34 | `kafka_broker_invalid_offset_or_sequence_records_total` | Counter | `kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total` | — | 无效 Offset/Sequence 记录 |
| 35 | `kafka_broker_no_key_compacted_topic_records_total` | Counter | `kafka_server_brokertopicmetrics_nokeycompactedtopicrecords_total` | — | 无 Key Compact Topic 记录 |
| 36 | `kafka_broker_request_errors_total` | Counter | `kafka_network_requestmetrics_errors_total` | request, error | 请求错误总数 |
| 37 | `kafka_broker_requests_total` | Counter | `kafka_network_requestmetrics_requests_total` | request, error | 请求总数 |

---

## 四、JMX 指标 — 请求延迟（11 项）

> 映射模式：`quantileWithRequestMappings`（取 P99 分位 + `request` 标签）
> `request` 标签取值：Produce / FetchConsumer / FetchFollower / Metadata / OffsetCommit / FindCoordinator / JoinGroup / SyncGroup / Heartbeat / LeaveGroup / DescribeGroups / ListGroups / StopReplica / UpdateMetadata / ControlledShutdown / LeaderAndIsr 等

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 38 | `kafka_broker_request_latency_ms` | Gauge | `kafka_network_requestmetrics_totaltimems` | 总请求延迟 P99 |
| 39 | `kafka_broker_request_queue_time_ms` | Gauge | `kafka_network_requestmetrics_requestqueuetimems` | 请求排队延迟 P99 |
| 40 | `kafka_broker_request_local_time_ms` | Gauge | `kafka_network_requestmetrics_localtimems` | 本地处理延迟 P99 |
| 41 | `kafka_broker_request_remote_time_ms` | Gauge | `kafka_network_requestmetrics_remotetimems` | 远程等待延迟 P99 |
| 42 | `kafka_broker_request_queue_time_ms_response` | Gauge | `kafka_network_requestmetrics_responsequeuetimems` | 响应排队延迟 P99 |
| 43 | `kafka_broker_request_response_send_time_ms` | Gauge | `kafka_network_requestmetrics_responsesendtimems` | 响应发送延迟 P99 |
| 44 | `kafka_broker_throttle_time_ms` | Gauge | `kafka_network_requestmetrics_throttletimems` | 限流延迟 P99 |
| 45 | `kafka_broker_message_conversions_time_ms` | Gauge | `kafka_network_requestmetrics_messageconversionstimems` | 消息转换延迟 P99 |
| 46 | `kafka_broker_request_bytes` | Gauge | `kafka_network_requestmetrics_requestbytes` | 请求字节数 P99 |
| 47 | `kafka_broker_controller_event_queue_time_ms` | Gauge | `kafka_controller_controllereventmanager_eventqueuetimems` | Controller 事件排队延迟 P99 |
| 48 | `kafka_broker_log_flush_time_ms` | Gauge | `kafka_log_logflushstats_logflushrateandtimems` | Log Flush 耗时 P99 |

---

## 五、JMX 指标 — 副本与分区（13 项）

> 映射模式：`directMappings` / `dualNameMappings` / `brokerTopicMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | JMX 旧版名（仅 dualName） | 说明 |
|---|-----------|------|-----------|--------------------------|------|
| 49 | `kafka_broker_under_replicated_partitions` | Gauge | `kafka_server_replicamanager_underreplicatedpartitions` | `kafka_server_ReplicaManager_UnderReplicatedPartitions` | 未同步副本分区数 |
| 50 | `kafka_broker_under_min_isr_partition_count` | Gauge | `kafka_server_replicamanager_underminisrpartitioncount` | Under MinISR 分区数 |
| 51 | `kafka_broker_at_min_isr_partition_count` | Gauge | `kafka_server_replicamanager_atminisrpartitioncount` | At MinISR 分区数 |
| 52 | `kafka_broker_offline_replica_count` | Gauge | `kafka_server_replicamanager_offlinereplicacount` | 离线副本数 |
| 53 | `kafka_broker_isr_shrinks_total` | Counter | `kafka_server_replicamanager_isrshrinks_total` | ISR 收缩总数 |
| 54 | `kafka_broker_isr_expands_total` | Counter | `kafka_server_replicamanager_isrexpands_total` | ISR 扩展总数 |
| 55 | `kafka_broker_isr_updates_failed_total` | Counter | `kafka_server_replicamanager_failedisrupdates_total` | ISR 更新失败总数 |
| 56 | `kafka_broker_partition_count` | Gauge | `kafka_server_replicamanager_partitioncount` | Broker 分区数 |
| 57 | `kafka_broker_reassigning_partitions` | Gauge | `kafka_server_replicamanager_reassigningpartitions` | 正在迁移分区数 |
| 58 | `kafka_broker_replica_max_lag` | Gauge | `kafka_server_replicafetchermanager_maxlag` | Follower 副本最大 Lag |
| 59 | `kafka_broker_min_fetch_rate` | Gauge | `kafka_server_replicafetchermanager_minfetchrate` | Follower 最小拉取速率 |
| 60 | `kafka_broker_failed_partitions_count` | Gauge | `kafka_server_replicafetchermanager_failedpartitionscount` | Follower 失败分区数 |
| 61 | `kafka_broker_dead_thread_count` | Gauge | `kafka_server_replicafetchermanager_deadthreadcount` | Follower 拉取死线程数 |

---

## 六、JMX 指标 — Controller 状态（8 项）

> 映射模式：`directMappings` / `dualNameMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | JMX 旧版名（仅 dualName） | 说明 |
|---|-----------|------|-----------|--------------------------|------|
| 62 | `kafka_broker_active_controller` | Gauge | `kafka_controller_kafkacontroller_activecontrollercount` | | 是否为活跃 Controller |
| 63 | `kafka_broker_offline_partitions` | Gauge | `kafka_controller_kafkacontroller_offlinepartitionscount` | `kafka_controller_KafkaController_OfflinePartitionsCount` | 离线分区数 |
| 64 | `kafka_broker_unclean_leader_elections_total` | Counter | `kafka_controller_controllerstats_uncleanleaderelections_total` | Unclean 选举总数 |
| 65 | `kafka_broker_active_broker_count` | Gauge | `kafka_controller_kafkacontroller_activebrokercount` | 活跃 Broker 数 |
| 66 | `kafka_broker_fenced_broker_count` | Gauge | `kafka_controller_kafkacontroller_fencedbrokercount` | Fenced Broker 数 |
| 67 | `kafka_broker_global_partition_count` | Gauge | `kafka_controller_kafkacontroller_globalpartitioncount` | 全局分区总数 |
| 68 | `kafka_broker_global_topic_count` | Gauge | `kafka_controller_kafkacontroller_globaltopiccount` | 全局 Topic 总数 |
| 69 | `kafka_broker_preferred_replica_imbalance` | Gauge | `kafka_controller_kafkacontroller_preferredreplicaimbalancecount` | 非首选 Leader 分区数 |

---

## 七、JMX 指标 — 网络与线程（8 项）

> 映射模式：`directMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 70 | `kafka_broker_request_queue_size` | Gauge | `kafka_network_requestchannel_requestqueuesize` | 请求队列大小 |
| 71 | `kafka_broker_response_queue_size` | Gauge | `kafka_network_requestchannel_responsequeuesize` | 响应队列大小 |
| 72 | `kafka_broker_processor_idle_percent` | Gauge | `kafka_network_processor_idlepercent` | 网络 Processor 空闲率 |
| 73 | `kafka_broker_network_processor_avg_idle_percent` | Gauge | `kafka_network_socketserver_networkprocessoravgidlepercent` | 网络 Processor 平均空闲率 |
| 74 | `kafka_broker_request_handler_avg_idle_percent` | Gauge | `kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent` | Request Handler 平均空闲率 |
| 75 | `kafka_broker_expired_connections_killed_count` | Counter | `kafka_network_socketserver_expiredconnectionskilledcount` | 过期连接关闭数 |
| 76 | `kafka_broker_memory_pool_available` | Gauge | `kafka_network_socketserver_memorypoolavailable` | 内存池可用量 |
| 77 | `kafka_broker_memory_pool_used` | Gauge | `kafka_network_socketserver_memorypoolused` | 内存池已用量 |

---

## 八、JMX 指标 — 延迟操作（3 项）

> 映射模式：`directMappings` / `purgatoryMappings`（带 `purgatory` 标签）

| # | VM 指标名 | 类型 | JMX 原始名 | 额外标签 | 说明 |
|---|-----------|------|-----------|---------|------|
| 78 | `kafka_broker_delayed_fetch_expires_total` | Counter | `kafka_server_delayedfetchmetrics_expires_total` | — | Fetch 延迟过期总数 |
| 79 | `kafka_broker_delayed_operations` | Gauge | `kafka_server_delayedoperationpurgatory_numdelayedoperations` | purgatory | Purgatory 等待操作数 |
| 80 | `kafka_broker_purgatory_size` | Gauge | `kafka_server_delayedoperationpurgatory_purgatorysize` | purgatory | Purgatory 等待队列大小 |

---

## 九、JMX 指标 — Broker 状态与磁盘（5 项）

> 映射模式：`directMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 81 | `kafka_broker_state` | Gauge | `kafka_server_kafkaserver_brokerstate` | Broker 状态（1=未恢复, 2=作为Controller, 3=作为Follower） |
| 82 | `kafka_broker_offline_log_directory_count` | Gauge | `kafka_log_logmanager_offlinelogdirectorycount` | 离线日志目录数 |
| 83 | `kafka_broker_log_directory_offline` | Gauge | `kafka_log_logmanager_logdirectoryoffline` | 日志目录离线状态 |
| 84 | `kafka_broker_disk_read_bytes` | Counter | `kafka_server_kafkaserver_linux_disk_read_bytes` | 磁盘读取字节数 |
| 85 | `kafka_broker_disk_write_bytes` | Counter | `kafka_server_kafkaserver_linux_disk_write_bytes` | 磁盘写入字节数 |

---

## 十、JMX 指标 — Log Cleaner（9 项）

> 映射模式：`brokerTopicMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 86 | `kafka_broker_log_cleaner_max_dirty_percent` | Gauge | `kafka_log_logcleanermanager_max_dirty_percent` | 最大脏数据比例 |
| 87 | `kafka_broker_log_cleaner_time_since_last_run_ms` | Gauge | `kafka_log_logcleanermanager_time_since_last_run_ms` | 上次清理间隔(ms) |
| 88 | `kafka_broker_log_cleaner_uncleanable_bytes` | Gauge | `kafka_log_logcleanermanager_uncleanable_bytes` | 不可清理字节数 |
| 89 | `kafka_broker_log_cleaner_uncleanable_partitions_count` | Gauge | `kafka_log_logcleanermanager_uncleanable_partitions_count` | 不可清理分区数 |
| 90 | `kafka_broker_log_cleaner_recopy_percent` | Gauge | `kafka_log_logcleaner_cleaner_recopy_percent` | Cleaner 重新复制比例 |
| 91 | `kafka_broker_log_cleaner_dead_thread_count` | Gauge | `kafka_log_logcleaner_deadthreadcount` | Cleaner 死线程数 |
| 92 | `kafka_broker_log_cleaner_max_buffer_utilization_percent` | Gauge | `kafka_log_logcleaner_max_buffer_utilization_percent` | Cleaner 最大缓冲利用率 |
| 93 | `kafka_broker_log_cleaner_max_clean_time_secs` | Gauge | `kafka_log_logcleaner_max_clean_time_secs` | Cleaner 最大清理时间(秒) |
| 94 | `kafka_broker_log_cleaner_max_compaction_delay_secs` | Gauge | `kafka_log_logcleaner_max_compaction_delay_secs` | Cleaner 最大压缩延迟(秒) |

---

## 十一、JMX 指标 — Topic 分区级（9 项）

> 映射模式：`topicPartitionMappings`（带 `topic` + `partition` 标签）

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 95 | `kafka_topic_log_size` | Gauge | `kafka_log_log_size` | Topic 日志大小（字节） |
| 96 | `kafka_topic_log_end_offset` | Gauge | `kafka_log_log_logendoffset` | Topic LogEndOffset |
| 97 | `kafka_topic_log_start_offset` | Gauge | `kafka_log_log_logstartoffset` | Topic LogStartOffset |
| 98 | `kafka_topic_log_num_segments` | Gauge | `kafka_log_log_numlogsegments` | Topic 日志段数量 |
| 99 | `kafka_topic_partition_under_replicated` | Gauge (0/1) | `kafka_cluster_partition_underreplicated` | 分区是否未同步 |
| 100 | `kafka_topic_partition_under_min_isr` | Gauge (0/1) | `kafka_cluster_partition_underminisr` | 分区是否 Under MinISR |
| 101 | `kafka_topic_partition_isr_count` | Gauge | `kafka_cluster_partition_insyncreplicascount` | 分区 ISR 数 |
| 102 | `kafka_topic_partition_replica_count` | Gauge | `kafka_cluster_partition_replicascount` | 分区副本数 |
| 103 | `kafka_topic_partition_last_stable_offset_lag` | Gauge | `kafka_cluster_partition_laststableoffsetlag` | 分区 Last Stable Offset Lag |

---

## 十二、JMX 指标 — 系统进程（6 项）

> 映射模式：`directMappings`

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 104 | `kafka_broker_process_cpu_seconds_total` | Counter | `process_cpu_seconds_total` | 进程累计 CPU 秒 |
| 105 | `kafka_broker_process_resident_memory_bytes` | Gauge | `process_resident_memory_bytes` | 进程驻留内存 |
| 106 | `kafka_broker_process_virtual_memory_bytes` | Gauge | `process_virtual_memory_bytes` | 进程虚拟内存 |
| 107 | `kafka_broker_process_start_time_seconds` | Gauge | `process_start_time_seconds` | 进程启动时间戳 |
| 108 | `kafka_broker_process_max_fds` | Gauge | `process_max_fds` | 最大文件描述符数 |
| 109 | `kafka_broker_process_open_fds` | Gauge | `process_open_fds` | 已用文件描述符数 |

---

## 十三、JMX 指标 — JVM（7 项）

> 映射模式：`directMappings` / `gcMappings`（带 `gc` 标签）/ `poolMappings`（带 `pool` 标签）

| # | VM 指标名 | 类型 | JMX 原始名 | 额外标签 | 说明 |
|---|-----------|------|-----------|---------|------|
| 110 | `kafka_broker_jvm_threads_current` | Gauge | `jvm_threads_current` | — | JVM 当前线程数 |
| 111 | `kafka_broker_jvm_threads_deadlocked` | Gauge | `jvm_threads_deadlocked` | — | JVM 死锁线程数 |
| 112 | `kafka_broker_jvm_gc_seconds_sum` | Counter | `jvm_gc_collection_seconds_sum` | gc | JVM GC 累计耗时(秒) |
| 113 | `kafka_broker_jvm_gc_count` | Counter | `jvm_gc_collection_seconds_count` | gc | JVM GC 累计次数 |
| 114 | `kafka_broker_jvm_memory_pool_used_bytes` | Gauge | `jvm_memory_pool_collection_used_bytes` | pool | JVM 内存池已用字节 |
| 115 | `kafka_broker_jvm_memory_pool_max_bytes` | Gauge | `jvm_memory_pool_collection_max_bytes` | pool | JVM 内存池最大字节 |
| 116 | `kafka_broker_buffer_pool_used_bytes` | Gauge | `jvm_buffer_pool_used_bytes` | pool | JVM Buffer 池已用字节 |

---

## 十四、JMX 指标 — Consumer Group 状态（7 项）

> 映射模式：`directMappings`（JMX 维度的消费组统计）

| # | VM 指标名 | 类型 | JMX 原始名 | 说明 |
|---|-----------|------|-----------|------|
| 117 | `kafka_broker_consumer_group_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroups` | JMX 维度消费组总数 |
| 118 | `kafka_broker_consumer_group_stable_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroupsstable` | Stable 状态消费组数 |
| 119 | `kafka_broker_consumer_group_empty_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroupsempty` | Empty 状态消费组数 |
| 120 | `kafka_broker_consumer_group_preparing_rebalance_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroupspreparingrebalance` | Preparing Rebalance 数 |
| 121 | `kafka_broker_consumer_group_completing_rebalance_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroupscompletingrebalance` | Completing Rebalance 数 |
| 122 | `kafka_broker_consumer_group_dead_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numgroupsdead` | Dead 状态消费组数 |
| 123 | `kafka_broker_consumer_group_offsets_count` | Gauge | `kafka_coordinator_group_groupmetadatamanager_numoffsets` | 已提交 Offset 总数 |

---

## 十五、计算指标（2 项）

> 从 AdminClient 分区详情统计，非 JMX 原始指标，按 Broker 粒度写入
> 采集函数：`collectJMXMetrics`（与 JMX 共用函数，数据来源为 `DescribeTopics(nil)` 返回的分区元数据）

| # | VM 指标名 | 类型 | 采集来源 | 标签 | 说明 |
|---|-----------|------|---------|------|------|
| 124 | `kafka_broker_leader_count` | Gauge | `DescribeTopics(nil)` → 遍历分区 `Leader` 字段，按 broker_id 计数 | cluster_id, cluster_name, broker_id, broker_host | Broker Leader 分区数 |
| 125 | `kafka_broker_replica_count` | Gauge | `DescribeTopics(nil)` → 遍历分区 `Replicas` 切片，按 broker_id 计数 | cluster_id, cluster_name, broker_id, broker_host | Broker 副本总数 |

---

## 十六、前端 PromQL 表达式

> 所有查询通过 `queryVMInstant`（即时，实际为最近 1min 范围取最后值）和 `queryVM` / `queryVMMulti`（范围查询）执行。
> `${clusterId}` 为当前选中集群 ID，`${brokerFilter}` 为可选的 `,broker_id="X"` 过滤。

### 16.1 集群概览 Tab

#### Stat 卡片（17 个即时查询）

| # | 用途 | PromQL |
|---|------|--------|
| 1 | 分区总数 | `sum(kafka_topic_partitions{cluster_id="${clusterId}",topic!~"__.*"})` |
| 2 | 消费组数量 | `count(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})` |
| 3 | 消费组成员总数 | `sum(kafka_consumergroup_members{cluster_id="${clusterId}",consumergroup!~"__.*"})` |
| 4 | ISR 总数 | `sum(kafka_topic_partition_in_sync_replica{cluster_id="${clusterId}"})` |
| 5 | 非首选 Leader 分区数 | `count(kafka_topic_partition_leader_is_preferred{cluster_id="${clusterId}"}<1)` |
| 6 | 活跃 Broker 数 | `max(kafka_broker_active_broker_count{cluster_id="${clusterId}"})` |
| 7 | 不健康(Fenced) Broker 数 | `max(kafka_broker_fenced_broker_count{cluster_id="${clusterId}"})` |
| 8 | 全局分区数 | `max(kafka_broker_global_partition_count{cluster_id="${clusterId}"})` |
| 9 | 全局 Topic 数 | `max(kafka_broker_global_topic_count{cluster_id="${clusterId}"})` |
| 10 | 副本不均衡数 | `max(kafka_broker_preferred_replica_imbalance{cluster_id="${clusterId}"})` |
| 11 | 离线分区数 | `max(kafka_broker_offline_partitions{cluster_id="${clusterId}"})` |
| 12 | 活跃 Controller 数 | `max(kafka_broker_active_controller{cluster_id="${clusterId}"})` |
| 13 | 离线日志目录数 | `max(kafka_broker_offline_log_directory_count{cluster_id="${clusterId}"})` |
| 14 | 日志目录离线状态 | `max(kafka_broker_log_directory_offline{cluster_id="${clusterId}"})` |
| 15 | 无效 Magic Number 记录 | `sum(kafka_broker_invalid_magic_number_records_total{cluster_id="${clusterId}"})` |
| 16 | 无效 CRC 记录 | `sum(kafka_broker_invalid_message_crc_records_total{cluster_id="${clusterId}"})` |
| 17 | 无效 Offset/Sequence 记录 | `sum(kafka_broker_invalid_offset_or_sequence_records_total{cluster_id="${clusterId}"})` |

> 注：Broker 数量、Topic 数量、消费组数量、总 Lag 这 4 个 Stat 卡片直接使用 `metrics` prop（来自后端 `GET /metrics/cluster/:id` 实时 API），不经过 VM PromQL 查询。

#### 趋势图表（9 个范围查询）

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 1 | 消费者组总 Lag | `sum(kafka_consumergroup_lag_sum{cluster_id="${clusterId}"})` | Lag |
| 2 | 集群生产速率 | `sum(rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic!~"__.*"}[30s]))` | msg/s |
| 3 | 集群消费速率 | `sum(rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}"}[30s]))` | msg/s |
| 4 | 字节流入速率 | `sum(rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"}[30s]))` | bytes/s |
| 5 | 字节流出速率 | `sum(rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"}[30s]))` | bytes/s |
| 6 | 拒绝字节速率 | `sum(rate(kafka_broker_bytes_rejected_total{cluster_id="${clusterId}"}[30s]))` | bytes/s |
| 7 | 生产请求失败率 | `sum(rate(kafka_broker_failed_produce_requests_total{cluster_id="${clusterId}"}[30s]))` | 次/秒 |
| 8 | 拉取请求失败率 | `sum(rate(kafka_broker_failed_fetch_requests_total{cluster_id="${clusterId}"}[30s]))` | 次/秒 |
| 9 | 消息流入速率 | `sum(rate(kafka_broker_messages_in_total{cluster_id="${clusterId}"}[30s]))` | msg/s |

### 16.2 Broker 监控 Tab

#### Broker 总览表格（后端 API：`GET /api/v1/metrics/broker-overview/:id`）

后端从 VM 查询以下指标并聚合：

| # | 数据项 | PromQL |
|---|--------|--------|
| 1 | Broker 列表 | `kafka_broker_info{cluster_id="${clusterId}"}` |
| 2 | Leader 数 | `kafka_broker_leader_count{cluster_id="${clusterId}"}` |
| 3 | Replica 数 | `kafka_broker_replica_count{cluster_id="${clusterId}"}` |
| 4 | Controller | `kafka_broker_active_controller{cluster_id="${clusterId}"}` |

> Leader% = Go 层计算：`leader_count / replica_count * 100`

#### Stat 卡片（10 个即时查询）

| # | 用途 | PromQL |
|---|------|--------|
| 1 | ISR 更新失败总数 | `sum(kafka_broker_isr_updates_failed_total{cluster_id="${clusterId}"${brokerFilter}})` |
| 2 | Unclean Leader 选举数 | `sum(kafka_broker_unclean_leader_elections_total{cluster_id="${clusterId}"${brokerFilter}})` |
| 3 | Follower 失败分区数 | `max(kafka_broker_failed_partitions_count{cluster_id="${clusterId}"${brokerFilter}})` |
| 4 | Follower 死线程数 | `max(kafka_broker_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})` |
| 5 | Fetch 延迟过期数 | `sum(kafka_broker_delayed_fetch_expires_total{cluster_id="${clusterId}"${brokerFilter}})` |
| 6 | 进程启动时间 | `max(kafka_broker_process_start_time_seconds{cluster_id="${clusterId}"${brokerFilter}})` |
| 7 | 最大文件描述符数 | `max(kafka_broker_process_max_fds{cluster_id="${clusterId}"${brokerFilter}})` |
| 8 | 不可清理分区数 | `max(kafka_broker_log_cleaner_uncleanable_partitions_count{cluster_id="${clusterId}"${brokerFilter}})` |
| 9 | Cleaner 死线程数 | `max(kafka_broker_log_cleaner_dead_thread_count{cluster_id="${clusterId}"${brokerFilter}})` |
| 10 | JVM 死锁线程数 | `max(kafka_broker_jvm_threads_deadlocked{cluster_id="${clusterId}"${brokerFilter}})` |

#### 趋势图表（43 个范围查询，按 broker_id 分组多 series）

**请求延迟**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 1 | 生产请求延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}` | ms |
| 2 | 消费请求延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}` | ms |
| 3 | 副本同步延迟 P99 | `kafka_broker_request_latency_ms{cluster_id="${clusterId}",request="FetchFollower"${brokerFilter}}` | ms |

**副本同步 Lag**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 4 | 副本同步 Lag | `kafka_broker_replica_max_lag{cluster_id="${clusterId}"${brokerFilter}}` | Lag |

**字节速率**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 5 | 字节流入速率 | `rate(kafka_broker_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |
| 6 | 字节流出速率 | `rate(kafka_broker_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |

**请求延迟分解**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 7 | 请求排队延迟 P99 | `kafka_broker_request_queue_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}` | ms |
| 8 | 本地处理延迟 P99 | `kafka_broker_request_local_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}` | ms |
| 9 | 远程等待延迟 P99 | `kafka_broker_request_remote_time_ms{cluster_id="${clusterId}",request="FetchConsumer"${brokerFilter}}` | ms |

**限流**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 10 | 限流延迟 P99 | `kafka_broker_throttle_time_ms{cluster_id="${clusterId}",request="Produce"${brokerFilter}}` | ms |

**请求错误速率**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 11 | 请求错误速率 | `sum by (request, error) (rate(kafka_broker_request_errors_total{cluster_id="${clusterId}",error!~"NONE"${brokerFilter}}[30s]))` | errors/s |

**副本同步流量**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 12 | 副本同步流入 | `rate(kafka_broker_replication_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |
| 13 | 副本同步流出 | `rate(kafka_broker_replication_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |

**分区迁移流量**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 14 | 分区迁移流入 | `rate(kafka_broker_reassignment_bytes_in_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |
| 15 | 分区迁移流出 | `rate(kafka_broker_reassignment_bytes_out_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |

**ISR 变化**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 16 | ISR 收缩速率 | `rate(kafka_broker_isr_shrinks_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | 次/秒 |
| 17 | ISR 扩展速率 | `rate(kafka_broker_isr_expands_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | 次/秒 |

**网络/线程**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 18 | 响应队列大小 | `kafka_broker_response_queue_size{cluster_id="${clusterId}"${brokerFilter}}` | 个 |
| 19 | Handler 空闲率 | `kafka_broker_request_handler_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}` | % (x100) |
| 20 | 网络 Processor 空闲率 | `kafka_broker_network_processor_avg_idle_percent{cluster_id="${clusterId}"${brokerFilter}}` | % (x100) |

**磁盘**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 21 | 磁盘读取速率 | `rate(kafka_broker_disk_read_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |
| 22 | 磁盘写入速率 | `rate(kafka_broker_disk_write_bytes{cluster_id="${clusterId}"${brokerFilter}}[30s])` | B/s |

**延迟操作**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 23 | Controller 事件排队耗时 | `kafka_broker_controller_event_queue_time_ms{cluster_id="${clusterId}"${brokerFilter}}` | ms |
| 24 | 延迟操作数 | `kafka_broker_delayed_operations{cluster_id="${clusterId}"${brokerFilter}}` | 个 |

**Replica Fetcher**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 25 | Follower 最小拉取速率 | `kafka_broker_min_fetch_rate{cluster_id="${clusterId}"${brokerFilter}}` | 条/秒 |
| 26 | 日志 Flush 耗时 P99 | `kafka_broker_log_flush_time_ms{cluster_id="${clusterId}"${brokerFilter}}` | ms |
| 27 | Purgatory 大小 | `kafka_broker_purgatory_size{cluster_id="${clusterId}"${brokerFilter}}` | 个 |

**系统资源**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 28 | 进程 CPU 使用率 | `rate(kafka_broker_process_cpu_seconds_total{cluster_id="${clusterId}"${brokerFilter}}[30s])` | % (x100) |
| 29 | 进程驻留内存 | `kafka_broker_process_resident_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}` | B |
| 30 | 进程虚拟内存 | `kafka_broker_process_virtual_memory_bytes{cluster_id="${clusterId}"${brokerFilter}}` | B |
| 31 | 已用文件描述符 | `kafka_broker_process_open_fds{cluster_id="${clusterId}"${brokerFilter}}` | 个 |

**Log Cleaner**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 32 | 最大脏比例 | `kafka_broker_log_cleaner_max_dirty_percent{cluster_id="${clusterId}"${brokerFilter}}` | % |
| 33 | 上次清理间隔 | `kafka_broker_log_cleaner_time_since_last_run_ms{cluster_id="${clusterId}"${brokerFilter}}` | ms |
| 34 | 不可清理字节数 | `kafka_broker_log_cleaner_uncleanable_bytes{cluster_id="${clusterId}"${brokerFilter}}` | B |
| 35 | Cleaner 重新复制比例 | `kafka_broker_log_cleaner_recopy_percent{cluster_id="${clusterId}"${brokerFilter}}` | % |
| 36 | Cleaner 最大缓冲利用率 | `kafka_broker_log_cleaner_max_buffer_utilization_percent{cluster_id="${clusterId}"${brokerFilter}}` | % |
| 37 | Cleaner 最大清理时间 | `kafka_broker_log_cleaner_max_clean_time_secs{cluster_id="${clusterId}"${brokerFilter}}` | 秒 |
| 38 | Cleaner 最大压缩延迟 | `kafka_broker_log_cleaner_max_compaction_delay_secs{cluster_id="${clusterId}"${brokerFilter}}` | 秒 |

**JVM**

| # | 图表标题 | PromQL | 单位 |
|---|---------|--------|------|
| 39 | GC 耗时 | `kafka_broker_jvm_gc_seconds_sum{cluster_id="${clusterId}"${brokerFilter}}` | 秒 |
| 40 | GC 次数 | `kafka_broker_jvm_gc_count{cluster_id="${clusterId}"${brokerFilter}}` | 次 |
| 41 | JVM 内存池已用 | `kafka_broker_jvm_memory_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}` | B |
| 42 | JVM 线程数 | `kafka_broker_jvm_threads_current{cluster_id="${clusterId}"${brokerFilter}}` | 个 |
| 43 | JVM Buffer 池已用 | `kafka_broker_buffer_pool_used_bytes{cluster_id="${clusterId}"${brokerFilter}}` | B |

> 并发控制：Broker 监控的 53 个查询分批执行（每批 15 个，批间 100ms），避免 VM 429 限流。

### 16.3 Topic 监控 Tab

#### Topic 列表加载（即时查询）

| # | 用途 | PromQL |
|---|------|--------|
| 1 | Topic 列表 | `kafka_topic_partitions{cluster_id="${clusterId}"}` |

#### 分区级别趋势图表（8 个范围查询，按 partition 分组）

| # | 图表标题 | PromQL | 依赖条件 | 单位 |
|---|---------|--------|---------|------|
| 1 | Topic 生产速率 | `rate(kafka_topic_partition_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}[30s])` | 选 Topic | msg/s |
| 2 | 消费组消费速率 | `rate(kafka_consumergroup_current_offset{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}[30s])` | 选 Topic + 消费组 | msg/s |
| 3 | 消费组 Lag | `kafka_consumergroup_lag{cluster_id="${clusterId}",topic="${selectedTopic}",consumergroup="${selectedConsumerGroup}"}` | 选 Topic + 消费组 | Lag |
| 4 | Topic 日志大小 | `kafka_topic_log_size{cluster_id="${clusterId}",topic="${selectedTopic}"}` | 选 Topic | bytes |
| 5 | Topic LogEndOffset | `kafka_topic_log_end_offset{cluster_id="${clusterId}",topic="${selectedTopic}"}` | 选 Topic | Offset |
| 6 | 分区 ISR 数（柱状图） | `kafka_topic_partition_isr_count{cluster_id="${clusterId}",topic="${selectedTopic}"}` | 选 Topic | 个 |
| 7 | 分区副本数（柱状图） | `kafka_topic_partition_replica_count{cluster_id="${clusterId}",topic="${selectedTopic}"}` | 选 Topic | 个 |
| 8 | Under Replicated 分区数（Stat） | `sum(kafka_topic_partition_under_replicated{cluster_id="${clusterId}",topic="${selectedTopic}"})` | 选 Topic | — |

---

## 十七、JMX 映射模式汇总

| 映射表 | 指标数 | 模式说明 |
|--------|:------:|----------|
| `directMappings` | 38 | 直接映射，无特殊标签 |
| `dualNameMappings` | 7 | 新旧双 JMX 名称兼容（#18-22 / #49 / #63） |
| `quantileWithRequestMappings` | 11 | P99 分位 + `request` 标签 |
| `requestErrorMappings` | 2 | `request` + `error` 双标签 |
| `topicPartitionMappings` | 9 | `topic` + `partition` 标签 |
| `gcMappings` | 2 | `gc` 标签 |
| `poolMappings` | 3 | `pool` 标签 |
| `purgatoryMappings` | 2 | `purgatory` 标签 |
| `brokerTopicMappings` | 29 | BrokerTopicMetrics + 副本管理 + Log Cleaner + Broker 状态 |
| 计算指标 | 2 | 从分区元数据计算（`collectJMXMetrics` 内） |
| **合计** | **103** | — |
| AdminClient 指标 | 17 | — |
| **总计** | **127**（去重后 125 个唯一 VM 指标名，#49/#63 同时出现在 dualNameMappings） | — |

> 注意：部分映射表指标有重叠分类归属（如 `directMappings` 同时包含网络/磁盘/Controller/Consumer Group 等不同类别指标），上方分类章节已按业务维度重新组织。

---

## 十八、命名规范

| 类别 | 前缀 | 示例 |
|---|---|---|
| Broker 级 JMX 指标 | `kafka_broker_` | `kafka_broker_bytes_in_total` |
| Partition 级 AdminClient 指标 | `kafka_topic_partition_` | `kafka_topic_partition_replicas` |
| Partition 级 JMX 指标 | `kafka_topic_partition_` / `kafka_topic_` | `kafka_topic_log_size`, `kafka_topic_partition_isr_count` |
| ConsumerGroup 级指标 | `kafka_consumergroup_` | `kafka_consumergroup_lag` |
| 集群元数据 | `kafka_` | `kafka_broker_count`, `kafka_topic_count` |
| 计数器 | 后缀 `_total` | `kafka_broker_bytes_in_total` |
| Gauge | 无特殊后缀 | `kafka_broker_replica_max_lag` |

---

## 十九、变更记录

| 日期 | 变更内容 |
|---|---|
| 2026-05-09 | Broker Tab 上线：新增 per-broker JMX 指标写入，JMX 指标从集群聚合改为 per-broker 粒度，集群级由 PromQL 聚合 |
| 2026-05-20 | 全面更新：从 30 项指标扩充至 127 项（补全 44 项 gap-analysis 中缺失指标），前端图表从 6 项扩充至 89 项，涵盖集群概览 26 项 / Broker 监控 53 项 / Topic 监控 10 项 |
| 2026-05-20 | 补全原始指标名：AdminClient 指标新增采集来源列（sarama API 调用链），JMX 指标补全 dualNameMappings 新旧双名称 |

> AI生成