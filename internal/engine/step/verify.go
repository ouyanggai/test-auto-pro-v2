package step

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// readInstanceFacts 重读目标事实：实例状态、当前节点与演员待办。
// 读取属只读阶段，允许有界重试；失败时在快照里如实记录 ReadError 并返回错误，不伪造事实。
func (e *Executor) readInstanceFacts(ctx context.Context, session target.Session, instanceID, dueNodeKey string) (InstanceFacts, error) {
	facts := InstanceFacts{StepNodeKey: dueNodeKey}
	if instanceID == "" {
		// 发起前实例不存在：这是确定事实，不是读取失败。
		return facts, nil
	}
	_, currentNodes, status, _, found, err := e.target.FindSubmittedFlow(ctx, session, instanceID)
	if err != nil {
		facts.ReadError = err.Error()
		return facts, err
	}
	if !found {
		// 实例在已发列表不可见：发起成功前是常态；发起后不可见属于异常事实，由判定规则处理。
		return facts, nil
	}
	facts.Found = true
	facts.Status = status
	facts.CurrentNodes = currentNodes
	_, dueNodes, _, _, err := e.target.FindDueFlow(ctx, session, instanceID)
	if err != nil {
		facts.ReadError = err.Error()
		return facts, err
	}
	facts.DueNodes = dueNodes
	return facts, nil
}

// ClassifyReread 把前后两次事实对照为判定包的重读四值（纲领第 7.4 节：只依据事实）。
// 判定口径：
//   - 发起：之前实例不存在，之后存在且处于运行/待发即「已前进」；仍不存在即「明确未变」；
//     落成草稿或终态与发起语义矛盾即「自相矛盾」；
//   - 审批：本步待办仍在即「明确未变」；待办消失且实例未出现撤回类回退状态即「已前进」。
func ClassifyReread(action string, stepNodeKey string, before, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	if action == string(model.ActionSubmit) {
		if !after.Found {
			return verdict.RereadUnchanged
		}
		switch after.Status {
		case "run", "await_sent":
			return verdict.RereadAdvanced
		case "draft":
			// 发起非草稿却落成草稿，与动作语义矛盾。
			return verdict.RereadContradictory
		default:
			// 其余状态（撤回/终止/放弃/驳回/结束）都不是发起动作应有的事实。
			return verdict.RereadContradictory
		}
	}
	// 审批：本步节点的待办仍在，说明写未生效。
	for _, node := range after.DueNodes {
		if node == stepNodeKey {
			return verdict.RereadUnchanged
		}
	}
	if after.Found {
		switch after.Status {
		case "withdraw", "termination", "abandon", "rejected":
			// 同意动作不可能造成撤回类状态，事实与动作矛盾。
			return verdict.RereadContradictory
		}
	}
	// 待办已消失：无论实例推进到下一节点还是直接结束，写都已生效。
	return verdict.RereadAdvanced
}

// buildObservation 组装判定包的五项输入：动作与端点、传输结论、HTTP 状态码、响应包、重读结论。
// 全部来自传输层事实与目标重读事实，不靠错误文案推断（F-014 第 1.6 节）。
func buildObservation(endpoint string, writeErr error, response target.WriteResponse, reread verdict.Reread) verdict.Observation {
	transport := targetTransportOf(writeErr)
	observation := verdict.Observation{
		Endpoint:   endpoint,
		Transport:  transport,
		StatusCode: response.StatusCode,
		Reread:     reread,
	}
	if response.IsSuccessPresent {
		observation.Response = &verdict.Response{
			IsSuccess:        response.IsSuccess,
			IsSuccessPresent: true,
			Code:             response.Code,
			Message:          response.Message,
		}
	} else if transport == verdict.TransportResponded {
		// 收到完整响应但成功判据不可解析：按不可解析响应交给判定包，不允许乐观归档。
		observation.Response = &verdict.Response{Unparsable: true}
	}
	return observation
}

// targetTransportOf 把适配层的传输阶段事实转换为判定包的传输枚举；两侧取值一一对应。
// 目标业务拒绝（isSuccess=false 的完整响应）在传输层属于 responded，判定按响应侧初判走。
func targetTransportOf(err error) verdict.Transport {
	var rejection *target.BusinessRejection
	if errors.As(err, &rejection) {
		return verdict.TransportResponded
	}
	switch target.TransportOf(err) {
	case target.TransportResponded:
		return verdict.TransportResponded
	case target.TransportConnectFailed:
		return verdict.TransportConnectRefused
	case target.TransportInterrupted:
		return verdict.TransportInterrupted
	default:
		return verdict.TransportUnclassified
	}
}
