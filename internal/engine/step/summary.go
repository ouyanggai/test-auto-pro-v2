package step

import (
	"encoding/json"
	"strconv"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// gateSummary 生成门禁复验的中文一行摘要。
func gateSummary(item model.ActionCatalogItem, allowed bool) string {
	if allowed {
		return "放行条件已满足：" + item.Label
	}
	reason := item.DisabledReason
	if reason == "" {
		reason = "放行条件不满足"
	}
	return "放行条件不满足：" + reason
}

// submitSummary 生成发送阶段的中文一行摘要；目标拒绝时直接展示目标返回的错误原文。
func submitSummary(response target.WriteResponse, writeErr error, traceID string, durationMs int64) string {
	if writeErr != nil {
		return "执行失败：" + firstResultMessage(response, writeErr, "目标接口没有返回错误信息")
	}
	return "目标接口返回成功"
}

// settleSummary 生成落账阶段直接给用户看的结果摘要。
func settleSummary(result verdict.Verdict, message string) string {
	if message != "" {
		return message
	}
	switch result.Outcome {
	case verdict.OutcomeSucceeded:
		return "执行成功"
	case verdict.OutcomeFailed:
		return "执行失败"
	default:
		return "执行结果暂时无法确认"
	}
}

// verdictChinese 返回运行日志使用的中文结果名。
func verdictChinese(outcome verdict.Outcome) string {
	switch outcome {
	case verdict.OutcomeSucceeded:
		return "执行成功"
	case verdict.OutcomeFailed:
		return "执行失败"
	default:
		return "结果待确认"
	}
}

// actionSuccessMessage 生成每个动作执行成功后的中文结果，让界面直接说明发生了什么。
func actionSuccessMessage(action model.ActionKey) string {
	switch action {
	case model.ActionSaveDraft:
		return "执行成功：草稿已保存"
	case model.ActionSubmit:
		return "执行成功：流程已提交"
	case model.ActionResubmit:
		return "执行成功：流程已重新提交"
	case model.ActionStorageFormData:
		return "执行成功：表单已暂存"
	case model.ActionAddSign:
		return "执行成功：已完成加签"
	case model.ActionTransfer:
		return "执行成功：待办已移交"
	case model.ActionApprove:
		return "执行成功：已同意当前待办"
	case model.ActionReject:
		return "执行成功：已不同意当前待办"
	case model.ActionRollback:
		return "执行成功：已回退到上一节点"
	case model.ActionRetrieve:
		return "执行成功：已取回当前待办"
	case model.ActionWithdraw:
		return "执行成功：流程已撤回"
	case model.ActionUrge:
		return "执行成功：已发送催办"
	case model.ActionForward:
		return "执行成功：已完成转发"
	case model.ActionFollow:
		return "执行成功：已关注流程"
	case model.ActionUnfollow:
		return "执行成功：已取消关注"
	default:
		return "执行成功"
	}
}

// userResultMessage 把内部判定结果转换为页面与路径事件使用的人话。
// 成功只展示动作结果；失败优先展示目标接口原文；无法确认时展示实际读取或传输错误。
func userResultMessage(action model.ActionKey, result verdict.Verdict, response target.WriteResponse, writeErr error, after InstanceFacts) string {
	switch result.Outcome {
	case verdict.OutcomeSucceeded:
		return actionSuccessMessage(action)
	case verdict.OutcomeFailed:
		return "执行失败：" + firstResultMessage(response, writeErr, "目标接口没有返回错误信息")
	default:
		// 目标已经明确返回业务错误时，优先展示目标原文；不能因为后续读取失败把真实拒绝原因覆盖掉。
		if writeErr != nil || (response.IsSuccessPresent && !response.IsSuccess) {
			return "执行失败：" + firstResultMessage(response, writeErr, "目标接口没有返回错误信息")
		}
		if after.ReadError != "" {
			if writeErr == nil && response.IsSuccess {
				return "目标接口已返回成功，但执行结果暂时无法确认：" + after.ReadError
			}
			return "执行失败：" + after.ReadError
		}
		if writeErr != nil {
			return "执行失败：" + firstResultMessage(response, writeErr, "目标接口没有返回错误信息")
		}
		return "目标接口已返回成功，但执行结果暂时无法确认"
	}
}

// firstResultMessage 从目标响应、错误链中提取第一条可读原文，避免把内部分类名展示给用户。
func firstResultMessage(response target.WriteResponse, writeErr error, fallback string) string {
	if message := userFacingError(writeErr, response); message != "" {
		return message
	}
	return fallback
}

// userFacingError 提取目标响应中的原始 message/code 或错误链中的目标原文，避免把内部分类名展示给用户。
func userFacingError(err error, response target.WriteResponse) string {
	if message := target.UserFacingErrorMessage(response, err); message != "" {
		return message
	}
	if err != nil {
		return err.Error()
	}
	return ""
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
