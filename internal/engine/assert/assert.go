// Package assert 把一条执行路径的成功断言与目标事实重读结果映射为三值结论：成立、不成立、无法判定。
//
// 本包是 docs/features/F-015 的判定规则实现，与 F-014 的 internal/engine/verdict 是两件事：
// verdict 判"这次写请求生效了没有"，assert 判"这条路径跑出来的结果算不算成功"。两者不合并。
//
// 四条不可破坏的约束直接体现在实现里，全部来自 F-014 已勘定的目标事实：
//  1. 实例状态与已到达结束节点只能来自实例本身，禁止从待办列表推断——目标的待办列表在
//     taskStatus=pending 时不再查询流程实例，返回记录不带实例运行态字段。
//  2. 不读终态实例的当前处理人，目标已改为只对运行态实例解析当前处理人。
//  3. 不把 auditWay 作为断言事实输入。它属于流程配置而不是运行事实，也不为它写死数字兼容分支。
//  4. 字段缺失、事实读不到、结果自相矛盾一律「无法判定」，禁止用工具推断补齐，
//     也禁止把「无法判定」合并进「不成立」。拿不准就停，这与 F-014 的兜底是同一条底线。
//
// 本包是纯判定：无 IO、无目标调用、无数据库，不接入执行流程。
package assert

import (
	"strings"

	"test-auto-pro-v2/internal/model"
)

// Outcome 是断言判定的三值结论，取值集合固定，后续切片不得扩张也不得合并。
type Outcome string

const (
	// OutcomeHolds 表示断言成立，这条路径的运行结果算成功。
	OutcomeHolds Outcome = "holds"
	// OutcomeFails 表示断言不成立：实例已进入终态，但结束节点或状态与配置不符。
	// 这是"断言不成立"，不是"执行出错"，失败原因必须如实这样说明。
	OutcomeFails Outcome = "fails"
	// OutcomeUndecidable 表示无法判定：事实读不到、字段缺失、结果自相矛盾，或实例仍未进入终态。
	// 既不算成立也不算执行出错，必须停下等人工确认。
	OutcomeUndecidable Outcome = "undecidable"
)

// terminalStatuses 是目标实例的终态集合。
// 依据是目标自己的状态名义：撤销、终止、废弃、驳回、完结之后流程不再推进；
// 待发、草稿、运行中都还会继续。流程生命周期语义在 docs/TARGET_SEMANTICS.md 里仍是「未开始」，
// 因此这里只按状态名义划分，不对更细的生命周期行为下任何结论；
// 取值不在目标真实集合内时一律「无法判定」，绝不猜它是不是终态。
var terminalStatuses = map[string]bool{
	model.FlowInstanceStatusWithdraw:    true,
	model.FlowInstanceStatusTermination: true,
	model.FlowInstanceStatusAbandon:     true,
	model.FlowInstanceStatusRejected:    true,
	model.FlowInstanceStatusEnd:         true,
}

// IsTerminalStatus 判断目标实例状态是否属于终态；集合外取值返回 false，由调用方按无法判定处理。
func IsTerminalStatus(status string) bool {
	return terminalStatuses[strings.TrimSpace(status)]
}

// Fact 是 verify 阶段从目标实例重读到的事实投影，全部字段只能来自目标，不含工具推断。
type Fact struct {
	// Readable 为 false 表示这次重读没有拿到事实：连接失败、会话失效、响应不可解析都算。
	// 会话失效不一定伴随 HTTP 401（F-014 实测真实响应是 HTTP 200 加 code=RESP401），
	// 调用方必须把这类失败如实标成读不到，不得当成"实例还没进终态"。
	Readable bool
	// UnreadableReason 是重读失败的中文原因，只在 Readable 为 false 时有值。
	UnreadableReason string
	// InstanceStatusPresent 表示实例本身返回了状态字段。
	// 缺字段与显式空值含义不同，缺字段说明事实不完整，只能无法判定。
	InstanceStatusPresent bool
	// InstanceStatus 是实例自身的状态取值，只能读实例，禁止从待办列表推断。
	InstanceStatus string
	// ArrivedEndNodeKeys 是实例已到达的结束节点键，按到达先后排列，同一节点被到达多次就出现多次。
	ArrivedEndNodeKeys []string
	// Contradictory 表示重读结果自相矛盾，例如状态是终态但同时读到流程仍在推进。
	Contradictory bool
	// ContradictionReason 是矛盾的中文说明，只在 Contradictory 为 true 时有值。
	ContradictionReason string
}

// Verdict 是断言判定结果，除结论外一并给出中文原因与依据，便于直接写进运行记录与界面提示。
type Verdict struct {
	Outcome Outcome
	Reason  string
	Basis   string
}

