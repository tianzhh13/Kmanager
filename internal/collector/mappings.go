package collector

// MetricMapping 定义 JMX 指标到 VM 指标的映射规则
type MetricMapping struct {
	JMXNames     []string          // JMX Exporter 中的原始指标名（支持多名称兼容旧版）
	VMName       string            // 写入 VictoriaMetrics 的目标指标名
	Quantile     string            // 需要过滤的 quantile 值（空则不过滤）
	LabelKeys    []string          // 需要从 JMX labels 提取并保留的标签名
	LabelRenames map[string]string // 标签重命名（JMX label key -> VM label key）
}

// 以下为 5 种模式的注册表

// directMappings — 直接映射，无特殊标签处理
var directMappings = map[string]string{
	// 副本同步
	"kafka_server_replicafetchermanager_maxlag":                "kafka_broker_replica_max_lag",
	"kafka_server_replicafetchermanager_minfetchrate":          "kafka_broker_min_fetch_rate",
	"kafka_server_replicafetchermanager_failedpartitionscount": "kafka_broker_failed_partitions_count",
	"kafka_server_replicafetchermanager_deadthreadcount":       "kafka_broker_dead_thread_count",
	// Controller
	"kafka_controller_kafkacontroller_activecontrollercount":        "kafka_broker_active_controller",
	"kafka_controller_controllerstats_uncleanleaderelections_total": "kafka_broker_unclean_leader_elections_total",
	// 集群概览
	"kafka_controller_kafkacontroller_activebrokercount":              "kafka_broker_active_broker_count",
	"kafka_controller_kafkacontroller_fencedbrokercount":              "kafka_broker_fenced_broker_count",
	"kafka_controller_kafkacontroller_globalpartitioncount":           "kafka_broker_global_partition_count",
	"kafka_controller_kafkacontroller_globaltopiccount":               "kafka_broker_global_topic_count",
	"kafka_controller_kafkacontroller_preferredreplicaimbalancecount": "kafka_broker_preferred_replica_imbalance",
	// Broker 状态
	"kafka_server_kafkaserver_brokerstate":          "kafka_broker_state",
	"kafka_log_logmanager_offlinelogdirectorycount": "kafka_broker_offline_log_directory_count",
	"kafka_log_logmanager_logdirectoryoffline":      "kafka_broker_log_directory_offline",
	// 系统进程
	"process_cpu_seconds_total":     "kafka_broker_process_cpu_seconds_total",
	"process_resident_memory_bytes": "kafka_broker_process_resident_memory_bytes",
	"process_virtual_memory_bytes":  "kafka_broker_process_virtual_memory_bytes",
	"process_start_time_seconds":    "kafka_broker_process_start_time_seconds",
	"process_max_fds":               "kafka_broker_process_max_fds",
	"process_open_fds":              "kafka_broker_process_open_fds",
	// JVM
	"jvm_threads_current":    "kafka_broker_jvm_threads_current",
	"jvm_threads_deadlocked": "kafka_broker_jvm_threads_deadlocked",
	// Consumer Group
	"kafka_coordinator_group_groupmetadatamanager_numgroups":                    "kafka_broker_consumer_group_count",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsstable":              "kafka_broker_consumer_group_stable_count",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsempty":               "kafka_broker_consumer_group_empty_count",
	"kafka_coordinator_group_groupmetadatamanager_numgroupspreparingrebalance":  "kafka_broker_consumer_group_preparing_rebalance_count",
	"kafka_coordinator_group_groupmetadatamanager_numgroupscompletingrebalance": "kafka_broker_consumer_group_completing_rebalance_count",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsdead":                "kafka_broker_consumer_group_dead_count",
	"kafka_coordinator_group_groupmetadatamanager_numoffsets":                   "kafka_broker_consumer_group_offsets_count",
	// 延迟操作
	"kafka_server_delayedfetchmetrics_expires_total": "kafka_broker_delayed_fetch_expires_total",
	// 网络
	"kafka_network_requestchannel_requestqueuesize":                      "kafka_broker_request_queue_size",
	"kafka_network_requestchannel_responsequeuesize":                     "kafka_broker_response_queue_size",
	"kafka_network_processor_idlepercent":                                "kafka_broker_processor_idle_percent",
	"kafka_network_socketserver_networkprocessoravgidlepercent":          "kafka_broker_network_processor_avg_idle_percent",
	"kafka_network_socketserver_expiredconnectionskilledcount":           "kafka_broker_expired_connections_killed_count",
	"kafka_network_socketserver_memorypoolavailable":                     "kafka_broker_memory_pool_available",
	"kafka_network_socketserver_memorypoolused":                          "kafka_broker_memory_pool_used",
	"kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent": "kafka_broker_request_handler_avg_idle_percent",
	// 副本管理
	"kafka_server_replicamanager_partitioncount":        "kafka_broker_partition_count",
	"kafka_server_replicamanager_reassigningpartitions": "kafka_broker_reassigning_partitions",
}

