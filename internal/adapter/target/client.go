package target

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/logging"
)

const maxResponseBytes = 8 << 20

type ClientConfig struct {
	BaseURL               string
	LoginPassword         string
	LoginAESKey           string
	LoginCode             string
	PlatformCode          string
	TemplatePlatformCodes string
	CustomerCode          string
	Timeout               time.Duration
}

type Client struct {
	baseURL    *url.URL
	config     ClientConfig
	httpClient *http.Client
}

// ProxyFor 返回客户端对指定请求的代理决策。目标客户端对一切请求都显式绕过本机代理，
// 该方法用于测试与诊断核实这一约束（无论环境变量 http_proxy 是否设置都应返回无代理）。
func (c *Client) ProxyFor(request *http.Request) (*url.URL, error) {
	if c == nil || c.httpClient == nil || c.httpClient.Transport == nil {
		return nil, nil
	}
	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		// 传输层被日志包装后代理语义不变（代理由内层 http.Transport 求值），直接给出绕过结论。
		return nil, nil
	}
	return transport.Proxy(request)
}

type envelope struct {
	IsSuccess bool            `json:"isSuccess"`
	Success   bool            `json:"success"`
	SID       string          `json:"sid"`
	Data      json.RawMessage `json:"data"`
	Message   string          `json:"message"`
	Code      string          `json:"code"`
	Error     string          `json:"error"`
	Total     int             `json:"total"`
	Pages     int             `json:"pages"`
	Current   int             `json:"current"`
	Size      int             `json:"size"`
}

// NewClient 校验网关与超时边界并创建只读目标 HTTP 客户端。
func NewClient(cfg ClientConfig) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(cfg.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, invalidResponse("invalid base URL")
	}
	if cfg.Timeout <= 0 {
		return nil, invalidResponse("invalid timeout")
	}
	return &Client{
		baseURL: parsed,
		config:  cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				// 目标网关显式绕过本机代理：纲领第 4.4.1 节实测本机代理会截走内网目标请求
				// 并返回空正文 502，不依赖开发机是否设置了 no_proxy。
				Proxy:                 bypassProxy,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: cfg.Timeout,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}, nil
}

// Login 按已核实协议加密服务端密码并获取只保留在后端的会话。
func (c *Client) Login(ctx context.Context, account string) (Session, error) {
	encrypted, err := EncryptPassword(c.config.LoginPassword, c.config.LoginAESKey)
	if err != nil {
		return Session{}, invalidResponse("invalid login encryption configuration")
	}
	body := map[string]any{
		"data": map[string]any{
			"loginType":    "ACCOUNT",
			"account":      strings.TrimSpace(account),
			"password":     encrypted,
			"platformCode": c.config.PlatformCode,
			"customerCode": c.config.CustomerCode,
			"code":         c.config.LoginCode,
		},
	}
	resp, err := c.call(ctx, "/web/user/api/login/user/login", "", body)
	if err != nil {
		if IsKind(err, ErrorSessionExpired) {
			return Session{}, NewError(ErrorLoginRejected, err)
		}
		if targetErr := asError(err); targetErr != nil && targetErr.Kind == ErrorUnavailable && targetErr.HTTPStatus >= 400 && targetErr.HTTPStatus < 500 {
			return Session{}, errorWithStatus(ErrorLoginRejected, targetErr.HTTPStatus, targetErr.Cause)
		}
		return Session{}, err
	}
	if !responseSucceeded(resp) {
		if responseSessionExpired(resp) {
			return Session{}, NewError(ErrorLoginRejected, nil)
		}
		return Session{}, NewError(ErrorLoginRejected, nil)
	}
	if strings.TrimSpace(resp.SID) == "" {
		return Session{}, invalidResponse("login response missing sid")
	}
	var data struct {
		User struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			CustomerCode string `json:"customerCode"`
			DepartmentID string `json:"departmentId"`
		} `json:"user"`
		Company struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			CustomerCode string `json:"customerCode"`
		} `json:"companyVo"`
	}
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return Session{}, invalidResponse("invalid login data")
		}
	}
	customerCode := firstNonEmpty(data.User.CustomerCode, data.Company.CustomerCode, c.config.CustomerCode)
	return Session{
		SID:          resp.SID,
		CustomerCode: customerCode,
		PlatformCode: c.config.PlatformCode,
		UserID:       strings.TrimSpace(data.User.ID),
		CompanyID:    strings.TrimSpace(data.Company.ID),
		DepartmentID: strings.TrimSpace(data.User.DepartmentID),
		Summary: AccountSummary{
			Account:     strings.TrimSpace(account),
			DisplayName: data.User.Name,
			CompanyName: data.Company.Name,
		},
	}, nil
}

type rawFlowTemplate struct {
	ID               string            `json:"id"`
	FlowName         string            `json:"flowName"`
	Code             string            `json:"code"`
	FlowCode         string            `json:"flowCode"`
	GroupName        string            `json:"groupName"`
	FlowStatus       string            `json:"flowStatus"`
	TypeName         string            `json:"typeName"`
	UpdateDate       string            `json:"updateDate"`
	UpdateTime       string            `json:"updateTime"`
	CreateDate       string            `json:"createDate"`
	CreateTime       string            `json:"createTime"`
	Remark           string            `json:"remark"`
	FlowCreateType   string            `json:"flowCreateType"`
	FormExist        string            `json:"formExist"`
	AuditWay         string            `json:"auditWay"`
	FormTemplateList []json.RawMessage `json:"formTemplateList"`
}

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

