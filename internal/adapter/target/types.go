package target

import "strings"

type AccountSummary struct {
	Account     string `json:"account"`
	DisplayName string `json:"displayName"`
	CompanyName string `json:"companyName"`
}

// Session 只在后端内部流转，所有敏感字段均禁止 JSON 序列化。
type Session struct {
	SID          string         `json:"-"`
	CustomerCode string         `json:"-"`
	PlatformCode string         `json:"-"`
	UserID       string         `json:"-"`
	CompanyID    string         `json:"-"`
	DepartmentID string         `json:"-"`
	Summary      AccountSummary `json:"-"`
}

type FlowTemplate struct {
	ID                string `json:"id"`
	FlowName          string `json:"flowName"`
	Code              string `json:"code"`
	GroupName         string `json:"groupName"`
	FlowStatus        string `json:"flowStatus"`
	StatusText        string `json:"statusText"`
	TypeName          string `json:"typeName"`
	UpdateDate        string `json:"updateDate"`
	CreateDate        string `json:"createDate"`
	Remark            string `json:"remark"`
	FlowCreateType    string `json:"flowCreateType"`
	FormExist         string `json:"formExist"`
	AuditWay          string `json:"auditWay"`
	FormTemplateCount int    `json:"formTemplateCount"`
}

type SubmittedFlow struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	FormName              string   `json:"formName"`
	Title                 string   `json:"title"`
	Status                string   `json:"status"`
	StatusName            string   `json:"statusName"`
	CreateDate            string   `json:"createDate"`
	CurrentNodeName       string   `json:"currentNodeName"`
	CurrentAuditUserNames string   `json:"currentAuditUserNames"`
	FlowProxyID           string   `json:"-"`
	CurrentNodeProxyID    string   `json:"-"`
	ActiveNodeProxyIDs    []string `json:"-"`
	FlowCode              string   `json:"-"`
}

// HistoryInstance 是历史候选读取使用的目标原始实例摘要，目标内部标识只在后端保留。
type HistoryInstance struct {
	ID                 string
	FlowProxyID        string
	FormProxyIDs       []string
	FlowCode           string
	FlowName           string
	FormName           string
	Title              string
	BusinessSummary    string
	Initiator          string
	CompanyName        string
	CreatedAt          string
	Status             string
	StatusName         string
	CurrentNodeName    string
	CurrentNodeProxyID string
	ActiveNodeProxyIDs []string
}

// HistorySnapshotSource 是从目标历史实例读取的原始数据和运行时摘要，不能向浏览器直接透传。
type HistorySnapshotSource struct {
	Instance        HistoryInstance
	RenderType      FormRenderType
	TemplateSummary map[string]any
	RawFormData     map[string]any
	Issues          []string
}

// HistoryIdentity 是计划目标流程用于历史候选精确筛选的原始身份字段。
type HistoryIdentity struct {
	FlowCode        string
	FormName        string
	FlowName        string
	RenderType      FormRenderType
	TemplateSummary map[string]any
}

type DueFlow struct {
	FlowInstanceID   string `json:"flowInstanceId"`
	FlowInstanceName string `json:"flowInstanceName"`
	FormName         string `json:"formName"`
	Title            string `json:"title"`
	FlowStatus       string `json:"flowStatus"`
	StatusName       string `json:"statusName"`
	Initiator        string `json:"initiator"`
	InitiatorDate    string `json:"initiatorDate"`
	FlowProxyID      string `json:"-"`
	FlowNodeProxyID  string `json:"-"`
}

// FlowTreeSnapshot 把本次读取的真实代理树和运行态入口绑定在一起。
type FlowTreeSnapshot struct {
	Tree         *FlowNodeTemplate
	EntryNodeIDs []string
}

// FlowRequirementSnapshot 把同一次真实目标核对得到的结构、入口和表单字段绑定在一起。
type FlowRequirementSnapshot struct {
	Tree         *FlowNodeTemplate
	EntryNodeIDs []string
	FormFields   []FormFieldMetadata
}

