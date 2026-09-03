package model

// PathConfigPath 只公开稳定序号与名称，不暴露本地或目标内部标识。
type PathConfigPath struct {
	SequenceNo uint   `json:"sequenceNo"`
	Name       string `json:"name"`
}

// PathConfiguration 是节点人员与动作配置工作台模型；表单原始数据由 F-012 数据工作区独立承载。
type PathConfiguration struct {
	Path         PathConfigPath     `json:"path"`
	Revision     uint64             `json:"revision"`
	NodeRevision uint64             `json:"nodeRevision"`
	Status       string             `json:"status"`
	Progress     PathConfigProgress `json:"progress"`
	NextNodeKey  string             `json:"nextNodeKey"`
	Groups       []PathConfigGroup  `json:"groups"`
	Warnings     []string           `json:"warnings"`
}

// PathConfigActionBase 是系统自动提供且不可编辑的基础动作。
type PathConfigActionBase struct {
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// PathFormReadRequest 是 iframe 会话允许访问的目标只读端点清单；未列入清单的目标请求默认拒绝。
type PathFormReadRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// PathVueCustomPageRule 是宿主 Vue 页面在表单工作区使用的公开规则投影，不包含源码和内部标识。
type PathVueCustomPageRule struct {
	Status        string                   `json:"status"`
	PageName      string                   `json:"pageName"`
	ComponentName string                   `json:"componentName"`
	Route         string                   `json:"route"`
	Fields        []PathVueCustomFieldRule `json:"fields"`
	Issues        []string                 `json:"issues"`
}

// PathVueCustomFieldRule 描述可安全生成、回显和保存复验的 Vue 页面字段。
type PathVueCustomFieldRule struct {
	Path                 string                     `json:"path"`
	Name                 string                     `json:"name"`
	ValueType            string                     `json:"valueType"`
	ValueShape           string                     `json:"valueShape"`
	Serialization        string                     `json:"serialization"`
	Required             bool                       `json:"required"`
	ReadOnly             bool                       `json:"readOnly"`
	Hidden               bool                       `json:"hidden"`
	Disabled             bool                       `json:"disabled"`
	Nested               bool                       `json:"nested"`
	Collection           bool                       `json:"collection"`
	CandidateKind        string                     `json:"candidateKind"`
	CandidateSource      string                     `json:"candidateSource"`
	DefaultValue         any                        `json:"defaultValue,omitempty"`
	DataSource           string                     `json:"dataSource,omitempty"`
	Format               string                     `json:"format,omitempty"`
	Validation           []string                   `json:"validation"`
	ValidationCapability []string                   `json:"validationCapability"`
	Evidence             string                     `json:"evidence"`
	Options              []PathVueCustomFieldOption `json:"options"`
}

// PathVueCustomFieldOption 是 Vue 页面字段真实选项的安全回显投影。
type PathVueCustomFieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// PathFormPermission 是 iframe 应用字段权限所需的最小字段键与权限。
type PathFormPermission struct {
	Field string `json:"field"`
	Power string `json:"power"`
}

// PathFormRuntimeSession 是 iframe 当前会话使用的短期目标读取上下文；绝不持久化。
// 公司/人员选择组件按目标登录上下文读取本地公司树，因此需把已核实账号的公司、用户与租户字段一并透传。
type PathFormRuntimeSession struct {
	SID            string `json:"sid"`
	BaseURL        string `json:"baseURL"`
	AccountName    string `json:"accountName"`
	UserID         string `json:"userId"`
	CompanyID      string `json:"companyId"`
	CustomerCode   string `json:"customerCode"`
	CompanyName    string `json:"companyName"`
	DepartmentID   string `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
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

// PathConfigNode 是路径顺序上的一个业务节点及其人员和动作配置。
type PathConfigNode struct {
	Key                 string                        `json:"key"`
	Name                string                        `json:"name"`
	TypeName            string                        `json:"typeName"`
	Kind                string                        `json:"kind"`
	Status              string                        `json:"status"`
	StatusName          string                        `json:"statusName"`
	Fields              []PathConfigField             `json:"fields"`
	Persons             []PathConfigPerson            `json:"persons"`
	Gaps                []PathConfigGap               `json:"gaps"`
	Requirements        []RequirementItem             `json:"requirements"`
	ActionConfiguration PathConfigActionConfiguration `json:"actionConfiguration"`
	LineBlocked         bool                          `json:"lineBlocked"`
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

// PathConfigActionConfiguration 是当前节点独立动作配置；每项对应一次真实到达。
type PathConfigActionConfiguration struct {
	Base     *PathConfigActionBase         `json:"base,omitempty"`
	Catalog  []PathConfigActionCatalogItem `json:"catalog"`
	Actions  []PathConfigConfiguredAction  `json:"actions"`
	Affected bool                          `json:"affected"`
	Note     string                        `json:"note"`
}

// PathConfigConfiguredAction 是配置期动作组合中的一项；重复动作通过多条记录表达。
type PathConfigConfiguredAction struct {
	Key         string                         `json:"key"`
	Kind        string                         `json:"kind"`
	Label       string                         `json:"label"`
	Person      *PathConfigPersonStrategyInput `json:"person,omitempty"`
	Parameters  map[string]any                 `json:"parameters,omitempty"`
	ActorPolicy string                         `json:"actorPolicy,omitempty"`
	Note        string                         `json:"note,omitempty"`
}

// PathConfigActionCatalogItem 说明当前节点可静态证明合法的动作及必要参数。
type PathConfigActionCatalogItem struct {
	Kind           string            `json:"kind"`
	Label          string            `json:"label"`
	Description    string            `json:"description"`
	Enabled        bool              `json:"enabled"`
	DisabledReason string            `json:"disabledReason"`
	RequiresPerson bool              `json:"requiresPerson"`
	Person         *PathConfigPerson `json:"person,omitempty"`
}

// PathConfigConfiguredActionInput 是浏览器保存的一条独立动作记录，人员和目标仍为不透明键。
type PathConfigConfiguredActionInput struct {
	Key    string                         `json:"key"`
	Kind   string                         `json:"kind"`
	Person *PathConfigPersonStrategyInput `json:"person,omitempty"`
}

// PathConfigAffectedItem 是保存校验失败时定位到具体字段或动作的安全说明。
type PathConfigAffectedItem struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
