package collector

// MetricMapping 定义 JMX 指标到 VM 指标的映射规则
// 注意：当前所有映射均为恒等映射（JMX name = VM name），不再重命名
type MetricMapping struct {
	JMXNames     []string          // JMX Exporter 中的原始指标名（支持多名称兼容旧版）
	VMName       string            // 写入 VictoriaMetrics 的目标指标名（= JMX 原名）
	Quantile     string            // 需要过滤的 quantile 值（空则不过滤）
	LabelKeys    []string          // 需要从 JMX labels 提取并保留的标签名
	LabelRenames map[string]string // 标签重命名（JMX label key -> VM label key）
}

// directMappings — 直接映射（JMX name = VM name），无特殊标签处理
var directMappings = map[string]string{
	// 副本同步
	"kafka_server_replicafetchermanager_maxlag":                "kafka_server_replicafetchermanager_maxlag",
	"kafka_server_replicafetchermanager_minfetchrate":          "kafka_server_replicafetchermanager_minfetchrate",
	"kafka_server_replicafetchermanager_failedpartitionscount": "kafka_server_replicafetchermanager_failedpartitionscount",
	"kafka_server_replicafetchermanager_deadthreadcount":       "kafka_server_replicafetchermanager_deadthreadcount",
	// Controller
	"kafka_controller_kafkacontroller_activecontrollercount":        "kafka_controller_kafkacontroller_activecontrollercount",
	"kafka_controller_controllerstats_uncleanleaderelections_total": "kafka_controller_controllerstats_uncleanleaderelections_total",
	// 集群概览
	"kafka_controller_kafkacontroller_activebrokercount":              "kafka_controller_kafkacontroller_activebrokercount",
	"kafka_controller_kafkacontroller_fencedbrokercount":              "kafka_controller_kafkacontroller_fencedbrokercount",
	"kafka_controller_kafkacontroller_globalpartitioncount":           "kafka_controller_kafkacontroller_globalpartitioncount",
	"kafka_controller_kafkacontroller_globaltopiccount":               "kafka_controller_kafkacontroller_globaltopiccount",
	"kafka_controller_kafkacontroller_preferredreplicaimbalancecount": "kafka_controller_kafkacontroller_preferredreplicaimbalancecount",
	// Broker 状态
	"kafka_server_kafkaserver_brokerstate":          "kafka_server_kafkaserver_brokerstate",
	"kafka_log_logmanager_offlinelogdirectorycount": "kafka_log_logmanager_offlinelogdirectorycount",
	"kafka_log_logmanager_logdirectoryoffline":      "kafka_log_logmanager_logdirectoryoffline",
	// 系统进程
	"process_cpu_seconds_total":     "process_cpu_seconds_total",
	"process_resident_memory_bytes": "process_resident_memory_bytes",
	"process_virtual_memory_bytes":  "process_virtual_memory_bytes",
	"process_start_time_seconds":    "process_start_time_seconds",
	"process_max_fds":               "process_max_fds",
	"process_open_fds":              "process_open_fds",
	// JVM
	"jvm_threads_current":    "jvm_threads_current",
	"jvm_threads_deadlocked": "jvm_threads_deadlocked",
	// Consumer Group
	"kafka_coordinator_group_groupmetadatamanager_numgroups":                    "kafka_coordinator_group_groupmetadatamanager_numgroups",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsstable":              "kafka_coordinator_group_groupmetadatamanager_numgroupsstable",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsempty":               "kafka_coordinator_group_groupmetadatamanager_numgroupsempty",
	"kafka_coordinator_group_groupmetadatamanager_numgroupspreparingrebalance":  "kafka_coordinator_group_groupmetadatamanager_numgroupspreparingrebalance",
	"kafka_coordinator_group_groupmetadatamanager_numgroupscompletingrebalance": "kafka_coordinator_group_groupmetadatamanager_numgroupscompletingrebalance",
	"kafka_coordinator_group_groupmetadatamanager_numgroupsdead":                "kafka_coordinator_group_groupmetadatamanager_numgroupsdead",
	"kafka_coordinator_group_groupmetadatamanager_numoffsets":                   "kafka_coordinator_group_groupmetadatamanager_numoffsets",
	// 延迟操作
	"kafka_server_delayedfetchmetrics_expires_total": "kafka_server_delayedfetchmetrics_expires_total",
	// 网络
	"kafka_network_requestchannel_requestqueuesize":                      "kafka_network_requestchannel_requestqueuesize",
	"kafka_network_requestchannel_responsequeuesize":                     "kafka_network_requestchannel_responsequeuesize",
	"kafka_network_processor_idlepercent":                                "kafka_network_processor_idlepercent",
	"kafka_network_socketserver_networkprocessoravgidlepercent":          "kafka_network_socketserver_networkprocessoravgidlepercent",
	"kafka_network_socketserver_expiredconnectionskilledcount":           "kafka_network_socketserver_expiredconnectionskilledcount",
	"kafka_network_socketserver_memorypoolavailable":                     "kafka_network_socketserver_memorypoolavailable",
	"kafka_network_socketserver_memorypoolused":                          "kafka_network_socketserver_memorypoolused",
	"kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent": "kafka_server_kafkarequesthandlerpool_requesthandleravgidle_percent",
	// 副本管理
	"kafka_server_replicamanager_partitioncount":        "kafka_server_replicamanager_partitioncount",
	"kafka_server_replicamanager_reassigningpartitions": "kafka_server_replicamanager_reassigningpartitions",
}

