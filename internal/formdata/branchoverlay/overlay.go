package branchoverlay

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/model"
)

const (
	maxPatchFields       = 8
	maxCandidatesPerPath = 16
	maxSearchAttempts    = 20000
)

// Status 表示分支补丁是否已经被目标条件完整证明。
type Status string

const (
	// StatusReady 表示原始数据或唯一最小补丁已经通过完整路径复验。
	StatusReady Status = "ready"
	// StatusNeedsInput 表示数据、条件或最小解缺少可证明证据。
	StatusNeedsInput Status = "needs_input"
)

// Input 描述一次基于目标流程树的原始表单数据分支补丁请求。
type Input struct {
	Tree    *target.FlowNodeTemplate
	Choices []model.ExecutionPathChoice
	Values  map[string]any
	// Candidates 只能由调用方填入目标字段真实候选；模块不会按标签、类型或名称推导值。
	Candidates map[string][]any
	// TargetCandidates 是 Candidates 的语义明确别名，便于调用方标记候选来源。
	TargetCandidates map[string][]any
}

// Result 返回原始数据深复制、实际分支和窄范围字段补丁。
type Result struct {
	Status        Status                      `json:"status"`
	Values        map[string]any              `json:"values"`
	ActualChoices []model.ExecutionPathChoice `json:"actualChoices"`
	Patches       []model.HistoryBranchPatch  `json:"patches"`
	Issues        []Issue                     `json:"issues"`
}

// Issue 是无法确定分支或补丁时的稳定说明，不包含目标响应原文。
type Issue struct {
	Code      string `json:"code"`
	Path      string `json:"path,omitempty"`
	BranchKey string `json:"branchKey,omitempty"`
	Operator  string `json:"operator,omitempty"`
	Message   string `json:"message"`
}

// Apply 在目标原始表单数据上计算实际路径并应用唯一的最小字段补丁。
func Apply(input Input) Result {
	if input.Values == nil {
		return needsInputResult(nil, Issue{Code: "raw_data_missing", Message: "目标原始表单数据缺失，不能退回空对象"})
	}
	original, err := cloneMap(input.Values)
	if err != nil {
		return needsInputResult(nil, Issue{Code: "raw_data_uncloneable", Message: "目标原始表单数据无法完整复制"})
	}
	result := Result{Status: StatusNeedsInput, Values: original, Patches: []model.HistoryBranchPatch{}, Issues: []Issue{}}
	if input.Tree == nil {
		return appendIssue(result, Issue{Code: "flow_tree_missing", Message: "目标流程树缺失，无法复验路径"})
	}
	choices, choiceIssues := choiceMap(input.Choices)
	result.Issues = append(result.Issues, choiceIssues...)
	if len(choiceIssues) > 0 {
		return result
	}
	references, collectIssues := collectReferences(input.Tree, choices)
	result.Issues = append(result.Issues, collectIssues...)
	if len(collectIssues) > 0 {
		return result
	}
	if extra := extraChoiceIssues(input.Tree, choices); len(extra) > 0 {
		result.Issues = append(result.Issues, extra...)
		return result
	}
	if cycle := referenceCycle(references); cycle != "" {
		return appendIssue(result, Issue{Code: "field_cycle", Path: cycle, Message: "条件字段形成循环依赖，无法确定安全补丁"})
	}

	currentWalk := walkTree(input.Tree, original, choices)
	if currentWalk.complete {
		if currentWalk.matches {
			result.Status = StatusReady
			result.ActualChoices = currentWalk.choices
			return result
		}
	} else if !walkCanBecomeEvaluable(currentWalk, references) {
		result.Issues = append(result.Issues, currentWalk.issues...)
		return result
	}

	variables, variableIssues := buildVariables(original, mergeCandidates(input.Candidates, input.TargetCandidates), references)
	result.Issues = append(result.Issues, variableIssues...)
	if len(variableIssues) > 0 {
		return result
	}
	if len(variables) > maxPatchFields {
		return appendIssue(result, Issue{Code: "search_limit", Message: "条件字段超过安全补丁范围"})
	}
	if len(variables) == 0 {
		result.Issues = append(result.Issues, currentWalk.issues...)
		return appendIssue(result, Issue{Code: "no_solution", Message: "历史数据无法命中当前目标路径"})
	}

	solutions, attempts := searchSolutions(input.Tree, original, choices, variables)
	if len(solutions) == 0 {
		result.Issues = append(result.Issues, currentWalk.issues...)
		code := "no_solution"
		message := "现有目标候选无法得到可复验的最小分支补丁"
		if attempts >= maxSearchAttempts {
			code = "search_limit"
			message = "分支补丁搜索超过安全边界，需要人工核对"
		}
		if hasBooleanCondition(references) {
			code = "unsatisfiable_condition"
			message = "目标 boolean_value 规则不可满足"
		}
		return appendIssue(result, Issue{Code: code, Message: message})
	}
	selected, ambiguous := chooseSolution(solutions)
	if ambiguous {
		return appendIssue(result, Issue{Code: "ambiguous_solution", Message: "存在多个同等最小补丁，无法安全选择"})
	}
	finalWalk := walkTree(input.Tree, selected.values, choices)
	if !finalWalk.complete || !finalWalk.matches {
		return appendIssue(result, Issue{Code: "path_recheck_failed", Message: "最小补丁未通过完整目标路径复验"})
	}
	result.Status = StatusReady
	result.Values = selected.values
	result.ActualChoices = finalWalk.choices
	result.Patches = buildPatches(original, selected.values, references)
	return result
}

