package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// requestLogger 是中间件依赖的最小日志入口，便于测试注入假日志器。
type requestLogger interface {
	Info(scope logging.Scope, message string, extra ...logging.Field)
	Error(scope logging.Scope, record logging.ErrorRecord)
}

// WithRequestLogging 包装现有 handler：生成请求标识并注入作用域、记录请求与响应、
// 把失败响应实际写出的稳定错误码与中文文案落进程序错误日志、恢复 panic 并返回稳定中文 500。
// 不改 writeFailure 签名，也不加长构造链，只在组装处包一层。
func WithRequestLogging(next http.Handler, logger requestLogger) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		scope := logging.Scope{RequestID: logging.NewTraceID()}
		ctx := logging.WithScope(request.Context(), scope)
		recorder := &failureCapturingWriter{ResponseWriter: response}
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(scope, logging.ErrorRecord{
					Message:     "请求处理发生未预期错误",
					Class:       logging.ClassToolBug,
					Err:         panicError(recovered),
					UserMessage: panicUserMessage,
					Stack:       string(debug.Stack()),
					Extra: []logging.Field{
						{Key: "method", Value: request.Method},
						{Key: "route", Value: request.URL.Path},
					},
				})
				if !recorder.wroteHeader {
					// 栈只进日志，响应体只给用户一句稳定中文提示。
					writeFailure(recorder, http.StatusInternalServerError, "TOOL_INTERNAL_ERROR", panicUserMessage, true)
				}
			}
			fields := []logging.Field{
				{Key: "method", Value: request.Method},
				{Key: "route", Value: request.URL.Path},
				{Key: "status_code", Value: statusText(recorder.status)},
				{Key: "duration_s", Value: durationText(time.Since(started))},
				{Key: "error_code", Value: recorder.failureCode},
			}
			logger.Info(scope, "接口请求已处理", fields...)
			if recorder.failureCode != "" {
				logger.Error(scope, logging.ErrorRecord{
					Message:     "接口返回失败响应",
					Class:       failureClass(recorder.status),
					UserMessage: recorder.failureMessage,
					Extra:       fields,
				})
			}
		}()
		next.ServeHTTP(recorder, request.WithContext(ctx))
	})
}

// panicUserMessage 是 panic 恢复后返回给用户的稳定中文提示，与日志里的 user_message 同源。
const panicUserMessage = "工具内部发生错误，请重试或联系维护"

// failureCapturingWriter 记录实际写出的状态码，并在失败响应里取出稳定错误码与中文文案。
type failureCapturingWriter struct {
	http.ResponseWriter
	status         int
	wroteHeader    bool
	failureCode    string
	failureMessage string
	buffer         bytes.Buffer
	captureBody    bool
}

// WriteHeader 记录状态码，并决定是否需要抓取响应体来取错误码与提示。
func (w *failureCapturingWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status, w.wroteHeader = status, true
	w.captureBody = status >= 400
	w.ResponseWriter.WriteHeader(status)
}

// Write 透传响应体，失败响应额外抓取有界副本用于解析稳定错误码与中文文案。
func (w *failureCapturingWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.captureBody && w.buffer.Len() < 8192 {
		w.buffer.Write(data)
		w.parseFailure()
	}
	return w.ResponseWriter.Write(data)
}

// Flush 透传 flush，保持流式响应行为不变。
func (w *failureCapturingWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// parseFailure 解析统一失败包络，取出界面上实际显示的那句中文提示。
func (w *failureCapturingWriter) parseFailure() {
	var failure struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.buffer.Bytes(), &failure); err != nil {
		return
	}
	if strings.TrimSpace(failure.Error.Code) == "" {
		return
	}
	w.failureCode, w.failureMessage = failure.Error.Code, failure.Error.Message
}

// failureClass 按响应状态把失败归入工具侧或目标侧类别，无法判断时按工具问题跟进。
func failureClass(status int) string {
	switch {
	case status == http.StatusBadGateway || status == http.StatusGatewayTimeout:
		return logging.ClassNetwork
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return logging.ClassTargetRuntime
	case status == http.StatusServiceUnavailable:
		return logging.ClassToolDependency
	case status >= 500:
		return logging.ClassToolBug
	case status >= 400:
		return logging.ClassToolConfig
	default:
		return logging.ClassUnknown
	}
}

// panicError 把 recover 得到的任意值收敛为 error，供错误链展开。
func panicError(recovered any) error {
	if err, ok := recovered.(error); ok {
		return err
	}
	return &recoveredPanic{value: recovered}
}

// recoveredPanic 承载非 error 类型的 panic 值。
type recoveredPanic struct{ value any }

// Error 返回 panic 值的稳定单行说明。
func (e *recoveredPanic) Error() string {
	return strings.TrimSpace(strings.ReplaceAll(sprint(e.value), "\n", " "))
}

// sprint 只用于把 panic 值转成字符串，避免在错误路径上再引入格式化依赖分支。
func sprint(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "panic"
	}
	return string(encoded)
}

// statusText 把状态码转成字段值，未写出响应时留空由日志层写占位符。
func statusText(status int) string {
	if status <= 0 {
		return ""
	}
	return itoa(status)
}

// durationText 输出秒级耗时，保留三位小数，与网络日志同格式。
func durationText(elapsed time.Duration) string {
	seconds := elapsed.Seconds()
	whole := int(seconds)
	milli := int((seconds - float64(whole)) * 1000)
	if milli < 0 {
		milli = 0
	}
	return itoa(whole) + "." + pad3(milli)
}

// itoa 是不依赖 strconv 的最小整数转换，保持中间件依赖面最小。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// pad3 把毫秒补齐为三位，保证耗时字段列宽稳定。
func pad3(value int) string {
	text := itoa(value)
	for len(text) < 3 {
		text = "0" + text
	}
	return text
}