// dualNameMappings — 支持 JMX 新旧两种命名的直接映射
// 旧版 CamelCase 名统一归一化为新版 lowercase 名（VM name = NewName）
var dualNameMappings = []struct {
	OldName string
	NewName string
	VMName  string
}{
	// 流量指标
	{"kafka_server_brokertopicmetrics_bytesin_total", "kafka_server_BrokerTopicMetrics_BytesInPersec", "kafka_server_brokertopicmetrics_bytesin_total"},
	{"kafka_server_brokertopicmetrics_bytesout_total", "kafka_server_BrokerTopicMetrics_BytesOutPersec", "kafka_server_brokertopicmetrics_bytesout_total"},
	{"kafka_server_brokertopicmetrics_messagesin_total", "kafka_server_BrokerTopicMetrics_MessagesInPersec", "kafka_server_brokertopicmetrics_messagesin_total"},
	// 副本
	{"kafka_server_replicamanager_underreplicatedpartitions", "kafka_server_ReplicaManager_UnderReplicatedPartitions", "kafka_server_replicamanager_underreplicatedpartitions"},
	// Controller
	{"kafka_controller_kafkacontroller_offlinepartitionscount", "kafka_controller_KafkaController_OfflinePartitionsCount", "kafka_controller_kafkacontroller_offlinepartitionscount"},
	// 请求
	{"kafka_server_brokertopicmetrics_totalproducerequests_total", "kafka_server_BrokerTopicMetrics_TotalProduceRequestsPersec", "kafka_server_brokertopicmetrics_totalproducerequests_total"},
	{"kafka_server_brokertopicmetrics_totalfetchrequests_total", "kafka_server_BrokerTopicMetrics_TotalFetchRequestsPersec", "kafka_server_brokertopicmetrics_totalfetchrequests_total"},
}

// quantileWithRequestMappings — 带 quantile 过滤 + request 标签的指标（JMX name = VM name）
var quantileWithRequestMappings = []struct {
	JMXName  string
	VMName   string
	Quantile string
}{
	// 请求延迟
	{"kafka_network_requestmetrics_totaltimems", "kafka_network_requestmetrics_totaltimems", "0.99"},
	{"kafka_network_requestmetrics_requestqueuetimems", "kafka_network_requestmetrics_requestqueuetimems", "0.99"},
	{"kafka_network_requestmetrics_localtimems", "kafka_network_requestmetrics_localtimems", "0.99"},
	{"kafka_network_requestmetrics_remotetimems", "kafka_network_requestmetrics_remotetimems", "0.99"},
	{"kafka_network_requestmetrics_responsequeuetimems", "kafka_network_requestmetrics_responsequeuetimems", "0.99"},
	{"kafka_network_requestmetrics_responsesendtimems", "kafka_network_requestmetrics_responsesendtimems", "0.99"},
	{"kafka_network_requestmetrics_throttletimems", "kafka_network_requestmetrics_throttletimems", "0.99"},
	{"kafka_network_requestmetrics_messageconversionstimems", "kafka_network_requestmetrics_messageconversionstimems", "0.99"},
	{"kafka_network_requestmetrics_requestbytes", "kafka_network_requestmetrics_requestbytes", "0.99"},
}