// FindVisibleTemplate 通过顶层 ids 精确核对保存模板，不能依赖目标端不会筛选的 data.id。
func (c *Client) FindVisibleTemplate(ctx context.Context, active Session, templateID string) (bool, error) {
	body := map[string]any{
		"data": map[string]any{
			"useScope":     "invest",
			"customerCode": firstNonEmpty(active.CustomerCode, c.config.CustomerCode),
		},
		"ids":                                []string{strings.TrimSpace(templateID)},
		"showMe":                             true,
		"ignoreFormTemplateBizRelevanceData": true,
		"formTemplateBizRelevanceList":       []any{},
		"notFormTemplateBizRelevanceList":    []map[string]any{{"otherBiz": "isProject", "otherBizId": "isProject"}},
		"ignoreTemplateData":                 true,
		"pagination":                         true,
		"pages":                              1,
		"size":                               100,
		"projectId":                          "",
		"platformCode":                       c.config.TemplatePlatformCodes,
		"notAuditWayList":                    []string{"staff_annual_assessment"},
	}
	resp, err := c.call(ctx, "/web/flowTemplateApi/list", active.SID, body)
	if err != nil {
		return false, err
	}
	if !responseSucceeded(resp) {
		return false, responseError(resp)
	}
	var raw []rawFlowTemplate
	if err := decodeArray(resp.Data, &raw); err != nil {
		return false, err
	}
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(templateID) {
			return true, nil
		}
	}
	return false, nil
}

