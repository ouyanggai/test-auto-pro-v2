package model

import "time"

// PathConfigPath 只公开稳定序号与名称，不暴露本地或目标内部标识。
type PathConfigPath struct {
	SequenceNo uint   `json:"sequenceNo"`
	Name       string `json:"name"`
}

// PathConfigSaveResult 是保存配置成功后返回的最小结果，供幂等重试返回同一事实。
type PathConfigSaveResult struct {
	Path     PathConfigPath `json:"path"`
	Revision uint64         `json:"revision"`
	Status   string         `json:"status"`
}

// PathConfiguration 是单条已保存路径的完整配置工作台模型。
type PathConfiguration struct {
	Path     PathConfigPath    `json:"path"`
	Revision uint64            `json:"revision"`
	Status   string            `json:"status"`
	Groups   []PathConfigGroup `json:"groups"`
	Warnings []string          `json:"warnings"`
}

// PathConfigGroup 表示主线或一条并行分支的节点顺序分组。
type PathConfigGroup struct {
	Title string           `json:"title"`
	Kind  string           `json:"kind"`
	Nodes []PathConfigNode `json:"nodes"`
}

// PathConfigNode 是路径顺序上的一个业务节点及其字段、缺口和标准动作。
type PathConfigNode struct {
	Name        string             `json:"name"`
	TypeName    string             `json:"typeName"`
	Kind        string             `json:"kind"`
	Fields      []PathConfigField  `json:"fields"`
	Gaps        []PathConfigGap    `json:"gaps"`
	Actions     []PathConfigAction `json:"actions"`
	LineBlocked bool               `json:"lineBlocked"`
}

// PathConfigField 是可配置基础字段的安全展示与回写载体。
type PathConfigField struct {
	Key      string             `json:"key"`
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	Required bool               `json:"required"`
	Value    string             `json:"value"`
	Options  []PathConfigOption `json:"options"`
	Editable bool               `json:"editable"`
	Affected bool               `json:"affected"`
	Note     string             `json:"note"`
}

// PathConfigOption 只保留目标平台返回的可展示选项标签与回写值。
type PathConfigOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PathConfigGap 表示当前工具无法安全编辑的项目及具体原因。
type PathConfigGap struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// PathConfigAction 是发起提交或审批/协同结果的配置项。
type PathConfigAction struct {
	Key             string                   `json:"key"`
	Kind            string                   `json:"kind"`
	Label           string                   `json:"label"`
	Current         string                   `json:"current"`
	Default         string                   `json:"default"`
	Options         []PathConfigActionOption `json:"options"`
	DisagreeWarning string                   `json:"disagreeWarning"`
}

// PathConfigActionOption 是动作的稳定候选与中文标签。
type PathConfigActionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PathConfigFieldValue 是浏览器回写的一个字段值；Key 为不透明回写键。
type PathConfigFieldValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PathConfigActionValue 是浏览器回写的一个节点动作；Key 为不透明回写键。
type PathConfigActionValue struct {
	Key    string `json:"key"`
	Action string `json:"action"`
}

// PathConfigAffectedItem 是保存校验失败时定位到具体字段或动作的安全说明。
type PathConfigAffectedItem struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// StoredPathConfig 是数据库中的路径唯一配置记录。
type StoredPathConfig struct {
	PathID         uint64
	Revision       uint64
	IdempotencyKey string
	Status         string
	FieldValues    map[string]map[string]string
	ActionValues   map[string]string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
