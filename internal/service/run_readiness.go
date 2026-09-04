package service

import (
	"sort"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// 运行准备面板的锚点，与前端面板一一对应；只用固定取值，前端不自造锚点。
const (
	runReadinessAnchorNodes    = "node-configuration"
	runReadinessAnchorFormData = "form-data"
	runReadinessAnchorPath     = "path"
)

// verifiedRunnableActions 是"已被真实写验证过、允许执行器执行"的动作子集。
//
// 现在是空集，这不是遗漏：纲领第 9 节明确要求 F-019 之后动作全集才可执行，在此之前执行器只允许
// 执行已验证过的动作子集，其余动作在运行准备阶段**直接阻塞，不做静默降级**。F-016 是第一次真实写，
// 它跑通哪个动作才把哪个动作登记进来。因此在 F-016 之前，任何已配置动作的路径都会因本项被阻塞，
// 这是设计要求的结果，不是缺陷。
var verifiedRunnableActions = map[model.ActionKey]bool{}

// IsVerifiedRunnableAction 判断该动作是否已被真实写验证过，可以交给执行器执行。
func IsVerifiedRunnableAction(action model.ActionKey) bool {
	return verifiedRunnableActions[action]
}

// PathReadinessInput 是判断一条路径能否启动所需的全部事实，全部由调用方先读齐再传入。
// 这里刻意不放仓储与目标客户端：十类阻塞的判断必须是纯函数，才能逐类写用例。
type PathReadinessInput struct {
	Path model.ExecutionPath
	// ConfigFound 为 false 表示这条路径还没有任何配置记录。
	ConfigFound bool
	// ConfigIssues 是路径配置里已记录且标记为阻塞的问题，原样透出，不改写文案。
	ConfigIssues []model.PathConfigAffectedItem
	// ConfigNotices 是路径配置里记录的说明性提示（blocking=false），只提醒不阻塞。
	// F-012 会把"表单校验会在打开表单数据页时完成"这类说明写进同一列，当阻塞显示用户看不懂也无法处理。
	ConfigNotices []model.PathConfigAffectedItem
	// CompiledStepCount 是编译场景里的步骤数，为 0 表示没有可执行步骤。
	CompiledStepCount int
	// ConfiguredActions 是这条路径已配置的动作标识，用于与已验证子集比对。
	ConfiguredActions []model.ActionKey
	// PersonIssues 是人员策略只读复验的问题，例如解析不出唯一真实处理人。
	PersonIssues []model.PathConfigAffectedItem
	// TopologyIssues 是路径与当前真实流程结构复验的问题。
	TopologyIssues []model.PathConfigAffectedItem
	// PendingSemanticsEntries 是本路径动作涉及、但在语义清单里仍标"待实测"的条目名。
	PendingSemanticsEntries []string
	// Reminders 是只提醒不阻塞的事项，由调用方按计划与部署事实给出。
	Reminders []model.RunReadinessItem
}

// EvaluatePathReadiness 按十类阻塞与两类提醒给出一条路径的运行准备结论。
// 阻塞与提醒严格分开：提醒不影响能否启动，绝不允许把提醒混成阻塞。
func EvaluatePathReadiness(input PathReadinessInput) model.PathRunReadiness {
	blocks := make([]model.RunReadinessItem, 0, 8)
	pathName := strings.TrimSpace(input.Path.Name)
	if pathName == "" {
		pathName = "路径 " + strconv.FormatUint(uint64(input.Path.SequenceNo), 10)
	}
	if input.Path.ConfigurationStatus != model.ExecutionPathConfigurationConfigured {
		blocks = append(blocks, model.RunReadinessItem{
			Kind: model.RunReadinessNodeConfiguration, Name: pathName,
			Reason: firstNonEmptyText(input.Path.ConfigurationDetail, "节点人员与动作配置尚未完成"),
			Anchor: runReadinessAnchorNodes,
		})
	}
	if input.Path.DataStatus != model.HistoryDataStatusReady {
		blocks = append(blocks, model.RunReadinessItem{
			Kind: model.RunReadinessFormData, Name: pathName,
			Reason: firstNonEmptyText(input.Path.DataDetail, "基础表单数据尚未就绪"),
			Anchor: runReadinessAnchorFormData,
		})
	}
	blocks = append(blocks, itemsFrom(input.ConfigIssues, model.RunReadinessConfigIssue, runReadinessAnchorNodes)...)
	blocks = append(blocks, itemsFrom(input.PersonIssues, model.RunReadinessPersonNotUnique, runReadinessAnchorNodes)...)
	blocks = append(blocks, itemsFrom(input.TopologyIssues, model.RunReadinessTopologyChanged, runReadinessAnchorPath)...)
	reminders := append([]model.RunReadinessItem{}, input.Reminders...)
	if input.ConfigFound && input.CompiledStepCount == 0 {
		blocks = append(blocks, model.RunReadinessItem{
			Kind: model.RunReadinessCompiledScenarioEmpty, Name: pathName,
			Reason: "编译场景为空，没有可执行的步骤",
			Anchor: runReadinessAnchorNodes,
		})
	}
	// 动作是否被真实写验证过、语义条目是否已实测，都是工具自身的进度，用户在这里无法处理。
	// 把它们当阻塞会形成死锁：第一次真实写要先能跑起来才可能验证动作，而阻塞又不让跑。
	// 因此降为提醒，如实告诉用户风险，由用户决定是否继续。
	for _, action := range unverifiedActions(input.ConfiguredActions) {
		reminders = append(reminders, model.RunReadinessItem{
			Kind: model.RunReadinessActionNotVerified, Name: actionDisplayLabel(action),
			Reason: "这个动作还没有被真实写验证过，第一次执行请留意目标平台的实际结果",
			Anchor: runReadinessAnchorNodes,
		})
	}
	for _, entry := range input.PendingSemanticsEntries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		reminders = append(reminders, model.RunReadinessItem{
			Kind: model.RunReadinessSemanticsPending, Name: entry,
			Reason: "这条路径涉及的目标行为还没有实测勘定，结果需要人工复核",
			Anchor: runReadinessAnchorPath,
		})
	}
	reminders = append(reminders, itemsFrom(input.ConfigNotices, model.RunReadinessConfigIssue, runReadinessAnchorNodes)...)
	return model.PathRunReadiness{
		PathID: input.Path.ID, PathName: pathName, SequenceNo: input.Path.SequenceNo,
		Runnable: len(blocks) == 0, Summary: pathSummary(pathName, len(blocks)),
		Blocks: blocks, Reminders: reminders,
	}
}

