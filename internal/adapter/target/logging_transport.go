package target

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptrace"
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
	// 同一入口既供传输层逐请求记录，也供精确实例查询等业务诊断写范围事实；
	// 未注入时两种记录一起静默跳过，行为与旧实现一致。
	c.networkLogger = logger
	if _, alreadyWrapped := c.httpClient.Transport.(*loggingTransport); alreadyWrapped {
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
	traceID := requestTraceIDFromContext(request.Context())
	requestBody := readRequestBody(request)
	record := logging.NetworkRecord{
		TraceID: traceID, CurlTraceID: traceID,
		Method:   request.Method,
		Endpoint: request.URL.Path,
		// 分类由唯一请求出口标记；未标记的历史只读调用按 read 落盘。
		RequestClass: RequestClassFromContext(request.Context()),
		Curl:         logging.CurlCommand(request.Method, request.URL.String(), requestHeaders(request), requestBody),
		Retry:        RetryFromContext(request.Context()),
		RetryAttempt: RetryAttemptFromContext(request.Context()),
	}
	started := time.Now()
	probe := &transportProbe{}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), probe.trace()))
	response, err := next.RoundTrip(request)
	record.Duration = time.Since(started)
	record.TransportPhase = string(probe.classify(err))
	record.RequestWritten = probe.wroteRequest.Load()
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
	record.OutcomeKind, record.TargetInstanceID, record.TargetTaskID, record.TargetMessage, record.TargetCode = inspectEnvelope(body)
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
	data, err := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if err != nil {
		request.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	request.Body = io.NopCloser(bytes.NewReader(data))
	request.ContentLength = int64(len(data))
	return string(truncateLoggedBody(data))
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
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		response.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	return string(truncateLoggedBody(data))
}

// truncateLoggedBody 只限制日志字段，不截断回填到 HTTP 请求或响应的原始正文。
// 日志容量约束不能改变目标协议正文，否则大表单会被截断、完整响应也可能无法解析。
func truncateLoggedBody(data []byte) []byte {
	if len(data) <= maxLoggedBodyBytes {
		return data
	}
	return data[:maxLoggedBodyBytes]
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

// inspectEnvelope 从目标业务包络提取结果分类、目标实例/任务标识与 message/code 安全摘要（F-034 评审 #3）。
// message/code 只取包络层一句话与错误码，不含任何请求/响应正文；解析失败时不猜测、留空。
// InspectEnvelopeForTest 是包络提取的导出形态：供 test 目录按公开行为锁定 message/code 摘要。
func InspectEnvelopeForTest(body string) (outcomeKind, instanceID, taskID, targetMessage, targetCode string) {
	return inspectEnvelope(body)
}

func inspectEnvelope(body string) (outcomeKind, instanceID, taskID, targetMessage, targetCode string) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || (!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[")) {
		return "", "", "", "", ""
	}
	var parsed struct {
		IsSuccess bool   `json:"isSuccess"`
		Success   bool   `json:"success"`
		Message   string `json:"message"`
		Code      string `json:"code"`
		Data      struct {
			ID         string `json:"id"`
			InstanceID string `json:"flowInstanceId"`
			TaskID     string `json:"jobTaskId"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", "", "", "", ""
	}
	outcomeKind = "business_success"
	if !parsed.IsSuccess && !parsed.Success {
		outcomeKind = "business_failure"
	}
	instanceID = strings.TrimSpace(firstNonEmpty(parsed.Data.InstanceID, parsed.Data.ID))
	taskID = strings.TrimSpace(parsed.Data.TaskID)
	// 安全摘要：压成单行并限制长度，保证单行日志格式且不携带正文。
	targetMessage = sanitizeTargetSummary(parsed.Message)
	targetCode = sanitizeTargetSummary(parsed.Code)
	return outcomeKind, instanceID, taskID, targetMessage, targetCode
}

// sanitizeTargetSummary 把目标包络摘要压成单行安全文本：去换行/制表符并限制长度，不含正文。
func sanitizeTargetSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	value = replacer.Replace(value)
	// F-034 评审建议：按 rune 截断避免把中文切成非法 UTF-8 字节序列。
	if runes := []rune(value); len(runes) > 200 {
		value = string(runes[:200])
	}
	return value
}
