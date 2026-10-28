// T007 runtime 页面请求转发：把无头页面拦截到的目标请求经同一 HTTP 出口发给目标平台。
// 纪律（docs/s/01-执行引擎设计.md 5.3）：这是唯一出口的扩展，不是第二个目标 HTTP 客户端——
// 复用 Client 的 baseURL、传输层（含日志与代理绕过）、会话身份与响应分类；
// 页面请求的路径必须已通过 runtimeexec 的允许清单校验后才允许进入本方法。
package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"

	"test-auto-pro-v2/internal/logging"
)

// PageForwardRequest 是 runtime 页面发来的目标请求事实（已通过路径允许清单校验）。
// Header 保留页面原样请求头；Host/Cookie/内容长度等由本出口重建，防止伪造会话归属。
type PageForwardRequest struct {
	Method string
	// RequestClass 是唯一目标出口登记的读写事实，不能从 HTTP 方法推断（页面大量只读接口也使用 POST）。
	RequestClass string
	// RequestSeq 对应执行器完整动作轨迹中的原始顺序号；普通 Go 目标调用不设置。
	RequestSeq int
	// RetryAttempt 是本次会话刷新重放序号：首次为 1，SID 刷新后的唯一重放为 2。
	RetryAttempt int
	// Path 是目标端点路径（如 /web/flowInstanceApi/submit）；Query 原样保留页面查询参数。
	Path  string
	Query url.Values
	// Header 是页面请求头（键已小写归一）。
	Header map[string]string
	// Body 是原始请求正文；二进制（上传）按原样转发，不脱敏不改写。
	Body []byte
	// CustomerCode 是当前会话客户码；非空时覆盖页面请求头里的同名字段语义。
	CustomerCode string
	// PlatformCode 是本次运行应强制的目标平台码（实例重建类装载取实例真实平台）；
	// 非空时覆盖页面查询里的默认平台码——页面 axios 默认 200001，会把 999999 实例查空导致装载超时，
	// 这里以运行事实为准统一覆盖，与 sid「以 Go 会话为准」同一纪律。发起类为空则保留页面默认。
	PlatformCode string
}

// PageForwardResponse 是目标平台的原始响应事实；任何 HTTP 状态都原样返回给页面。
// TraceID 是本次转发在传输层的链路 ID（写请求据此与 network.log 互查）。
type PageForwardResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	TraceID    string
}

// ForwardRuntimeRequest 以当前账号会话发出页面目标请求。
// 会话失效（HTTP 401 或响应包络 RESP401）返回 ErrorSessionExpired，由调用方刷新会话后重试一次；
// 传输层错误原样上抛（Transport 阶段供客户端判断「是否已写出」）。
func (c *Client) ForwardRuntimeRequest(ctx context.Context, session Session, request PageForwardRequest) (*PageForwardResponse, error) {
	endpoint := *c.baseURL
	// 页面里并存多套 axios 实例：部分实例的 baseURL 被执行装载清空（页面相对 /web/...），
	// 部分实例仍持有构建期烘焙的绝对网关（请求路径自带 /api 前缀）。两类请求落到同一出口时，
	// 先剥掉转发路径上与网关 base 重复的前缀再拼接——否则烘焙前缀叠加 base 自身的 /api
	// 会拼出 /api/api/web/... 被网关 404（2026-09-16 真实验收实测缺陷）。
	// 归一化对无前缀路径是空操作；对真实以 /api 开头的目标路径（如 /api/type/list）
	// 剥掉后重新拼接结果不变，不改变任何合法请求的落点。
	forwardPath := request.Path
	if basePath := strings.TrimRight(c.baseURL.Path, "/"); basePath != "" {
		forwardPath = strings.TrimPrefix(forwardPath, basePath)
	}
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + forwardPath
	endpoint.RawPath = ""
	query := endpoint.Query()
	for key, values := range request.Query {
		for _, value := range values {
			query.Set(key, value)
		}
	}
	// 会话身份以 Go 当前账号缓存为准：页面残留的旧 sid 参数/请求头一律覆盖，
	// 保证「runtime 的每个目标请求都使用当前账号缓存的 SID」这一硬约束。
	if session.SID != "" {
		query.Set("sid", session.SID)
	}
	// 平台码以本次运行事实为准：实例重建类装载带回实例真实平台码（200001/999999），
	// 覆盖页面 axios 的默认 200001；否则 999999 实例被查空、详情弹窗不打开、装载 60s 超时。
	// 发起类装载 PlatformCode 为空，保留页面默认平台码不动。
	if pc := strings.TrimSpace(request.PlatformCode); pc != "" {
		query.Set("platformCode", pc)
	}
	endpoint.RawQuery = query.Encode()
	// 目标网关按「请求体顶层 sid」鉴权（直连出口 InjectWriteEnvelope 复刻其 axios 拦截器语义）：
	// 页面 JSON 体里的 sid 来自页面 localStorage，装载载荷未带 SID 时必然为空，只覆盖
	// query/header 会被空 body sid 顶掉，全部转发读取被目标判为未登录（2026-09-16 真实验收实测）。
	// 因此 JSON 对象体在这里统一重写会话信封（顶层 sid/空 projectId/data.customerCode）；
	// 非对象体（上传等二进制）保持原样直通。
	if len(request.Body) > 0 && session.SID != "" {
		var parsed map[string]any
		if json.Unmarshal(request.Body, &parsed) == nil && parsed != nil {
			if encoded, err := json.Marshal(InjectWriteEnvelope(parsed, session.SID, session.CustomerCode)); err == nil {
				request.Body = encoded
			}
		}
	}
	var bodyReader io.Reader
	if len(request.Body) > 0 {
		bodyReader = bytes.NewReader(request.Body)
	}
	req, err := http.NewRequestWithContext(ctx, request.Method, endpoint.String(), bodyReader)
	if err != nil {
		return nil, NewError(ErrorUnavailable, err)
	}
	for name, value := range request.Header {
		switch name {
		case "host", "content-length", "connection", "accept-encoding", "origin", "referer", "sid":
			// 逐跳头与会话头由本出口重建，不信任页面取值。
		default:
			req.Header.Set(name, value)
		}
	}
	if session.SID != "" {
		req.Header.Set("sid", session.SID)
		req.Header.Set("origin", strings.TrimRight(c.baseURL.String(), "/"))
		req.Header.Set("Referer", strings.TrimRight(c.baseURL.String(), "/")+"/")
	}
	traceID := logging.NewTraceID()
	probe := &transportProbe{}
	requestClass := strings.TrimSpace(request.RequestClass)
	if requestClass != "write" {
		requestClass = "read"
	}
	ctx = WithRuntimeRequestSeq(ctx, request.RequestSeq)
	ctx = WithRetryAttempt(ctx, request.RetryAttempt > 1, request.RetryAttempt)
	req = req.WithContext(ctx)
	req = req.WithContext(withRequestMetadata(httptrace.WithClientTrace(req.Context(), probe.trace()), requestClass, traceID))
	response, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		phase := probe.classify(err)
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, &Error{Kind: ErrorTimeout, Cause: err, Transport: phase, TraceID: traceID}
		}
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: phase, TraceID: traceID}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: TransportInterrupted, TraceID: traceID}
	}
	if len(data) > maxResponseBytes {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("response too large"), TraceID: traceID}
	}
	if response.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorSessionExpired, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data), TraceID: traceID}
	}
	// 目标习惯用 HTTP 200 包裹会话失效包络（RESP401）：转发路径同样要识别，
	// 否则 SID 失效会被当成普通响应写进业务，后续读全会失败且无法自动恢复。
	var envelopeBody envelope
	if json.Unmarshal(data, &envelopeBody) == nil && responseSessionExpired(&envelopeBody) {
		return nil, &Error{Kind: ErrorSessionExpired, Transport: TransportResponded, Cause: httpResponseDetail(data), TraceID: traceID}
	}
	headers := make(map[string]string, len(response.Header))
	for name, values := range response.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return &PageForwardResponse{
		StatusCode: response.StatusCode,
		Headers:    headers,
		Body:       data,
		TraceID:    traceID,
	}, nil
}