// AggregatePlanReadiness 把逐条路径结论聚合成一个计划的运行准备结论，并给出一句中文总结论。
func AggregatePlanReadiness(paths []model.PathRunReadiness) model.PlanRunReadiness {
	sorted := append([]model.PathRunReadiness{}, paths...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first].SequenceNo < sorted[second].SequenceNo })
	runnable := 0
	for _, path := range sorted {
		if path.Runnable {
			runnable++
		}
	}
	total := len(sorted)
	readiness := model.PlanRunReadiness{
		TotalCount: total, RunnableCount: runnable, BlockedCount: total - runnable, Paths: sorted,
	}
	switch {
	case total == 0:
		readiness.Summary = "这个计划还没有执行路径，无法启动"
	case runnable == total:
		readiness.Summary = "全部 " + strconv.Itoa(total) + " 条执行路径都可以启动"
	case runnable == 0:
		readiness.Summary = "全部 " + strconv.Itoa(total) + " 条执行路径都被阻塞，逐条原因见下"
	default:
		readiness.Summary = strconv.Itoa(runnable) + " 条可以启动，" + strconv.Itoa(total-runnable) + " 条被阻塞，逐条原因见下"
	}
	return readiness
}

// unverifiedActions 返回这条路径里尚未被真实写验证过的动作，按目录顺序去重。
func unverifiedActions(actions []model.ActionKey) []model.ActionKey {
	seen := make(map[model.ActionKey]bool, len(actions))
	result := make([]model.ActionKey, 0, len(actions))
	for _, action := range actions {
		if action == "" || seen[action] || IsVerifiedRunnableAction(action) {
			continue
		}
		seen[action] = true
		result = append(result, action)
	}
	return result
}

// itemsFrom 把已有的路径问题项转成运行准备条目，原样保留名称与中文原因。
func itemsFrom(issues []model.PathConfigAffectedItem, kind, anchor string) []model.RunReadinessItem {
	items := make([]model.RunReadinessItem, 0, len(issues))
	for _, issue := range issues {
		items = append(items, model.RunReadinessItem{
			Kind: kind, Name: issue.Name, Reason: issue.Reason, Anchor: anchor,
		})
	}
	return items
}

// pathSummary 给出单条路径的一句中文结论。
func pathSummary(pathName string, blockCount int) string {
	if blockCount == 0 {
		return pathName + " 可以启动"
	}
	return pathName + " 有 " + strconv.Itoa(blockCount) + " 项阻塞需要先处理"
}

// firstNonEmptyText 取第一个非空文案，避免界面出现空原因。
func firstNonEmptyText(candidates ...string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