// Evaluate 按 F-015 的三值规则判定断言，任何拿不准的情况都落「无法判定」。
// 判定顺序不可调换：先确认事实本身站得住脚，再看实例是否已进终态，最后才比对结束节点与状态。
func Evaluate(assertion model.PathSuccessAssertion, fact Fact) Verdict {
	if strings.TrimSpace(assertion.EndNodeKey) == "" || strings.TrimSpace(assertion.ExpectedStatus) == "" {
		return undecidable("这条路径没有配置可判定的成功断言", "F-015：断言缺失时不判成立也不判失败，先补配置")
	}
	if fact.Contradictory {
		reason := strings.TrimSpace(fact.ContradictionReason)
		if reason == "" {
			reason = "目标事实重读结果自相矛盾"
		}
		return undecidable(reason, "F-015：事实自相矛盾一律无法判定，不得合并进不成立")
	}
	if !fact.Readable {
		reason := strings.TrimSpace(fact.UnreadableReason)
		if reason == "" {
			reason = "目标事实读不到，无法确认这条路径跑成什么样"
		}
		return undecidable(reason, "F-015：事实读不到时无法判定；会话失效与连接失败都算读不到，不算实例未进终态")
	}
	if !fact.InstanceStatusPresent {
		return undecidable("目标实例没有返回状态字段，事实不完整",
			"F-015：字段缺失一律无法判定，禁止用工具推断补齐")
	}
	status := strings.TrimSpace(fact.InstanceStatus)
	if !model.IsFlowInstanceStatus(status) {
		return undecidable("目标实例返回了不在已知取值范围内的状态，无法判断是否已经结束",
			"F-015：状态取值超出目标真实集合时不猜是否终态")
	}
	if !IsTerminalStatus(status) {
		return undecidable("目标实例还没有进入终态，当前状态是"+model.FlowInstanceStatusLabel(status),
			"F-015：实例未进终态时不算成立也不算失败，等它跑完再判")
	}
	arrivals := countArrivals(fact.ArrivedEndNodeKeys, assertion.EndNodeKey)
	expectedOrdinal := assertion.ArrivalOrdinal
	if expectedOrdinal == 0 {
		expectedOrdinal = 1
	}
	statusMatched := status == strings.TrimSpace(assertion.ExpectedStatus)
	arrivalMatched := arrivals >= expectedOrdinal
	if statusMatched && arrivalMatched {
		return Verdict{
			Outcome: OutcomeHolds,
			Reason:  "实例已到达配置的结束节点" + assertionDisplayName(assertion) + "，状态是" + model.FlowInstanceStatusLabel(status),
			Basis:   "F-015：结束节点、到达次数与期望状态三项全部相符即断言成立",
		}
	}
	return Verdict{
		Outcome: OutcomeFails,
		Reason:  failureReason(assertion, status, arrivals, expectedOrdinal, statusMatched, arrivalMatched),
		Basis:   "F-015：实例已进终态但断言三项不全相符即断言不成立；这是断言不成立，不是执行出错",
	}
}

// countArrivals 统计实例事实里到达指定结束节点的次数。
func countArrivals(arrived []string, endNodeKey string) uint {
	target := strings.TrimSpace(endNodeKey)
	count := uint(0)
	for _, key := range arrived {
		if strings.TrimSpace(key) == target {
			count++
		}
	}
	return count
}

// failureReason 说清断言不成立到底差在哪一项，避免界面上只给一句"失败"。
func failureReason(assertion model.PathSuccessAssertion, status string, arrivals, expectedOrdinal uint, statusMatched, arrivalMatched bool) string {
	name := assertionDisplayName(assertion)
	switch {
	case !arrivalMatched && arrivals == 0:
		return "实例已进入终态" + model.FlowInstanceStatusLabel(status) + "，但从未到达配置的结束节点" + name
	case !arrivalMatched:
		return "实例只到达配置的结束节点" + name + " " + ordinalText(arrivals) + "，少于配置要求的" + ordinalText(expectedOrdinal)
	case !statusMatched:
		return "实例已到达配置的结束节点" + name + "，但状态是" + model.FlowInstanceStatusLabel(status) +
			"，与配置期望的" + model.FlowInstanceStatusLabel(assertion.ExpectedStatus) + "不符"
	default:
		return "实例终态与配置的成功断言不符"
	}
}

// assertionDisplayName 优先用配置时记录的节点名称，缺失时退回节点键，只用于文案显示。
func assertionDisplayName(assertion model.PathSuccessAssertion) string {
	if name := strings.TrimSpace(assertion.EndNodeName); name != "" {
		return name
	}
	return assertion.EndNodeKey
}

// ordinalText 把次数写成中文的"第 N 次"，让界面提示可直接阅读。
func ordinalText(count uint) string {
	return "第 " + uintText(count) + " 次"
}

// uintText 是不引入额外依赖的最小无符号整数转换。
func uintText(value uint) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// undecidable 组装一条无法判定结论。
func undecidable(reason, basis string) Verdict {
	return Verdict{Outcome: OutcomeUndecidable, Reason: reason, Basis: basis}
}
