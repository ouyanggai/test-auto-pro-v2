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
	// Path 是目标端点路径（如 /web/flowInstanceApi/submit）；Query 原样保留页面查询参数。
	Path  string
	Query url.Values
	// Header 是页面请求头（键已小写归一）。
	Header map[string]string
	// Body 是原始请求正文；二进制（上传）按原样转发，不脱敏不改写。
	Body []byte
	// CustomerCode 是当前会话客户码；非空时覆盖页面请求头里的同名字段语义。
	CustomerCode string
}

// PageForwardResponse 是目标平台的原始响应事实；任何 HTTP 状态都原样返回给页面。
type PageForwardResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// ForwardRuntimeRequest 以当前账号会话发出页面目标请求。
// 会话失效（HTTP 401 或响应包络 RESP401）返回 ErrorSessionExpired，由调用方刷新会话后重试一次；
// 传输层错误原样上抛（Transport 阶段供客户端判断「是否已写出」）。
func (c *Client) ForwardRuntimeRequest(ctx context.Context, session Session, request PageForwardRequest) (*PageForwardResponse, error) {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + request.Path
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
	endpoint.RawQuery = query.Encode()
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
	req = req.WithContext(withRequestMetadata(httptrace.WithClientTrace(req.Context(), probe.trace()), "runtime_forward", traceID))
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
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &Error{Kind: ErrorUnavailable, Cause: err, Transport: TransportInterrupted}
	}
	if len(data) > maxResponseBytes {
		return nil, &Error{Kind: ErrorResponseInvalid, Transport: TransportResponded, Cause: errors.New("response too large")}
	}
	if response.StatusCode == http.StatusUnauthorized {
		return nil, &Error{Kind: ErrorSessionExpired, HTTPStatus: response.StatusCode, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	// 目标习惯用 HTTP 200 包裹会话失效包络（RESP401）：转发路径同样要识别，
	// 否则 SID 失效会被当成普通响应写进业务，后续读全会失败且无法自动恢复。
	var envelopeBody envelope
	if json.Unmarshal(data, &envelopeBody) == nil && responseSessionExpired(&envelopeBody) {
		return nil, &Error{Kind: ErrorSessionExpired, Transport: TransportResponded, Cause: httpResponseDetail(data)}
	}
	headers := make(map[string]string, len(response.Header))
	for name, values := range response.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return &PageForwardResponse{
		StatusCode: response.StatusCode,
		Headers:    headers,
		Body:       data,
	}, nil
}

// writeEndpointSet 是全部已登记目标写端点：runtime 页面转发命中其中之一且已发出请求时，
// 连接断开必须按「可能已写出」处理（结果待确认），绝不允许自动重发。
var writeEndpointSet = map[string]bool{
	WriteEndpointSubmit:          true,
	WriteEndpointAudit:           true,
	WriteEndpointReSubmit:        true,
	WriteEndpointStorageForm:     true,
	WriteEndpointApproverAppend:  true,
	WriteEndpointUpdateFlowProxy: true,
	WriteEndpointRollBack:        true,
	WriteEndpointRetrieve:        true,
	WriteEndpointRevocation:      true,
	WriteEndpointUrge:            true,
	WriteEndpointTranspond:       true,
	WriteEndpointFlowTracking:    true,
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