// quantileOnlyMappings — 仅带 quantile 过滤（无 request 标签）的指标（JMX name = VM name）
var quantileOnlyMappings = []struct {
	JMXName  string
	VMName   string
	Quantile string
}{
	// Controller 事件（有 quantile 但没有 request 标签）
	{"kafka_controller_controllereventmanager_eventqueuetimems", "kafka_controller_controllereventmanager_eventqueuetimems", "0.99"},
	// Log Flush（有 quantile 但没有 request 标签）
	{"kafka_log_logflushstats_logflushrateandtimems", "kafka_log_logflushstats_logflushrateandtimems", "0.99"},
}

// requestErrorMappings — 请求错误指标（带 request + error 双标签，JMX name = VM name）
var requestErrorMappings = []struct {
	JMXName string
	VMName  string
}{
	{"kafka_network_requestmetrics_errors_total", "kafka_network_requestmetrics_errors_total"},
	{"kafka_network_requestmetrics_requests_total", "kafka_network_requestmetrics_requests_total"},
}

// topicPartitionMappings — 带 topic + partition 标签的指标（JMX name = VM name）
var topicPartitionMappings = []struct {
	JMXName string
	VMName  string
}{
	// Topic 分区级
	{"kafka_log_log_size", "kafka_log_log_size"},
	{"kafka_log_log_logendoffset", "kafka_log_log_logendoffset"},
	{"kafka_log_log_logstartoffset", "kafka_log_log_logstartoffset"},
	{"kafka_log_log_numlogsegments", "kafka_log_log_numlogsegments"},
	{"kafka_cluster_partition_underreplicated", "kafka_cluster_partition_underreplicated"},
	{"kafka_cluster_partition_underminisr", "kafka_cluster_partition_underminisr"},
	{"kafka_cluster_partition_insyncreplicascount", "kafka_cluster_partition_insyncreplicascount"},
	{"kafka_cluster_partition_replicascount", "kafka_cluster_partition_replicascount"},
	{"kafka_cluster_partition_laststableoffsetlag", "kafka_cluster_partition_laststableoffsetlag"},
}

// gcMappings — 带 gc 标签的 JVM GC 指标（JMX name = VM name）
var gcMappings = []struct {
	JMXName string
	VMName  string
}{
	{"jvm_gc_collection_seconds_sum", "jvm_gc_collection_seconds_sum"},
	{"jvm_gc_collection_seconds_count", "jvm_gc_collection_seconds_count"},
}

// poolMappings — 带 pool 标签的 JVM 指标（JMX name = VM name）
var poolMappings = []struct {
	JMXName string
	VMName  string
}{
	{"jvm_memory_pool_collection_used_bytes", "jvm_memory_pool_collection_used_bytes"},
	{"jvm_memory_pool_collection_max_bytes", "jvm_memory_pool_collection_max_bytes"},
	{"jvm_buffer_pool_used_bytes", "jvm_buffer_pool_used_bytes"},
}

// purgatoryMappings — 带 purgatory 标签的延迟操作指标（JMX name = VM name）
var purgatoryMappings = []struct {
	JMXName string
	VMName  string
}{
	{"kafka_server_delayedoperationpurgatory_numdelayedoperations", "kafka_server_delayedoperationpurgatory_numdelayedoperations"},
	{"kafka_server_delayedoperationpurgatory_purgatorysize", "kafka_server_delayedoperationpurgatory_purgatorysize"},
}

