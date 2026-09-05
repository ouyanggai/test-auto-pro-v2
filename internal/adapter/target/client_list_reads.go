package target

import (
	"context"
	"encoding/json"
	"strings"
)

// 本文件承接目标客户端的列表类只读读取（模板、已发起、历史实例、待办），
// 与 client.go 同属 target 包；拆分只为满足纲领第 10 节的单文件行数上限，不改任何行为。
// ListTemplates 分页读取账号可见模板并映射已核实公开字段。
func (c *Client) ListTemplates(ctx context.Context, session Session, query string, page, pageSize int) (Page[FlowTemplate], error) {
	body := map[string]any{
		"data": map[string]any{
			"flowName":     strings.TrimSpace(query),
			"useScope":     "invest",
			"customerCode": firstNonEmpty(session.CustomerCode, c.config.CustomerCode),
		},
		"showMe":                             true,
		"ignoreFormTemplateBizRelevanceData": true,
		"formTemplateBizRelevanceList":       []any{},
		"notFormTemplateBizRelevanceList":    []map[string]any{{"otherBiz": "isProject", "otherBizId": "isProject"}},
		"ignoreTemplateData":                 true,
		"pagination":                         true,
		"pages":                              page,
		"size":                               pageSize,
		"projectId":                          "",
		"platformCode":                       c.config.TemplatePlatformCodes,
		"notAuditWayList":                    []string{"staff_annual_assessment"},
	}
	resp, err := c.call(ctx, "/web/flowTemplateApi/list", session.SID, body)
	if err != nil {
		return Page[FlowTemplate]{}, err
	}
	if !responseSucceeded(resp) {
		return Page[FlowTemplate]{}, responseError(resp)
	}
	var raw []rawFlowTemplate
	if err := decodeArray(resp.Data, &raw); err != nil {
		return Page[FlowTemplate]{}, err
	}
	items := make([]FlowTemplate, 0, len(raw))
	for _, item := range raw {
		items = append(items, FlowTemplate{
			ID:                item.ID,
			FlowName:          item.FlowName,
			Code:              firstNonEmpty(item.Code, item.FlowCode),
			GroupName:         item.GroupName,
			FlowStatus:        item.FlowStatus,
			StatusText:        templateStatusText(item.FlowStatus),
			TypeName:          item.TypeName,
			UpdateDate:        firstNonEmpty(item.UpdateDate, item.UpdateTime),
			CreateDate:        firstNonEmpty(item.CreateDate, item.CreateTime),
			Remark:            item.Remark,
			FlowCreateType:    item.FlowCreateType,
			FormExist:         item.FormExist,
			AuditWay:          strings.TrimSpace(item.AuditWay),
			FormTemplateCount: len(item.FormTemplateList),
		})
	}
	return normalizePage(items, resp, page, pageSize)
}

// ListSubmitted 分页读取已发实例，并保留后端路径入口核对所需的非公开字段。
func (c *Client) ListSubmitted(ctx context.Context, session Session, query string, page, pageSize int) (Page[SubmittedFlow], error) {
	return c.listSubmitted(ctx, session, query, "", page, pageSize)
}

// HistoryInstanceQuery 是目标实例列表协议支持的原始过滤字段；工具不新增身份字段。
type HistoryInstanceQuery struct {
	// FlowName 是目标流程列表页实际使用的过滤字段，取值只能来自目标流程详情返回的流程名称。
	FlowName string
	// StatusList 为空时使用目标全状态枚举；按分组传入可让已完成实例优先返回。
	StatusList []string
}

// historyInstanceAllStatuses 是目标实例列表的全状态枚举；draft 与 await_sent 都是可选的未完整业务状态。
func historyInstanceAllStatuses() []string {
	return []string{"draft", "await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"}
}

