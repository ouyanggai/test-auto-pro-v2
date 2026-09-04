package step

import (
	"encoding/json"
	"strconv"

	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// gateSummary 生成门禁复验的中文一行摘要。
func gateSummary(item model.ActionCatalogItem, allowed bool) string {
	if allowed {
		return "门禁复验通过：" + item.Label
	}
	reason := item.DisabledReason
	if reason == "" {
		reason = "门禁未通过"
	}
	return "门禁复验未通过：" + reason
}

// submitSummary 生成发送阶段的中文一行摘要：请求是否发出、关联 trace 与耗时。
func submitSummary(writeErr error, traceID string, durationMs int64) string {
	if writeErr != nil {
		return "写请求发送失败（trace_id=" + traceID + "）：" + writeErr.Error()
	}
	return "写请求已发出（trace_id=" + traceID + "，耗时 " + formatInt64(durationMs) + "ms），等待事实重读"
}

// settleSummary 生成落账阶段的中文一行摘要。
func settleSummary(result verdict.Verdict) string {
	switch result.Outcome {
	case verdict.OutcomeSucceeded:
		return "结论确定成功：" + result.Reason
	case verdict.OutcomeFailed:
		return "结论确定失败：" + result.Reason
	default:
		return "结论不确定：" + result.Reason
	}
}

// verdictChinese 返回三值结论的中文显示名。
func verdictChinese(outcome verdict.Outcome) string {
	switch outcome {
	case verdict.OutcomeSucceeded:
		return "确定成功"
	case verdict.OutcomeFailed:
		return "确定失败"
	default:
		return "不确定"
	}
}

// statusOfVerdict 把三值结论映射为步骤事实状态。
func statusOfVerdict(outcome verdict.Outcome) model.RunStepStatus {
	switch outcome {
	case verdict.OutcomeSucceeded:
		return model.RunStepSucceeded
	case verdict.OutcomeFailed:
		return model.RunStepFailed
	default:
		return model.RunStepUncertain
	}
}

// runResultOf 返回路径结果的指针形态，便于终态落库。
func runResultOf(result model.RunResult) *model.RunResult {
	return &result
}

// previewJSON 把载荷渲染为预览 JSON 文本；渲染失败时给中文占位，不阻塞预览。
func previewJSON(payload map[string]any) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "（载荷无法序列化为预览，请查看日志）"
	}
	return string(encoded)
}

// formatInt64 输出整数十进制文本。
func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
