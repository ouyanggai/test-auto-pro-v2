package target

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
	AuditType       string
	Mode            string
	CountersignNum  *int
	FormPersonField string
	Details         []FlowAuditDetail
	Scopes          []FlowAuditScope
}

// FlowAuditDetail 只保留可展示名称，业务 ID 仅用于内部计数且不会公开。
type FlowAuditDetail struct {
	Name string
	Type string
}

// FlowAuditScope 只保留范围类型，目标业务 ID 不进入公开响应。
type FlowAuditScope struct {
	Type string
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

type Page[T any] struct {
	Items    []T  `json:"items"`
	Page     int  `json:"page"`
	PageSize int  `json:"pageSize"`
	Total    int  `json:"total"`
	HasMore  bool `json:"hasMore"`
}
