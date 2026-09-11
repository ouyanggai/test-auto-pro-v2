package logging

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NetworkRecord 是一次目标平台请求的日志事实；调用方只填自己能证明的字段。
// RequestClass 区分 read/write，传输阶段与 requestWritten 共同说明请求是否可能产生副作用。
type NetworkRecord struct {
	TraceID      string
	CurlTraceID  string
	Method       string
	Endpoint     string
	RequestClass string
	StatusCode   int
	Duration     time.Duration
	// Result 为 success 或 failure；failure 同时进 network-error.log。
	Result string
	// OutcomeKind 是目标业务包络给出的结果分类，例如 isSuccess=false 时的稳定说明。
	OutcomeKind string
	// ErrorType 是目标适配层的稳定错误分类，成功时留空。
	ErrorType string
	// TargetInstanceID 与 TargetTaskID 按目标原样记录，供人工对照目标平台。
	TargetInstanceID string
	TargetTaskID     string
	// Curl 是可复制检查的请求命令；会话、密码和令牌字段统一替换为占位符。
	Curl string
	// ResponseBody 是目标返回的响应正文，只进 curl.log 的块内，敏感字段已替换。
	ResponseBody string
	// Retry 标记该次请求是否是受控重试。
	Retry bool
	// RetryAttempt 是受控请求的尝试序号，首次请求为 1。
	RetryAttempt int
	// TransportPhase 是连接失败、响应中断或已收到完整响应的传输事实。
	TransportPhase string
	// RequestWritten 表示请求已经由 HTTP 传输层完整写入连接。
	RequestWritten bool
}

// NewTraceID 生成一次目标请求的追踪标识；随机源失败时回退到时间戳，不阻塞主流程。
func NewTraceID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buffer)
}

// Network 记录一次目标请求：成功与运行提示进 network.log，失败进 network-error.log，
// 完整可重放命令与响应正文进 curl.log，三者用 trace_id 与 curl_trace_id 双向可查。
func (l *Logger) Network(scope Scope, record NetworkRecord) {
	if l == nil || l.router == nil {
		return
	}
	traceID := strings.TrimSpace(record.TraceID)
	if traceID == "" {
		traceID = NewTraceID()
	}
	curlTraceID := strings.TrimSpace(record.CurlTraceID)
	if curlTraceID == "" {
		curlTraceID = traceID
	}
	requestClass := strings.TrimSpace(record.RequestClass)
	if requestClass == "" {
		requestClass = "read"
	}
	result := strings.TrimSpace(record.Result)
	if result == "" {
		result = "success"
	}
	level := "info"
	if result != "success" {
		level = "error"
	}
	fields := append([]Field{}, scope.Fields()...)
	fields = append(fields,
		Field{Key: "trace_id", Value: traceID},
		Field{Key: "curl_trace_id", Value: curlTraceID},
		Field{Key: "method", Value: strings.ToUpper(strings.TrimSpace(record.Method))},
		Field{Key: "endpoint", Value: record.Endpoint},
		Field{Key: "request_class", Value: requestClass},
		Field{Key: "status_code", Value: statusCodeValue(record.StatusCode)},
		Field{Key: "duration_s", Value: fmt.Sprintf("%.3f", record.Duration.Seconds())},
		Field{Key: "result", Value: result},
		Field{Key: "outcome_kind", Value: record.OutcomeKind},
		Field{Key: "error_type", Value: record.ErrorType},
		Field{Key: "retry", Value: boolValue(record.Retry)},
		Field{Key: "retry_attempt", Value: strconv.Itoa(retryAttemptValue(record.RetryAttempt))},
		Field{Key: "transport_phase", Value: record.TransportPhase},
		Field{Key: "request_written", Value: boolValue(record.RequestWritten)},
		Field{Key: "target_instance_id", Value: record.TargetInstanceID},
		Field{Key: "target_task_id", Value: record.TargetTaskID},
	)
	at := l.now()
	line := FormatLine(at, level, fields)
	l.router.Bucket(scope, "network.log").WriteLine(line)
	if result != "success" {
		l.router.Bucket(scope, "network-error.log").WriteLine(line)
	}
	l.writeCurlBlock(scope, curlTraceID, record)
}

// retryAttemptValue 统一日志中的尝试序号，避免旧调用方写入零值。
func retryAttemptValue(attempt int) int {
	if attempt < 1 {
		return 1
	}
	return attempt
}

