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
	"strings"
	"time"

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
	// 传输层可能被日志包装（SetNetworkLogger）：沿包装链找到内层 *http.Transport 再求值，
	// 绝不凭“链路不可识别”直接下“无代理”的结论——那会掩盖代理绕过被破坏的回归（评审缺陷 19）。
	transport := c.httpClient.Transport
	for transport != nil {
		switch typed := transport.(type) {
		case *http.Transport:
			if typed.Proxy == nil {
				return nil, nil
			}
			return typed.Proxy(request)
		case *loggingTransport:
			transport = typed.next
		default:
			return nil, nil
		}
	}
	return nil, nil
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
	return c.callOfClassPlatform(ctx, path, sid, body, class, traceID, "")
}

// callWithPlatform 以指定平台码发出只读请求：目标不同目录数据挂在不同平台码下
// （固定角色在 999999，而流程模板等在 200001），统一网关参数会查空导致解析失败。
func (c *Client) callWithPlatform(ctx context.Context, path, sid string, body map[string]any, platformCode string) (*envelope, error) {
	return c.callOfClassPlatform(ctx, path, sid, body, "read", "", platformCode)
}

// callOfClassPlatform 是带平台码覆盖的统一出口；platformCode 为空时回落全局配置。
func (c *Client) callOfClassPlatform(ctx context.Context, path, sid string, body map[string]any, class, traceID, platformCode string) (*envelope, error) {
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
	platform := platformCode
	if platform == "" {
		platform = c.config.PlatformCode
	}
	if platform != "" {
		query.Set("platformCode", platform)
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
