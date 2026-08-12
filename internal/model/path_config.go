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
	Unsupported         []string              `json:"unsupported"`
}

// PathNodeSaveInput 是单个节点人员与动作的最小回写体。
type PathNodeSaveInput struct {
	Revision uint64                          `json:"revision"`
	Persons  []PathConfigPersonStrategyInput `json:"persons"`
	Arrivals []PathConfigArrivalInput        `json:"arrivals"`
	Actions  []PathConfigActionValue         `json:"actions,omitempty"`
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

// PathConfigNode 是路径顺序上的一个业务节点及其人员、规则和标准动作。
type PathConfigNode struct {
	Key          string               `json:"key"`
	Name         string               `json:"name"`
	TypeName     string               `json:"typeName"`
	Kind         string               `json:"kind"`
	Status       string               `json:"status"`
	StatusName   string               `json:"statusName"`
	Fields       []PathConfigField    `json:"fields"` // 兼容旧响应，节点配置不再投影表单字段。
	Persons      []PathConfigPerson   `json:"persons"`
	Gaps         []PathConfigGap      `json:"gaps"` // 兼容旧响应，组件缺口只由表单工作区呈现。
	Requirements []RequirementItem    `json:"requirements"`
	Actions      []PathConfigAction   `json:"actions"`
	ActionPlan   PathConfigActionPlan `json:"actionPlan"`
	LineBlocked  bool                 `json:"lineBlocked"`
}

// PathConfigPerson 是模板约束下的处理人呈现；只有 editable=true 时浏览器才允许回写候选。
type PathConfigPerson struct {
	Key             string                           `json:"key"`
	Title           string                           `json:"title"`
	Mode            string                           `json:"mode"`
	Detail          string                           `json:"detail"`
	Items           []PathConfigPersonDisplayItem    `json:"items"`
	Editable        bool                             `json:"editable"`
	Multiple        bool                             `json:"multiple"`
	Required        bool                             `json:"required"`
	MinCount        int                              `json:"minCount"`
	MaxCount        int                              `json:"maxCount"`
	Selected        []string                         `json:"selected"`
	DefaultSelected []string                         `json:"defaultSelected"`
	Options         []PathConfigPersonOption         `json:"options"`
	Strategy        string                           `json:"strategy"`
	StrategySeed    int64                            `json:"strategySeed"`
	Strategies      []PathConfigPersonStrategyOption `json:"strategies"`
	Affected        bool                             `json:"affected"`
	Note            string                           `json:"note"`
}

// PathConfigPersonDisplayItem 公开目标模板中可证明的人员规则类别、中文名称和同类数量，不携带业务 ID。
type PathConfigPersonDisplayItem struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
}

// PathConfigPersonOption 只公开合法候选的中文名称与不透明回写键。
type PathConfigPersonOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PathConfigPersonStrategyOption 是目标模板当前允许的人员选择策略。
type PathConfigPersonStrategyOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PathConfigPersonStrategyInput 是浏览器保存人员策略的最小不透明回写体。
type PathConfigPersonStrategyInput struct {
	Key      string   `json:"key"`
	Strategy string   `json:"strategy"`
	Seed     int64    `json:"seed"`
	Selected []string `json:"selected"`
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

// PathConfigActionPlan 是节点有序动作列表及其兼容存储投影。
type PathConfigActionPlan struct {
	Catalog         []PathConfigActionCatalogItem `json:"catalog"`
	RollbackTargets []PathConfigActionOption      `json:"rollbackTargets"`
	Arrivals        []PathConfigArrivalPlan       `json:"arrivals"`
	MaxArrivals     int                           `json:"maxArrivals"`
	MaxPathSteps    int                           `json:"maxPathSteps"`
	Affected        bool                          `json:"affected"`
	Note            string                        `json:"note"`
}

// PathConfigActionCatalogItem 说明当前节点可静态证明合法的动作及必要参数。
type PathConfigActionCatalogItem struct {
	Kind           string            `json:"kind"`
	Label          string            `json:"label"`
	Description    string            `json:"description"`
	Enabled        bool              `json:"enabled"`
	DisabledReason string            `json:"disabledReason"`
	AllowsOpinion  bool              `json:"allowsOpinion"`
	RequiresTarget bool              `json:"requiresTarget"`
	RequiresPerson bool              `json:"requiresPerson"`
	Person         *PathConfigPerson `json:"person,omitempty"`
}

// PathConfigArrivalPlan 表示兼容存储中的一组有序动作。
type PathConfigArrivalPlan struct {
	Visit int                    `json:"visit"`
	Steps []PathConfigActionStep `json:"steps"`
}

// PathConfigActionStep 是公开动作项；目标节点与人员均使用不透明键。
type PathConfigActionStep struct {
	Kind    string                         `json:"kind"`
	Label   string                         `json:"label"`
	Opinion string                         `json:"opinion"`
	Target  string                         `json:"target"`
	Person  *PathConfigPersonStrategyInput `json:"person,omitempty"`
}

// PathConfigArrivalInput 是浏览器保存兼容动作分组的最小回写体。
type PathConfigArrivalInput struct {
	Visit int                         `json:"visit"`
	Steps []PathConfigActionStepInput `json:"steps"`
}

// PathConfigActionStepInput 只允许稳定动作枚举、不透明回退目标和受约束人员策略。
type PathConfigActionStepInput struct {
	Kind    string                         `json:"kind"`
	Opinion string                         `json:"opinion"`
	Target  string                         `json:"target"`
	Person  *PathConfigPersonStrategyInput `json:"person,omitempty"`
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