// ResolveActualPath 只按目标条件语义计算当前原始表单值实际命中的路径。
// 自动条件由目标分支顺序决定，手动分支和非并行候选分支必须沿用调用方提交的真实选择。
func ResolveActualPath(input Input) Result {
	if input.Values == nil {
		return needsInputResult(nil, Issue{Code: "raw_data_missing", Message: "目标原始表单数据缺失，不能退回空对象"})
	}
	values, err := cloneMap(input.Values)
	if err != nil {
		return needsInputResult(nil, Issue{Code: "raw_data_uncloneable", Message: "目标原始表单数据无法完整复制"})
	}
	result := Result{Status: StatusNeedsInput, Values: values, ActualChoices: []model.ExecutionPathChoice{}, Patches: []model.HistoryBranchPatch{}, Issues: []Issue{}}
	if input.Tree == nil {
		return appendIssue(result, Issue{Code: "flow_tree_missing", Message: "目标流程树缺失，无法计算实际路径"})
	}
	selected, choiceIssues := choiceMap(input.Choices)
	result.Issues = append(result.Issues, choiceIssues...)
	if len(choiceIssues) > 0 {
		return result
	}
	walk := actualPathWalk{choices: make([]model.ExecutionPathChoice, 0), issues: make([]Issue, 0)}
	walkActualPath(input.Tree, values, selected, &walk, make(map[string]bool), make(map[string]bool))
	result.ActualChoices = walk.choices
	result.Issues = append(result.Issues, walk.issues...)
	if len(walk.issues) == 0 {
		result.Status = StatusReady
	}
	return result
}

type actualPathWalk struct {
	choices []model.ExecutionPathChoice
	issues  []Issue
}

