// Package reconcile 是写结果不确定时的只读对账判定（纲领第 4.1、4.4 节）。
// 硬边界：本包是纯判定，不做 IO、不持有目标客户端、不改变运行状态；
// 它不得引用任何写端点——这一条由契约测试锁定依赖集合，不靠自觉。
package reconcile

import (
	"fmt"
	"strings"
)

// DimensionState 是单个对账维度的证据状态。
type DimensionState string

const (
	// DimUnchanged 表示该维度读到且与写之前一致。
	DimUnchanged DimensionState = "unchanged"
	// DimChanged 表示该维度读到且与写之前不同（写可能已生效）。
	DimChanged DimensionState = "changed"
	// DimMissing 表示该维度读不到（为空或读取失败）。
	DimMissing DimensionState = "missing"
	// DimConflict 表示该维度读数互相矛盾。
	DimConflict DimensionState = "conflict"
)

// Dimension 是五个对账维度（纲领 F-018 强证据规则）。
type Dimension string

const (
	DimInstanceStatus Dimension = "instance_status" // 实例状态
	DimCurrentNode    Dimension = "current_node"    // 当前节点
	DimCurrentTask    Dimension = "current_task"    // 当前待办
	DimDoneRecords    Dimension = "done_records"    // 已办记录
	DimActionTraces   Dimension = "action_traces"   // 动作痕迹
)

// DimensionEvidence 是一个维度的证据：状态 + 中文说明。
type DimensionEvidence struct {
	State DimensionState
	Note  string
}

// Verdict 是对账三值结论（领域固定取值，不得扩展）。
type Verdict string

const (
	VerdictEffective     Verdict = "effective"     // 已生效
	VerdictNotEffective  Verdict = "not_effective" // 未生效
	VerdictIndeterminate Verdict = "indeterminate" // 仍无法判定
)

// RecoveryAction 是唯一合法后续动作（与结论一一对应，不得并列）。
type RecoveryAction string

const (
	ActionAdvance        RecoveryAction = "advance"         // 确认并前进到下一步（仅 effective）
	ActionReplay         RecoveryAction = "replay"          // 重放这一步（仅 not_effective）
	ActionManualEnd      RecoveryAction = "manual_end"      // 登记人工核对结论并结束（仅 indeterminate）
	ActionReconcileAgain RecoveryAction = "reconcile_again" // 重新对账（仅对账读取失败）
)

// Input 是一次只读对账的输入：写之前的基准事实 + 现在重读到的逐维度证据。
// 输入不允许包含目标写客户端、凭证或用户提交的“继续/重试”选择。
type Input struct {
	// StepNodeKey 是本次写动作所在节点；已办/待办/当前节点对照都围绕它。
	StepNodeKey string
	// BeforeStatus 是写之前读到的实例状态；空表示写之前实例不存在（发起新建）。
	BeforeStatus string
	// BeforeHadInstance 表示写之前实例已存在（审批类动作）。
	BeforeHadInstance bool
	// Dims 是五个维度的证据；缺失的维度按 missing 处理，绝不允许默认成“未生效”。
	Dims map[Dimension]DimensionEvidence
	// PartialEffect 标记“表单数据已变但流程未推进”（语义第 2.4 节）。
	// 一旦为真，结论固定为仍无法判定，绝不允许重放。
	PartialEffect     bool
	PartialEffectNote string
}

// Result 是对账结论：三值结论、唯一合法动作、逐维度依据与中文结论。
type Result struct {
	Verdict  Verdict
	Action   RecoveryAction
	Reasons  []string
	Headline string
}

