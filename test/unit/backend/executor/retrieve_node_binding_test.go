package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 夜间全矩阵真实运行缺陷回归（路径 9 取回「空指针异常」）：
// 实例经历加签（updateFlowProxy 重建实例私有代理）后，旧节点 ID 可能失效；
// 已办任务读取在按节点查空时曾"撇开节点"回退到整实例唯一任务，
// 结果把加签节点（2843705d…）的 done 任务绑到了取回请求的 jobTaskId 上，
// 目标对该节点返回「空指针异常」，整条路径停止。
//
// 修复纪律：done（已办）与节点强绑定，绝不撇开节点回退匹配；
// pending（待办）保留原有的代理重建回退，不改变既有已通过行为。

// nodeBoundRetrieveTarget 记录每次任务快照读取的节点过滤条件，用于验证读取口径。
type nodeBoundRetrieveTarget struct {
	*fakeTarget
	// doneSnapshots 按节点 ID 返回已办任务；空 nodeID 表示"撇开节点"的整实例读取。
	doneSnapshots map[string]target.TaskSnapshot
	// reads 按调用顺序记录 (nodeProxyID, taskStatus)。
	reads [][2]string
}

// FindTaskSnapshot 按 (节点, 状态) 返回任务，并记录实际读取口径。
// 这里刻意让 nodeID 为空时返回别的节点的任务，复现旧回退把错节点已办绑上取回的缺陷。
func (t *nodeBoundRetrieveTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _ string, nodeProxyID, taskStatus string) (target.TaskSnapshot, error) {
	t.reads = append(t.reads, [2]string{nodeProxyID, taskStatus})
	if taskStatus != "done" {
		return target.TaskSnapshot{}, nil
	}
	if snapshot, ok := t.doneSnapshots[nodeProxyID]; ok {
		return snapshot, nil
	}
	return target.TaskSnapshot{}, nil
}

// TestReadTaskSnapshotKeepsNodeBindingForDone 锁定：已办读取必须始终带本步节点过滤，
// 代理重建后也不得回退到"整实例唯一任务"，否则取回会绑上其他节点的已办任务。
func TestReadTaskSnapshotKeepsNodeBindingForDone(t *testing.T) {
	targetFake := &nodeBoundRetrieveTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{
			Found: true, Status: "run", CurrentNodes: []string{"node-other"}, DueNodes: []string{"node-other"},
		}},
		doneSnapshots: map[string]target.TaskSnapshot{
			// 本步节点上没有已办。
			"node-step": {},
			// 整实例"撇开节点"读取会命中的、属于其他节点的已办任务。
			"": {LinkID: "other-link", JobTaskID: "other-task", FlowNodeProxyID: "node-other", TaskStatus: "done"},
		},
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	// 模拟加签后的代理重建：这正是旧实现允许回退的条件。
	runCtx.FlowProxyRemapped = true
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-step"}

	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	if _, _, err := executor.BuildPreview(context.Background(), runCtx, 0); err != nil {
		t.Fatalf("取回预览失败：%v", err)
	}

	for _, read := range targetFake.reads {
		if read[1] == "done" && read[0] == "" {
			t.Fatalf("已办读取不得撇开节点回退（实际读取口径 %v），否则会绑上其他节点的已办任务", targetFake.reads)
		}
	}
}
