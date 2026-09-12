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
		var rejection *BusinessRejection
		if errors.As(err, &rejection) {
			return Session{}, NewError(ErrorLoginRejected, err)
		}
		if targetErr := asError(err); targetErr != nil && targetErr.Kind == ErrorUnavailable && targetErr.HTTPStatus >= 400 && targetErr.HTTPStatus < 500 {
			return Session{}, errorWithStatus(ErrorLoginRejected, targetErr.HTTPStatus, targetErr.Cause)
		}
		return Session{}, err
	}
	if !responseSucceeded(resp) {
		if responseSessionExpired(resp) {
			return Session{}, NewError(ErrorLoginRejected, &BusinessRejection{Code: strings.TrimSpace(resp.Code), Message: strings.TrimSpace(resp.Message)})
		}
		return Session{}, NewError(ErrorLoginRejected, &BusinessRejection{Code: strings.TrimSpace(resp.Code), Message: strings.TrimSpace(resp.Message)})
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
	reader := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		// 响应头已到但正文读取中断：响应不完整，写是否生效无法确定，按响应丢失分档。
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: TransportInterrupted}
	}
	if len(data) > maxResponseBytes {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("response too large")}
	}
	// 以下分支都已收到完整 HTTP 响应，传输阶段一律记为 responded；同时保留正文里的 message，
	// 这样页面能直接显示目标原文，而不是把 401/403/500 统一翻译成内部分类名。
	if response.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorSessionExpired, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	if response.StatusCode == http.StatusForbidden {
		return nil, &Error{Kind: ErrorPermissionDenied, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{Kind: ErrorUnavailable, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	var result envelope
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("invalid json")}
	}
	if responseSessionExpired(&result) {
		// 完整响应里带会话失效包络：传输事实是「已收到完整响应」，
		// 丢掉它会把可判确定失败的鉴权拒绝升级成待对账（评审 P2）。
		return nil, &Error{Kind: ErrorSessionExpired, Transport: TransportResponded, Cause: responseDetail(&result)}
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

// responseError 把目标完整响应按会话、权限或业务拒绝分类。
// 完整业务响应必须保留原始 code/message，不能伪装成网络不可用，否则上层会错误重试。
func responseError(resp *envelope) error {
	cause := responseDetail(resp)
	if responseSessionExpired(resp) {
		return NewError(ErrorSessionExpired, cause)
	}
	if resp != nil {
		message := strings.ToLower(strings.TrimSpace(resp.Message))
		if strings.TrimSpace(resp.Code) == "403" || strings.Contains(message, "forbidden") || strings.Contains(message, "permission") || strings.Contains(message, "无权限") || strings.Contains(message, "没有权限") {
			return NewError(ErrorPermissionDenied, cause)
		}
		return &BusinessRejection{Code: strings.TrimSpace(resp.Code), Message: strings.TrimSpace(resp.Message)}
	}
	return NewError(ErrorResponseInvalid, errors.New("目标响应为空"))
}

// responseDetail 提取目标响应里用户能够直接理解的原始错误信息。
func responseDetail(resp *envelope) error {
	if resp == nil {
		return nil
	}
	message := strings.TrimSpace(resp.Message)
	if message == "" {
		message = strings.TrimSpace(resp.Error)
	}
	if message == "" {
		message = strings.TrimSpace(resp.Code)
	}
	if message == "" {
		return nil
	}
	return errors.New(message)
}

// httpResponseDetail 从非 2xx 响应正文中提取目标原文；正文不是 JSON 时保留原始文本。
func httpResponseDetail(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	var resp envelope
	if err := json.Unmarshal(data, &resp); err == nil {
		if detail := responseDetail(&resp); detail != nil {
			return detail
		}
	}
	return errors.New(trimmed)
}

// responseSessionExpired 只识别已有证据支持的会话失效代码和文案。
// 2026-09-07 实测补充 AUTH_401：目标平台存在「新登录的 SID 首个请求即返回 AUTH_401」的
// 失效形状（实测同一新 SID 连续三次 list 全部 AUTH_401，重登后恢复），漏认它会让探活把
// 已失效的会话当成业务失败而放行，写请求刚发出就被拒，运行反复死在同一个地方。
func responseSessionExpired(resp *envelope) bool {
	if resp == nil || responseSucceeded(resp) {
		return false
	}
	switch strings.TrimSpace(resp.Code) {
	case "RESP401", "AUTH_401", "-1":
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

// TaskPendingUser 是当前节点待处理人员的稳定形状：目标在部分节点只返回姓名与手机号，
// 不再返回用户 ID；人员目录按两者都能解析出登录账号。
type TaskPendingUser struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// NodeCurrentHandler 是「已发」列表 currentAuditUserInfo 里一个当前节点的待办处理人信息：
// auditType 是节点审批方式，bizIds 是自选类节点已选的用户 ID，users 是固定类节点解析出的
// 待处理人员（姓名+手机号）。会签节点在全部处理人审批完成前一直出现在实例事实里。
type NodeCurrentHandler struct {
	NodeID    string
	AuditType string
	BizIDs    []string
	Users     []TaskPendingUser
}

// auditHandlerInfo 解析 currentAuditUserInfo 为逐节点处理人信息；结构异常时按无数据处理，
// 调用方会退回 currentNodeProxyId 的既有语义，绝不猜测人员。
func auditHandlerInfo(data json.RawMessage) []NodeCurrentHandler {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var nodes map[string]struct {
		AuditType string   `json:"auditType"`
		BizIds    []string `json:"bizIds"`
		UserList  []struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		} `json:"userList"`
	}
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil
	}
	result := make([]NodeCurrentHandler, 0, len(nodes))
	for nodeID, info := range nodes {
		id := strings.TrimSpace(nodeID)
		if id == "" {
			continue
		}
		handler := NodeCurrentHandler{NodeID: id, AuditType: strings.TrimSpace(info.AuditType)}
		for _, bizID := range info.BizIds {
			if trimmed := strings.TrimSpace(bizID); trimmed != "" {
				handler.BizIDs = append(handler.BizIDs, trimmed)
			}
		}
		for _, user := range info.UserList {
			handler.Users = append(handler.Users, TaskPendingUser{
				Name:  strings.TrimSpace(user.Name),
				Phone: strings.TrimSpace(user.Phone),
			})
		}
		result = append(result, handler)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].NodeID < result[j].NodeID })
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

// ListTaskSnapshots 读取指定实例、指定目标任务状态的任务链接快照。
// 目标服务只接受 pending 或 done；空状态不是“全部状态”，会被目标拒绝。任务较多时必须完整遍历分页，
// 否则当前待办、批次或已办归属可能落在第二页而被错误判为不存在。
func (c *Client) ListTaskSnapshots(ctx context.Context, active Session, instanceID, taskStatus string) ([]TaskSnapshot, error) {
	return c.ListTaskSnapshotsForUser(ctx, active, instanceID, taskStatus, "")
}

// ListTaskSnapshotsForUser 以指定用户视角读取任务列表（F-030 评审 P2）：
// queryUserID 非空时不再切换会话也能读其他用户的任务——目标源码
// （FlowJobTaskLinkApiServiceImpl.list）只在 queryUserId 为空时才默认 SID 用户，
// 非空时按该用户查询；pending 写入协议顶层 queryUserId，done 写入 data.executorId
// （两个参数都是目标原生支持的既有字段，不是新增端点）。候选处理人扫描由此从
// 「逐个候选登录 + 各自完整分页扫描」收敛为「同一会话逐候选一次窄查询」，
// 只有命中者才需要登录会话。
func (c *Client) ListTaskSnapshotsForUser(ctx context.Context, active Session, instanceID, taskStatus, queryUserID string) ([]TaskSnapshot, error) {
	// F-030/T02：同一次事实读取边界内同一 (有效用户, 实例, 状态) 的列表只扫一次。
	// 键用账号而不是 SID：同账号锁内会话重建（重登）后列表内容对查找语义不变；
	// 有效用户 = 会话账号与显式 queryUserId 的组合，不同用户视角必须各自扫描。
	effectiveUser := strings.TrimSpace(active.Summary.Account)
	if id := strings.TrimSpace(queryUserID); id != "" {
		effectiveUser = "user:" + id
	}
	scopeKey := effectiveUser + "|" + strings.TrimSpace(instanceID) + "|" + strings.TrimSpace(taskStatus)
	if cached := taskListFromScope(ctx, scopeKey); cached != nil {
		return cached, nil
	}
	taskStatus = strings.TrimSpace(taskStatus)
	if taskStatus != "pending" && taskStatus != "done" {
		return nil, invalidResponse("unsupported task status")
	}
	data := map[string]any{
		"flowInstanceId":               strings.TrimSpace(instanceID),
		"taskStatus":                   taskStatus,
		"auditWayList":                 []string{},
		"useScope":                     "invest",
		"flowInstanceBizRelevance":     map[string]any{},
		"flowInstanceBizRelevanceList": []any{},
	}
	// 视角用户：done 列表必须限定实际执行人；指定视角用户时同样把 executorId 换成该用户。
	// 待办列表由目标网关按 SID 解析当前用户，但目标协议顶层 queryUserId 可显式指定待办视角用户
	// （源码：queryUserId 为空才默认 SID 用户），两个分支都用同一个既有字段，不新增端点。
	viewUserID := strings.TrimSpace(queryUserID)
	if taskStatus == "done" {
		executorID := viewUserID
		if executorID == "" {
			executorID = strings.TrimSpace(active.UserID)
		}
		if executorID == "" {
			return nil, invalidResponse("done task lookup missing executor id")
		}
		data["executorId"] = executorID
	}
	type rawTaskSnapshot struct {
		LinkID                 string `json:"id"`
		ParentLinkID           string `json:"pid"`
		JobTaskID              string `json:"jobTaskId"`
		FlowInstanceID         string `json:"flowInstanceId"`
		FlowNodeProxyID        string `json:"flowNodeProxyId"`
		BatchNo                string `json:"batchNo"`
		TaskStatus             string `json:"taskStatus"`
		ExecutorID             string `json:"executorId"`
		FormProxyID            string `json:"formProxyId"`
		FlowProxyID            string `json:"flowProxyId"`
		AuditWay               string `json:"auditWay"`
		FlowNextNodeAuditType  string `json:"flowNextNodeAuditType"`
		BranchExecuteType      string `json:"branchExecuteType"`
		CurrentPendingUserID   string `json:"currentPendingUserId"`
		CurrentPendingUserName string `json:"currentPendingUserName"`
	}
	wantInstance := strings.TrimSpace(instanceID)
	matched := make([]TaskSnapshot, 0)
	const pageSize = 100
	const maxPages = 20
	for page := 1; page <= maxPages; page++ {
		body := map[string]any{
			"data": data, "pagination": true, "pages": page, "size": pageSize,
		}
		if taskStatus == "pending" && viewUserID != "" {
			body["queryUserId"] = viewUserID
		}
		resp, err := c.call(ctx, "/web/flowJobTaskLink/list", active.SID, body)
		if err != nil {
			return nil, err
		}
		if !responseSucceeded(resp) {
			return nil, responseError(resp)
		}
		var raw []rawTaskSnapshot
		if err := decodeArray(resp.Data, &raw); err != nil {
			return nil, err
		}
		for _, item := range raw {
			if strings.TrimSpace(item.FlowInstanceID) != wantInstance {
				continue
			}
			jobTaskID := strings.TrimSpace(item.JobTaskID)
			if jobTaskID == "" {
				// BaseVo.id 是数据库关联行 ID，不是审批接口需要的 jobTaskId；
				// 缺少协议字段说明响应不完整，不能用 id 猜测替代。
				return nil, invalidResponse("task response missing jobTaskId")
			}
			matched = append(matched, TaskSnapshot{
				LinkID: strings.TrimSpace(item.LinkID), ParentLinkID: strings.TrimSpace(item.ParentLinkID),
				JobTaskID: jobTaskID, FlowInstanceID: strings.TrimSpace(item.FlowInstanceID),
				FlowNodeProxyID: strings.TrimSpace(item.FlowNodeProxyID), BatchNo: strings.TrimSpace(item.BatchNo),
				TaskStatus: strings.TrimSpace(item.TaskStatus), ExecutorID: strings.TrimSpace(item.ExecutorID),
				PendingUserID: strings.TrimSpace(item.CurrentPendingUserID), PendingUserName: strings.TrimSpace(item.CurrentPendingUserName),
				FormProxyID: strings.TrimSpace(item.FormProxyID), FlowProxyID: strings.TrimSpace(item.FlowProxyID),
				AuditWay: strings.TrimSpace(item.AuditWay), FlowNextNodeAuditType: strings.TrimSpace(item.FlowNextNodeAuditType),
				BranchExecuteType: strings.TrimSpace(item.BranchExecuteType),
			})
		}
		hasMore := false
		if resp.Pages > 0 {
			if resp.Pages > maxPages {
				return nil, invalidResponse("task pagination exceeds safe limit")
			}
			hasMore = page < resp.Pages
		} else {
			hasMore = len(raw) >= pageSize
		}
		if !hasMore {
			break
		}
		if page == maxPages {
			return nil, invalidResponse("task pagination exceeds safe limit")
		}
	}
	storeTaskListToScope(ctx, scopeKey, matched)
	return matched, nil
}

// FindTaskSnapshot 精确重查当前账号在指定实例、指定节点上的一条任务链接。
// pending 用于当前待办，done 用于当前账号的已办；同一实例同一节点出现多条匹配任务时
// 无法证明唯一归属，必须报错而不是任选一条。
func (c *Client) FindTaskSnapshot(ctx context.Context, active Session, instanceID, nodeProxyID, taskStatus string) (TaskSnapshot, error) {
	snapshots, err := c.ListTaskSnapshots(ctx, active, instanceID, taskStatus)
	if err != nil {
		return TaskSnapshot{}, err
	}
	wantNode := strings.TrimSpace(nodeProxyID)
	matched := make([]TaskSnapshot, 0, 1)
	for _, snapshot := range snapshots {
		if wantNode != "" && strings.TrimSpace(snapshot.FlowNodeProxyID) != wantNode {
			continue
		}
		matched = append(matched, snapshot)
	}
	if len(matched) == 0 {
		return TaskSnapshot{}, nil
	}
	if len(matched) > 1 {
		return TaskSnapshot{}, invalidResponse("multiple tasks on the same instance and node")
	}
	return matched[0], nil
}

// FindDueTaskID 精确重查当前账号在指定实例、指定节点上的活动待办任务链接 ID。
// 审批写请求的 data.jobTaskId 是目标硬性必填项；目标待办状态是 pending，不能误用 waiting_send。
func (c *Client) FindDueTaskID(ctx context.Context, active Session, instanceID, nodeProxyID string) (string, error) {
	task, err := c.FindTaskSnapshot(ctx, active, instanceID, nodeProxyID, "pending")
	if err != nil {
		return "", err
	}
	return task.JobTaskID, nil
}

// Ping 用一次轻量只读请求探活会话：会话有效时无论业务结果如何都返回 nil；
// 只有会话失效（RESP401/HTTP 401）或传输失败才返回错误。
// 用途：执行器 prepare 阶段在登录后立即验证 sid 可用——实测目标存在
// “首次登录的 sid 立即失效、重新登录后才有效”的现象（纲领第 4.4.1 节抖动家族）。
// Ping 只回答一个问题：这个 SID 在目标上是否真的存在有效会话。
// 判据是带完整业务参数的模板列表请求（与 ListTemplates 同形状的最小分页）：
// 有效会话返回成功列表；会话无效时返回 AUTH_401/RESP401，或触发「分组id不存在」——
// 2026-09-07 实测确认该业务异常会让目标把会话作废，此后同一 SID 的写端点一律
// RESP401。因此探活必须用不会触发该异常的正常请求形状，任何错误都判探活失败并重登。
func (c *Client) Ping(ctx context.Context, session Session) error {
	body := map[string]any{
		"data": map[string]any{
			"flowName":     "",
			"useScope":     "invest",
			"customerCode": firstNonEmpty(session.CustomerCode, c.config.CustomerCode),
		},
		"showMe":                             true,
		"ignoreFormTemplateBizRelevanceData": true,
		"formTemplateBizRelevanceList":       []any{},
		"notFormTemplateBizRelevanceList":    []map[string]any{{"otherBiz": "isProject", "otherBizId": "isProject"}},
		"ignoreTemplateData":                 true,
		"pagination":                         true,
		"pages":                              1,
		"size":                               1,
		"projectId":                          "",
		"platformCode":                       c.config.TemplatePlatformCodes,
		"notAuditWayList":                    []string{"staff_annual_assessment"},
	}
	_, err := c.call(ctx, "/web/flowTemplateApi/list", session.SID, body)
	return err
}