// Reconcile 是纯判定函数：
//   - 部分生效 → 一律 indeterminate（重放会把表单数据再写一次，绝不给重放）；
//   - 五个维度全部读到且全部“写已生效”的证据一致 → effective；
//   - 五个维度全部读到且全部与写之前一致（强证据规则）→ not_effective；
//   - 任一维度缺失、矛盾、或读数方向不一致 → indeterminate，降级理由逐条列出。
func Reconcile(input Input) Result {
	if input.PartialEffect {
		note := input.PartialEffectNote
		if strings.TrimSpace(note) == "" {
			note = "检测到表单数据已变化但流程未推进"
		}
		return Result{
			Verdict: VerdictIndeterminate, Action: ActionManualEnd,
			Headline: "表单数据可能已经写进去了，重放会再写一次；只能登记人工核对结论并结束本路径运行",
			Reasons:  []string{"部分生效（" + note + "）：按语义第 2.4 节固定判仍无法判定"},
		}
	}

	// 收集五维证据；任何维度缺失都直接降级（读不到不允许当“未变”用）。
	reasons := make([]string, 0, 8)
	missing := false
	changedCount := 0
	unchangedCount := 0
	for _, dim := range []Dimension{DimInstanceStatus, DimCurrentNode, DimCurrentTask, DimDoneRecords, DimActionTraces} {
		evidence, ok := input.Dims[dim]
		if !ok || strings.TrimSpace(string(evidence.State)) == "" {
			missing = true
			reasons = append(reasons, fmt.Sprintf("维度 %s：无证据（缺失按无法判定处理，绝不当成未变化）", dim))
			continue
		}
		switch evidence.State {
		case DimMissing:
			missing = true
			reasons = append(reasons, fmt.Sprintf("维度 %s：读不到（%s）", dim, evidence.Note))
		case DimConflict:
			missing = true
			reasons = append(reasons, fmt.Sprintf("维度 %s：读数互相矛盾（%s）", dim, evidence.Note))
		case DimChanged:
			changedCount++
			reasons = append(reasons, fmt.Sprintf("维度 %s：与写之前不同（%s）", dim, evidence.Note))
		case DimUnchanged:
			unchangedCount++
			reasons = append(reasons, fmt.Sprintf("维度 %s：与写之前一致（%s）", dim, evidence.Note))
		}
	}
	if missing {
		return Result{
			Verdict: VerdictIndeterminate, Action: ActionManualEnd,
			Headline: "对账证据不完整，无法判定写是否生效；只能登记人工核对结论并结束本路径运行",
			Reasons:  reasons,
		}
	}
	if changedCount > 0 && unchangedCount > 0 {
		return Result{
			Verdict: VerdictIndeterminate, Action: ActionManualEnd,
			Headline: "对账证据方向不一致，无法判定写是否生效；只能登记人工核对结论并结束本路径运行",
			Reasons:  reasons,
		}
	}
	if changedCount == len(allDims()) {
		return Result{
			Verdict: VerdictEffective, Action: ActionAdvance,
			Headline: "对账确认：上一次写已在目标生效，可确认并前进到下一步",
			Reasons:  reasons,
		}
	}
	if unchangedCount == len(allDims()) {
		return Result{
			Verdict: VerdictNotEffective, Action: ActionReplay,
			Headline: "对账确认：上一次写未在目标生效（五维证据全部未变），唯一动作是重放这一步",
			Reasons:  reasons,
		}
	}
	return Result{
		Verdict: VerdictIndeterminate, Action: ActionManualEnd,
		Headline: "对账证据无法归类，按仍无法判定处理",
		Reasons:  reasons,
	}
}

// allDims 返回五个维度的稳定顺序。
func allDims() []Dimension {
	return []Dimension{DimInstanceStatus, DimCurrentNode, DimCurrentTask, DimDoneRecords, DimActionTraces}
}

// FactInput 是从目标只读事实构造对账输入的便捷结构（由收集器填充，本包不做 IO）。
type FactInput struct {
	// StepNodeKey 是写动作所在节点。
	StepNodeKey string
	// BeforeStatus/BeforeHadInstance 是写之前的实例状态基准。
	BeforeStatus      string
	BeforeHadInstance bool
	// NowFound/NowStatus/NowCurrentNodes 是现在重读到的实例事实。
	NowFound        bool
	NowStatus       string
	NowCurrentNodes []string
	// NowDueNodes 是现在重读到的当前待办节点集合。
	NowDueNodes []string
	// NowReadError 非空表示重读失败：唯一动作是重新对账。
	NowReadError string
	// DoneRecordsRead 表示已办记录维度是否成功读取（当前工具尚未接入该读取时为 false）。
	DoneRecordsRead  bool
	DoneRecordFound  bool   // 已办记录里是否已有本次动作痕迹
	ActionTraceFound bool   // 动作痕迹里是否已有本次写迹象
	FormChanged      bool   // 表单数据相对写之前发生变化
	FormChangedNote  string // 表单变化的中文说明（部分生效警告用）
}