// walkActualPath 沿目标流程树递归计算实际分支，循环和不可求值条件均停止并保留原始数据。
func walkActualPath(node *target.FlowNodeTemplate, values map[string]any, selected map[string]string, result *actualPathWalk, visited, active map[string]bool) {
	if node == nil || len(result.issues) > 0 {
		return
	}
	key := nodeVisitKey(node)
	if active[key] {
		result.issues = append(result.issues, Issue{Code: "flow_cycle", Path: node.ID, Message: "目标流程树存在循环，无法计算实际路径"})
		return
	}
	if visited[key] {
		return
	}
	active[key] = true
	defer delete(active, key)
	visited[key] = true
	if len(node.ConditionNodes) > 0 {
		branches := orderedBranches(node.ConditionNodes)
		var branch target.FlowBranchTemplate
		var evaluation Evaluation
		if isManualNode(node) {
			branchID, exists := selected[node.ID]
			if !exists {
				result.issues = append(result.issues, Issue{Code: "choice_missing", Path: node.ID, Message: "手动分支需要显式选择"})
				return
			}
			index := branchIndex(branches, branchID)
			if index < 0 {
				result.issues = append(result.issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
				return
			}
			branch, evaluation = branches[index], Evaluation{Satisfied: true, Evaluable: true}
		} else {
			branch, evaluation = chooseConditionBranch(branches, values, false)
		}
		if !evaluation.Evaluable {
			result.issues = append(result.issues, Issue{Code: "condition_unavailable", Path: node.ID, Message: evaluation.Reason})
			return
		}
		result.choices = append(result.choices, model.ExecutionPathChoice{RouteNodeID: node.ID, BranchID: branch.ID})
		walkActualPath(branch.Child, values, selected, result, visited, active)
		walkActualPath(node.Child, values, selected, result, visited, active)
		return
	}
	if len(node.ParallelNodes) > 0 {
		if strings.EqualFold(strings.TrimSpace(node.Type), "parallel") {
			for _, branch := range orderedBranches(node.ParallelNodes) {
				walkActualPath(branch.Child, values, selected, result, visited, active)
			}
		} else {
			branchID, exists := selected[node.ID]
			if !exists {
				result.issues = append(result.issues, Issue{Code: "choice_missing", Path: node.ID, Message: "当前路由缺少候选分支选择"})
				return
			}
			index := branchIndex(node.ParallelNodes, branchID)
			if index < 0 {
				result.issues = append(result.issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
				return
			}
			result.choices = append(result.choices, model.ExecutionPathChoice{RouteNodeID: node.ID, BranchID: node.ParallelNodes[index].ID})
			walkActualPath(node.ParallelNodes[index].Child, values, selected, result, visited, active)
		}
		walkActualPath(node.Child, values, selected, result, visited, active)
		return
	}
	walkActualPath(node.Child, values, selected, result, visited, active)
}

// mergeCandidates 合并调用方提供的目标真实候选，不引入任何表单数据包装或生成规则。
func mergeCandidates(primary, secondary map[string][]any) map[string][]any {
	if len(primary) == 0 && len(secondary) == 0 {
		return nil
	}
	result := make(map[string][]any, len(primary)+len(secondary))
	for path, values := range primary {
		result[path] = append(result[path], values...)
	}
	for path, values := range secondary {
		result[path] = append(result[path], values...)
	}
	return result
}

// ApplyMinimalPatch 是 Apply 的语义别名，方便调用方明确表达只允许窄范围改写。
func ApplyMinimalPatch(input Input) Result {
	return Apply(input)
}

// Resolve 是 Apply 的语义别名，供只需要分支复验的调用方复用同一目标语义。
func Resolve(input Input) Result {
	return Apply(input)
}

type conditionReference struct {
	RouteNodeID string
	BranchID    string
	Condition   target.FlowCondition
	Paths       []string
	Selected    bool
}

type patchVariable struct {
	path       string
	current    any
	currentOK  bool
	candidates []any
}

type patchSolution struct {
	values       map[string]any
	changedCount int
	offset       *big.Rat
	paths        []string
	encoded      string
}

type treeWalk struct {
	complete bool
	matches  bool
	choices  []model.ExecutionPathChoice
	issues   []Issue
}

// choiceMap 校验路径选择的路由唯一性并保留真实不透明分支键。
func choiceMap(choices []model.ExecutionPathChoice) (map[string]string, []Issue) {
	result := make(map[string]string, len(choices))
	issues := make([]Issue, 0)
	for _, choice := range choices {
		route := strings.TrimSpace(choice.RouteNodeID)
		branch := strings.TrimSpace(choice.BranchID)
		if route == "" || branch == "" {
			issues = append(issues, Issue{Code: "choice_missing", Message: "路径分支选择缺少真实路由或分支标识"})
			continue
		}
		if previous, exists := result[route]; exists {
			if previous != branch {
				issues = append(issues, Issue{Code: "choice_duplicate", BranchKey: branch, Message: "同一路由存在多个分支选择"})
			}
			continue
		}
		result[route] = branch
	}
	return result, uniqueIssues(issues)
}

// collectReferences 收集当前目标路径真正会计算的条件字段，不触碰后续未达分支。
func collectReferences(tree *target.FlowNodeTemplate, selected map[string]string) ([]conditionReference, []Issue) {
	references := make([]conditionReference, 0)
	issues := make([]Issue, 0)
	visited := make(map[string]bool)
	active := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil {
			return
		}
		key := nodeVisitKey(node)
		if active[key] {
			issues = append(issues, Issue{Code: "flow_cycle", Message: "目标流程树存在循环，无法复验路径"})
			return
		}
		if visited[key] {
			return
		}
		active[key] = true
		defer delete(active, key)
		visited[key] = true
		if len(node.ConditionNodes) > 0 {
			branches := orderedBranches(node.ConditionNodes)
			branchID, exists := selected[node.ID]
			if !exists {
				issues = append(issues, Issue{Code: "choice_missing", Path: node.ID, Message: "当前路径缺少条件分支选择"})
				return
			}
			index := branchIndex(branches, branchID)
			if index < 0 {
				issues = append(issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
				return
			}
			manual := isManualNode(node)
			if !manual {
				last := len(branches) - 1
				for branchIndex := 0; branchIndex <= index && branchIndex < last; branchIndex++ {
					for _, condition := range branches[branchIndex].Conditions {
						references = append(references, newReference(node.ID, branches[branchIndex].ID, condition, branchIndex == index))
					}
				}
			}
			visit(branches[index].Child)
			visit(node.Child)
			return
		}
		if len(node.ParallelNodes) > 0 {
			if strings.EqualFold(strings.TrimSpace(node.Type), "parallel") {
				for _, branch := range orderedBranches(node.ParallelNodes) {
					visit(branch.Child)
				}
			} else {
				branchID, exists := selected[node.ID]
				if !exists {
					issues = append(issues, Issue{Code: "choice_missing", Path: node.ID, Message: "当前路径缺少并行候选分支选择"})
					return
				}
				index := branchIndex(node.ParallelNodes, branchID)
				if index < 0 {
					issues = append(issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
					return
				}
				visit(node.ParallelNodes[index].Child)
			}
			visit(node.Child)
			return
		}
		visit(node.Child)
	}
	visit(tree)
	return references, uniqueIssues(issues)
}

// extraChoiceIssues 拒绝不属于本次目标流程的额外选择，避免浏览器注入不可达分支。
func extraChoiceIssues(tree *target.FlowNodeTemplate, selected map[string]string) []Issue {
	routes := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil {
			return
		}
		key := nodeVisitKey(node)
		if visited[key] {
			return
		}
		visited[key] = true
		if len(node.ConditionNodes) > 0 || len(node.ParallelNodes) > 0 && !strings.EqualFold(strings.TrimSpace(node.Type), "parallel") {
			routes[node.ID] = true
		}
		if len(node.ConditionNodes) > 0 {
			branches := orderedBranches(node.ConditionNodes)
			if branchID, exists := selected[node.ID]; exists {
				if index := branchIndex(branches, branchID); index >= 0 {
					visit(branches[index].Child)
				}
			}
		} else if len(node.ParallelNodes) > 0 {
			if strings.EqualFold(strings.TrimSpace(node.Type), "parallel") {
				for _, branch := range node.ParallelNodes {
					visit(branch.Child)
				}
			} else if branchID, exists := selected[node.ID]; exists {
				if index := branchIndex(node.ParallelNodes, branchID); index >= 0 {
					visit(node.ParallelNodes[index].Child)
				}
			}
		}
		visit(node.Child)
	}
	visit(tree)
	issues := make([]Issue, 0)
	for route := range selected {
		if !routes[route] {
			issues = append(issues, Issue{Code: "choice_extra", Path: route, Message: "路径包含不属于当前目标流程的额外选择"})
		}
	}
	sort.SliceStable(issues, func(left, right int) bool { return issues[left].Path < issues[right].Path })
	return issues
}

// newReference 记录条件所属真实路由，补丁明细据此保留目标分支键。
func newReference(routeNodeID, branchID string, condition target.FlowCondition, selected bool) conditionReference {
	paths := []string{strings.TrimSpace(condition.FieldA)}
	if strings.TrimSpace(condition.FieldB) != "" && strings.TrimSpace(condition.ValueB) == "" {
		paths = append(paths, strings.TrimSpace(condition.FieldB))
	}
	return conditionReference{RouteNodeID: routeNodeID, BranchID: branchID, Condition: condition, Paths: paths, Selected: selected}
}

// walkTree 使用目标分支顺序计算实际路径，并把显式选择与实际命中逐节点比较。
func walkTree(tree *target.FlowNodeTemplate, values map[string]any, selected map[string]string) treeWalk {
	result := treeWalk{complete: true, matches: true, choices: []model.ExecutionPathChoice{}}
	visited := make(map[string]bool)
	active := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil || !result.complete {
			return
		}
		key := nodeVisitKey(node)
		if active[key] {
			result.complete = false
			result.matches = false
			result.issues = append(result.issues, Issue{Code: "flow_cycle", Message: "目标流程树存在循环，无法复验路径"})
			return
		}
		if visited[key] {
			return
		}
		active[key] = true
		defer delete(active, key)
		visited[key] = true
		if len(node.ConditionNodes) > 0 {
			branches := orderedBranches(node.ConditionNodes)
			branchID, selectedOK := selected[node.ID]
			if !selectedOK {
				result.complete = false
				result.matches = false
				result.issues = append(result.issues, Issue{Code: "choice_missing", Path: node.ID, Message: "当前路径缺少条件分支选择"})
				return
			}
			branchIndex := branchIndex(branches, branchID)
			if branchIndex < 0 {
				result.complete = false
				result.matches = false
				result.issues = append(result.issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
				return
			}
			var branch target.FlowBranchTemplate
			var actualOK Evaluation
			if isManualNode(node) {
				branch = branches[branchIndex]
				actualOK = Evaluation{Satisfied: true, Evaluable: true}
			} else {
				branch, actualOK = chooseConditionBranch(branches, values, false)
			}
			if !actualOK.Evaluable {
				result.complete = false
				result.matches = false
				result.issues = append(result.issues, Issue{Code: "condition_unavailable", Path: node.ID, BranchKey: branchID, Message: actualOK.Reason})
				return
			}
			if branch.ID != branchID {
				result.matches = false
				result.issues = append(result.issues, Issue{Code: "branch_mismatch", Path: node.ID, BranchKey: branchID, Message: "目标实际命中分支与路径选择不一致"})
			}
			result.choices = append(result.choices, model.ExecutionPathChoice{RouteNodeID: node.ID, BranchID: branch.ID})
			visit(branch.Child)
			visit(node.Child)
			return
		}
		if len(node.ParallelNodes) > 0 {
			if strings.EqualFold(strings.TrimSpace(node.Type), "parallel") {
				for _, branch := range orderedBranches(node.ParallelNodes) {
					visit(branch.Child)
				}
			} else {
				branchID, selectedOK := selected[node.ID]
				if !selectedOK {
					result.complete = false
					result.matches = false
					result.issues = append(result.issues, Issue{Code: "choice_missing", Path: node.ID, Message: "当前路径缺少候选分支选择"})
					return
				}
				index := branchIndex(node.ParallelNodes, branchID)
				if index < 0 {
					result.complete = false
					result.matches = false
					result.issues = append(result.issues, Issue{Code: "choice_invalid", Path: node.ID, BranchKey: branchID, Message: "路径分支选择不属于当前目标路由"})
					return
				}
				result.choices = append(result.choices, model.ExecutionPathChoice{RouteNodeID: node.ID, BranchID: node.ParallelNodes[index].ID})
				visit(node.ParallelNodes[index].Child)
			}
			visit(node.Child)
			return
		}
		visit(node.Child)
	}
	visit(tree)
	return result
}

// chooseConditionBranch 执行首个命中和最后分支兜底，手动分支只接受显式选择。
func chooseConditionBranch(branches []target.FlowBranchTemplate, values map[string]any, manual bool) (target.FlowBranchTemplate, Evaluation) {
	if len(branches) == 0 {
		return target.FlowBranchTemplate{}, unevaluable("目标条件分支为空")
	}
	if manual {
		return target.FlowBranchTemplate{}, unevaluable("手动分支需要显式选择")
	}
	for index := 0; index < len(branches)-1; index++ {
		evaluation := EvaluateConditions(branches[index].Conditions, values)
		if !evaluation.Evaluable {
			return target.FlowBranchTemplate{}, evaluation
		}
		if evaluation.Satisfied {
			return branches[index], Evaluation{Satisfied: true, Evaluable: true}
		}
	}
	return branches[len(branches)-1], Evaluation{Satisfied: true, Evaluable: true}
}

// isManualNode 识别目标 custom_choose 条件节点，禁止用表单值替代人工分支选择。
func isManualNode(node *target.FlowNodeTemplate) bool {
	return node != nil && strings.EqualFold(strings.TrimSpace(node.Type), "condition") && strings.EqualFold(strings.TrimSpace(node.BranchExecuteType), "custom_choose")
}

// orderedBranches 按目标 sort 稳定排序，同序时保留接口返回顺序。
func orderedBranches(branches []target.FlowBranchTemplate) []target.FlowBranchTemplate {
	ordered := append([]target.FlowBranchTemplate(nil), branches...)
	sort.SliceStable(ordered, func(left, right int) bool { return ordered[left].Sort < ordered[right].Sort })
	return ordered
}

// branchIndex 用真实分支 ID 定位目标分支，不按名称或显示标签猜测。
func branchIndex(branches []target.FlowBranchTemplate, branchID string) int {
	for index := range branches {
		if strings.TrimSpace(branches[index].ID) == strings.TrimSpace(branchID) {
			return index
		}
	}
	return -1
}

// walkCanBecomeEvaluable 判断当前未命中是否可能仅通过条件字段候选恢复。
func walkCanBecomeEvaluable(walk treeWalk, references []conditionReference) bool {
	if walk.complete {
		return false
	}
	if len(references) == 0 {
		return false
	}
	for _, issue := range walk.issues {
		if issue.Code == "flow_cycle" || issue.Code == "choice_missing" || issue.Code == "choice_invalid" {
			return false
		}
	}
	return true
}

// buildVariables 构造真实条件字段的有限候选域，不创建不存在的业务值。
func buildVariables(values map[string]any, provided map[string][]any, references []conditionReference) ([]patchVariable, []Issue) {
	paths := make(map[string]bool)
	for _, reference := range references {
		for _, path := range reference.Paths {
			if strings.TrimSpace(path) != "" {
				paths[strings.TrimSpace(path)] = true
			}
		}
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)
	variables := make([]patchVariable, 0, len(orderedPaths))
	issues := make([]Issue, 0)
	for _, path := range orderedPaths {
		current, currentOK := getPath(values, path)
		candidates := make([]any, 0, maxCandidatesPerPath)
		if currentOK {
			candidates = append(candidates, current)
		}
		for _, value := range provided[path] {
			candidates = append(candidates, value)
		}
		for _, reference := range references {
			if !containsPath(reference.Paths, path) {
				continue
			}
			condition := reference.Condition
			if strings.TrimSpace(condition.FieldA) == path && strings.TrimSpace(condition.ValueB) != "" {
				var constant any = condition.ValueB
				if isBoundaryJudge(normalizeJudge(condition.Judge)) {
					constant = adaptNumericCandidate(constant, current, currentOK)
				}
				candidates = append(candidates, constant)
				for _, boundary := range numericBoundaries(condition.ValueB, condition.Judge) {
					candidates = append(candidates, adaptNumericCandidate(boundary, current, currentOK))
				}
			}
			if strings.TrimSpace(condition.FieldB) != "" && strings.TrimSpace(condition.ValueB) == "" {
				if other, found := getPath(values, strings.TrimSpace(condition.FieldA)); found {
					candidates = append(candidates, other)
				}
			}
		}
		candidates = dedupeCandidates(candidates)
		if len(candidates) > maxCandidatesPerPath {
			candidates = candidates[:maxCandidatesPerPath]
		}
		if len(candidates) == 0 {
			issues = append(issues, Issue{Code: "candidate_missing", Path: path, Message: "条件字段没有历史值或目标真实候选值"})
			continue
		}
		variables = append(variables, patchVariable{path: path, current: current, currentOK: currentOK, candidates: candidates})
	}
	return variables, uniqueIssues(issues)
}

// adaptNumericCandidate 保持条件边界与历史字段的 JSON 数值形态，避免同值字符串制造伪多解。
func adaptNumericCandidate(value any, current any, currentOK bool) any {
	if !currentOK {
		return value
	}
	rat, ok := toBigDecimal(value)
	if !ok {
		return value
	}
	text := formatRat(rat)
	switch current.(type) {
	case float32:
		parsed, err := strconv.ParseFloat(text, 32)
		if err == nil {
			return float32(parsed)
		}
	case float64:
		parsed, err := strconv.ParseFloat(text, 64)
		if err == nil {
			return parsed
		}
	case int:
		parsed, err := strconv.ParseInt(text, 10, 0)
		if err == nil {
			return int(parsed)
		}
	case int8:
		parsed, err := strconv.ParseInt(text, 10, 8)
		if err == nil {
			return int8(parsed)
		}
	case int16:
		parsed, err := strconv.ParseInt(text, 10, 16)
		if err == nil {
			return int16(parsed)
		}
	case int32:
		parsed, err := strconv.ParseInt(text, 10, 32)
		if err == nil {
			return int32(parsed)
		}
	case int64:
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err == nil {
			return parsed
		}
	case uint:
		parsed, err := strconv.ParseUint(text, 10, 0)
		if err == nil {
			return uint(parsed)
		}
	case uint8:
		parsed, err := strconv.ParseUint(text, 10, 8)
		if err == nil {
			return uint8(parsed)
		}
	case uint16:
		parsed, err := strconv.ParseUint(text, 10, 16)
		if err == nil {
			return uint16(parsed)
		}
	case uint32:
		parsed, err := strconv.ParseUint(text, 10, 32)
		if err == nil {
			return uint32(parsed)
		}
	case uint64:
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err == nil {
			return parsed
		}
	case json.Number:
		return json.Number(text)
	}
	return value
}

// searchSolutions 在固定字段数量和尝试次数内枚举可复验的目标路径解。
func searchSolutions(tree *target.FlowNodeTemplate, original map[string]any, selected map[string]string, variables []patchVariable) ([]patchSolution, int) {
	solutions := make([]patchSolution, 0)
	attempts := 0
	assignment := make(map[string]any, len(variables))
	var visit func(int)
	visit = func(index int) {
		if attempts >= maxSearchAttempts {
			return
		}
		if index < len(variables) {
			for _, candidate := range variables[index].candidates {
				assignment[variables[index].path] = candidate
				visit(index + 1)
				if attempts >= maxSearchAttempts {
					return
				}
			}
			return
		}
		attempts++
		values, err := cloneMap(original)
		if err != nil {
			return
		}
		for path, value := range assignment {
			if !setPath(values, path, value) {
				return
			}
		}
		walk := walkTree(tree, values, selected)
		if !walk.complete || !walk.matches {
			return
		}
		changedCount, offset, paths := patchRank(original, values, variables)
		encoded, err := json.Marshal(values)
		if err != nil {
			return
		}
		solutions = append(solutions, patchSolution{values: values, changedCount: changedCount, offset: offset, paths: paths, encoded: string(encoded)})
	}
	visit(0)
	return dedupeSolutions(solutions), attempts
}

// patchRank 以修改字段数、值偏移和字段路径顺序构造稳定最小补丁排序键。
func patchRank(original, values map[string]any, variables []patchVariable) (int, *big.Rat, []string) {
	changed := 0
	offset := new(big.Rat)
	paths := make([]string, 0)
	for _, variable := range variables {
		before, beforeOK := getPath(original, variable.path)
		after, afterOK := getPath(values, variable.path)
		if beforeOK == afterOK && valuesEqual(before, after) {
			continue
		}
		changed++
		paths = append(paths, variable.path)
		if beforeOK && afterOK {
			if left, leftOK := toBigDecimal(before); leftOK {
				if right, rightOK := toBigDecimal(after); rightOK {
					distance := new(big.Rat).Sub(left, right)
					if distance.Sign() < 0 {
						distance.Neg(distance)
					}
					offset.Add(offset, distance)
					continue
				}
			}
		}
		offset.Add(offset, big.NewRat(1, 1))
	}
	sort.Strings(paths)
	return changed, offset, paths
}

// chooseSolution 选择最少字段、最小偏移并按字段路径稳定排序的解。
func chooseSolution(solutions []patchSolution) (patchSolution, bool) {
	if len(solutions) == 0 {
		return patchSolution{}, false
	}
	sort.SliceStable(solutions, func(left, right int) bool {
		if solutions[left].changedCount != solutions[right].changedCount {
			return solutions[left].changedCount < solutions[right].changedCount
		}
		if comparison := solutions[left].offset.Cmp(solutions[right].offset); comparison != 0 {
			return comparison < 0
		}
		return strings.Join(solutions[left].paths, "\x00") < strings.Join(solutions[right].paths, "\x00")
	})
	best := solutions[0]
	for _, candidate := range solutions[1:] {
		if candidate.changedCount != best.changedCount || candidate.offset.Cmp(best.offset) != 0 {
			break
		}
		if strings.Join(candidate.paths, "\x00") == strings.Join(best.paths, "\x00") && candidate.encoded != best.encoded {
			return patchSolution{}, true
		}
	}
	return best, false
}

// dedupeSolutions 按完整原始 JSON 结果去重，避免相同补丁被不同候选来源重复计数。
func dedupeSolutions(solutions []patchSolution) []patchSolution {
	seen := make(map[string]bool, len(solutions))
	result := make([]patchSolution, 0, len(solutions))
	for _, solution := range solutions {
		if seen[solution.encoded] {
			continue
		}
		seen[solution.encoded] = true
		result = append(result, solution)
	}
	return result
}

// buildPatches 只输出实际变更的目标字段，普通历史正文不会进入补丁列表。
func buildPatches(original, values map[string]any, references []conditionReference) []model.HistoryBranchPatch {
	paths := make(map[string]string)
	for _, reference := range references {
		for _, path := range reference.Paths {
			if _, exists := paths[path]; !exists || reference.Selected {
				paths[path] = reference.BranchID
			}
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	patches := make([]model.HistoryBranchPatch, 0, len(ordered))
	for _, path := range ordered {
		before, beforeOK := getPath(original, path)
		after, afterOK := getPath(values, path)
		if beforeOK == afterOK && valuesEqual(before, after) {
			continue
		}
		patches = append(patches, model.HistoryBranchPatch{Path: path, Before: cloneAny(before), After: cloneAny(after), Reason: "为命中目标分支条件的最小补丁", BranchKey: paths[path]})
	}
	return patches
}

// referenceCycle 检查字段对字段条件的有向环，禁止以任意求值顺序打破环。
func referenceCycle(references []conditionReference) string {
	edges := make(map[string][]string)
	for _, reference := range references {
		condition := reference.Condition
		left := strings.TrimSpace(condition.FieldA)
		right := strings.TrimSpace(condition.FieldB)
		if left == "" || right == "" || strings.TrimSpace(condition.ValueB) != "" {
			continue
		}
		edges[left] = append(edges[left], right)
	}
	state := make(map[string]uint8)
	stack := make([]string, 0)
	var visit func(string) string
	visit = func(path string) string {
		if state[path] == 1 {
			for index := range stack {
				if stack[index] == path {
					return strings.Join(append(stack[index:], path), " -> ")
				}
			}
			return path
		}
		if state[path] == 2 {
			return ""
		}
		state[path] = 1
		stack = append(stack, path)
		for _, next := range edges[path] {
			if cycle := visit(next); cycle != "" {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		state[path] = 2
		return ""
	}
	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if cycle := visit(key); cycle != "" {
			return cycle
		}
	}
	return ""
}

// hasBooleanCondition 判断目标路径是否包含当前实现不可满足的 boolean_value 条件。
func hasBooleanCondition(references []conditionReference) bool {
	for _, reference := range references {
		if normalizeJudge(reference.Condition.Judge) == "boolean_value" {
			return true
		}
	}
	return false
}

// numericBoundaries 只为数值条件提供常量自身及相邻整数边界，不生成业务随机值。
func numericBoundaries(raw, judge string) []any {
	if !isBoundaryJudge(normalizeJudge(judge)) {
		return nil
	}
	value, scale, ok := toBigDecimalWithScale(raw)
	if !ok {
		return nil
	}
	step := big.NewRat(1, 1)
	if scale > 0 {
		step = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil))
	} else if scale < 0 {
		step = new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-scale)), nil))
	}
	return []any{formatRat(new(big.Rat).Sub(value, step)), formatRat(new(big.Rat).Add(value, step))}
}

