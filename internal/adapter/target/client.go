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
	"net/url"
	"sort"
	"strings"
	"time"
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
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: cfg.Timeout,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}, nil
}

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
			Name         string `json:"name"`
			CustomerCode string `json:"customerCode"`
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
		CompanyID:    data.Company.ID,
		Summary: AccountSummary{
			Account:     strings.TrimSpace(account),
			DisplayName: data.User.Name,
			CompanyName: data.Company.Name,
		},
	}, nil
}

type templateCompanyRelation struct {
	OtherBiz   string `json:"otherBiz"`
	OtherBizID string `json:"otherBizId"`
}

type rawFlowTemplate struct {
	ID                             string                    `json:"id"`
	FlowName                       string                    `json:"flowName"`
	Code                           string                    `json:"code"`
	FlowCode                       string                    `json:"flowCode"`
	GroupName                      string                    `json:"groupName"`
	FlowStatus                     string                    `json:"flowStatus"`
	TypeName                       string                    `json:"typeName"`
	UpdateDate                     string                    `json:"updateDate"`
	UpdateTime                     string                    `json:"updateTime"`
	CreateDate                     string                    `json:"createDate"`
	CreateTime                     string                    `json:"createTime"`
	Remark                         string                    `json:"remark"`
	FlowCreateType                 string                    `json:"flowCreateType"`
	FormExist                      string                    `json:"formExist"`
	FormTemplateList               []json.RawMessage         `json:"formTemplateList"`
	FormTemplateBizRelevanceVoList []templateCompanyRelation `json:"formTemplateBizRelevanceVoList"`
}

func (c *Client) ListTemplates(ctx context.Context, session Session, query string, page, pageSize int) (Page[FlowTemplate], error) {
	body := map[string]any{
		"data": map[string]any{
			"flowName":     strings.TrimSpace(query),
			"useScope":     "invest",
			"customerCode": firstNonEmpty(session.CustomerCode, c.config.CustomerCode),
		},
		"showMe":                          true,
		"formTemplateBizRelevanceList":    []any{},
		"notFormTemplateBizRelevanceList": []map[string]any{{"otherBiz": "isProject", "otherBizId": "isProject"}},
		"ignoreTemplateData":              true,
		"pagination":                      true,
		"pages":                           page,
		"size":                            pageSize,
		"projectId":                       "",
		"platformCode":                    c.config.TemplatePlatformCodes,
		"notAuditWayList":                 []string{"staff_annual_assessment"},
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
	companyNames := c.templateCompanyNames(ctx, session, raw)
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
			FormTemplateCount: len(item.FormTemplateList),
			CompanyName:       companyNames[templateCompanyID(item.FormTemplateBizRelevanceVoList)],
		})
	}
	return normalizePage(items, resp, page, pageSize)
}

func templateCompanyID(relations []templateCompanyRelation) string {
	for _, relation := range relations {
		if relation.OtherBiz == "company" && strings.TrimSpace(relation.OtherBizID) != "" {
			return strings.TrimSpace(relation.OtherBizID)
		}
	}
	return ""
}

func (c *Client) templateCompanyNames(ctx context.Context, session Session, templates []rawFlowTemplate) map[string]string {
	if strings.TrimSpace(session.CompanyID) == "" {
		return nil
	}
	hasCompanyRelation := false
	for _, template := range templates {
		if templateCompanyID(template.FormTemplateBizRelevanceVoList) != "" {
			hasCompanyRelation = true
			break
		}
	}
	if !hasCompanyRelation {
		return nil
	}

	response, err := c.call(ctx, "/web/user/api/company/getParentCompanyList", session.SID, map[string]any{
		"data": map[string]any{"id": session.CompanyID},
	})
	if err != nil || !responseSucceeded(response) {
		return nil
	}
	var companies []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if decodeArray(response.Data, &companies) != nil {
		return nil
	}
	names := make(map[string]string, len(companies))
	for _, company := range companies {
		id := strings.TrimSpace(company.ID)
		name := strings.TrimSpace(company.Name)
		if id != "" && name != "" {
			names[id] = name
		}
	}
	return names
}

func (c *Client) ListSubmitted(ctx context.Context, session Session, query string, page, pageSize int) (Page[SubmittedFlow], error) {
	data := map[string]any{
		"useScope":                     "invest",
		"auditWayList":                 []string{},
		"statusList":                   []string{"await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"},
		"flowInstanceBizRelevanceList": []map[string]any{{"otherBiz": "company", "otherBizId": ""}},
	}
	if value := strings.TrimSpace(query); value != "" {
		data["name"] = value
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
		Name                 string          `json:"name"`
		FormName             string          `json:"formName"`
		Status               string          `json:"status"`
		CreateDate           string          `json:"createDate"`
		CreateTime           string          `json:"createTime"`
		CurrentNodeName      string          `json:"currentNodeName"`
		CurrentAuditUserInfo json.RawMessage `json:"currentAuditUserInfo"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return Page[SubmittedFlow]{}, err
	}
	items := make([]SubmittedFlow, 0, len(raw))
	for _, item := range raw {
		items = append(items, SubmittedFlow{
			ID:                    item.ID,
			Name:                  item.Name,
			FormName:              item.FormName,
			Title:                 firstNonEmpty(item.Name, item.FormName),
			Status:                item.Status,
			CreateDate:            firstNonEmpty(item.CreateDate, item.CreateTime),
			CurrentNodeName:       item.CurrentNodeName,
			CurrentAuditUserNames: auditUserNames(item.CurrentAuditUserInfo),
		})
	}
	return normalizePage(items, resp, page, pageSize)
}

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

func (c *Client) call(ctx context.Context, path, sid string, body map[string]any) (*envelope, error) {
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
	response, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, NewError(ErrorTimeout, err)
		}
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, NewError(ErrorUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		return nil, errorWithStatus(ErrorSessionExpired, response.StatusCode, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errorWithStatus(ErrorUnavailable, response.StatusCode, nil)
	}
	reader := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, NewError(ErrorUnavailable, err)
	}
	if len(data) > maxResponseBytes {
		return nil, invalidResponse("response too large")
	}
	var result envelope
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, invalidResponse("invalid json")
	}
	if responseSessionExpired(&result) {
		return nil, NewError(ErrorSessionExpired, nil)
	}
	return &result, nil
}

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

func responseSucceeded(resp *envelope) bool {
	return resp != nil && (resp.IsSuccess || resp.Success)
}

func responseError(resp *envelope) error {
	if responseSessionExpired(resp) {
		return NewError(ErrorSessionExpired, nil)
	}
	return NewError(ErrorUnavailable, nil)
}

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

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