// Collect 把目标只读事实转换为对账输入（T02 收集器与判定器的边界）。
// 已办记录维度当前工具未接入读取时如实标 missing——强证据规则会把它降级为仍无法判定，绝不冒充未变化。
func Collect(f FactInput) Input {
	dims := map[Dimension]DimensionEvidence{}
	if f.NowReadError != "" {
		dims[DimInstanceStatus] = DimensionEvidence{State: DimMissing, Note: f.NowReadError}
		dims[DimCurrentNode] = DimensionEvidence{State: DimMissing, Note: f.NowReadError}
		dims[DimCurrentTask] = DimensionEvidence{State: DimMissing, Note: f.NowReadError}
		dims[DimDoneRecords] = DimensionEvidence{State: DimMissing, Note: f.NowReadError}
		dims[DimActionTraces] = DimensionEvidence{State: DimMissing, Note: f.NowReadError}
		return Input{StepNodeKey: f.StepNodeKey, BeforeStatus: f.BeforeStatus, BeforeHadInstance: f.BeforeHadInstance, Dims: dims}
	}
	if !f.NowFound {
		// 实例现在读不到：发起类动作基准是“写之前不存在”，这属于读不到事实而不是未变化。
		dims[DimInstanceStatus] = DimensionEvidence{State: DimMissing, Note: "实例在已发列表不可见"}
		dims[DimCurrentNode] = DimensionEvidence{State: DimMissing, Note: "实例在已发列表不可见"}
		dims[DimCurrentTask] = DimensionEvidence{State: DimMissing, Note: "实例在已发列表不可见"}
		dims[DimDoneRecords] = DimensionEvidence{State: DimMissing, Note: "已办记录读取未接入"}
		dims[DimActionTraces] = DimensionEvidence{State: DimMissing, Note: "动作痕迹读取未接入"}
		return Input{StepNodeKey: f.StepNodeKey, BeforeStatus: f.BeforeStatus, BeforeHadInstance: f.BeforeHadInstance, Dims: dims}
	}
	// 实例状态：与写之前比较。
	if !f.BeforeHadInstance {
		dims[DimInstanceStatus] = DimensionEvidence{State: DimChanged, Note: "写之前实例不存在，现在状态为 " + f.NowStatus}
	} else if f.NowStatus == f.BeforeStatus {
		dims[DimInstanceStatus] = DimensionEvidence{State: DimUnchanged, Note: "实例状态仍为 " + f.NowStatus}
	} else {
		dims[DimInstanceStatus] = DimensionEvidence{State: DimChanged, Note: "实例状态由 " + f.BeforeStatus + " 变为 " + f.NowStatus}
	}
	// 当前节点：已配置路径的预期推进不含本步节点时视为已变化。
	nodeMoved := false
	for _, node := range f.NowCurrentNodes {
		if node == f.StepNodeKey {
			nodeMoved = false
			break
		}
		nodeMoved = true
	}
	if len(f.NowCurrentNodes) == 0 {
		dims[DimCurrentNode] = DimensionEvidence{State: DimMissing, Note: "目标未返回当前节点事实"}
	} else if nodeMoved {
		dims[DimCurrentNode] = DimensionEvidence{State: DimChanged, Note: "当前节点已不在本步节点上"}
	} else {
		dims[DimCurrentNode] = DimensionEvidence{State: DimUnchanged, Note: "当前节点仍是本步节点"}
	}
	// 当前待办：本步节点的待办仍在即未变化。
	dueOnStep := false
	for _, node := range f.NowDueNodes {
		if node == f.StepNodeKey {
			dueOnStep = true
			break
		}
	}
	if dueOnStep {
		dims[DimCurrentTask] = DimensionEvidence{State: DimUnchanged, Note: "本步节点的待办仍然存在"}
	} else {
		dims[DimCurrentTask] = DimensionEvidence{State: DimChanged, Note: "本步节点的待办已消失"}
	}
	// 已办记录与动作痕迹：当前工具未接入读取，如实按缺失处理（强证据规则降级）。
	if f.DoneRecordsRead {
		if f.DoneRecordFound {
			dims[DimDoneRecords] = DimensionEvidence{State: DimChanged, Note: "已办记录出现本次动作"}
		} else {
			dims[DimDoneRecords] = DimensionEvidence{State: DimUnchanged, Note: "已办记录无本次动作"}
		}
	} else {
		dims[DimDoneRecords] = DimensionEvidence{State: DimMissing, Note: "已办记录读取未接入"}
	}
	if f.ActionTraceFound {
		dims[DimActionTraces] = DimensionEvidence{State: DimChanged, Note: "动作痕迹出现本次写迹象"}
	} else {
		dims[DimActionTraces] = DimensionEvidence{State: DimMissing, Note: "动作痕迹读取未接入"}
	}
	return Input{
		StepNodeKey: f.StepNodeKey, BeforeStatus: f.BeforeStatus, BeforeHadInstance: f.BeforeHadInstance,
		Dims: dims, PartialEffect: f.FormChanged, PartialEffectNote: f.FormChangedNote,
	}
}
