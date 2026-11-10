package ticket

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// 工单表单常量
const (
	// ModeNewUser 新建 SCRAM 用户并授权
	ModeNewUser = "new_user"
	// ModeExistingUser 已有 SCRAM 用户增加权限
	ModeExistingUser = "existing_user"

	// PermissionRead 只读（消费者）
	PermissionRead = "read"
	// PermissionWrite 只写（生产者）
	PermissionWrite = "write"
	// PermissionReadWrite 读写
	PermissionReadWrite = "read_write"

	// ExtraDescribeConfigs 允许查看 Topic 配置（TOPIC describeconfigs）
	ExtraDescribeConfigs = "describeconfigs"
	// ExtraGroupDescribe 允许自查消费位点（GROUP describe）
	ExtraGroupDescribe = "group_describe"
)

var (
	topicNameRegex  = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,249}$`)
	usernameRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
	groupNameRegex  = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,200}$`)
)

// TopicCreateItem 批量申请 Topic 时的单个条目（与单个申请字段一致）
type TopicCreateItem struct {
	TopicName         string `json:"topic_name"`
	Partitions        int32  `json:"partitions"`
	ReplicationFactor int16  `json:"replication_factor"`
	RetentionDays     int64  `json:"retention_days"` // 可选，0 表示使用集群默认
	BusinessDesc      string `json:"business_desc"`
}

// TopicCreateForm 申请 Topic 工单表单。
// 单个申请填 TopicName 等字段；批量导入填 Items（两者互斥，Items 优先）
type TopicCreateForm struct {
	TopicName         string            `json:"topic_name"`
	Partitions        int32             `json:"partitions"`
	ReplicationFactor int16             `json:"replication_factor"`
	RetentionDays     int64             `json:"retention_days"` // 可选，0 表示使用集群默认
	BusinessDesc      string            `json:"business_desc"`
	Items             []TopicCreateItem `json:"items"`
}

// MaxTopicBatchItems 单张批量工单最多申请的 topic 数（与前端 CSV 行数上限一致）
const MaxTopicBatchItems = 100

// IsBatch 是否批量申请（批量导入合并为一张工单提交）
func (f *TopicCreateForm) IsBatch() bool {
	return len(f.Items) > 0
}

// TopicNames 返回申请的全部 topic 名（批量按 Items 顺序，单个仅一个）
func (f *TopicCreateForm) TopicNames() []string {
	if !f.IsBatch() {
		return []string{f.TopicName}
	}
	names := make([]string, 0, len(f.Items))
	for _, item := range f.Items {
		names = append(names, item.TopicName)
	}
	return names
}

// PermissionGrantForm 申请权限工单表单
type PermissionGrantForm struct {
	Mode         string   `json:"mode"`         // new_user | existing_user
	Username     string   `json:"username"`     // SCRAM 用户名
	Mechanism    string   `json:"mechanism"`    // SCRAM-SHA-256 | SCRAM-SHA-512（new_user 时生效）
	Permission   string   `json:"permission"`   // read | write | read_write
	Topics       []string `json:"topics"`       // topic 名列表
	TopicPattern string   `json:"topic_pattern"` // literal | prefixed
	GroupPrefix  string   `json:"group_prefix"` // 消费组前缀，"*" 表示通配所有组
	Extras       []string `json:"extras"`       // describeconfigs | group_describe
}

// ACLRule 展开后的 ACL 明细（预览与执行共用）
type ACLRule struct {
	ResourceType    string `json:"resource_type"`
	ResourceName    string `json:"resource_name"`
	ResourcePattern string `json:"resource_pattern"`
	Operation       string `json:"operation"`
}