// ListHistoryInstances 分页读取目标可见业务实例，保留候选筛选和快照读取所需的原始身份字段。
// 过滤字段使用目标平台自己在流程列表页使用的 flowName：实例列表返回的行不携带 flowCode，
// 按 flowCode 过滤会在真实环境返回空结果（FlowInstanceServiceImpl.save 只在新建实例时写入 flow_code）。
func (c *Client) ListHistoryInstances(ctx context.Context, session Session, query HistoryInstanceQuery, page, pageSize int) (Page[HistoryInstance], error) {
	statusList := query.StatusList
	if len(statusList) == 0 {
		statusList = historyInstanceAllStatuses()
	}
	data := map[string]any{
		"useScope":                     "invest",
		"auditWayList":                 []string{},
		"statusList":                   statusList,
		"flowInstanceBizRelevanceList": []map[string]any{{"otherBiz": "company", "otherBizId": ""}},
	}
	if value := strings.TrimSpace(query.FlowName); value != "" {
		data["flowName"] = value
	}
	resp, err := c.call(ctx, "/web/flowInstanceApi/list", session.SID, map[string]any{
		"data": data, "pagination": true, "pages": page, "size": pageSize,
	})
	if err != nil {
		return Page[HistoryInstance]{}, err
	}
	if !responseSucceeded(resp) {
		return Page[HistoryInstance]{}, responseError(resp)
	}
	var raw []rawHistoryInstance
	if err := decodeArray(resp.Data, &raw); err != nil {
		return Page[HistoryInstance]{}, err
	}
	items := make([]HistoryInstance, 0, len(raw))
	resolver := &auditDirectoryResolver{
		client: c, ctx: ctx, active: session,
		trees: make(map[string][]rawAuditDirectoryNode), named: make(map[string][]rawAuditNamedItem),
		roleUsers: make(map[string][]FlowAuditCandidate), positionUsers: make(map[string][]FlowAuditCandidate),
	}
	for _, item := range raw {
		formProxyIDs := make([]string, 0, 1)
		if formProxyID := strings.TrimSpace(item.FormProxyID); formProxyID != "" {
			formProxyIDs = append(formProxyIDs, formProxyID)
		}
		instance := HistoryInstance{
			ID: item.ID, FlowProxyID: item.FlowProxyID, FormProxyIDs: formProxyIDs,
			FlowCode:           strings.TrimSpace(item.FlowCode),
			FlowName:           strings.TrimSpace(item.FlowName),
			FormName:           item.FormName,
			Title:              firstNonEmpty(item.Name, item.FormName, item.FlowName),
			BusinessSummary:    strings.TrimSpace(item.Name),
			Initiator:          historyInitiatorName(resolver, session, item.CreaterID),
			CompanyName:        historyCompanyName(resolver, session, item.CompanyID),
			CreatedAt:          firstNonEmpty(item.CreateDate, item.CreateTime),
			Status:             item.Status,
			StatusName:         submittedStatusText(item.Status),
			CurrentNodeName:    item.CurrentNodeName,
			CurrentNodeProxyID: strings.TrimSpace(item.CurrentNodeProxyID),
			ActiveNodeProxyIDs: []string{},
		}
		items = append(items, instance)
	}
	return normalizePage(items, resp, page, pageSize)
}

// rawHistoryInstance 是历史实例列表的最小原始字段集合，未知业务字段不进入工具模型或日志。
type rawHistoryInstance struct {
	ID                 string `json:"id"`
	FlowProxyID        string `json:"flowProxyId"`
	FormProxyID        string `json:"formProxyId"`
	Name               string `json:"name"`
	FlowName           string `json:"flowName"`
	FormName           string `json:"formName"`
	Status             string `json:"status"`
	CreateDate         string `json:"createDate"`
	CreateTime         string `json:"createTime"`
	CurrentNodeName    string `json:"currentNodeName"`
	CurrentNodeProxyID string `json:"currentNodeProxyId"`
	FlowCode           string `json:"flowCode"`
	CreaterID          string `json:"createrId"`
	CompanyID          string `json:"companyId"`
}

