package model

import "time"

// PathConfigPath 只公开稳定序号与名称，不暴露本地或目标内部标识。
type PathConfigPath struct {
	SequenceNo uint   `json:"sequenceNo"`
	Name       string `json:"name"`
}

// PathConfigSaveResult 是保存配置成功后返回的最小结果，供幂等重试返回同一事实。
type PathConfigSaveResult struct {
	Path         PathConfigPath `json:"path"`
	Revision     uint64         `json:"revision"`
	NodeRevision uint64         `json:"nodeRevision"`
	FormRevision uint64         `json:"formRevision"`
	Status       string         `json:"status"`
}

// PathConfiguration 是单条已保存路径的完整配置工作台模型。
type PathConfiguration struct {
	Path         PathConfigPath     `json:"path"`
	Revision     uint64             `json:"revision"`
	NodeRevision uint64             `json:"nodeRevision"`
	Status       string             `json:"status"`
	Progress     PathConfigProgress `json:"progress"`
	NextNodeKey  string             `json:"nextNodeKey"`
	Groups       []PathConfigGroup  `json:"groups"`
	Warnings     []string           `json:"warnings"`
	Form         PathFormConfig     `json:"form"`
}

// PathFormConfig 是与节点配置分离的真实 FormMaking 表单工作区模型。
type PathFormConfig struct {
	Revision            uint64                   `json:"revision"`
	Status              string                   `json:"status"`
	StatusName          string                   `json:"statusName"`
	ReadOnly            bool                     `json:"readOnly"`
	Template            map[string]any           `json:"template"`
	Permissions         []PathFormPermission     `json:"permissions"`
	Values              map[string]any           `json:"values"`
	Seed                int64                    `json:"seed"`
	GeneratedFieldPaths []string                 `json:"generatedFieldPaths"`
	ManualOverridePaths []string                 `json:"manualOverridePaths"`
	SampleSummary       PathFormSampleSummary    `json:"sampleSummary"`
	Validated           bool                     `json:"validated"`
	Unsupported         []string                 `json:"unsupported"`
	Affected            []PathConfigAffectedItem `json:"affected"`
	AutoFilled          int                      `json:"autoFilled"`
	ManualPending       int                      `json:"manualPending"`
}

// PathFormPermission 是 iframe 应用字段权限所需的最小字段键与权限。
type PathFormPermission struct {
	Field string `json:"field"`
	Power string `json:"power"`
}

// PathFormSampleSummary 说明生成使用的样本层级，不公开实例、账号或目标内部标识。
type PathFormSampleSummary struct {
	Saved    bool `json:"saved"`
	Defaults int  `json:"defaults"`
	Recent   int  `json:"recent"`
	Fallback int  `json:"fallback"`
}

// PathFormGenerateResult 是智能生成或换一组返回的权威表单草稿。
type PathFormGenerateResult struct {
	Revision            uint64                `json:"revision"`
	Status              string                `json:"status"`
	Values              map[string]any        `json:"values"`
	Seed                int64                 `json:"seed"`
	GeneratedFieldPaths []string              `json:"generatedFieldPaths"`
	ManualOverridePaths []string              `json:"manualOverridePaths"`
	SampleSummary       PathFormSampleSummary `json:"sampleSummary"`
	AutoFilled          int                   `json:"autoFilled"`
	ManualPending       int                   `json:"manualPending"`
	Unsupported         []string              `json:"unsupported"`
}

// PathFormSaveInput 是表单运行时校验后提交给服务层的完整 values 与生成元数据。
type PathFormSaveInput struct {
	Revision            uint64                `json:"revision"`
	Values              map[string]any        `json:"values"`
	Seed                int64                 `json:"seed"`
	GeneratedFieldPaths []string              `json:"generatedFieldPaths"`
	ManualOverridePaths []string              `json:"manualOverridePaths"`
	SampleSummary       PathFormSampleSummary `json:"sampleSummary"`
	Validated           bool                  `json:"validated"`
}

// PathNodeSaveInput 是单个节点人员与动作的最小回写体。
type PathNodeSaveInput struct {
	Revision uint64                  `json:"revision"`
	Actions  []PathConfigActionValue `json:"actions"`
}

// PathFormRuntimeSession 是 iframe 当前会话使用的短期目标读取上下文；绝不持久化。
type PathFormRuntimeSession struct {
	SID         string `json:"sid"`
	BaseURL     string `json:"baseURL"`
	AccountName string `json:"accountName"`
}

// PathConfigProgress 汇总当前路径节点配置进度，不把结构上下文节点误计为待处理项。
type PathConfigProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
}

// PathConfigGroup 表示主线或一条并行分支的节点顺序分组。
type PathConfigGroup struct {
	Title string           `json:"title"`
	Kind  string           `json:"kind"`
	Nodes []PathConfigNode `json:"nodes"`
}

// PathConfigNode 是路径顺序上的一个业务节点及其字段、缺口和标准动作。
type PathConfigNode struct {
	Key          string             `json:"key"`
	Name         string             `json:"name"`
	TypeName     string             `json:"typeName"`
	Kind         string             `json:"kind"`
	Status       string             `json:"status"`
	StatusName   string             `json:"statusName"`
	Fields       []PathConfigField  `json:"fields"`
	Persons      []PathConfigPerson `json:"persons"`
	Gaps         []PathConfigGap    `json:"gaps"`
	Requirements []RequirementItem  `json:"requirements"`
	Actions      []PathConfigAction `json:"actions"`
	LineBlocked  bool               `json:"lineBlocked"`
}

// PathConfigPerson 是模板约束下的处理人呈现；只有 editable=true 时浏览器才允许回写候选。
type PathConfigPerson struct {
	Key      string                   `json:"key"`
	Title    string                   `json:"title"`
	Mode     string                   `json:"mode"`
	Detail   string                   `json:"detail"`
	Editable bool                     `json:"editable"`
	Multiple bool                     `json:"multiple"`
	Required bool                     `json:"required"`
	MinCount int                      `json:"minCount"`
	Selected []string                 `json:"selected"`
	Options  []PathConfigPersonOption `json:"options"`
	Affected bool                     `json:"affected"`
	Note     string                   `json:"note"`
}

// PathConfigPersonOption 只公开合法候选的中文名称与不透明回写键。
type PathConfigPersonOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
	PathID              uint64
	Revision            uint64
	NodeRevision        uint64
	FormRevision        uint64
	IdempotencyKey      string
	Status              string
	ConfigVersion       uint
	FieldValues         map[string]map[string]string
	ActionValues        map[string]string
	ConfirmedNodeKeys   []string
	FormValues          map[string]any
	FormStatus          string
	FormValidated       bool
	FormSeed            int64
	GeneratedFieldPaths []string
	ManualOverridePaths []string
	SampleSummary       PathFormSampleSummary
	FormTemplateVersion string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