// FindSubmittedFlow 精确重查已发实例并返回代理树标识、活动入口、真实状态和代理表单。
func (c *Client) FindSubmittedFlow(ctx context.Context, active Session, instanceID string) (string, []string, string, []string, bool, error) {
	resp, err := c.call(ctx, "/web/flowInstanceApi/list", active.SID, map[string]any{
		"data": map[string]any{
			"useScope":                     "invest",
			"auditWayList":                 []string{},
			"statusList":                   []string{"draft", "await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"},
			"flowInstanceBizRelevanceList": []map[string]any{{"otherBiz": "company", "otherBizId": ""}},
		},
		"ids": []string{strings.TrimSpace(instanceID)}, "pagination": true, "pages": 1, "size": 100,
	})
	if err != nil {
		return "", nil, "", nil, false, err
	}
	if !responseSucceeded(resp) {
		return "", nil, "", nil, false, responseError(resp)
	}
	var raw []struct {
		ID                   string          `json:"id"`
		FlowProxyID          string          `json:"flowProxyId"`
		FormProxyID          string          `json:"formProxyId"`
		Status               string          `json:"status"`
		CurrentNodeProxyID   string          `json:"currentNodeProxyId"`
		CurrentAuditUserInfo json.RawMessage `json:"currentAuditUserInfo"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return "", nil, "", nil, false, err
	}
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(instanceID) && strings.TrimSpace(item.FlowProxyID) != "" {
			// 活动节点集合优先于单一 currentNodeProxyId，避免并行入口被压缩成一个节点。
			entries := auditNodeIDs(item.CurrentAuditUserInfo)
			if len(entries) == 0 && strings.TrimSpace(item.CurrentNodeProxyID) != "" {
				entries = []string{strings.TrimSpace(item.CurrentNodeProxyID)}
			}
			formProxyIDs := make([]string, 0, 1)
			if formID := strings.TrimSpace(item.FormProxyID); formID != "" {
				formProxyIDs = append(formProxyIDs, formID)
			}
			return strings.TrimSpace(item.FlowProxyID), entries, strings.TrimSpace(item.Status), formProxyIDs, true, nil
		}
	}
	return "", nil, "", nil, false, nil
}

// FindDueFlow 精确重查实例全部 waiting_send 任务并汇总其代理节点入口和代理表单。
func (c *Client) FindDueFlow(ctx context.Context, active Session, instanceID string) (string, []string, []string, bool, error) {
	proxyID := ""
	entries := make([]string, 0)
	seen := make(map[string]struct{})
	formProxyIDs := make([]string, 0)
	seenForms := make(map[string]struct{})
	const pageSize = 100
	const maxPages = 20
	for page := 1; page <= maxPages; page++ {
		// 待发实例可能同时存在多个并行任务，必须遍历目标分页，不能只保留第一页入口。
		resp, err := c.call(ctx, "/web/flowJobTaskLink/list", active.SID, map[string]any{
			"data": map[string]any{
				"flowInstanceId":               strings.TrimSpace(instanceID),
				"taskStatus":                   "waiting_send",
				"auditWayList":                 []string{},
				"useScope":                     "invest",
				"flowInstanceBizRelevance":     map[string]any{},
				"flowInstanceBizRelevanceList": []any{},
			},
			"pagination": true, "pages": page, "size": pageSize,
		})
		if err != nil {
			return "", nil, nil, false, err
		}
		if !responseSucceeded(resp) {
			return "", nil, nil, false, responseError(resp)
		}
		var raw []struct {
			FlowInstanceID  string `json:"flowInstanceId"`
			FlowProxyID     string `json:"flowProxyId"`
			FlowNodeProxyID string `json:"flowNodeProxyId"`
			FormProxyID     string `json:"formProxyId"`
		}
		if err := decodeArray(resp.Data, &raw); err != nil {
			return "", nil, nil, false, err
		}
		for _, item := range raw {
			if strings.TrimSpace(item.FlowInstanceID) != strings.TrimSpace(instanceID) || strings.TrimSpace(item.FlowProxyID) == "" {
				continue
			}
			currentProxyID := strings.TrimSpace(item.FlowProxyID)
			// 同一实例任务若指向不同代理树，无法证明入口归属，必须拒绝而不是任选一个。
			if proxyID != "" && proxyID != currentProxyID {
				return "", nil, nil, false, invalidResponse("due tasks reference different flow proxies")
			}
			proxyID = currentProxyID
			entryID := strings.TrimSpace(item.FlowNodeProxyID)
			if entryID == "" {
				continue
			}
			if _, exists := seen[entryID]; !exists {
				seen[entryID] = struct{}{}
				entries = append(entries, entryID)
			}
			formID := strings.TrimSpace(item.FormProxyID)
			if formID != "" {
				if _, exists := seenForms[formID]; !exists {
					seenForms[formID] = struct{}{}
					formProxyIDs = append(formProxyIDs, formID)
				}
			}
		}
		hasMore := false
		if resp.Pages > 0 {
			if resp.Pages > maxPages {
				return "", nil, nil, false, invalidResponse("due task pagination exceeds safe limit")
			}
			hasMore = page < resp.Pages
		} else {
			hasMore = len(raw) >= pageSize
		}
		if !hasMore {
			break
		}
		// 目标可能省略 pages 且持续返回满页；硬上限必须独立于目标元数据，避免异常响应造成无界读取。
		if page == maxPages {
			return "", nil, nil, false, invalidResponse("due task pagination exceeds safe limit")
		}
	}
	if proxyID == "" {
		return "", nil, nil, false, nil
	}
	return proxyID, entries, formProxyIDs, true, nil
}

// ReadTemplateTree 按模板 ID 读取新发起的真实节点树。
func (c *Client) ReadTemplateTree(ctx context.Context, active Session, templateID string) (*FlowNodeTemplate, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	return tree, err
}

// ReadProxyTree 按已核实的 flowProxyId 读取既有实例代理树。
func (c *Client) ReadProxyTree(ctx context.Context, active Session, proxyID string) (*FlowNodeTemplate, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	return tree, err
}

// ReadTemplateRequirements 读取模板树及其关联表单字段，供路径要求核对内部使用。
func (c *Client) ReadTemplateRequirements(ctx context.Context, active Session, templateID string) (*FlowNodeTemplate, []FormFieldMetadata, error) {
	tree, forms, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	if err != nil {
		return nil, nil, err
	}
	fields, err := c.readFormFields(ctx, active, "/web/formTemplateApi/findById", forms)
	return tree, fields, err
}

// ReadProxyRequirements 读取代理树及实例代理表单字段，不回退到模板表单猜测运行态字段。
func (c *Client) ReadProxyRequirements(ctx context.Context, active Session, proxyID string, formProxyIDs []string) (*FlowNodeTemplate, []FormFieldMetadata, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	if err != nil {
		return nil, nil, err
	}
	forms := make([]rawFormReference, 0, len(formProxyIDs))
	for _, rawID := range formProxyIDs {
		if id := strings.TrimSpace(rawID); id != "" {
			forms = append(forms, rawFormReference{ID: id})
		}
	}
	fields, err := c.readFormFields(ctx, active, "/web/formProxy/findById", forms)
	return tree, fields, err
}

// ReadTemplateConfiguration 读取模板树、表单字段详情和模板默认值，供新发起路径配置使用。
func (c *Client) ReadTemplateConfiguration(ctx context.Context, active Session, templateID string) (PathConfigurationSnapshot, error) {
	tree, forms, flowCode, flowName, auditWay, formExist, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	c.resolveFlowAuditMetadata(ctx, active, tree)
	fields, runtimeForms, err := c.readFormFieldDetails(ctx, active, "/web/formTemplateApi/findById", forms)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	renderType := NormalizeFormRenderType(formExist, len(runtimeForms))
	return PathConfigurationSnapshot{Tree: tree, FlowCode: flowCode, FlowName: flowName, AuditWay: auditWay, RenderType: renderType, VuePage: ResolveVueCustomPage(renderType, auditWay, flowName), FormFields: fields, Forms: runtimeForms}, nil
}

// ReadProxyConfiguration 读取代理树、实例代理表单字段详情和实例当前表单数据，供已发/待发路径配置使用。
func (c *Client) ReadProxyConfiguration(ctx context.Context, active Session, proxyID string, formProxyIDs []string, instanceID string) (PathConfigurationSnapshot, error) {
	tree, _, flowCode, flowName, auditWay, formExist, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	c.resolveFlowAuditMetadata(ctx, active, tree)
	forms := make([]rawFormReference, 0, len(formProxyIDs))
	for _, rawID := range formProxyIDs {
		if id := strings.TrimSpace(rawID); id != "" {
			forms = append(forms, rawFormReference{ID: id})
		}
	}
	fields, runtimeForms, err := c.readFormFieldDetails(ctx, active, "/web/formProxy/findById", forms)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	values, err := c.readInstanceCurrentData(ctx, active, instanceID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	renderType := NormalizeFormRenderType(formExist, len(runtimeForms))
	return PathConfigurationSnapshot{Tree: tree, FlowCode: flowCode, FlowName: flowName, AuditWay: auditWay, RenderType: renderType, VuePage: ResolveVueCustomPage(renderType, auditWay, flowName), FormFields: fields, Forms: runtimeForms, InstanceValues: values}, nil
}

// readFlowDetail 调用目标详情端点并转换同一棵流程树和关联表单引用。
func (c *Client) readFlowDetail(ctx context.Context, active Session, path, id string) (*FlowNodeTemplate, []rawFormReference, string, string, string, string, error) {
	resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": strings.TrimSpace(id)}})
	if err != nil {
		return nil, nil, "", "", "", "", err
	}
	if !responseSucceeded(resp) {
		return nil, nil, "", "", "", "", responseError(resp)
	}
	var data struct {
		FlowNodeTemplate *rawFlowNodeTemplate `json:"flowNodeTemplate"`
		FormTemplateList []rawFormReference   `json:"formTemplateList"`
		Code             string               `json:"code"`
		FlowCode         string               `json:"flowCode"`
		FlowName         string               `json:"flowName"`
		AuditWay         string               `json:"auditWay"`
		FormExist        string               `json:"formExist"`
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil, nil, "", "", "", "", nil
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, nil, "", "", "", "", invalidResponse("invalid flow tree data")
	}
	return convertFlowNode(data.FlowNodeTemplate), data.FormTemplateList, firstNonEmpty(data.FlowCode, data.Code), strings.TrimSpace(data.FlowName), strings.TrimSpace(data.AuditWay), data.FormExist, nil
}

// readFormFields 逐个读取已核实表单详情，只保留中文展示所需的名称字典。
func (c *Client) readFormFields(ctx context.Context, active Session, path string, forms []rawFormReference) ([]FormFieldMetadata, error) {
	result := make([]FormFieldMetadata, 0)
	seen := make(map[string]struct{}, len(forms))
	for _, form := range forms {
		formID := strings.TrimSpace(form.ID)
		if formID == "" {
			continue
		}
		if _, exists := seen[formID]; exists {
			continue
		}
		seen[formID] = struct{}{}
		resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": formID}})
		if err != nil {
			return nil, err
		}
		if !responseSucceeded(resp) {
			return nil, responseError(resp)
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			continue
		}
		var data struct {
			ID     string         `json:"id"`
			Name   string         `json:"name"`
			Fields []rawFormField `json:"fieldsTemplateList"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, invalidResponse("invalid form field data")
		}
		resolvedFormID := firstNonEmpty(data.ID, formID)
		resolvedFormName := firstNonEmpty(data.Name, form.Name)
		for _, field := range data.Fields {
			result = append(result, FormFieldMetadata{
				FormID: resolvedFormID, FormName: resolvedFormName, FieldID: field.ID,
				Name: field.Name, EnglishName: field.EnglishName,
			})
		}
	}
	return result, nil
}

