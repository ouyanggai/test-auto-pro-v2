package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
)

const maxPathFormSolveAttempts = 20000

type pathFormSolveVariable struct {
	field      formdata.Field
	candidates []any
}

type pathFormSolveResult struct {
	values  map[string]any
	issues  []model.PathFormGenerationIssue
	matched bool
	reason  string
}

// solveTargetPathValues 在模板真实候选构成的有界空间内确定性搜索完整路径解。
func solveTargetPathValues(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template, base map[string]any, seed int64) pathFormSolveResult {
	values := cloneFormValues(base)
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	conditions := selectedPathSolveConditions(tree, selected)
	fields, _ := formdata.ParseTemplate(template)
	fieldByPath := make(map[string]formdata.Field, len(fields)*3)
	for _, field := range fields {
		fieldByPath[normalizeFormFieldPath(field.Path)] = field
		fieldByPath[normalizeFormFieldPath(field.Path+"__virtualName")] = field
		fieldByPath[normalizeFormFieldPath(field.Path+"__condition")] = field
	}
	issues := make([]model.PathFormGenerationIssue, 0)
	constants := make(map[string][]any)
	variables := make(map[string]formdata.Field)
	for _, condition := range conditions {
		op := normalizeConditionJudge(condition.Judge)
		if !supportedPathSolveOperator(op) {
			issues = appendPathSolveIssue(issues, "当前路径条件", "存在无法安全计算的比较方式", true)
			continue
		}
		leftPath := normalizeFormFieldPath(condition.FieldA)
		left, ok := fieldByPath[leftPath]
		if !ok {
			issues = appendPathSolveIssue(issues, "当前路径条件", "条件字段无法与真实表单精确对应", true)
			continue
		}
		variables[left.Path] = left
		if strings.TrimSpace(condition.FieldB) != "" {
			rightPath := normalizeFormFieldPath(condition.FieldB)
			right, rightOK := fieldByPath[rightPath]
			if !rightOK {
				issues = appendPathSolveIssue(issues, left.Name, "比较字段无法与真实表单精确对应", true)
				continue
			}
			variables[right.Path] = right
			continue
		}
		constants[left.Path] = append(constants[left.Path], pathConditionValue(condition.ValueB))
	}
	if len(variables) == 0 {
		reasons := validateTargetPathSelection(tree, choices, values)
		return pathFormSolveResult{values: values, issues: issues, matched: len(reasons) == 0, reason: firstPathSolveReason(reasons)}
	}
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		issues = appendPathSolveIssue(issues, "当前路径条件", "关联条件字段过多，需要人工核对", true)
		return pathFormSolveResult{values: values, issues: issues, reason: "条件组合超过安全求解范围"}
	}
	search := make([]pathFormSolveVariable, 0, len(keys))
	for _, key := range keys {
		field := variables[key]
		candidates := pathSolveCandidates(field, values, constants[key])
		if len(candidates) == 0 {
			issues = appendPathSolveIssue(issues, field.Name, "没有可用于当前路径的安全候选值", true)
			continue
		}
		rotatePathSolveCandidates(candidates, seed+int64(len(search)))
		search = append(search, pathFormSolveVariable{field: field, candidates: candidates})
	}
	if len(search) != len(keys) {
		return pathFormSolveResult{values: values, issues: issues, reason: "部分条件字段没有安全候选值"}
	}
	attempts := 0
	assignment := make(map[string]any, len(search))
	var solved map[string]any
	var visit func(int) bool
	visit = func(index int) bool {
		if attempts >= maxPathFormSolveAttempts {
			return false
		}
		if index < len(search) {
			for _, candidate := range search[index].candidates {
				assignment[search[index].field.Path] = candidate
				if visit(index + 1) {
					return true
				}
			}
			return false
		}
		attempts++
		candidateValues := cloneFormValues(values)
		for path, value := range assignment {
			setPathFormValue(candidateValues, path, value)
		}
		formdata.ApplyVirtualValues(template, candidateValues)
		if len(validateTargetPathSelection(tree, choices, candidateValues)) == 0 {
			solved = candidateValues
			return true
		}
		return false
	}
	if visit(0) {
		return pathFormSolveResult{values: solved, issues: issues, matched: true, reason: "生成数据已命中当前完整路径"}
	}
	issues = appendPathSolveIssue(issues, "当前路径条件", "现有模板候选无法同时避开更靠前分支并命中当前路径", true)
	return pathFormSolveResult{values: values, issues: issues, reason: "未找到可安全命中当前完整路径的数据"}
}

// selectedPathSolveConditions 收集当前路径分支及其所有前置分支条件，供候选域构造。
func selectedPathSolveConditions(tree *target.FlowNodeTemplate, selected map[string]string) []target.FlowCondition {
	result := make([]target.FlowCondition, 0)
	visited := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		if len(node.ConditionNodes) > 0 {
			branches := orderedTargetBranches(node.ConditionNodes)
			if isTargetManualBranchNode(node) {
				for _, branch := range branches {
					if branch.ID == selected[node.ID] {
						visit(branch.Child)
						break
					}
				}
				visit(node.Child)
				return
			}
			for _, branch := range branches {
				result = append(result, branch.Conditions...)
				if branch.ID == selected[node.ID] {
					visit(branch.Child)
					break
				}
			}
			visit(node.Child)
			return
		}
		if len(node.ParallelNodes) > 0 {
			if strings.TrimSpace(node.Type) == "parallel" {
				for _, branch := range node.ParallelNodes {
					visit(branch.Child)
				}
			} else {
				for _, branch := range node.ParallelNodes {
					if branch.ID == selected[node.ID] {
						visit(branch.Child)
						break
					}
				}
			}
			visit(node.Child)
			return
		}
		visit(node.Child)
	}
	visit(tree)
	return result
}