// ParseTopicCreateForm 解析并校验申请 Topic 表单（单个 / 批量共用一个入口，Items 优先）
func ParseTopicCreateForm(formJSON string) (*TopicCreateForm, error) {
	var form TopicCreateForm
	if err := json.Unmarshal([]byte(formJSON), &form); err != nil {
		return nil, fmt.Errorf("表单格式错误: %w", err)
	}

	if form.IsBatch() {
		if err := validateTopicCreateItems(form.Items); err != nil {
			return nil, err
		}
		return &form, nil
	}

	single := TopicCreateItem{
		TopicName:         form.TopicName,
		Partitions:        form.Partitions,
		ReplicationFactor: form.ReplicationFactor,
		RetentionDays:     form.RetentionDays,
		BusinessDesc:      form.BusinessDesc,
	}
	if err := validateTopicCreateItem(&single); err != nil {
		return nil, err
	}
	return &form, nil
}

// validateTopicCreateItems 校验批量条目：逐条校验 + 条目间不重名
func validateTopicCreateItems(items []TopicCreateItem) error {
	if len(items) == 0 {
		return fmt.Errorf("批量申请至少包含 1 个 topic")
	}
	if len(items) > MaxTopicBatchItems {
		return fmt.Errorf("单张工单最多申请 %d 个 topic", MaxTopicBatchItems)
	}
	seen := make(map[string]bool, len(items))
	for i := range items {
		if err := validateTopicCreateItem(&items[i]); err != nil {
			return fmt.Errorf("第 %d 个 topic: %w", i+1, err)
		}
		if seen[items[i].TopicName] {
			return fmt.Errorf("topic 重复: %s", items[i].TopicName)
		}
		seen[items[i].TopicName] = true
	}
	return nil
}

// validateTopicCreateItem 校验单个 topic 申请条目
func validateTopicCreateItem(item *TopicCreateItem) error {
	if !topicNameRegex.MatchString(item.TopicName) || item.TopicName == "." || item.TopicName == ".." {
		return fmt.Errorf("topic 名称不合法（仅允许字母、数字、.、_、-，长度 1-249）")
	}
	if item.Partitions < 1 || item.Partitions > 200 {
		return fmt.Errorf("分区数需在 1-200 之间")
	}
	if item.ReplicationFactor < 1 || item.ReplicationFactor > 5 {
		return fmt.Errorf("副本数需在 1-5 之间")
	}
	if item.RetentionDays < 0 || item.RetentionDays > 3650 {
		return fmt.Errorf("保留天数需在 0-3650 之间（0 表示使用集群默认）")
	}
	if strings.TrimSpace(item.BusinessDesc) == "" {
		return fmt.Errorf("业务说明不能为空")
	}
	return nil
}

// ParsePermissionGrantForm 解析并校验申请权限表单
func ParsePermissionGrantForm(formJSON string) (*PermissionGrantForm, error) {
	var form PermissionGrantForm
	if err := json.Unmarshal([]byte(formJSON), &form); err != nil {
		return nil, fmt.Errorf("表单格式错误: %w", err)
	}

	if form.Mode != ModeNewUser && form.Mode != ModeExistingUser {
		return nil, fmt.Errorf("模式必须为 new_user 或 existing_user")
	}
	if !usernameRegex.MatchString(form.Username) {
		return nil, fmt.Errorf("SCRAM 用户名不合法（仅允许字母、数字、_、-）")
	}
	switch form.Permission {
	case PermissionRead, PermissionWrite, PermissionReadWrite:
	default:
		return nil, fmt.Errorf("权限模板必须为 read、write 或 read_write")
	}
	if len(form.Topics) == 0 {
		return nil, fmt.Errorf("请至少填写一个 topic")
	}
	if len(form.Topics) > 20 {
		return nil, fmt.Errorf("单张工单最多申请 20 个 topic")
	}
	seen := make(map[string]bool)
	for _, t := range form.Topics {
		if !topicNameRegex.MatchString(t) || t == "." || t == ".." {
			return nil, fmt.Errorf("topic 名称不合法: %s", t)
		}
		if seen[t] {
			return nil, fmt.Errorf("topic 重复: %s", t)
		}
		seen[t] = true
	}
	if form.TopicPattern == "" {
		form.TopicPattern = "literal"
	}
	if form.TopicPattern != "literal" && form.TopicPattern != "prefixed" {
		return nil, fmt.Errorf("topic 匹配方式必须为 literal 或 prefixed")
	}

	// 消费权限需要消费组授权
	if form.hasRead() {
		if form.GroupPrefix == "" {
			return nil, fmt.Errorf("申请读权限时消费组前缀不能为空（可填 * 通配所有组）")
		}
		if form.GroupPrefix != "*" && !groupNameRegex.MatchString(form.GroupPrefix) {
			return nil, fmt.Errorf("消费组前缀不合法（仅允许字母、数字、_、-）")
		}
	}

	for _, extra := range form.Extras {
		if extra != ExtraDescribeConfigs && extra != ExtraGroupDescribe {
			return nil, fmt.Errorf("不支持的增强选项: %s", extra)
		}
	}

	if form.Mode == ModeNewUser {
		if form.Mechanism == "" {
			form.Mechanism = "SCRAM-SHA-256"
		}
		if form.Mechanism != "SCRAM-SHA-256" && form.Mechanism != "SCRAM-SHA-512" {
			return nil, fmt.Errorf("机制必须为 SCRAM-SHA-256 或 SCRAM-SHA-512")
		}
	}

	return &form, nil
}