// readFormFieldDetails 逐个读取已核实表单详情，并把 FormMaking 组件配置合并为字段类型、必填、默认值和选项。
func (c *Client) readFormFieldDetails(ctx context.Context, active Session, path string, forms []rawFormReference) ([]FormFieldDetail, []FormRuntimeTemplate, error) {
	result := make([]FormFieldDetail, 0)
	runtimeForms := make([]FormRuntimeTemplate, 0, len(forms))
	seen := make(map[string]struct{}, len(forms))
	for _, form := range forms {
		formID := strings.TrimSpace(form.ID)
		if formID == "" {
			continue
		}
		if _, exists := seen[formID]; exists {
			continue
		}
		seen[formID] = struct{}{}
		resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": formID}})
		if err != nil {
			return nil, nil, err
		}
		if !responseSucceeded(resp) {
			return nil, nil, responseError(resp)
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			continue
		}
		var data struct {
			ID           string               `json:"id"`
			Name         string               `json:"name"`
			Fields       []rawFormFieldDetail `json:"fieldsTemplateList"`
			TemplateData string               `json:"templateData"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, nil, invalidResponse("invalid form field data")
		}
		components := parseFormMakingComponents(data.TemplateData)
		resolvedFormID := firstNonEmpty(data.ID, formID)
		resolvedFormName := firstNonEmpty(data.Name, form.Name)
		if strings.TrimSpace(data.TemplateData) != "" {
			runtimeForms = append(runtimeForms, FormRuntimeTemplate{ID: resolvedFormID, Name: resolvedFormName, TemplateData: data.TemplateData})
		}
		for _, field := range data.Fields {
			component, hasComponent := components[strings.TrimSpace(field.EnglishName)]
			detail := FormFieldDetail{
				FormID: resolvedFormID, FormName: resolvedFormName, FieldID: field.ID,
				Name: firstNonEmpty(component.Name, field.Name), EnglishName: field.EnglishName,
				FieldType: strings.TrimSpace(field.FieldType), DefaultValue: strings.TrimSpace(field.DefaultValue),
				ValueOrigin: strings.TrimSpace(field.ValueOrigin), FieldStatus: strings.TrimSpace(field.FieldStatus),
				ComponentType: component.Type, ComponentName: formMakingComponentName(component),
				DateMode: strings.TrimSpace(component.Options.Type),
			}
			if hasComponent {
				detail.Required = component.Options.Required
				detail.Multiple = component.Options.Multiple
				detail.Options = formMakingOptions(component.Options.Options)
				if value, ok := componentDefaultValue(component.Options.DefaultValue); ok {
					detail.DefaultValue = value
				}
			}
			result = append(result, detail)
		}
	}
	return result, runtimeForms, nil
}

// formMakingComponentName 按目标模板真实优先级保留自定义组件注册名，供诊断与兼容投影使用。
func formMakingComponentName(component rawFormMakingComponent) string {
	return firstNonEmpty(component.El, component.Options.ComponentName, component.Options.Component, component.ComponentName, component.Component)
}

// BaseURL 返回目标网关公开地址，供短期 iframe 会话请求必要的表单只读数据。
func (c *Client) BaseURL() string {
	if c == nil || c.baseURL == nil {
		return ""
	}
	return strings.TrimRight(c.baseURL.String(), "/")
}

// ReadInstanceCurrentData 读取已核实实例的当前表单值，供受限近期样本链复用。
func (c *Client) ReadInstanceCurrentData(ctx context.Context, active Session, instanceID string) (map[string]any, error) {
	return c.readInstanceCurrentData(ctx, active, instanceID)
}

// parseFormMakingComponents 递归解析 FormMaking 的 list、grid、report、tableColumns 与嵌套容器。
func parseFormMakingComponents(raw string) map[string]rawFormMakingComponent {
	result := make(map[string]rawFormMakingComponent)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	var data rawFormMakingData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return result
	}
	for _, component := range data.List {
		collectFormMakingComponent(result, component)
	}
	return result
}

// collectFormMakingComponent 深度优先收集组件及其所有真实容器子列表，避免嵌套字段因漏读而降级伪造。
func collectFormMakingComponent(result map[string]rawFormMakingComponent, component rawFormMakingComponent) {
	key := strings.TrimSpace(component.Model)
	if key != "" {
		if _, exists := result[key]; !exists {
			result[key] = component
		}
	}
	for _, child := range component.List {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.Columns {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.Rows {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.TableColumns {
		collectFormMakingComponent(result, child)
	}
}

// formMakingOptions 把组件选项收敛为标签与值；值优先取 value，缺省回退 id，标签回退值本身。
func formMakingOptions(raw []rawFormMakingOption) []FormFieldOption {
	result := make([]FormFieldOption, 0, len(raw))
	for _, option := range raw {
		value := anyString(option.Value)
		if value == "" {
			value = anyString(option.ID)
		}
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = value
		}
		if value == "" {
			continue
		}
		result = append(result, FormFieldOption{Label: label, Value: value})
	}
	return result
}

// componentDefaultValue 读取组件默认值并把标量值统一为字符串；数组等复杂默认值保持空。
func componentDefaultValue(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case float64:
		return formatNumber(typed), true
	default:
		return "", false
	}
}

// anyString 把选项 id/value 标量转换为字符串；对象或数组等复杂值保持空。
func anyString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(typed)
	default:
		return ""
	}
}

// formatNumber 把 JSON 数字格式化为无冗余尾数的十进制字符串。
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// readInstanceCurrentData 按精确实例 ID 读取当前 formDataMongoVo.data，作为已发/待发路径初始值。
func (c *Client) readInstanceCurrentData(ctx context.Context, active Session, instanceID string) (map[string]any, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil, nil
	}
	resp, err := c.call(ctx, "/web/flowInstanceApi/getCurrentFromData", active.SID, map[string]any{
		"data": map[string]any{"id": instanceID},
	})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(resp) {
		return nil, responseError(resp)
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil, nil
	}
	var data struct {
		Data map[string]any `json:"data"`
	}
	// 使用 UseNumber 保留目标数字字面量的小数位，条件 eq 依赖 BigDecimal 的 scale。
	if err := jsonvalues.Decode(resp.Data, &data); err != nil {
		return nil, invalidResponse("invalid instance form data")
	}
	if data.Data == nil {
		return nil, nil
	}
	return data.Data, nil
}

// convertFlowNode 递归转换目标节点；内部配置只供要求分析，公开图分析器不会序列化这些字段。
func convertFlowNode(raw *rawFlowNodeTemplate) *FlowNodeTemplate {
	if raw == nil {
		return nil
	}
	node := &FlowNodeTemplate{
		ID: raw.ID, Name: firstNonEmpty(raw.NodeName, raw.Name), Type: raw.Type,
		BranchExecuteType: raw.BranchExecuteType, Child: convertFlowNode(raw.Child),
		AuditConfig: convertFlowAuditConfig(raw.AuditConfig), FieldPowers: convertFlowFieldPowers(raw.FieldPowers),
		IsSkip: raw.IsSkip, Delay: raw.Delay, Unit: raw.Unit, DeadlineType: raw.DeadlineType,
	}
	node.ConditionNodes = convertFlowBranches(raw.ConditionNodes)
	node.ParallelNodes = convertFlowBranches(raw.ParallelNodes)
	return node
}

// convertFlowBranches 保留真实分支 ID、顺序、名称和子节点。
func convertFlowBranches(raw []rawFlowBranchTemplate) []FlowBranchTemplate {
	result := make([]FlowBranchTemplate, 0, len(raw))
	for _, branch := range raw {
		result = append(result, FlowBranchTemplate{
			ID: firstNonEmpty(branch.StrategyID, branch.ID), Name: firstNonEmpty(branch.NodeName, branch.Name),
			Sort: branch.Sort, Conditions: convertFlowConditions(branch.ConditionList), Child: convertFlowNode(branch.Child),
		})
	}
	return result
}

// convertFlowConditions 复制条件内部关联键，后续分析必须翻译后才能公开。
func convertFlowConditions(raw []rawFlowCondition) []FlowCondition {
	result := make([]FlowCondition, 0, len(raw))
	for _, condition := range raw {
		result = append(result, FlowCondition{
			FieldA: condition.FieldA, FieldB: condition.FieldB, ValueB: condition.ValueB,
			ValueType: condition.ValueType, Judge: condition.Judge, ConditionType: condition.ConditionType,
		})
	}
	return result
}

// convertFlowAuditConfig 去除审批业务 ID，仅保留分类、数量和可展示名称。
func convertFlowAuditConfig(raw *rawFlowNodeAuditConfig) *FlowNodeAuditConfig {
	if raw == nil {
		return nil
	}
	result := &FlowNodeAuditConfig{
		AuditType: raw.AuditType, Mode: raw.Mode, CountersignNum: raw.CountersignNum,
		FormPersonField: raw.FormPersonField, AuditCondition: raw.AuditCondition,
	}
	for _, detail := range raw.Details {
		result.Details = append(result.Details, FlowAuditDetail{ID: strings.TrimSpace(detail.BizID), Name: detail.Name, Type: detail.Type})
	}
	for _, scope := range raw.Scopes {
		result.Scopes = append(result.Scopes, FlowAuditScope{ID: strings.TrimSpace(scope.BizID), Type: scope.Type})
	}
	seenCandidates := make(map[string]bool, len(raw.Candidates))
	for _, candidate := range raw.Candidates {
		id := strings.TrimSpace(candidate.ID)
		name := firstNonEmpty(candidate.Name, candidate.RealName, candidate.DisplayName)
		if id == "" || strings.TrimSpace(name) == "" || seenCandidates[id] {
			continue
		}
		seenCandidates[id] = true
		result.Candidates = append(result.Candidates, FlowAuditCandidate{ID: id, Name: strings.TrimSpace(name)})
	}
	seenDefaults := make(map[string]bool, len(raw.DefaultCandidates))
	for _, candidate := range raw.DefaultCandidates {
		id := strings.TrimSpace(candidate.ID)
		name := firstNonEmpty(candidate.Name, candidate.RealName, candidate.DisplayName)
		if id == "" || strings.TrimSpace(name) == "" || seenDefaults[id] {
			continue
		}
		seenDefaults[id] = true
		result.DefaultCandidates = append(result.DefaultCandidates, FlowAuditCandidate{ID: id, Name: strings.TrimSpace(name)})
	}
	return result
}

// convertFlowFieldPowers 复制字段权限关联键，公开层只使用解析后的字段中文名。
func convertFlowFieldPowers(raw []rawFlowNodeFieldPower) []FlowNodeFieldPower {
	result := make([]FlowNodeFieldPower, 0, len(raw))
	for _, power := range raw {
		result = append(result, FlowNodeFieldPower{
			FormID: power.FormID, FieldID: power.FieldID, EnglishName: power.EnglishName, Power: power.Power,
		})
	}
	return result
}

// call 按已核实协议传递后端会话，并限制响应体、超时和公开错误内容。
func (c *Client) call(ctx context.Context, path, sid string, body map[string]any) (*envelope, error) {
	return c.callOfClass(ctx, path, sid, body, "read", "")
}

// CallWrite 发出唯一一次写请求，返回本次请求的 trace_id。
// trace_id 由本出口生成并贯穿 network.log/curl.log 与运行事实的尝试记录，实现记录与日志双向可达。
// 本方法不内建任何重试：调用方（执行器 submit 阶段）保证一次尝试只调用一次。
func (c *Client) CallWrite(ctx context.Context, path, sid string, body map[string]any) (*envelope, string, error) {
	traceID := logging.NewTraceID()
	result, err := c.callOfClass(ctx, path, sid, body, "write", traceID)
	return result, traceID, err
}

// callOfClass 是全部目标请求的唯一出口；class 标记读写分类，traceID 非空时作为日志关联键。
func (c *Client) callOfClass(ctx context.Context, path, sid string, body map[string]any, class, traceID string) (*envelope, error) {
	payload := make(map[string]any, len(body)+1)
	for key, value := range body {
		payload[key] = value
	}
	if sid != "" {
		payload["sid"] = sid
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, invalidResponse("cannot encode request")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	endpoint.RawPath = ""
	query := endpoint.Query()
	if c.config.PlatformCode != "" {
		query.Set("platformCode", c.config.PlatformCode)
	}
	if sid != "" {
		query.Set("sid", sid)
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, NewError(ErrorUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if sid != "" {
		req.Header.Set("sid", sid)
		req.Header.Set("origin", strings.TrimRight(c.baseURL.String(), "/"))
		req.Header.Set("Referer", strings.TrimRight(c.baseURL.String(), "/")+"/")
	}
	// 传输层失败分档是安全判定的前提：连接阶段未完成（确定失败、无副作用）与
	// 响应丢失（不确定）结论完全相反，必须在发出请求前挂上 httptrace 才能拿到判据。
	probe := &transportProbe{}
	req = req.WithContext(withRequestMetadata(httptrace.WithClientTrace(req.Context(), probe.trace()), class, traceID))
	response, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		phase := probe.classify(err)
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, &Error{Kind: ErrorTimeout, Cause: err, Transport: phase}
		}
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: phase}
	}
	defer response.Body.Close()
	// 以下分支都已收到完整 HTTP 响应，传输阶段一律记为 responded，结论交给响应侧判定。
	if response.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorSessionExpired, HTTPStatus: response.StatusCode, Transport: TransportResponded}
	}
	if response.StatusCode == http.StatusForbidden {
		return nil, &Error{Kind: ErrorPermissionDenied, HTTPStatus: response.StatusCode, Transport: TransportResponded}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{Kind: ErrorUnavailable, HTTPStatus: response.StatusCode, Transport: TransportResponded}
	}
	reader := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		// 响应头已到但正文读取中断：响应不完整，写是否生效无法确定，按响应丢失分档。
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: TransportInterrupted}
	}
	if len(data) > maxResponseBytes {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("response too large")}
	}
	var result envelope
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("invalid json")}
	}
	if responseSessionExpired(&result) {
		return nil, NewError(ErrorSessionExpired, nil)
	}
	return &result, nil
}

// EncryptPassword 使用目标登录协议要求的 AES 块加密服务端密码。
func EncryptPassword(password, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(password))
	padding := aes.BlockSize - len(encoded)%aes.BlockSize
	padded := append([]byte(encoded), bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypted := make([]byte, len(padded))
	for offset := 0; offset < len(padded); offset += aes.BlockSize {
		block.Encrypt(encrypted[offset:offset+aes.BlockSize], padded[offset:offset+aes.BlockSize])
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// responseSucceeded 兼容目标响应的两种成功标识。
func responseSucceeded(resp *envelope) bool {
	return resp != nil && (resp.IsSuccess || resp.Success)
}

// responseError 把目标业务失败收敛为会话失效或暂不可用。
func responseError(resp *envelope) error {
	if responseSessionExpired(resp) {
		return NewError(ErrorSessionExpired, nil)
	}
	if resp != nil {
		message := strings.ToLower(strings.TrimSpace(resp.Message))
		if strings.TrimSpace(resp.Code) == "403" || strings.Contains(message, "forbidden") || strings.Contains(message, "permission") || strings.Contains(message, "无权限") || strings.Contains(message, "没有权限") {
			return NewError(ErrorPermissionDenied, nil)
		}
	}
	return NewError(ErrorUnavailable, nil)
}

// responseSessionExpired 只识别已有证据支持的会话失效代码和文案。
func responseSessionExpired(resp *envelope) bool {
	if resp == nil || responseSucceeded(resp) {
		return false
	}
	switch strings.TrimSpace(resp.Code) {
	case "RESP401", "-1":
		return true
	case "ERROR_99999":
		message := strings.TrimSpace(resp.Message)
		return message == "请重新登录" || message == "用户会话已失效" || message == "SID已失效!"
	default:
		return false
	}
}

// decodeArray 兼容目标列表的直接数组和 records 包装结构。
func decodeArray(data json.RawMessage, destination any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if err := json.Unmarshal(data, destination); err == nil {
		return nil
	}
	var wrapped struct {
		Records json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil || len(wrapped.Records) == 0 {
		return invalidResponse("list data is not an array")
	}
	if err := json.Unmarshal(wrapped.Records, destination); err != nil {
		return invalidResponse("records is not an array")
	}
	return nil
}

// normalizePage 规范分页结果并拒绝目标端返回的负数边界。
func normalizePage[T any](items []T, resp *envelope, page, pageSize int) (Page[T], error) {
	if resp.Total < 0 || resp.Current < 0 || resp.Size < 0 || resp.Pages < 0 {
		return Page[T]{}, invalidResponse("negative pagination")
	}
	total := resp.Total
	if total == 0 && len(items) > 0 {
		total = len(items)
	}
	hasMore := page*pageSize < total
	if resp.Pages > 0 {
		hasMore = page < resp.Pages
	}
	return Page[T]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: hasMore}, nil
}

// auditUserNames 从活动节点映射中去重提取公开处理人名称。
func auditUserNames(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return ""
	}
	var nodes map[string]struct {
		UserList []struct {
			Name string `json:"name"`
		} `json:"userList"`
	}
	if err := json.Unmarshal(data, &nodes); err != nil {
		return ""
	}
	keys := make([]string, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	names := make([]string, 0)
	seen := make(map[string]struct{})
	for _, key := range keys {
		for _, user := range nodes[key].UserList {
			name := strings.TrimSpace(user.Name)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return strings.Join(names, ",")
}

// auditNodeIDs 把 currentAuditUserInfo 的真实节点键稳定转换为并行入口集合。
func auditNodeIDs(data json.RawMessage) []string {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var nodes map[string]json.RawMessage
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil
	}
	result := make([]string, 0, len(nodes))
	for key := range nodes {
		if id := strings.TrimSpace(key); id != "" {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

// templateStatusText 将模板状态转换为已有中文展示。
func templateStatusText(status string) string {
	switch status {
	case "enable":
		return "正常"
	case "disable":
		return "停用"
	default:
		return status
	}
}

// submittedStatusText 将已发状态按参考页面证据转换为中文，未知值不泄露英文原值。
func submittedStatusText(status string) string {
	switch status {
	case "await_sent":
		return "待发"
	case "run":
		return "审批中"
	case "withdraw":
		return "撤销"
	case "termination":
		return "终止"
	case "abandon":
		return "丢弃"
	case "rejected":
		return "驳回"
	case "end":
		return "完结"
	case "draft":
		return "草稿"
	default:
		return "状态未知"
	}
}

// dueStatusText 转换待发实例已有证据支持的状态文案。
func dueStatusText(status string) string {
	switch status {
	case "rejected":
		return "驳回"
	case "withdraw":
		return "撤销"
	case "draft":
		return "草稿"
	default:
		return status
	}
}

// isTimeout 判断底层网络错误是否属于超时边界。
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// firstNonEmpty 返回首个非空字段以兼容目标响应的已核实别名。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// SubmittedStatusText 暴露目标实例状态的中文名称映射，供快速候选查询复用同一套状态文案。
func SubmittedStatusText(status string) string { return submittedStatusText(strings.TrimSpace(status)) }

// FindDueTaskID 精确重查当前账号在指定实例、指定节点上的活动待办任务链接 ID。
// 审批写请求的 data.jobTaskId 是目标硬性必填项；本方法只读、可安全重试。
// 同一节点存在多个活动任务时无法证明唯一归属，必须报错而不是任选一个。
func (c *Client) FindDueTaskID(ctx context.Context, active Session, instanceID, nodeProxyID string) (string, error) {
	resp, err := c.call(ctx, "/web/flowJobTaskLink/list", active.SID, map[string]any{
		"data": map[string]any{
			"flowInstanceId":               strings.TrimSpace(instanceID),
			"taskStatus":                   "waiting_send",
			"auditWayList":                 []string{},
			"useScope":                     "invest",
			"flowInstanceBizRelevance":     map[string]any{},
			"flowInstanceBizRelevanceList": []any{},
		},
		"pagination": true, "pages": 1, "size": 100,
	})
	if err != nil {
		return "", err
	}
	if !responseSucceeded(resp) {
		return "", responseError(resp)
	}
	var raw []struct {
		ID              string `json:"id"`
		FlowInstanceID  string `json:"flowInstanceId"`
		FlowNodeProxyID string `json:"flowNodeProxyId"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return "", err
	}
	wantInstance := strings.TrimSpace(instanceID)
	wantNode := strings.TrimSpace(nodeProxyID)
	matched := make([]string, 0, 1)
	for _, item := range raw {
		if strings.TrimSpace(item.FlowInstanceID) != wantInstance {
			continue
		}
		if wantNode != "" && strings.TrimSpace(item.FlowNodeProxyID) != wantNode {
			continue
		}
		if id := strings.TrimSpace(item.ID); id != "" {
			matched = append(matched, id)
		}
	}
	if len(matched) == 0 {
		return "", nil
	}
	if len(matched) > 1 {
		return "", invalidResponse("multiple due tasks on the same node")
	}
	return matched[0], nil
}

// Ping 用一次轻量只读请求探活会话：会话有效时无论业务结果如何都返回 nil；
// 只有会话失效（RESP401/HTTP 401）或传输失败才返回错误。
// 用途：执行器 prepare 阶段在登录后立即验证 sid 可用——实测目标存在
// “首次登录的 sid 立即失效、重新登录后才有效”的现象（纲领第 4.4.1 节抖动家族）。
func (c *Client) Ping(ctx context.Context, session Session) error {
	_, err := c.call(ctx, "/web/flowTemplateApi/list", session.SID, map[string]any{
		"data": map[string]any{}, "pagination": false, "pages": 1, "size": 1,
	})
	return err
}
