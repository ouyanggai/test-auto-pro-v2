package target

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// maxLoggedBodyBytes 限制写入 curl 日志的正文长度，避免单次响应把日志文件顶满。
const maxLoggedBodyBytes = 1 << 20

// networkLogger 是目标请求日志的最小依赖；只用接口便于测试注入假日志器。
type networkLogger interface {
	Network(scope logging.Scope, record logging.NetworkRecord)
}

// SetNetworkLogger 在唯一出口接入目标请求日志。未注入时客户端行为完全不变。
func (c *Client) SetNetworkLogger(logger networkLogger) {
	if c == nil || logger == nil {
		return
	}
	c.httpClient.Transport = &loggingTransport{next: c.httpClient.Transport, logger: logger}
}

// loggingTransport 在 HTTP 传输层记录每一次目标请求。
// 放在传输层而不是业务方法里，是为了让 curl 命令与实际发出的请求逐字一致：
// 记录的就是真正写到网络上的方法、URL、请求头和正文。
type loggingTransport struct {
	next   http.RoundTripper
	logger networkLogger
}

// RoundTrip 记录请求与响应事实后返回原始响应，任何日志失败都不影响请求结果。
func (t *loggingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	scope := logging.ScopeFrom(request.Context())
	traceID := logging.NewTraceID()
	requestBody := readRequestBody(request)
	record := logging.NetworkRecord{
		TraceID: traceID, CurlTraceID: traceID,
		Method:   request.Method,
		Endpoint: request.URL.Path,
		// 当前工具只发只读请求；写端点白名单为空，所以分类固定为 read。
		RequestClass: "read",
		Curl:         logging.CurlCommand(request.Method, request.URL.String(), requestHeaders(request), requestBody),
	}
	started := time.Now()
	response, err := next.RoundTrip(request)
	record.Duration = time.Since(started)
	if err != nil {
		record.Result, record.ErrorType = "failure", transportErrorType(err)
		t.logger.Network(scope, record)
		return nil, err
	}
	record.StatusCode = response.StatusCode
	body := captureResponseBody(response)
	record.ResponseBody = body
	record.Result = "success"
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		record.Result, record.ErrorType = "failure", "http_status"
	}
	record.OutcomeKind, record.TargetInstanceID, record.TargetTaskID = inspectEnvelope(body)
	if record.Result == "success" && record.OutcomeKind == "business_failure" {
		record.Result = "failure"
	}
	t.logger.Network(scope, record)
	return response, nil
}

// readRequestBody 读取并复位请求正文，保证记录的正文与实际发出的完全一致。
func readRequestBody(request *http.Request) string {
	if request.Body == nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, maxLoggedBodyBytes))
	_ = request.Body.Close()
	if err != nil {
		request.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	request.Body = io.NopCloser(bytes.NewReader(data))
	request.ContentLength = int64(len(data))
	return string(data)
}

// requestHeaders 收集实际发出的请求头，供生成可重放命令。
func requestHeaders(request *http.Request) map[string]string {
	headers := make(map[string]string, len(request.Header))
	for key := range request.Header {
		headers[key] = request.Header.Get(key)
	}
	return headers
}

// captureResponseBody 读出响应正文并复位，供日志记录完整响应。
func captureResponseBody(response *http.Response) string {
	if response.Body == nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxLoggedBodyBytes))
	_ = response.Body.Close()
	if err != nil {
		response.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	return string(data)
}

// transportErrorType 把传输层失败收敛为稳定分类，与目标适配层的错误分类保持同一套词汇。
func transportErrorType(err error) string {
	if err == nil {
		return ""
	}
	if isTimeout(err) {
		return string(ErrorTimeout)
	}
	if strings.Contains(err.Error(), "context canceled") {
		return "canceled"
	}
	return string(ErrorUnavailable)
}

// inspectEnvelope 从目标业务包络提取结果分类与目标实例、任务标识，解析失败时不猜测。
func inspectEnvelope(body string) (outcomeKind, instanceID, taskID string) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[")) {
		return "", "", ""
	}
	var parsed struct {
		IsSuccess bool `json:"isSuccess"`
		Success   bool `json:"success"`
		Data      struct {
			ID         string `json:"id"`
			InstanceID string `json:"flowInstanceId"`
			TaskID     string `json:"jobTaskId"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", "", ""
	}
	outcomeKind = "business_success"
	if !parsed.IsSuccess && !parsed.Success {
		outcomeKind = "business_failure"
	}
	instanceID = strings.TrimSpace(firstNonEmpty(parsed.Data.InstanceID, parsed.Data.ID))
	taskID = strings.TrimSpace(parsed.Data.TaskID)
	return outcomeKind, instanceID, taskID
}