func (f *PermissionGrantForm) hasRead() bool {
	return f.Permission == PermissionRead || f.Permission == PermissionReadWrite
}

func (f *PermissionGrantForm) hasWrite() bool {
	return f.Permission == PermissionWrite || f.Permission == PermissionReadWrite
}

func (f *PermissionGrantForm) hasExtra(extra string) bool {
	for _, e := range f.Extras {
		if e == extra {
			return true
		}
	}
	return false
}

// ExpandACLs 将权限模板展开为 ACL 明细。
// describe 随 read/write 自动授予（metadata 请求需要，缺失会导致客户端拿不到分区信息）；
// group 只授 read/describe（broker 仅检查这两个操作）。
func (f *PermissionGrantForm) ExpandACLs() []*ACLRule {
	rules := make([]*ACLRule, 0)

	// Topic 维度
	topicOps := make([]string, 0)
	if f.hasRead() {
		topicOps = append(topicOps, "read", "describe")
	}
	if f.hasWrite() {
		topicOps = append(topicOps, "write", "describe")
	}
	if f.hasExtra(ExtraDescribeConfigs) {
		topicOps = append(topicOps, "describeconfigs")
	}
	for _, t := range f.Topics {
		for _, op := range topicOps {
			rules = append(rules, &ACLRule{
				ResourceType:    "topic",
				ResourceName:    t,
				ResourcePattern: f.TopicPattern,
				Operation:       op,
			})
		}
	}

	// Group 维度（消费场景）
	if f.hasRead() {
		groupName, groupPattern := f.GroupPrefix, "prefixed"
		if f.GroupPrefix == "*" {
			groupName, groupPattern = "*", "literal"
		}
		groupOps := []string{"read"}
		if f.hasExtra(ExtraGroupDescribe) {
			groupOps = append(groupOps, "describe")
		}
		for _, op := range groupOps {
			rules = append(rules, &ACLRule{
				ResourceType:    "group",
				ResourceName:    groupName,
				ResourcePattern: groupPattern,
				Operation:       op,
			})
		}
	}

	return rules
}

// Principal 返回 ACL Principal（Kafka SCRAM 认证的 principal 为 User:用户名）
func (f *PermissionGrantForm) Principal() string {
	return "User:" + f.Username
}

// BuildTopicDescription 拼接 topic 描述：申请人信息由平台自动带入，不依赖申请人手写
func BuildTopicDescription(applicantUsername string, appliedAt time.Time, businessDesc string) string {
	desc := fmt.Sprintf("[申请人:%s %s 申请] %s", applicantUsername, appliedAt.Format("2006-01-02"), strings.TrimSpace(businessDesc))
	// topic.description 列为 varchar(512)，超长截断
	if len(desc) > 512 {
		desc = desc[:512]
	}
	return desc
}
