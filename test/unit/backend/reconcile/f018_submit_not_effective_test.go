package reconcile_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/reconcile"
)

// TestF018SubmitWriteCannotReachNotEffective 锁定一条结构性结论，并防止后人"顺手放宽"它：
// 发起类写（写之前实例不存在）永远得不到「未生效」，因此也永远不会走到重放。
//
// 为什么：判「未生效」要求五维全部读到且全部与写之前一致，而实例状态这一维在发起场景下
// 只有两种可能——实例现在读到了（写之前不存在、现在存在，属"已变化"），
// 或者实例现在读不到（属"读不到"，按缺失降级）。两条都不可能是"与写之前一致"。
//
// 现实后果（F-018 实测记录）：执行器当前只能真实执行发起，对已有实例动手属 F-019，
// 所以真实环境里的不确定写只可能落在「仍无法判定」，重放链路拿不到真实目标证据。
// 这个用例把这件事变成可执行的断言，而不是文档里的一句话。
func TestF018SubmitWriteCannotReachNotEffective(t *testing.T) {
	cases := []struct {
		name  string
		facts reconcile.FactInput
	}{
		{
			name: "发起后实例可见且五维读数都在",
			facts: reconcile.FactInput{
				StepNodeKey: "node-start", BeforeHadInstance: false, BeforeStatus: "",
				NowFound: true, NowStatus: "run",
				NowCurrentNodes: []string{"node-start"}, NowDueNodes: []string{"node-start"},
				DoneRecordsRead: true, DoneRecordFound: false,
				ActionTraceRead: true, ActionTraceFound: false, ActionTraceTotal: 0,
			},
		},
		{
			name: "发起后实例仍不可见（响应丢失、拿不到实例引用）",
			facts: reconcile.FactInput{
				StepNodeKey: "node-start", BeforeHadInstance: false, BeforeStatus: "",
				NowFound: false,
			},
		},
	}
	for _, item := range cases {
		result := reconcile.Reconcile(reconcile.Collect(item.facts))
		if result.Verdict == reconcile.VerdictNotEffective {
			t.Fatalf("%s：发起类写不得得出未生效（那会导致重放再发一次发起，造成双写）", item.name)
		}
		if result.Action == reconcile.ActionReplay {
			t.Fatalf("%s：发起类写不得给出重放动作", item.name)
		}
		t.Logf("%s → 结论=%s 动作=%s", item.name, result.Verdict, result.Action)
	}
}

// TestF018ApprovalWriteReachesNotEffectiveOnlyWithFiveDimensions 给出「未生效」成立的唯一形状：
// 写之前实例已存在（审批类动作），且五维全部读到、全部与写之前一致。
// 少任何一维都必须降级——逐维各测一次，防止后人只改一个分支就把门槛放宽。
func TestF018ApprovalWriteReachesNotEffectiveOnlyWithFiveDimensions(t *testing.T) {
	base := reconcile.FactInput{
		StepNodeKey: "node-audit", BeforeHadInstance: true, BeforeStatus: "run",
		NowFound: true, NowStatus: "run",
		NowCurrentNodes: []string{"node-audit"}, NowDueNodes: []string{"node-audit"},
		DoneRecordsRead: true, DoneRecordFound: false,
		ActionTraceRead: true, ActionTraceFound: false, ActionTraceTotal: 1,
	}
	full := reconcile.Reconcile(reconcile.Collect(base))
	if full.Verdict != reconcile.VerdictNotEffective || full.Action != reconcile.ActionReplay {
		t.Fatalf("五维齐备且全部未变时应判未生效并给出重放，实际 结论=%s 动作=%s", full.Verdict, full.Action)
	}

	degraded := map[string]func(*reconcile.FactInput){
		"实例状态变了":  func(f *reconcile.FactInput) { f.NowStatus = "end" },
		"当前节点走了":  func(f *reconcile.FactInput) { f.NowCurrentNodes = []string{"node-next"} },
		"本步待办消失":  func(f *reconcile.FactInput) { f.NowDueNodes = nil },
		"已办记录读不到": func(f *reconcile.FactInput) { f.DoneRecordsRead = false },
		"动作痕迹读不到": func(f *reconcile.FactInput) { f.ActionTraceRead = false },
		"实例读不到":   func(f *reconcile.FactInput) { f.NowFound = false },
		"重读整体失败":  func(f *reconcile.FactInput) { f.NowReadError = "目标暂时不可读" },
		"表单已部分生效": func(f *reconcile.FactInput) { f.FormChanged = true },
	}
	for name, mutate := range degraded {
		facts := base
		mutate(&facts)
		result := reconcile.Reconcile(reconcile.Collect(facts))
		if result.Verdict == reconcile.VerdictNotEffective || result.Action == reconcile.ActionReplay {
			t.Fatalf("%s：任一维度不成立都必须降级，不得判未生效或给出重放", name)
		}
		t.Logf("%s → 结论=%s 动作=%s", name, result.Verdict, result.Action)
	}
}
