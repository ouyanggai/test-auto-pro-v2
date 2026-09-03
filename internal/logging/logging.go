// Package logging 提供 F-013 分层日志底座：日志根解析、作用域注入、统一单行格式、
// 有界写入器与目录路由。所有写入失败都必须降级为一次标准错误输出，绝不影响主流程。
package logging

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// LogRootEnv 覆盖日志根目录，供测试写入临时目录。
	LogRootEnv = "TEST_AUTO_PRO_LOG_ROOT"
	// RetentionDaysEnv 覆盖配置桶与运行目录的保留天数。
	RetentionDaysEnv = "TEST_AUTO_PRO_LOG_RETENTION_DAYS"
	// DefaultRetentionDays 是按天日志文件、配置桶与运行目录的默认保留天数。
	DefaultRetentionDays = 7
	// DefaultMaxFileBytes 是单个日志文件的默认上限。
	DefaultMaxFileBytes int64 = 8 << 20
	// DefaultMaxBackups 是轮转后保留的历史文件个数。
	DefaultMaxBackups = 3
	// missingValue 是字段缺值时的固定占位，保证列对齐可 grep。
	missingValue = "-"
)

// unsafePathSegment 匹配所有不允许直接落盘的目录段字符；目标返回的名称一律先清洗。
var unsafePathSegment = regexp.MustCompile(`[^0-9A-Za-z\p{Han}._-]+`)

// Root 返回日志根目录：优先环境变量，其次工作区根下的 logs。
func Root(workspaceRoot string) string {
	if override := strings.TrimSpace(os.Getenv(LogRootEnv)); override != "" {
		return filepath.Clean(override)
	}
	return filepath.Join(filepath.Clean(workspaceRoot), "logs")
}

// RetentionDays 返回配置桶与运行目录的保留天数，非法取值回落默认值。
func RetentionDays() int {
	value := strings.TrimSpace(os.Getenv(RetentionDaysEnv))
	if value == "" {
		return DefaultRetentionDays
	}
	days := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return DefaultRetentionDays
		}
		days = days*10 + int(char-'0')
		if days > 3650 {
			return DefaultRetentionDays
		}
	}
	if days < 1 {
		return DefaultRetentionDays
	}
	return days
}

// Scope 是一次操作的关联键集合，由 context 携带，日志字段不靠调用方逐处传参。
// 配置阶段只有 RequestID 有值，运行相关键写占位符，等后续执行器切片填充。
type Scope struct {
	RequestID string
	RunID     string
	PathRunID string
	StepID    string
	Attempt   string
	Phase     string
	// PlanName、PathName 与 RunSeq 只用于运行目录路由，不进入日志字段。
	PlanName string
	PathName string
	RunSeq   string
}

type scopeContextKey struct{}

// WithScope 把作用域写入 context，供下游日志调用自动读取。
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

// ScopeFrom 读取 context 中的作用域；没有作用域时返回零值，字段按占位符输出。
func ScopeFrom(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	scope, ok := ctx.Value(scopeContextKey{}).(Scope)
	if !ok {
		return Scope{}
	}
	return scope
}

// Fields 把作用域展开为固定顺序的关联键字段，缺值写占位符。
func (s Scope) Fields() []Field {
	return []Field{
		{Key: "request_id", Value: s.RequestID},
		{Key: "run_id", Value: s.RunID},
		{Key: "path_run_id", Value: s.PathRunID},
		{Key: "step_id", Value: s.StepID},
		{Key: "attempt", Value: s.Attempt},
		{Key: "phase", Value: s.Phase},
	}
}

// IsRun 判断作用域是否指向一次真实运行；只有运行作用域才路由到运行目录。
func (s Scope) IsRun() bool {
	return strings.TrimSpace(s.PlanName) != "" && strings.TrimSpace(s.PathName) != "" && strings.TrimSpace(s.RunSeq) != ""
}

// Field 是一个日志字段；值在格式化时统一清洗。
type Field struct {
	Key   string
	Value string
}

// SanitizeValue 把字段值收敛为单行 token：空格换下划线，换行去掉，空值写占位符。
func SanitizeValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return missingValue
	}
	replaced := strings.NewReplacer("\r", "", "\n", " ", "\t", " ").Replace(trimmed)
	replaced = strings.Join(strings.Fields(replaced), "_")
	if replaced == "" {
		return missingValue
	}
	return replaced
}

// SanitizePathSegment 清洗目录段，拒绝路径穿越并限制长度，绝不把目标原始名称直接落盘。
func SanitizePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, string(filepath.Separator), "_")
	trimmed = strings.ReplaceAll(trimmed, "/", "_")
	cleaned := unsafePathSegment.ReplaceAllString(trimmed, "_")
	cleaned = strings.Trim(cleaned, "._-")
	if cleaned == "" {
		return "unknown"
	}
	if len([]rune(cleaned)) > 64 {
		cleaned = string([]rune(cleaned)[:64])
	}
	return cleaned
}

// FormatLine 生成统一单行格式：time 与 level 固定在前，其余字段按传入顺序输出。
func FormatLine(at time.Time, level string, fields []Field) string {
	builder := strings.Builder{}
	builder.WriteString("time=")
	builder.WriteString(at.Format("2006-01-02_15:04:05"))
	builder.WriteString(" level=")
	builder.WriteString(SanitizeValue(level))
	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		builder.WriteString(" ")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(SanitizeValue(field.Value))
	}
	return builder.String()
}