// historyInitiatorName 使用当前会话本人摘要或同公司人员目录解析目标发起人名称。
func historyInitiatorName(resolver *auditDirectoryResolver, active Session, initiatorID string) string {
	initiatorID = strings.TrimSpace(initiatorID)
	if initiatorID == "" {
		return ""
	}
	if initiatorID == strings.TrimSpace(active.UserID) && strings.TrimSpace(active.Summary.DisplayName) != "" {
		return strings.TrimSpace(active.Summary.DisplayName)
	}
	name, err := resolver.nameFromPersonnel(initiatorID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// historyCompanyName 使用当前登录公司摘要或目标公司目录解析实例所属公司名称。
func historyCompanyName(resolver *auditDirectoryResolver, active Session, companyID string) string {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return ""
	}
	if companyID == strings.TrimSpace(active.CompanyID) && strings.TrimSpace(active.Summary.CompanyName) != "" {
		return strings.TrimSpace(active.Summary.CompanyName)
	}
	name, err := resolver.nameFromTree("7", companyID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// listSubmitted 统一拼装已发实例列表请求，名称搜索和流程编码过滤不能混用或相互猜测。
func (c *Client) listSubmitted(ctx context.Context, session Session, query, flowCode string, page, pageSize int) (Page[SubmittedFlow], error) {
	data := map[string]any{
		"useScope":                     "invest",
		"auditWayList":                 []string{},
		"statusList":                   []string{"await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"},
		"flowInstanceBizRelevanceList": []map[string]any{{"otherBiz": "company", "otherBizId": ""}},
	}
	if value := strings.TrimSpace(query); value != "" {
		data["name"] = value
	}
	if value := strings.TrimSpace(flowCode); value != "" {
		// Java FlowInstanceRepository 已证明 flowCode 是列表查询字段；样本读取必须让目标端先精确过滤。
		data["flowCode"] = value
	}
	resp, err := c.call(ctx, "/web/flowInstanceApi/list", session.SID, map[string]any{
		"data": data, "pagination": true, "pages": page, "size": pageSize,
	})
	if err != nil {
		return Page[SubmittedFlow]{}, err
	}
	if !responseSucceeded(resp) {
		return Page[SubmittedFlow]{}, responseError(resp)
	}
	var raw []struct {
		ID                   string          `json:"id"`
		FlowProxyID          string          `json:"flowProxyId"`
		Name                 string          `json:"name"`
		FormName             string          `json:"formName"`
		Status               string          `json:"status"`
		CreateDate           string          `json:"createDate"`
		CreateTime           string          `json:"createTime"`
		CurrentNodeName      string          `json:"currentNodeName"`
		CurrentNodeProxyID   string          `json:"currentNodeProxyId"`
		FlowCode             string          `json:"flowCode"`
		FlowTemplateCode     string          `json:"flowTemplateCode"`
		CurrentAuditUserInfo json.RawMessage `json:"currentAuditUserInfo"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return Page[SubmittedFlow]{}, err
	}
	items := make([]SubmittedFlow, 0, len(raw))
	for _, item := range raw {
		items = append(items, SubmittedFlow{
			ID:                    item.ID,
			FlowProxyID:           item.FlowProxyID,
			Name:                  item.Name,
			FormName:              item.FormName,
			Title:                 firstNonEmpty(item.Name, item.FormName),
			Status:                item.Status,
			StatusName:            submittedStatusText(item.Status),
			CreateDate:            firstNonEmpty(item.CreateDate, item.CreateTime),
			CurrentNodeName:       item.CurrentNodeName,
			CurrentAuditUserNames: auditUserNames(item.CurrentAuditUserInfo),
			CurrentNodeProxyID:    strings.TrimSpace(item.CurrentNodeProxyID),
			FlowCode:              firstNonEmpty(item.FlowCode, item.FlowTemplateCode),
			ActiveNodeProxyIDs:    auditNodeIDs(item.CurrentAuditUserInfo),
		})
	}
	return normalizePage(items, resp, page, pageSize)
}

// ListDue 分页读取 waiting_send 待发任务，并保留非公开代理节点入口。
func (c *Client) ListDue(ctx context.Context, session Session, query string, page, pageSize int) (Page[DueFlow], error) {
	data := map[string]any{
		"taskStatus":                   "waiting_send",
		"auditWayList":                 []string{},
		"useScope":                     "invest",
		"flowInstanceBizRelevance":     map[string]any{},
		"flowInstanceBizRelevanceList": []any{},
	}
	if value := strings.TrimSpace(query); value != "" {
		data["flowInstanceName"] = value
	}
	resp, err := c.call(ctx, "/web/flowJobTaskLink/list", session.SID, map[string]any{
		"data": data, "pagination": true, "pages": page, "size": pageSize,
	})
	if err != nil {
		return Page[DueFlow]{}, err
	}
	if !responseSucceeded(resp) {
		return Page[DueFlow]{}, responseError(resp)
	}
	var raw []struct {
		FlowInstanceID   string `json:"flowInstanceId"`
		FlowProxyID      string `json:"flowProxyId"`
		FlowNodeProxyID  string `json:"flowNodeProxyId"`
		FlowInstanceName string `json:"flowInstanceName"`
		FormName         string `json:"formName"`
		FlowStatus       string `json:"flowStatus"`
		StatusName       string `json:"statusName"`
		Initiator        string `json:"initiator"`
		InitiatorName    string `json:"initiatorName"`
		InitiatorDate    string `json:"initiatorDate"`
		CreateTime       string `json:"createTime"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return Page[DueFlow]{}, err
	}
	items := make([]DueFlow, 0, len(raw))
	for _, item := range raw {
		items = append(items, DueFlow{
			FlowInstanceID:   item.FlowInstanceID,
			FlowProxyID:      item.FlowProxyID,
			FlowNodeProxyID:  strings.TrimSpace(item.FlowNodeProxyID),
			FlowInstanceName: item.FlowInstanceName,
			FormName:         item.FormName,
			Title:            firstNonEmpty(item.FlowInstanceName, item.FormName),
			FlowStatus:       item.FlowStatus,
			StatusName:       firstNonEmpty(item.StatusName, dueStatusText(item.FlowStatus)),
			Initiator:        firstNonEmpty(item.Initiator, item.InitiatorName),
			InitiatorDate:    firstNonEmpty(item.InitiatorDate, item.CreateTime),
		})
	}
	return normalizePage(items, resp, page, pageSize)
}

type rawFlowNodeTemplate struct {
	ID                string                  `json:"id"`
	NodeName          string                  `json:"nodeName"`
	Name              string                  `json:"name"`
	Type              string                  `json:"type"`
	BranchExecuteType string                  `json:"branchExecuteType"`
	Child             *rawFlowNodeTemplate    `json:"childFlowNodeTemplate"`
	ConditionNodes    []rawFlowBranchTemplate `json:"conditionNodes"`
	ParallelNodes     []rawFlowBranchTemplate `json:"parallelNodes"`
	AuditConfig       *rawFlowNodeAuditConfig `json:"flowNodeAuditConfig"`
	FieldPowers       []rawFlowNodeFieldPower `json:"flowNodeFieldPowerTemplateList"`
	IsSkip            *bool                   `json:"isSkip"`
	Delay             *int                    `json:"delay"`
	Unit              string                  `json:"unit"`
	DeadlineType      string                  `json:"deadlineType"`
}

type rawFlowBranchTemplate struct {
	ID            string               `json:"id"`
	StrategyID    string               `json:"strategyId"`
	NodeName      string               `json:"nodeName"`
	Name          string               `json:"name"`
	Sort          int                  `json:"sort"`
	ConditionList []rawFlowCondition   `json:"conditionList"`
	Child         *rawFlowNodeTemplate `json:"childFlowNodeTemplate"`
}

type rawFlowCondition struct {
	FieldA        string `json:"fieldaName"`
	FieldB        string `json:"fieldbName"`
	ValueB        string `json:"bvalue"`
	ValueType     string `json:"btype"`
	Judge         string `json:"judge"`
	ConditionType string `json:"conditionType"`
}

type rawFlowNodeAuditConfig struct {
	AuditType         string               `json:"auditType"`
	PlatformCode      string               `json:"platformCode"`
	Mode              string               `json:"type"`
	CountersignNum    *int                 `json:"countersignNum"`
	FormPersonField   string               `json:"formPersonFields"`
	AuditCondition    string               `json:"auditCondition"`
	Details           []rawFlowAuditDetail `json:"flowNodeDetailConfigList"`
	Scopes            []rawFlowAuditScope  `json:"nodeAuditScopeList"`
	Candidates        []rawFlowAuditUser   `json:"userVoList"`
	DefaultCandidates []rawFlowAuditUser   `json:"defaultUserVoList"`
}

type rawFlowAuditDetail struct {
	BizID string `json:"bizId"`
	Name  string `json:"name"`
	Type  string `json:"auditDetailType"`
}

type rawFlowAuditScope struct {
	BizID string `json:"bizId"`
	Type  string `json:"type"`
}

// rawFlowAuditUser 只解析目标节点配置已经返回的候选身份与中文名称，不额外查询全员目录。
type rawFlowAuditUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"realName"`
	DisplayName string `json:"displayName"`
}

type rawFlowNodeFieldPower struct {
	FormID      string `json:"formTemplateId"`
	FieldID     string `json:"formFieldTemplateId"`
	EnglishName string `json:"formFieldTemplateEnglishName"`
	Power       string `json:"fieldPower"`
}

type rawFormReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type rawFormField struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	EnglishName string `json:"englishName"`
}

// rawFormFieldDetail 是模板或代理表单字段详情，补充真实类型、默认值、值来源和启用状态。
type rawFormFieldDetail struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	EnglishName  string `json:"englishName"`
	FieldType    string `json:"fieldType"`
	DefaultValue string `json:"defaultValue"`
	ValueOrigin  string `json:"valueOrigin"`
	FieldStatus  string `json:"fieldStatus"`
}

// rawFormMakingOption 是表单组件选项，值可能来自 value 或 id 字段。
type rawFormMakingOption struct {
	ID    any    `json:"id"`
	Value any    `json:"value"`
	Label string `json:"label"`
}

// rawFormMakingComponent 是模板数据中 FormMaking 组件的递归结构；容器字段统一保留，未知组件不会回退成文本。
type rawFormMakingComponent struct {
	Type          string `json:"type"`
	El            string `json:"el"`
	ComponentName string `json:"componentName"`
	Component     string `json:"component"`
	Model         string `json:"model"`
	Name          string `json:"name"`
	Options       struct {
		DefaultValue  any                   `json:"defaultValue"`
		Required      bool                  `json:"required"`
		Multiple      bool                  `json:"multiple"`
		Type          string                `json:"type"`
		ComponentName string                `json:"componentName"`
		Component     string                `json:"component"`
		Options       []rawFormMakingOption `json:"options"`
	} `json:"options"`
	List         []rawFormMakingComponent `json:"list"`
	Columns      []rawFormMakingComponent `json:"columns"`
	Rows         []rawFormMakingComponent `json:"rows"`
	TableColumns []rawFormMakingComponent `json:"tableColumns"`
}

// rawFormMakingData 是模板数据 JSON 的外层结构，组件列表位于 list 字段。
type rawFormMakingData struct {
	List []rawFormMakingComponent `json:"list"`
}