// brokerTopicMappings — BrokerTopicMetrics 直接映射（JMX name = VM name），无特殊标签
var brokerTopicMappings = map[string]string{
	"kafka_server_brokertopicmetrics_replicationbytesin_total":             "kafka_server_brokertopicmetrics_replicationbytesin_total",
	"kafka_server_brokertopicmetrics_replicationbytesout_total":            "kafka_server_brokertopicmetrics_replicationbytesout_total",
	"kafka_server_brokertopicmetrics_reassignmentbytesin_total":            "kafka_server_brokertopicmetrics_reassignmentbytesin_total",
	"kafka_server_brokertopicmetrics_reassignmentbytesout_total":           "kafka_server_brokertopicmetrics_reassignmentbytesout_total",
	"kafka_server_brokertopicmetrics_bytesrejected_total":                  "kafka_server_brokertopicmetrics_bytesrejected_total",
	"kafka_server_brokertopicmetrics_failedproducerequests_total":          "kafka_server_brokertopicmetrics_failedproducerequests_total",
	"kafka_server_brokertopicmetrics_failedfetchrequests_total":            "kafka_server_brokertopicmetrics_failedfetchrequests_total",
	"kafka_server_brokertopicmetrics_producemessageconversions_total":      "kafka_server_brokertopicmetrics_producemessageconversions_total",
	"kafka_server_brokertopicmetrics_fetchmessageconversions_total":        "kafka_server_brokertopicmetrics_fetchmessageconversions_total",
	"kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total":      "kafka_server_brokertopicmetrics_invalidmagicnumberrecords_total",
	"kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total":       "kafka_server_brokertopicmetrics_invalidmessagecrcrecords_total",
	"kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total": "kafka_server_brokertopicmetrics_invalidoffsetorsequencerecords_total",
	"kafka_server_brokertopicmetrics_nokeycompactedtopicrecords_total":     "kafka_server_brokertopicmetrics_nokeycompactedtopicrecords_total",
	// 副本管理
	"kafka_server_replicamanager_underminisrpartitioncount": "kafka_server_replicamanager_underminisrpartitioncount",
	"kafka_server_replicamanager_atminisrpartitioncount":    "kafka_server_replicamanager_atminisrpartitioncount",
	"kafka_server_replicamanager_offlinereplicacount":       "kafka_server_replicamanager_offlinereplicacount",
	"kafka_server_replicamanager_isrshrinks_total":          "kafka_server_replicamanager_isrshrinks_total",
	"kafka_server_replicamanager_isrexpands_total":          "kafka_server_replicamanager_isrexpands_total",
	"kafka_server_replicamanager_failedisrupdates_total":    "kafka_server_replicamanager_failedisrupdates_total",
	// Log Cleaner
	"kafka_log_logcleanermanager_max_dirty_percent":            "kafka_log_logcleanermanager_max_dirty_percent",
	"kafka_log_logcleanermanager_time_since_last_run_ms":       "kafka_log_logcleanermanager_time_since_last_run_ms",
	"kafka_log_logcleanermanager_uncleanable_bytes":            "kafka_log_logcleanermanager_uncleanable_bytes",
	"kafka_log_logcleanermanager_uncleanable_partitions_count": "kafka_log_logcleanermanager_uncleanable_partitions_count",
	"kafka_log_logcleaner_cleaner_recopy_percent":              "kafka_log_logcleaner_cleaner_recopy_percent",
	"kafka_log_logcleaner_deadthreadcount":                     "kafka_log_logcleaner_deadthreadcount",
	"kafka_log_logcleaner_max_buffer_utilization_percent":      "kafka_log_logcleaner_max_buffer_utilization_percent",
	"kafka_log_logcleaner_max_clean_time_secs":                 "kafka_log_logcleaner_max_clean_time_secs",
	"kafka_log_logcleaner_max_compaction_delay_secs":           "kafka_log_logcleaner_max_compaction_delay_secs",
	// Broker 状态
	"kafka_server_kafkaserver_linux_disk_read_bytes":  "kafka_server_kafkaserver_linux_disk_read_bytes",
	"kafka_server_kafkaserver_linux_disk_write_bytes": "kafka_server_kafkaserver_linux_disk_write_bytes",
}