// FlowNodeTemplate 是目标平台流程树的最小只读传输结构。
type FlowNodeTemplate struct {
	ID                string
	Name              string
	Type              string
	BranchExecuteType string
	Child             *FlowNodeTemplate
	ConditionNodes    []FlowBranchTemplate
	ParallelNodes     []FlowBranchTemplate
	AuditConfig       *FlowNodeAuditConfig
	AddSignCandidates []FlowAuditCandidate
	AddSignIssues     []FlowAuditResolutionIssue
	FieldPowers       []FlowNodeFieldPower
	IsSkip            *bool
	Delay             *int
	Unit              string
	DeadlineType      string
}

type FlowBranchTemplate struct {
	ID         string
	Name       string
	Sort       int
	Conditions []FlowCondition
	Child      *FlowNodeTemplate
}

// FlowCondition 是条件分支的内部只读表达，字段键和枚举代码不会直接公开。
type FlowCondition struct {
	FieldA        string
	FieldB        string
	ValueB        string
	ValueType     string
	Judge         string
	ConditionType string
}

// FlowNodeAuditConfig 是审批节点的内部只读配置。
type FlowNodeAuditConfig struct {
	AuditType         string
	Mode              string
	CountersignNum    *int
	FormPersonField   string
	AuditCondition    string
	Details           []FlowAuditDetail
	Scopes            []FlowAuditScope
	Candidates        []FlowAuditCandidate
	DefaultCandidates []FlowAuditCandidate
	ResolutionIssues  []FlowAuditResolutionIssue
}

// FlowAuditDetail 保留目录解析所需内部 ID 与可展示名称，业务 ID 不会进入公开响应。
type FlowAuditDetail struct {
	ID   string
	Name string
	Type string
}

// FlowAuditScope 保留运行节点范围的内部 ID、类型和解析名称，目标业务 ID 不进入公开响应。
type FlowAuditScope struct {
	ID   string
	Type string
	Name string
}

// FlowAuditCandidate 是目标详情已经返回的受限人员候选，仅在后端内部参与不透明键映射。
type FlowAuditCandidate struct {
	ID   string
	Name string
}

// FlowAuditResolutionIssue 记录目标目录解析失败的公开类别与原因，不携带目标响应原文。
type FlowAuditResolutionIssue struct {
	Category string
	Reason   string
}

// FlowNodeFieldPower 记录节点字段权限与表单归属提示。
type FlowNodeFieldPower struct {
	FormID      string
	FieldID     string
	EnglishName string
	Power       string
}

// FormFieldMetadata 是模板或代理表单的字段名称字典。
type FormFieldMetadata struct {
	FormID      string
	FormName    string
	FieldID     string
	Name        string
	EnglishName string
}

// FormFieldDetail 在名称字典基础上补充字段类型、默认值、必填和真实选项，供路径配置使用。
type FormFieldDetail struct {
	FormID        string
	FormName      string
	FieldID       string
	Name          string
	EnglishName   string
	FieldType     string
	DefaultValue  string
	Required      bool
	Multiple      bool
	Options       []FormFieldOption
	ValueOrigin   string
	FieldStatus   string
	ComponentType string
	ComponentName string
	DateMode      string
}

// FormFieldOption 是目标平台表单组件提供的选项标签与值。
type FormFieldOption struct {
	Label string
	Value string
}

// FormRenderType 表示目标流程真实使用的表单渲染协议，不能由是否存在 FormMaking JSON 反推是否需要业务数据。
type FormRenderType string

const (
	FormRenderTypeFormMaking FormRenderType = "formmaking"
	FormRenderTypeVueCustom  FormRenderType = "vue_custom"
	FormRenderTypeUnknown    FormRenderType = "unknown"
)

