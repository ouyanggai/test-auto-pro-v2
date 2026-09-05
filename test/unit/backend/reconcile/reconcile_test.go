package reconcile_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/reconcile"
)

// dims 构造五维证据的便捷结构。
type dims map[reconcile.Dimension]reconcile.DimensionEvidence

func allUnchanged() dims {
	return dims{
		reconcile.DimInstanceStatus: {State: reconcile.DimUnchanged, Note: "状态一致"},
		reconcile.DimCurrentNode:    {State: reconcile.DimUnchanged, Note: "节点一致"},
		reconcile.DimCurrentTask:    {State: reconcile.DimUnchanged, Note: "待办仍在"},
		reconcile.DimDoneRecords:    {State: reconcile.DimUnchanged, Note: "已办无痕迹"},
		reconcile.DimActionTraces:   {State: reconcile.DimUnchanged, Note: "无动作痕迹"},
	}
}

func allChanged() dims {
	return dims{
		reconcile.DimInstanceStatus: {State: reconcile.DimChanged, Note: "状态已推进"},
		reconcile.DimCurrentNode:    {State: reconcile.DimChanged, Note: "节点已前进"},
		reconcile.DimCurrentTask:    {State: reconcile.DimChanged, Note: "待办已消失"},
		reconcile.DimDoneRecords:    {State: reconcile.DimChanged, Note: "已办出现本次动作"},
		reconcile.DimActionTraces:   {State: reconcile.DimChanged, Note: "动作痕迹存在"},
	}
}

// TestF018StrongEvidenceNotEffective 五维全部明确未变才判未生效，唯一动作是重放。
func TestF018StrongEvidenceNotEffective(t *testing.T) {
	result := reconcile.Reconcile(reconcile.Input{Dims: allUnchanged()})
	if result.Verdict != reconcile.VerdictNotEffective || result.Action != reconcile.ActionReplay {
		t.Fatalf("五维未变应判未生效且唯一动作是重放：%+v", result)
	}
}

// TestF018EffectiveRequiresAllChanged 五维全部变化才判已生效，唯一动作是确认前进。
func TestF018EffectiveRequiresAllChanged(t *testing.T) {
	result := reconcile.Reconcile(reconcile.Input{Dims: allChanged()})
	if result.Verdict != reconcile.VerdictEffective || result.Action != reconcile.ActionAdvance {
		t.Fatalf("五维全部变化应判已生效：%+v", result)
	}
}

// TestF018MissingDimensionDegrades 任一维度缺失即降级为仍无法判定，绝不当成未变化。
func TestF018MissingDimensionDegrades(t *testing.T) {
	for _, dim := range []reconcile.Dimension{
		reconcile.DimInstanceStatus, reconcile.DimCurrentNode, reconcile.DimCurrentTask,
		reconcile.DimDoneRecords, reconcile.DimActionTraces,
	} {
		evidence := allUnchanged()
		evidence[dim] = reconcile.DimensionEvidence{State: reconcile.DimMissing, Note: "读取失败"}
		result := reconcile.Reconcile(reconcile.Input{Dims: evidence})
		if result.Verdict != reconcile.VerdictIndeterminate || result.Action != reconcile.ActionManualEnd {
			t.Fatalf("维度 %s 缺失必须降级为仍无法判定：%+v", dim, result)
		}
		if len(result.Reasons) == 0 {
			t.Fatalf("降级必须逐条列出理由：%+v", result)
		}
	}
}

// TestF018ConflictDegrades 维度读数互相矛盾同样降级。
func TestF018ConflictDegrades(t *testing.T) {
	evidence := allUnchanged()
	evidence[reconcile.DimCurrentNode] = reconcile.DimensionEvidence{State: reconcile.DimConflict, Note: "两个接口给出不同节点"}
	result := reconcile.Reconcile(reconcile.Input{Dims: evidence})
	if result.Verdict != reconcile.VerdictIndeterminate {
		t.Fatalf("矛盾读数必须降级：%+v", result)
	}
}

// TestF018PartialEffectNeverReplays 部分生效固定判仍无法判定，绝不允许重放（语义第 2.4 节）。
func TestF018PartialEffectNeverReplays(t *testing.T) {
	result := reconcile.Reconcile(reconcile.Input{
		Dims:              allChanged(),
		PartialEffect:     true,
		PartialEffectNote: "表单数据已变化但流程未推进",
	})
	if result.Verdict != reconcile.VerdictIndeterminate || result.Action != reconcile.ActionManualEnd {
		t.Fatalf("部分生效必须固定为仍无法判定+登记人工结论：%+v", result)
	}
	if len(result.Reasons) == 0 || result.Headline == "" {
		t.Fatalf("部分生效必须有中文说明：%+v", result)
	}
}

// TestF018MixedDirectionsDegrade 部分维度变化部分未变（方向不一致）降级，不得二选一。
func TestF018MixedDirectionsDegrade(t *testing.T) {
	evidence := allUnchanged()
	evidence[reconcile.DimInstanceStatus] = reconcile.DimensionEvidence{State: reconcile.DimChanged, Note: "状态已推进"}
	result := reconcile.Reconcile(reconcile.Input{Dims: evidence})
	if result.Verdict != reconcile.VerdictIndeterminate {
		t.Fatalf("方向不一致必须降级：%+v", result)
	}
}

// TestF018CollectMarksUnwiredDimsMissing 收集器对读不到的维度如实标缺失（已办/动作痕迹），不冒充未变化。
func TestF018CollectMarksUnwiredDimsMissing(t *testing.T) {
	input := reconcile.Collect(reconcile.FactInput{
		StepNodeKey: "node-audit",
		NowFound:    true, NowStatus: "run", NowCurrentNodes: []string{"node-audit"}, NowDueNodes: []string{"node-audit"},
	})
	result := reconcile.Reconcile(input)
	if result.Verdict != reconcile.VerdictIndeterminate {
		t.Fatalf("已办/动作痕迹读不到时必须降级：%+v", result)
	}
}