// dualNameMappings — 支持 JMX 新旧两种命名的直接映射
var dualNameMappings = []struct {
	OldName string
	NewName string
	VMName  string
}{
	// 流量指标
	{"kafka_server_brokertopicmetrics_bytesin_total", "kafka_server_BrokerTopicMetrics_BytesInPersec", "kafka_broker_bytes_in_total"},
	{"kafka_server_brokertopicmetrics_bytesout_total", "kafka_server_BrokerTopicMetrics_BytesOutPersec", "kafka_broker_bytes_out_total"},
	{"kafka_server_brokertopicmetrics_messagesin_total", "kafka_server_BrokerTopicMetrics_MessagesInPersec", "kafka_broker_messages_in_total"},
	// 副本
	{"kafka_server_replicamanager_underreplicatedpartitions", "kafka_server_ReplicaManager_UnderReplicatedPartitions", "kafka_broker_under_replicated_partitions"},
	// Controller
	{"kafka_controller_kafkacontroller_offlinepartitionscount", "kafka_controller_KafkaController_OfflinePartitionsCount", "kafka_broker_offline_partitions"},
	// 请求
	{"kafka_server_brokertopicmetrics_totalproducerequests_total", "kafka_server_BrokerTopicMetrics_TotalProduceRequestsPersec", "kafka_broker_produce_requests_total"},
	{"kafka_server_brokertopicmetrics_totalfetchrequests_total", "kafka_server_BrokerTopicMetrics_TotalFetchRequestsPersec", "kafka_broker_fetch_requests_total"},
}

// quantileWithRequestMappings — 带 quantile 过滤 + request 标签的指标
var quantileWithRequestMappings = []struct {
	JMXName  string
	VMName   string
	Quantile string
}{
	// 请求延迟
	{"kafka_network_requestmetrics_totaltimems", "kafka_broker_request_latency_ms", "0.99"},
	{"kafka_network_requestmetrics_requestqueuetimems", "kafka_broker_request_queue_time_ms", "0.99"},
	{"kafka_network_requestmetrics_localtimems", "kafka_broker_request_local_time_ms", "0.99"},
	{"kafka_network_requestmetrics_remotetimems", "kafka_broker_request_remote_time_ms", "0.99"},
	{"kafka_network_requestmetrics_responsequeuetimems", "kafka_broker_request_queue_time_ms_response", "0.99"},
	{"kafka_network_requestmetrics_responsesendtimems", "kafka_broker_request_response_send_time_ms", "0.99"},
	{"kafka_network_requestmetrics_throttletimems", "kafka_broker_throttle_time_ms", "0.99"},
	{"kafka_network_requestmetrics_messageconversionstimems", "kafka_broker_message_conversions_time_ms", "0.99"},
	{"kafka_network_requestmetrics_requestbytes", "kafka_broker_request_bytes", "0.99"},
}

// quantileOnlyMappings — 仅带 quantile 过滤（无 request 标签）的指标
var quantileOnlyMappings = []struct {
	JMXName  string
	VMName   string
	Quantile string
}{
	// Controller 事件（有 quantile 但没有 request 标签）
	{"kafka_controller_controllereventmanager_eventqueuetimems", "kafka_broker_controller_event_queue_time_ms", "0.99"},
	// Log Flush（有 quantile 但没有 request 标签）
	{"kafka_log_logflushstats_logflushrateandtimems", "kafka_broker_log_flush_time_ms", "0.99"},
}

// requestErrorMappings — 请求错误指标（带 request + error 双标签）
var requestErrorMappings = []struct {
	JMXName string
	VMName  string
}{
	{"kafka_network_requestmetrics_errors_total", "kafka_broker_request_errors_total"},
	{"kafka_network_requestmetrics_requests_total", "kafka_broker_requests_total"},
}

// topicPartitionMappings — 带 topic + partition 标签的指标
var topicPartitionMappings = []struct {
	JMXName string
	VMName  string
}{
	// Topic 分区级
	{"kafka_log_log_size", "kafka_topic_log_size"},
	{"kafka_log_log_logendoffset", "kafka_topic_log_end_offset"},
	{"kafka_log_log_logstartoffset", "kafka_topic_log_start_offset"},
	{"kafka_log_log_numlogsegments", "kafka_topic_log_num_segments"},
	{"kafka_cluster_partition_underreplicated", "kafka_topic_partition_under_replicated"},
	{"kafka_cluster_partition_underminisr", "kafka_topic_partition_under_min_isr"},
	{"kafka_cluster_partition_insyncreplicascount", "kafka_topic_partition_isr_count"},
	{"kafka_cluster_partition_replicascount", "kafka_topic_partition_replica_count"},
	{"kafka_cluster_partition_laststableoffsetlag", "kafka_topic_partition_last_stable_offset_lag"},
}

