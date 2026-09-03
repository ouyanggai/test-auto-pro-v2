package logging

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// 错误分类固定为八类：工具侧四类与目标侧四类。产品原则要求工具问题与目标平台问题分开说明，
// 暂时无法判断时按工具问题处理，因此 ClassUnknown 必须同时写 level=error 并标注按工具问题跟进。
const (
	ClassToolBug        = "tool_bug"
	ClassToolConfig     = "tool_config"
	ClassToolStorage    = "tool_storage"
	ClassToolDependency = "tool_dependency"
	ClassTargetContract = "target_contract"
	ClassTargetRuntime  = "target_runtime"
	ClassNetwork        = "network"
	ClassUnknown        = "unknown"
)

// TargetKindClass 把目标适配层的稳定错误分类映射为日志错误类别。
// 映射表放在日志包里，目标包只传自己的分类字符串，避免包依赖反向。
func TargetKindClass(kind string) string {
	switch strings.TrimSpace(kind) {
	case "login_rejected", "permission_denied":
		return ClassTargetRuntime
	case "session_expired":
		return ClassTargetRuntime
	case "response_invalid":
		return ClassTargetContract
	case "timeout", "unavailable":
		return ClassNetwork
	default:
		return ClassUnknown
	}
}

// ErrorRecord 是一条程序错误日志的全部字段；调用方只填自己能证明的部分。
type ErrorRecord struct {
	Message string
	// Class 为空时按 ClassUnknown 处理，并按工具问题跟进。
	Class string
	Err   error
	// UserMessage 是界面上实际看到的那句中文提示，用于把页面提示与日志对上。
	UserMessage string
	// Stack 只在 panic 恢复时填写，写入时按块包裹并截断。
	Stack string
	// RunTerminated 标记该错误是否终止了一次运行。
	RunTerminated bool
	// Extra 追加调用方自有字段，顺序保持传入顺序。
	Extra []Field
	// SourceSkip 控制 source 定位向上跳过的调用层数。
	SourceSkip int
}

// Logger 是程序日志与程序错误日志的统一入口：全局文件与当前作用域桶内文件双写。
type Logger struct {
	router *Router
	now    func() time.Time
}

// NewLogger 基于目录路由创建日志器；now 可注入以固定测试时间。
func NewLogger(router *Router, now func() time.Time) *Logger {
	if now == nil {
		now = time.Now
	}
	return &Logger{router: router, now: now}
}

// Router 返回底层目录路由，供网络日志等专用写入复用同一份桶目录。
func (l *Logger) Router() *Router {
	if l == nil {
		return nil
	}
	return l.router
}

// Info 记录一条普通程序日志：全局 app.log 与当前桶内 program.log 双写。
func (l *Logger) Info(scope Scope, message string, extra ...Field) {
	l.write(scope, "info", message, extra, false)
}

// Error 记录一条程序错误日志：先写普通日志双写位置，再写错误日志双写位置。
// 错误链、错误类别、来源定位、用户可见提示和 panic 栈都在这里落盘。
func (l *Logger) Error(scope Scope, record ErrorRecord) {
	if l == nil || l.router == nil {
		return
	}
	class := strings.TrimSpace(record.Class)
	if class == "" {
		class = ClassUnknown
	}
	fields := append([]Field{}, scope.Fields()...)
	fields = append(fields,
		Field{Key: "error_class", Value: class},
		Field{Key: "error_chain", Value: ErrorChain(record.Err)},
		Field{Key: "source", Value: callerSource(record.SourceSkip)},
		Field{Key: "run_terminated", Value: boolValue(record.RunTerminated)},
		Field{Key: "user_message", Value: record.UserMessage},
		Field{Key: "message", Value: record.Message},
	)
	if class == ClassUnknown {
		fields = append(fields, Field{Key: "followup", Value: "按工具问题跟进"})
	}
	fields = append(fields, record.Extra...)
	at := l.now()
	line := FormatLine(at, "error", fields)
	for _, writer := range []*Writer{
		l.router.Global("app.log"), l.router.Global("app-error.log"),
		l.router.Bucket(scope, "program.log"), l.router.Bucket(scope, "program-error.log"),
	} {
		writer.WriteLine(line)
	}
	if stack := strings.TrimSpace(record.Stack); stack != "" {
		traceID := SanitizeValue(scope.RequestID)
		block := truncateStack(stack)
		for _, writer := range []*Writer{l.router.Global("app-error.log"), l.router.Bucket(scope, "program-error.log")} {
			writer.WriteBlock(
				fmt.Sprintf("--- begin stack trace_id=%s ---", traceID),
				block,
				fmt.Sprintf("--- end stack trace_id=%s ---", traceID),
			)
		}
	}
}

// write 按级别把一条记录写入全局日志、按日归档与当前桶。
func (l *Logger) write(scope Scope, level, message string, extra []Field, alsoError bool) {
	if l == nil || l.router == nil {
		return
	}
	fields := append([]Field{}, scope.Fields()...)
	fields = append(fields, Field{Key: "message", Value: message})
	fields = append(fields, extra...)
	at := l.now()
	line := FormatLine(at, level, fields)
	writers := []*Writer{
		l.router.Global("app.log"), l.router.Bucket(scope, "program.log"),
	}
	if alsoError {
		writers = append(writers, l.router.Global("app-error.log"), l.router.Bucket(scope, "program-error.log"))
	}
	for _, writer := range writers {
		writer.WriteLine(line)
	}
}

// ErrorChain 展开错误链为稳定的单行说明，最外层在前，最多展开六层。
func ErrorChain(err error) string {
	if err == nil {
		return ""
	}
	parts := make([]string, 0, 6)
	for current := err; current != nil && len(parts) < 6; current = errors.Unwrap(current) {
		parts = append(parts, fmt.Sprintf("%T:%s", current, current.Error()))
	}
	return strings.Join(parts, "<-")
}

// callerSource 返回错误发生位置的文件名与行号，只保留最后两段路径，避免泄露构建机绝对路径。
// 用 runtime.Callers 与 CallersFrames 逐帧向上找第一个不属于日志包的调用点：
// runtime.Caller 在内联后帧序会漂移，CallersFrames 才是受支持的展开方式。
func callerSource(skip int) string {
	// 从最顶层开始展开：内联会让固定 skip 层数失效，只能按包名过滤掉日志包与运行时自身的帧。
	counters := make([]uintptr, 24)
	count := runtime.Callers(1, counters)
	if count == 0 {
		return ""
	}
	frames := runtime.CallersFrames(counters[:count])
	remaining := skip
	for {
		frame, more := frames.Next()
		known := frame.File != "" &&
			!strings.HasPrefix(frame.Function, loggingPackagePrefix) &&
			!strings.HasPrefix(frame.Function, "runtime.")
		if known {
			if remaining > 0 {
				remaining--
			} else {
				return fmt.Sprintf("%s:%d", filepath.Join(filepath.Base(filepath.Dir(frame.File)), filepath.Base(frame.File)), frame.Line)
			}
		}
		if !more {
			return ""
		}
	}
}

// loggingPackagePrefix 用于跳过日志包自身的调用帧。
const loggingPackagePrefix = "test-auto-pro-v2/internal/logging."

// truncateStack 限制 panic 栈长度，避免单条记录把日志文件顶满。
func truncateStack(stack string) string {
	const maxStackBytes = 8 << 10
	if len(stack) <= maxStackBytes {
		return stack
	}
	return stack[:maxStackBytes] + "\n... 栈已截断"
}

// boolValue 把布尔值写成稳定的 true/false，便于 grep。
func boolValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