// writeCurlBlock 写入可直接复制重放的请求块；命令与实际发出的请求逐字一致。
func (l *Logger) writeCurlBlock(scope Scope, curlTraceID string, record NetworkRecord) {
	command := strings.TrimSpace(record.Curl)
	if command == "" {
		return
	}
	body := command
	if response := strings.TrimRight(SanitizeBody(record.ResponseBody), "\n"); response != "" {
		body = command + "\n--- response ---\n" + response
	}
	l.router.Bucket(scope, "curl.log").WriteBlock(
		fmt.Sprintf("--- begin curl trace_id=%s ---", SanitizeValue(curlTraceID)),
		body,
		fmt.Sprintf("--- end curl trace_id=%s ---", SanitizeValue(curlTraceID)),
	)
}

// statusCodeValue 把状态码转成字段值；请求未发出时写占位符而不是 0。
func statusCodeValue(status int) string {
	if status <= 0 {
		return ""
	}
	return strconv.Itoa(status)
}

// CurlCommand 生成便于排查的请求命令；敏感字段不进入日志，避免重试记录泄露会话或密码。
func CurlCommand(method, url string, headers map[string]string, body string) string {
	parts := []string{"curl", "-sS", "-X", strings.ToUpper(strings.TrimSpace(method)), shellQuote(SanitizeURL(url))}
	safeHeaders := SanitizeHeaders(headers)
	keys := make([]string, 0, len(safeHeaders))
	for key := range safeHeaders {
		keys = append(keys, key)
	}
	sortStrings(keys)
	for _, key := range keys {
		parts = append(parts, "-H", shellQuote(key+": "+safeHeaders[key]))
	}
	if safeBody := SanitizeBody(body); strings.TrimSpace(safeBody) != "" {
		parts = append(parts, "--data-raw", shellQuote(safeBody))
	}
	return strings.Join(parts, " ")
}

var sensitiveLogKeys = map[string]struct{}{
	"sid": {}, "password": {}, "aeskey": {}, "loginpassword": {}, "logincode": {},
	"authorization": {}, "cookie": {}, "set-cookie": {}, "token": {}, "accesstoken": {}, "refreshtoken": {},
}

// SanitizeURL 清理 URL 查询参数中的会话、令牌等敏感值；解析失败时返回固定占位符，避免泄露原始 URL。
func SanitizeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[REDACTED_URL]"
	}
	query := parsed.Query()
	for key := range query {
		if isSensitiveLogKey(key) {
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// SanitizeHeaders 清理 HTTP 头中的会话和认证值，返回独立副本以免修改实际请求头。
func SanitizeHeaders(headers map[string]string) map[string]string {
	result := make(map[string]string, len(headers))
	for key, value := range headers {
		if isSensitiveLogKey(key) {
			result[key] = "[REDACTED]"
			continue
		}
		result[key] = value
	}
	return result
}

// SanitizeBody 清理 JSON 正文中的敏感字段；非 JSON 正文只在明确含敏感键时整体替换。
func SanitizeBody(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err == nil {
		return string(mustMarshalSanitizedJSON(value))
	}
	lower := strings.ToLower(trimmed)
	for key := range sensitiveLogKeys {
		if strings.Contains(lower, `"`+key+`"`) || strings.Contains(lower, key+"=") {
			return "[REDACTED]"
		}
	}
	return raw
}

// mustMarshalSanitizedJSON 递归替换 JSON 值中的敏感字段；序列化失败时返回固定占位符。
func mustMarshalSanitizedJSON(value any) []byte {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if isSensitiveLogKey(key) {
				result[key] = "[REDACTED]"
				continue
			}
			result[key] = json.RawMessage(mustMarshalSanitizedJSON(child))
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return []byte(`"[REDACTED]"`)
		}
		return encoded
	case []any:
		result := make([]json.RawMessage, len(typed))
		for index, child := range typed {
			result[index] = json.RawMessage(mustMarshalSanitizedJSON(child))
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return []byte(`"[REDACTED]"`)
		}
		return encoded
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return []byte(`"[REDACTED]"`)
		}
		return encoded
	}
}

// isSensitiveLogKey 用大小写不敏感的键名判断日志字段是否包含凭证或会话值。
func isSensitiveLogKey(key string) bool {
	_, ok := sensitiveLogKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// shellQuote 用单引号包裹参数，内部单引号按 POSIX 规则转义，保证可直接粘贴执行。
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// sortStrings 对头部键做稳定排序，让同一请求每次生成的命令完全一致。
func sortStrings(values []string) {
	for outer := 1; outer < len(values); outer++ {
		for inner := outer; inner > 0 && values[inner] < values[inner-1]; inner-- {
			values[inner], values[inner-1] = values[inner-1], values[inner]
		}
	}
}
