package step

import (
	"fmt"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// StepLog 把一步七阶段的决策流水写进 step.log，每阶段一行，阶段名即 phase=。
// 行内贯穿 run_id/path_run_id/step_id 关联键，使日志能查回运行记录（纲领第 6.3、7.3 节）。
type StepLog struct {
	writer *logging.Writer
	scope  logging.Scope
	now    func() time.Time
	// relativePath 是 step.log 相对日志根的路径，落进尝试记录实现记录到日志的可达。
	relativePath string
}

// NewStepLog 绑定一个运行目录下的 step.log 写入器；scope 至少携带计划/路径/运行三层身份。
func NewStepLog(writer *logging.Writer, scope logging.Scope, now func() time.Time) *StepLog {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &StepLog{writer: writer, scope: scope, now: now}
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

// Phase 写一行阶段流水，返回行号（落进尝试记录，实现记录到日志的可达）。
// stepNo 从 1 计；attempt 从 1 计；本切片每步只有一次尝试，字段仍如实落盘。
func (l *StepLog) Phase(phase string, stepNo, attempt int, message string) uint64 {
	if l == nil || l.writer == nil {
		return 0
	}
	scope := l.scope
	scope.StepID = fmt.Sprintf("%d", stepNo)
	scope.Attempt = fmt.Sprintf("%d", attempt)
	scope.Phase = phase
	line := logging.FormatLine(l.now(), "info", append(scope.Fields(),
		logging.Field{Key: "message", Value: message},
	))
	lineNo := l.writer.WriteLine(line)
	if lineNo < 0 {
		return 0
	}
	return uint64(lineNo)
}