// supportedPathSolveOperator 限制求解器只处理已由目标语义验证的比较方式。
func supportedPathSolveOperator(value string) bool {
	switch value {
	case "eq", "neq", "gt", "gte", "lt", "lte", "contains", "in":
		return true
	default:
		return false
	}
}

// pathSolveCandidates 从真实选项、条件边界和已有值构造稳定有界候选。
func pathSolveCandidates(field formdata.Field, values map[string]any, constants []any) []any {
	result := make([]any, 0, 12)
	if current, ok := pathFormValue(values, field.Path); ok {
		result = append(result, current)
	}
	switch field.Type {
	case "number":
		for _, value := range constants {
			if number, ok := targetNumber(value); ok {
				result = append(result, number, number-1, number+1)
			}
		}
		result = append(result, float64(0), float64(1), float64(2), float64(100))
	case "select", "radio":
		result = append(result, field.Options...)
	case "cascader":
		for _, path := range field.OptionPaths {
			result = append(result, cloneAnyPath(path))
		}
	case "checkbox":
		for _, option := range field.Options {
			result = append(result, []any{option})
		}
	case "switch":
		result = append(result, false, true)
	case "date":
		for _, value := range constants {
			if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(fmt.Sprint(value))); err == nil {
				result = append(result, parsed.AddDate(0, 0, -1).Format("2006-01-02"), parsed.Format("2006-01-02"), parsed.AddDate(0, 0, 1).Format("2006-01-02"))
			}
		}
	case "input", "textarea":
		for _, value := range constants {
			text := strings.TrimSpace(fmt.Sprint(value))
			result = append(result, text, text+"内容")
		}
		result = append(result, "其他值", "")
	case "infoSelect":
		// 身份组件只允许使用生成阶段从真实目录得到的当前值，不编造人员或组织。
	}
	return uniquePathSolveValues(result, 10)
}

// cloneAnyPath 复制级联候选路径，防止搜索回溯修改模板候选数组。
func cloneAnyPath(path []any) []any {
	result := make([]any, len(path))
	copy(result, path)
	return result
}

// uniquePathSolveValues 按 JSON 值去重并限制单字段候选数量，保持内存与组合数有界。
func uniquePathSolveValues(values []any, limit int) []any {
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
		if len(result) == limit {
			break
		}
	}
	return result
}

// rotatePathSolveCandidates 使用稳定 seed 改变候选起点，实现可复现的“换一组”。
func rotatePathSolveCandidates(values []any, seed int64) {
	if len(values) < 2 {
		return
	}
	offset := int(seed % int64(len(values)))
	if offset < 0 {
		offset += len(values)
	}
	rotated := append(append([]any{}, values[offset:]...), values[:offset]...)
	copy(values, rotated)
}

// setPathFormValue 设置精确点路径，不跨数组或按展示名称猜测字段。
func setPathFormValue(values map[string]any, path string, value any) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	var current any = values
	for index, rawPart := range parts {
		part := strings.TrimSuffix(rawPart, "[]")
		isCollection := strings.HasSuffix(rawPart, "[]")
		object, ok := current.(map[string]any)
		if !ok {
			return
		}
		if index == len(parts)-1 {
			object[part] = value
			return
		}
		if isCollection {
			list, listOK := object[part].([]any)
			if !listOK || len(list) == 0 {
				list = []any{map[string]any{}}
				object[part] = list
			}
			current = list[0]
			continue
		}
		next, nextOK := object[part].(map[string]any)
		if !nextOK {
			next = map[string]any{}
			object[part] = next
		}
		current = next
	}
}

// appendPathSolveIssue 去重公开问题，避免同一未知字段重复刷屏。
func appendPathSolveIssue(issues []model.PathFormGenerationIssue, field, reason string, blocking bool) []model.PathFormGenerationIssue {
	for _, issue := range issues {
		if issue.Field == field && issue.Reason == reason {
			return issues
		}
	}
	return append(issues, model.PathFormGenerationIssue{Field: field, Reason: reason, Blocking: blocking})
}

// firstPathSolveReason 返回首个用户可读复验原因。
func firstPathSolveReason(reasons []string) string {
	if len(reasons) == 0 {
		return "生成数据已命中当前完整路径"
	}
	return reasons[0]
}

// targetComparableNumber 同时支持普通数字和目标表单标准日期比较。
func targetComparableNumber(value any) (float64, bool) {
	if number, ok := targetNumber(value); ok {
		return number, true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return float64(parsed.Unix()), true
		}
	}
	return 0, false
}

// pathSolveDebugValue 仅为测试失败摘要提供稳定标量，不进入公开 API。
func pathSolveDebugValue(value any) string {
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}