// isBoundaryJudge 判断操作符是否允许构造相邻数值边界。
func isBoundaryJudge(judge string) bool {
	switch judge {
	case "lt", "gt", "lte", "gte":
		return true
	default:
		return false
	}
}

// formatRat 将有限十进制候选稳定输出为目标可接受的字符串值。
func formatRat(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	text := value.FloatString(18)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	return text
}

// dedupeCandidates 按 JSON 值去重并保留历史值优先的来源顺序。
func dedupeCandidates(values []any) []any {
	seen := make(map[string]bool, len(values))
	result := make([]any, 0, len(values))
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			continue
		}
		key := string(encoded)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

// containsPath 判断条件路径是否属于当前补丁字段。
func containsPath(paths []string, wanted string) bool {
	for _, path := range paths {
		if strings.TrimSpace(path) == strings.TrimSpace(wanted) {
			return true
		}
	}
	return false
}

// uniqueIssues 去除重复的结构化问题并保留首次出现顺序。
func uniqueIssues(issues []Issue) []Issue {
	seen := make(map[string]bool, len(issues))
	result := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", issue.Code, issue.Path, issue.BranchKey, issue.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, issue)
	}
	return result
}

// appendIssue 在结果中追加稳定问题并保持 needs_input 状态。
func appendIssue(result Result, issue Issue) Result {
	result.Issues = uniqueIssues(append(result.Issues, issue))
	result.Status = StatusNeedsInput
	return result
}

