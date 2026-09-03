package logging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NetworkRecord 是一次目标平台请求的日志事实；调用方只填自己能证明的字段。
// 本工具当前只发只读请求，RequestClass 固定为 read，写请求白名单为空。
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
	// Curl 是可直接复制重放的完整命令，含真实会话值与完整请求正文。
	Curl string
	// ResponseBody 是目标返回的完整响应正文，只进 curl.log 的块内。
	ResponseBody string
	// Retry 标记该次请求是否是受控重试。
	Retry bool
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

// writeCurlBlock 写入可直接复制重放的请求块；命令与实际发出的请求逐字一致。
func (l *Logger) writeCurlBlock(scope Scope, curlTraceID string, record NetworkRecord) {
	command := strings.TrimSpace(record.Curl)
	if command == "" {
		return
	}
	body := command
	if response := strings.TrimRight(record.ResponseBody, "\n"); response != "" {
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

// CurlCommand 生成与实际请求逐字一致的可重放命令。
// 内网裁决要求含真实会话值与完整正文，因此不做任何脱敏。
func CurlCommand(method, url string, headers map[string]string, body string) string {
	parts := []string{"curl", "-sS", "-X", strings.ToUpper(strings.TrimSpace(method)), shellQuote(url)}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sortStrings(keys)
	for _, key := range keys {
		parts = append(parts, "-H", shellQuote(key+": "+headers[key]))
	}
	if strings.TrimSpace(body) != "" {
		parts = append(parts, "--data-raw", shellQuote(body))
	}
	return strings.Join(parts, " ")
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