// writeEndpointSet 是全部已登记目标写端点：runtime 页面转发命中其中之一且已发出请求时，
// 连接断开必须按「可能已写出」处理（结果待确认），绝不允许自动重发。
var writeEndpointSet = map[string]bool{
	WriteEndpointSubmit:               true,
	RuntimeReSubmitEndpoint:           true,
	RuntimeAuditEndpoint:              true,
	WriteEndpointAudit:                true,
	WriteEndpointReSubmit:             true,
	WriteEndpointStorageForm:          true,
	WriteEndpointApproverAppend:       true,
	WriteEndpointUpdateFlowProxy:      true,
	WriteEndpointRollBack:             true,
	WriteEndpointRetrieve:             true,
	WriteEndpointRevocation:           true,
	WriteEndpointUrge:                 true,
	WriteEndpointTranspond:            true,
	WriteEndpointFlowTracking:         true,
	ContractReviewIncreaseEndpoint:    true,
	ContractReviewModifyEndpoint:      true,
	ContractReviewSaveRefFileEndpoint: true,
	RelationFileSaveBatchEndpoint:     true,
	RelationFileDeleteEndpoint:        true,
}

// FlowPostscriptEndpoint 是转发复合动作的第二写请求（附言保存）；与 transpond 同属写出口纪律。
const FlowPostscriptEndpoint = "/web/flowInstancePostscript/save"

// IsKnownWritePath 判定路径是否为已登记目标写端点（含转发附言端点）。
func IsKnownWritePath(path string) bool {
	if path == FlowPostscriptEndpoint {
		return true
	}
	return writeEndpointSet[path]
}

// IsKnownWriteRequestPath 判定页面转发请求的原始路径是否命中登记写端点。
// 页面构建产物把网关绝对地址（含 /api 前缀）烘焙进请求 URL，路径形如 /api/web/flowInstanceApi/submit；
// 只按裸路径比对会把真实写判成「未写出」，连接断开时误报零写入失败并诱发重复发送（F-036 夜间验证实测）。
// 因此先剥掉一层 /api 前缀再比对，两种形态都认。
func IsKnownWriteRequestPath(path string) bool {
	if IsKnownWritePath(path) {
		return true
	}
	if rest, ok := strings.CutPrefix(path, "/api"); ok {
		return IsKnownWritePath(rest)
	}
	return false
}

// ReadFormTemplateData 读取实例绑定表单代理的 FormMaking 模板原文（审批 runtime 装载用）。
// 读取失败按传输错误上抛：调用方必须阻塞写动作，不得以空模板发送。
func (c *Client) ReadFormTemplateData(ctx context.Context, active Session, formProxyID string) (json.RawMessage, error) {
	resp, err := c.call(ctx, "/web/formProxy/findById", active.SID, map[string]any{"data": map[string]any{"id": strings.TrimSpace(formProxyID)}})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(resp) {
		return nil, responseError(resp)
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil, invalidResponse("form proxy response is empty")
	}
	var data struct {
		TemplateData string `json:"templateData"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, invalidResponse("form proxy response is not an object")
	}
	if strings.TrimSpace(data.TemplateData) == "" {
		return nil, invalidResponse("form proxy response missing templateData")
	}
	return json.RawMessage(data.TemplateData), nil
}