// NormalizeFormRenderType 按目标 formExist 与已读取表单正文确定渲染协议；noForm/notForm 均属于 Vue 业务页面。
func NormalizeFormRenderType(formExist string, formCount int) FormRenderType {
	switch strings.ToLower(strings.TrimSpace(formExist)) {
	case "noform", "notform":
		return FormRenderTypeVueCustom
	}
	if formCount > 0 {
		return FormRenderTypeFormMaking
	}
	return FormRenderTypeUnknown
}

// VueCustomPageRule 是从宿主 Vue 页面静态提取的只读页面规则，不保存或公开目标原始源码。
type VueCustomPageRule struct {
	Status        string
	PageKey       string
	PageName      string
	ComponentName string
	Route         string
	InitialState  map[string]any
	Fields        []VueCustomFieldRule
	Dependencies  []VueCustomDependencyRule
	ReadRequests  []VueCustomRequestRule
	Issues        []string
}

// VueCustomFieldRule 描述宿主 Vue 页面可静态识别的字段、初值、校验和候选来源。
type VueCustomFieldRule struct {
	Path                 string
	Name                 string
	ValueType            string
	ValueShape           string
	Serialization        string
	Required             bool
	ReadOnly             bool
	Hidden               bool
	Disabled             bool
	DefaultValue         any
	CandidateKind        string
	CandidateSource      string
	DataSource           string
	Nested               bool
	Collection           bool
	Format               string
	Validation           []string
	ValidationCapability []string
	Evidence             string
	Options              []VueCustomFieldOption
}

// VueCustomFieldOption 是 Vue 页面运行时协议声明的真实选项值。
type VueCustomFieldOption struct {
	Label string
	Value any
}

// VueCustomDependencyRule 描述字段联动和动态候选的已证明依赖，不保存动态脚本正文。
type VueCustomDependencyRule struct {
	Field   string
	Depends []string
	Kind    string
	Source  string
}

// VueCustomRequestRule 描述页面初始化或联动阶段发生的只读请求协议。
type VueCustomRequestRule struct {
	Name     string
	Method   string
	Path     string
	Phase    string
	Response string
	ReadOnly bool
	Issues   []string
}

// PathConfigurationSnapshot 把同一真实流程树、当前入口、表单字段详情和实例现值绑定在一起。
type PathConfigurationSnapshot struct {
	Tree           *FlowNodeTemplate
	EntryNodeIDs   []string
	FlowCode       string
	FlowName       string
	AuditWay       string
	RenderType     FormRenderType
	VuePage        *VueCustomPageRule
	FormFields     []FormFieldDetail
	Forms          []FormRuntimeTemplate
	InstanceValues map[string]any
}

// FormRuntimeTemplate 保留目标完整 FormMaking 模板及公开名称，只在后端和隔离运行时之间流转。
type FormRuntimeTemplate struct {
	ID           string
	Name         string
	TemplateData string
}

// FormRuntimeSession 是当前已验证账号的短期表单读取会话，不得持久化。
type FormRuntimeSession struct {
	SID            string
	BaseURL        string
	AccountName    string
	UserID         string
	CompanyID      string
	CustomerCode   string
	CompanyName    string
	DepartmentID   string
	DepartmentName string
}

// FormIdentityNode 是当前账号在目标公司目录树中的节点上下文，供表单人员/公司组件自动填充。
type FormIdentityNode struct {
	ID        string
	Name      string
	Type      string
	ParentID  string
	CompanyID string
}

// FormIdentityContext 汇总当前账号的公司、部门与本人节点；目标目录无法定位时对应项保持空。
type FormIdentityContext struct {
	Company    FormIdentityNode
	Department FormIdentityNode
	User       FormIdentityNode
}

type Page[T any] struct {
	Items    []T  `json:"items"`
	Page     int  `json:"page"`
	PageSize int  `json:"pageSize"`
	Total    int  `json:"total"`
	HasMore  bool `json:"hasMore"`
}
