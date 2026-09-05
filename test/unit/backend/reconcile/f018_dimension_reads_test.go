package reconcile_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/reconcile"
)

// f018FiveDimFacts 构造一份"五维都读到、且全部与写之前一致"的事实：这是唯一允许重放的形态。
func f018FiveDimFacts() reconcile.FactInput {
	return reconcile.FactInput{
		StepNodeKey: "real-audit", BeforeStatus: "run", BeforeHadInstance: true,
		NowFound: true, NowStatus: "run",
		NowCurrentNodes: []string{"real-audit"}, NowDueNodes: []string{"real-audit"},
		DoneRecordsRead: true, DoneRecordFound: false,
		ActionTraceRead: true, ActionTraceFound: false, ActionTraceTotal: 3,
	}
}

// TestF018NotEffectiveRequiresAllFiveDimensionsRead 锁定「未生效」只能在五维都真的读到时成立。
// 这是唯一会导致重放（再写一次）的结论：已办记录或审核记录读不到时必须降级为仍无法判定，
// 绝不能把"没读到"当成"没有痕迹"。
func TestF018NotEffectiveRequiresAllFiveDimensionsRead(t *testing.T) {
	full := reconcile.Reconcile(reconcile.Collect(f018FiveDimFacts()))
	if full.Verdict != reconcile.VerdictNotEffective || full.Action != reconcile.ActionReplay {
		t.Fatalf("五维全部读到且全部未变时必须判未生效并给出重放：%+v", full)
	}

	noDone := f018FiveDimFacts()
	noDone.DoneRecordsRead = false
	if got := reconcile.Reconcile(reconcile.Collect(noDone)); got.Verdict != reconcile.VerdictIndeterminate || got.Action == reconcile.ActionReplay {
		t.Fatalf("已办记录读不到时必须降级且不得给出重放：%+v", got)
	}

	noTrace := f018FiveDimFacts()
	noTrace.ActionTraceRead = false
	got := reconcile.Reconcile(reconcile.Collect(noTrace))
	if got.Verdict != reconcile.VerdictIndeterminate || got.Action == reconcile.ActionReplay {
		t.Fatalf("审核记录读不到时必须降级且不得给出重放：%+v", got)
	}
	// 两个新增维度已经真的接入读取，降级理由必须说"读取失败"而不是"未接入"——
	// 后者是接线之前的旧文案，留着会让界面上的依据与事实不符。
	if !strings.Contains(strings.Join(got.Reasons, "；"), "审核记录读取失败") {
		t.Fatalf("降级理由必须点明是哪个维度读不到：%v", got.Reasons)
	}
}

// TestF018EffectiveNeedsAllDimensionsChanged 锁定「已生效」的成立条件：五维方向一致地全部变化。
// 方向不一致（有的变了有的没变）一律降级，绝不乐观归类。
func TestF018EffectiveNeedsAllDimensionsChanged(t *testing.T) {
	advanced := f018FiveDimFacts()
	advanced.NowStatus = "end"
	advanced.NowCurrentNodes = []string{"real-next"}
	advanced.NowDueNodes = []string{"real-next"}
	advanced.DoneRecordFound = true
	advanced.ActionTraceFound = true
	got := reconcile.Reconcile(reconcile.Collect(advanced))
	if got.Verdict != reconcile.VerdictEffective || got.Action != reconcile.ActionAdvance {
		t.Fatalf("五维一致地全部变化时必须判已生效并给出前进：%+v", got)
	}

	mixed := advanced
	mixed.DoneRecordFound = false
	if conflict := reconcile.Reconcile(reconcile.Collect(mixed)); conflict.Verdict != reconcile.VerdictIndeterminate {
		t.Fatalf("证据方向不一致必须降级为仍无法判定：%+v", conflict)
	}
}

// TestF018ReadErrorOnlyAllowsReconcileAgain 锁定读取失败的唯一动作是重新对账，而不是重放或前进。
func TestF018ReadErrorOnlyAllowsReconcileAgain(t *testing.T) {
	facts := f018FiveDimFacts()
	facts.NowReadError = "连接超时"
	got := reconcile.Reconcile(reconcile.Collect(facts))
	if got.Verdict != reconcile.VerdictIndeterminate {
		t.Fatalf("读取失败必须判仍无法判定：%+v", got)
	}
	if got.Action == reconcile.ActionReplay || got.Action == reconcile.ActionAdvance {
		t.Fatalf("读取失败绝不允许重放或前进：%+v", got)
	}
}
