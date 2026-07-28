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

// FlowNodeTemplate 是目标平台流程树的最小只读传输结构。
type FlowNodeTemplate struct {
	ID                string
	Name              string
	Type              string
	BranchExecuteType string
	Child             *FlowNodeTemplate
	ConditionNodes    []FlowBranchTemplate
	ParallelNodes     []FlowBranchTemplate
}

type FlowBranchTemplate struct {
	ID    string
	Name  string
	Sort  int
	Child *FlowNodeTemplate
}

type Page[T any] struct {
	Items    []T  `json:"items"`
	Page     int  `json:"page"`
	PageSize int  `json:"pageSize"`
	Total    int  `json:"total"`
	HasMore  bool `json:"hasMore"`
}
