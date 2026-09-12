package step

import (
	"fmt"
	"strings"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// StepLog 把一步七阶段的决策流水写进 step.log，每阶段一行，阶段名即 phase=。
// 行内贯穿 run_id/path_run_id/step_id 关联键，使日志能查回运行记录（纲领第 6.3、7.3 节）；
// 写请求返回后补充 trace_id/curl_trace_id，让 submit 之后的阶段行能与 network.log、curl.log 按链路互查。
type StepLog struct {
	writer *logging.Writer
	scope  logging.Scope
	now    func() time.Time
	// relativePath 是 step.log 相对日志根的路径，落进尝试记录实现记录到日志的可达。
	relativePath string
	// onInstanceScope 由装配层注入：实例身份补进作用域后同步更新运行目录的 meta.json。
	// 目录本身不随实例名称变化，只补字段（F-031/T05）。
	onInstanceScope func(scope logging.Scope)
	// traceID 与 curlTraceID 由发送阶段回填；写请求发出前不存在链路 ID，保持为空。
	traceID     string
	curlTraceID string
}

// NewStepLog 绑定一个运行目录下的 step.log 写入器；scope 至少携带计划/路径/运行三层身份。
// 时间默认取本地时钟：同一运行目录里的 network.log、curl.log 都用本地时间，
// step.log 若单独用 UTC 会造成同目录时间戳相差时区倍数，按时间对照日志会错位。
func NewStepLog(writer *logging.Writer, scope logging.Scope, now func() time.Time) *StepLog {
	if now == nil {
		now = func() time.Time { return time.Now() }
	}
	return &StepLog{writer: writer, scope: scope, now: now}
}

// SetInstanceScopeUpdater 注入实例身份变化时的通知回调（由装配层接入运行目录 meta.json）。
func (l *StepLog) SetInstanceScopeUpdater(update func(scope logging.Scope)) {
	if l == nil {
		return
	}
	l.onInstanceScope = update
}

// SetInstance 把目标实例身份补进本步日志作用域，后续阶段行与 network.log 因此带上同一实例，
// 并通知装配层同步运行目录的 meta.json；名称缺失时只记录「不可用」，不冒充任何名称。
// 目录键（runId/pathRunId）不受影响：实例名称变化不会造成日志搬迁。
func (l *StepLog) SetInstance(instanceID, instanceName, note string) {
	if l == nil {
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	l.scope.InstanceID = instanceID
	if name := strings.TrimSpace(instanceName); name != "" {
		l.scope.InstanceName = name
		l.scope.InstanceNameAvailable = true
		l.scope.InstanceNameNote = ""
	} else if strings.TrimSpace(l.scope.InstanceName) == "" {
		l.scope.InstanceNameAvailable = false
		l.scope.InstanceNameNote = strings.TrimSpace(note)
	}
	if l.onInstanceScope != nil {
		l.onInstanceScope(l.scope)
	}
}

// SetRelativePath 记录 step.log 相对日志根的路径（由装配层按运行目录计算）。
func (l *StepLog) SetRelativePath(path string) {
	if l == nil {
		return
	}
	l.relativePath = path
}

// RelativePath 返回 step.log 相对日志根的路径；未设置时返回空串。
func (l *StepLog) RelativePath() string {
	if l == nil {
		return ""
	}
	return l.relativePath
}

// SetTraceID 回填本次写请求的链路 ID：trace_id 与 curl_trace_id 同源（尝试事实两列同值）。
// 只对回填之后的阶段行生效——写请求发出前的阶段本来就没有链路可关联。
func (l *StepLog) SetTraceID(traceID string) {
	if l == nil {
		return
	}
	l.traceID = traceID
	l.curlTraceID = traceID
}

// Phase 写一行阶段流水，返回行号（落进尝试记录，实现记录到日志的可达）。
// stepNo 从 1 计；attempt 从 1 计；重放尝试如实递增，阶段时间轴按 step_id:attempt 归组。
func (l *StepLog) Phase(phase string, stepNo, attempt int, message string) uint64 {
	if l == nil || l.writer == nil {
		return 0
	}
	scope := l.scope
	scope.StepID = fmt.Sprintf("%d", stepNo)
	scope.Attempt = fmt.Sprintf("%d", attempt)
	scope.Phase = phase
	fields := append(scope.Fields(),
		logging.Field{Key: "message", Value: message},
	)
	if l.traceID != "" {
		fields = append(fields,
			logging.Field{Key: "trace_id", Value: l.traceID},
			logging.Field{Key: "curl_trace_id", Value: l.curlTraceID},
		)
	}
	line := logging.FormatLine(l.now(), "info", fields)
	lineNo := l.writer.WriteLine(line)
	if lineNo < 0 {
		return 0
	}
	return uint64(lineNo)
}
