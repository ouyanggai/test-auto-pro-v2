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

// unsafePathChars 匹配不允许出现在目录名里的字符：路径分隔符、控制字符与常见文件系统保留符号。
// 中文、括号、普通横线与空格必须原样保留，目录名要能直接对上界面里的计划名与执行路径名。
var unsafePathChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f\x7f]+`)

const (
	// applicationDirName 是应用程序日志子树，只放启动停止与无法归属业务对象的系统级事件。
	applicationDirName = "application"
	// plansDirName 是计划业务日志子树，下面按计划、配置或运行、执行路径逐层分开。
	plansDirName = "plans"
	// configurationDirName 是某个计划的配置阶段日志子树。
	configurationDirName = "configuration"
	// runsDirName 是某个计划的执行阶段日志子树。
	runsDirName = "runs"
	// planLevelDirName 用于已知计划但还不知道执行路径的计划级操作。
	planLevelDirName = "_plan"
	// unknownSegment 是目录段清洗后为空时的占位，保证路径始终可落盘。
	unknownSegment = "unknown"
)

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

// Scope 是一次操作的业务归属与关联键集合，由 context 携带，日志字段不靠调用方逐处传参。
// 计划与执行路径既进日志字段也决定落盘目录：日志归属看这条日志描述的对象，不看它由哪个模块输出。
type Scope struct {
	// RequestID 是一次 HTTP 请求的标识，一次请求内的所有日志靠它串起来。
	RequestID string
	// PlanID 是计划的不可变主键。只要能确定它，日志就必须落进该计划目录，不允许降级到 application。
	PlanID string
	// PlanName 是计划的真实显示名，只能来自数据库记录，禁止从 URL 猜测。
	PlanName string
	// ExecutionPathID 是执行路径的不可变主键，决定计划目录下的第二层归档。
	ExecutionPathID string
	// ExecutionPathName 是执行路径的真实显示名，同样只能来自数据库记录。
	ExecutionPathName string
	// RunID 与 RunSeq 只在真实执行时出现；RunSeq 是运行目录名。F-013 不创建运行记录也不伪造运行号。
	RunID  string
	RunSeq string
	// PathRunID、StepID、Attempt、Phase 由后续执行器切片填充，配置阶段写占位符。
	PathRunID string
	StepID    string
	Attempt   string
	Phase     string
}

type scopeContextKey struct{}

// WithScope 把作用域合并进 context，供下游日志调用自动读取。
// 必须是合并而不是覆盖：中间件先注入 RequestID，随后才补充计划与执行路径，
// 直接覆盖会把已经注入的 RequestID 等字段丢掉。
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, ScopeFrom(ctx).Merge(scope))
}

// Merge 用后来的作用域补充当前作用域，只接受后者的非空字段，并返回新值不改原值。
func (s Scope) Merge(next Scope) Scope {
	merged := s
	for _, pair := range []struct {
		target *string
		value  string
	}{
		{&merged.RequestID, next.RequestID},
		{&merged.PlanID, next.PlanID},
		{&merged.PlanName, next.PlanName},
		{&merged.ExecutionPathID, next.ExecutionPathID},
		{&merged.ExecutionPathName, next.ExecutionPathName},
		{&merged.RunID, next.RunID},
		{&merged.RunSeq, next.RunSeq},
		{&merged.PathRunID, next.PathRunID},
		{&merged.StepID, next.StepID},
		{&merged.Attempt, next.Attempt},
		{&merged.Phase, next.Phase},
	} {
		if strings.TrimSpace(pair.value) != "" {
			*pair.target = pair.value
		}
	}
	return merged
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
// 业务归属键排在最前，方便按计划或执行路径直接 grep 全部相关日志。
func (s Scope) Fields() []Field {
	return []Field{
		{Key: "plan_id", Value: s.PlanID},
		{Key: "plan_name", Value: s.PlanName},
		{Key: "execution_path_id", Value: s.ExecutionPathID},
		{Key: "execution_path_name", Value: s.ExecutionPathName},
		{Key: "request_id", Value: s.RequestID},
		{Key: "run_id", Value: s.RunID},
		{Key: "path_run_id", Value: s.PathRunID},
		{Key: "step_id", Value: s.StepID},
		{Key: "attempt", Value: s.Attempt},
		{Key: "phase", Value: s.Phase},
	}
}

// HasPlan 判断这条日志能否归属到某个计划；只有确实不能归属时才允许写进 application 目录。
func (s Scope) HasPlan() bool {
	return strings.TrimSpace(s.PlanID) != ""
}

// HasExecutionPath 判断是否已经确定执行路径；只有计划没有路径的操作归入计划级目录。
func (s Scope) HasExecutionPath() bool {
	return strings.TrimSpace(s.ExecutionPathID) != ""
}

// RunFolder 返回运行目录名：优先运行号，其次运行标识；两者都空说明还没有真实运行。
func (s Scope) RunFolder() string {
	if sequence := strings.TrimSpace(s.RunSeq); sequence != "" {
		return sequence
	}
	return strings.TrimSpace(s.RunID)
}

// IsRun 判断作用域是否指向一次真实运行；只有运行作用域才路由到计划的 runs 子树。
func (s Scope) IsRun() bool {
	return s.HasPlan() && s.RunFolder() != ""
}

// PlanDirName 返回计划目录名：中文显示名在前、不可变 ID 在后，避免同名不同 ID 的计划互相覆盖。
func (s Scope) PlanDirName() string {
	return SanitizePathSegment(s.PlanName) + "__plan-" + SanitizePathSegment(s.PlanID)
}

// ExecutionPathDirName 返回执行路径目录名；只知道计划时返回计划级目录名。
func (s Scope) ExecutionPathDirName() string {
	if !s.HasExecutionPath() {
		return planLevelDirName
	}
	return SanitizePathSegment(s.ExecutionPathName) + "__path-" + SanitizePathSegment(s.ExecutionPathID)
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

// SanitizePathSegment 清洗目录段：只替换斜杠、反斜杠、控制字符、文件系统保留符号和路径穿越，
// 中文、括号、普通横线与空格一律保留，让目录名与界面上的计划名、执行路径名一致。
func SanitizePathSegment(value string) string {
	cleaned := unsafePathChars.ReplaceAllString(strings.TrimSpace(value), "_")
	cleaned = strings.ReplaceAll(cleaned, string(filepath.Separator), "_")
	// 连续点号构成路径穿越，压成下划线；单个点号出现在名称中间是正常的。
	for strings.Contains(cleaned, "..") {
		cleaned = strings.ReplaceAll(cleaned, "..", "_")
	}
	cleaned = strings.Trim(cleaned, " .")
	if cleaned == "" {
		return unknownSegment
	}
	if len([]rune(cleaned)) > 80 {
		cleaned = strings.Trim(string([]rune(cleaned)[:80]), " .")
	}
	if cleaned == "" {
		return unknownSegment
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