// gcMappings — 带 gc 标签的 JVM GC 指标
var gcMappings = []struct {
	JMXName string
	VMName  string
}{
	{"jvm_gc_collection_seconds_sum", "kafka_broker_jvm_gc_seconds_sum"},
	{"jvm_gc_collection_seconds_count", "kafka_broker_jvm_gc_count"},
}

// poolMappings — 带 pool 标签的 JVM 指标
var poolMappings = []struct {
	JMXName string
	VMName  string
}{
	{"jvm_memory_pool_collection_used_bytes", "kafka_broker_jvm_memory_pool_used_bytes"},
	{"jvm_memory_pool_collection_max_bytes", "kafka_broker_jvm_memory_pool_max_bytes"},
	{"jvm_buffer_pool_used_bytes", "kafka_broker_jvm_buffer_pool_used_bytes"},
}

// purgatoryMappings — 带 purgatory 标签的延迟操作指标
var purgatoryMappings = []struct {
	JMXName string
	VMName  string
}{
	{"kafka_server_delayedoperationpurgatory_numdelayedoperations", "kafka_broker_delayed_operations"},
	{"kafka_server_delayedoperationpurgatory_purgatorysize", "kafka_broker_purgatory_size"},
}

// brokerTopicMappings — BrokerTopicMetrics 直接映射（无特殊标签，但需排除 __.* Topic）
var brokerTopicMappings = map[string]string{
	"kafka_server_brokertopicmetrics_replicationbytesin_total":             "kafka_broker_replication_bytes_in_total",
	"kafka_server_brokertopicmetrics_replicationbytesout_total":            "kafka_broker_replication_bytes_out_total",
	"kafka_server_brokertopicmetrics_reassignmentbytesin_total":            "kafka_broker_reassignment_bytes_in_total",
	"kafka_server_brokertopicmetrics_reassignmentbytesout_total":           "kafka_broker_reassignment_bytes_out_total",
	"kafka_server_brokertopicmetrics_bytesrejected_total":                  "kafka_broker_bytes_rejected_total",
	"kafka_server_brokertopicmetrics_failedproducerequests_total":          "kafka_broker_failed_produce_requests_total",
	"kafka_server_brokertopicmetrics_failedfetchrequests_total":            "kafka_broker_failed_fetch_requests_total",
	"kafka_server_brokertopicmetrics_producemessageconversions_total":      "kafka_broker_produce_message_conversions_total",
	"kafka_server_brokertopicmetrics_fetchmessageconversions_total":        "kafka_broker_fetch_message_conversions_total",
	"kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total":      "kafka_broker_invalid_magic_number_records_total",
	"kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total":       "kafka_broker_invalid_message_crc_records_total",
	"kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total": "kafka_broker_invalid_offset_or_sequence_records_total",
	"kafka_server_brokertopicmetrics_nokeycompactedtopicrecords_total":     "kafka_broker_no_key_compacted_topic_records_total",
	// 副本管理
	"kafka_server_replicamanager_underminisrpartitioncount": "kafka_broker_under_min_isr_partition_count",
	"kafka_server_replicamanager_atminisrpartitioncount":    "kafka_broker_at_min_isr_partition_count",
	"kafka_server_replicamanager_offlinereplicacount":       "kafka_broker_offline_replica_count",
	"kafka_server_replicamanager_isrshrinks_total":          "kafka_broker_isr_shrinks_total",
	"kafka_server_replicamanager_isrexpands_total":          "kafka_broker_isr_expands_total",
	"kafka_server_replicamanager_failedisrupdates_total":    "kafka_broker_isr_updates_failed_total",
	// Log Cleaner
	"kafka_log_logcleanermanager_max_dirty_percent":            "kafka_broker_log_cleaner_max_dirty_percent",
	"kafka_log_logcleanermanager_time_since_last_run_ms":       "kafka_broker_log_cleaner_time_since_last_run_ms",
	"kafka_log_logcleanermanager_uncleanable_bytes":            "kafka_broker_log_cleaner_uncleanable_bytes",
	"kafka_log_logcleanermanager_uncleanable_partitions_count": "kafka_broker_log_cleaner_uncleanable_partitions_count",
	"kafka_log_logcleaner_cleaner_recopy_percent":              "kafka_broker_log_cleaner_recopy_percent",
	"kafka_log_logcleaner_deadthreadcount":                     "kafka_broker_log_cleaner_dead_thread_count",
	"kafka_log_logcleaner_max_buffer_utilization_percent":      "kafka_broker_log_cleaner_max_buffer_utilization_percent",
	"kafka_log_logcleaner_max_clean_time_secs":                 "kafka_broker_log_cleaner_max_clean_time_secs",
	"kafka_log_logcleaner_max_compaction_delay_secs":           "kafka_broker_log_cleaner_max_compaction_delay_secs",
	// Broker 状态
	"kafka_server_kafkaserver_linux_disk_read_bytes":  "kafka_broker_disk_read_bytes",
	"kafka_server_kafkaserver_linux_disk_write_bytes": "kafka_broker_disk_write_bytes",
}