// needsInputResult 构造原始数据复制失败时的不可执行结果。
func needsInputResult(values map[string]any, issue Issue) Result {
	return Result{Status: StatusNeedsInput, Values: values, Patches: []model.HistoryBranchPatch{}, Issues: []Issue{issue}}
}

// nodeVisitKey 为缺少目标 ID 的测试结构提供仅限本次遍历的稳定指针键。
func nodeVisitKey(node *target.FlowNodeTemplate) string {
	if id := strings.TrimSpace(node.ID); id != "" {
		return "id:" + id
	}
	return fmt.Sprintf("ptr:%p", node)
}

// cloneMap 通过 JSON 深复制原始业务数据，复制失败时不降级为空对象。
// 复制使用 UseNumber，避免 float64 抹掉目标数字字面量的小数位而改变 eq 结果。
func cloneMap(values map[string]any) (map[string]any, error) {
	return jsonvalues.DeepCopyObject(values)
}

// cloneAny 深复制补丁前后值，无法 JSON 编码的值按 nil 处理以避免共享引用。
func cloneAny(value any) any {
	return jsonvalues.DeepCopyValue(value)
}

// valuesEqual 按原始 JSON 结构比较值，类型变化也必须记录为字段改动。
func valuesEqual(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

// setPath 只写入条件声明的精确 JSON 路径，不创建缺失的嵌套对象或数组行。
func setPath(values map[string]any, path string, value any) bool {
	tokens, ok := parsePath(path)
	if !ok || len(tokens) == 0 {
		return false
	}
	var current any = values
	for index, token := range tokens {
		last := index == len(tokens)-1
		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[token.key]
			if token.index != nil {
				if !exists {
					return false
				}
				list, listOK := asAnySlice(next)
				if !listOK || *token.index < 0 || *token.index >= len(list) {
					return false
				}
				if last {
					list[*token.index] = cloneAny(value)
					return true
				}
				current = list[*token.index]
				continue
			}
			if last {
				typed[token.key] = cloneAny(value)
				return true
			}
			if !exists {
				return false
			}
			current = next
		case []any:
			if token.first {
				if len(typed) == 0 {
					return false
				}
				if last {
					typed[0] = cloneAny(value)
					return true
				}
				current = typed[0]
				continue
			}
			if token.index == nil || *token.index < 0 || *token.index >= len(typed) {
				return false
			}
			if last {
				typed[*token.index] = cloneAny(value)
				return true
			}
			current = typed[*token.index]
		default:
			return false
		}
	}
	return false
}

type pathToken struct {
	key   string
	index *int
	first bool
}

// getPath 读取点路径、JSON 指针和显式数组下标，严格区分缺失与 null。
func getPath(values map[string]any, path string) (any, bool) {
	tokens, ok := parsePath(path)
	if !ok || len(tokens) == 0 {
		return nil, false
	}
	var current any = values
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			next, exists := typed[token.key]
			if !exists {
				return nil, false
			}
			if token.index != nil {
				list, listOK := asAnySlice(next)
				if !listOK || *token.index < 0 || *token.index >= len(list) {
					return nil, false
				}
				current = list[*token.index]
				continue
			}
			current = next
		case []any:
			if token.first {
				if len(typed) == 0 {
					return nil, false
				}
				current = typed[0]
				continue
			}
			if token.index == nil || *token.index < 0 || *token.index >= len(typed) {
				return nil, false
			}
			current = typed[*token.index]
		default:
			return nil, false
		}
	}
	return current, true
}

// parsePath 解析目标字段的精确路径表示，不把标签或相似名称转换为字段键。
func parsePath(path string) ([]pathToken, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	if strings.HasPrefix(path, "/") {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		result := make([]pathToken, 0, len(parts))
		for index, part := range parts {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			if part == "" {
				return nil, false
			}
			if index > 0 {
				if parsed, err := strconv.Atoi(part); err == nil && parsed >= 0 {
					result = append(result, pathToken{index: &parsed})
					continue
				}
			}
			result = append(result, pathToken{key: part})
		}
		return result, true
	}
	parts := strings.Split(path, ".")
	result := make([]pathToken, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		for {
			open := strings.IndexByte(part, '[')
			if open < 0 {
				result = append(result, pathToken{key: part})
				break
			}
			if open == 0 {
				return nil, false
			}
			key := part[:open]
			close := strings.IndexByte(part[open:], ']')
			if close < 0 {
				return nil, false
			}
			close += open
			result = append(result, pathToken{key: key})
			indexText := part[open+1 : close]
			if indexText == "" {
				result = append(result, pathToken{first: true})
				part = part[close+1:]
				if part == "" {
					break
				}
				continue
			}
			parsed, err := strconv.Atoi(indexText)
			if err != nil || parsed < 0 {
				return nil, false
			}
			result = append(result, pathToken{index: &parsed})
			part = part[close+1:]
			if part == "" {
				break
			}
		}
	}
	return result, len(result) > 0
}

// asAnySlice 只接受 JSON 数组，避免把字符串或其他对象误当成集合字段。
func asAnySlice(value any) ([]any, bool) {
	list, ok := value.([]any)
	return list, ok
}
