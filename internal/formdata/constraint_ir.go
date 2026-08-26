package formdata

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	// ConstraintStatusReady 表示约束可由静态规则完整证明。
	ConstraintStatusReady = "ready"
	// ConstraintStatusNeedsAttention 表示约束包含动态或未知语义，需要人工核对。
	ConstraintStatusNeedsAttention = "needs_attention"
	// ConstraintStatusBlocked 表示约束已经静态证明不可满足。
	ConstraintStatusBlocked = "blocked"
)

// ConstraintIssue 是编译、传播和复验共用的结构化问题。
type ConstraintIssue struct {
	Code          string   `json:"code"`
	Status        string   `json:"status"`
	Source        string   `json:"source"`
	FieldPath     string   `json:"fieldPath,omitempty"`
	FieldLabel    string   `json:"fieldLabel,omitempty"`
	Operator      string   `json:"operator,omitempty"`
	Expected      any      `json:"expected,omitempty"`
	Actual        any      `json:"actual,omitempty"`
	RelatedFields []string `json:"relatedFields"`
	Message       string   `json:"message"`
	CanRetry      bool     `json:"canRetry"`
}

// NumericDomain 描述数值字段传播后的闭开区间。
type NumericDomain struct {
	Min          *float64 `json:"min,omitempty"`
	Max          *float64 `json:"max,omitempty"`
	MinInclusive bool     `json:"minInclusive"`
	MaxInclusive bool     `json:"maxInclusive"`
}

// FieldDomain 描述字段可编辑性、必填性及静态候选域。
type FieldDomain struct {
	Field    Field          `json:"field"`
	Numeric  *NumericDomain `json:"numeric,omitempty"`
	Allowed  []any          `json:"allowed,omitempty"`
	Required bool           `json:"required"`
	Editable bool           `json:"editable"`
	// Constrained 表示字段被当前路径约束直接收敛，传播不得改写无关字段。
	Constrained bool `json:"constrained"`
}

// ConstraintIR 是 Field、Constraint 与日期绑定编译后的唯一可执行投影。
type ConstraintIR struct {
	Fields            []Field                `json:"fields"`
	Constraints       []Constraint           `json:"constraints"`
	DateRangeBindings []DateRangeBinding     `json:"dateRangeBindings"`
	Domains           map[string]FieldDomain `json:"domains"`
	Order             []string               `json:"order"`
	Status            string                 `json:"status"`
	Issues            []ConstraintIssue      `json:"issues"`
}

// ConstraintPropagationResult 是一次确定性传播的 values、所有权和复验结果。
type ConstraintPropagationResult struct {
	Values              map[string]any    `json:"values"`
	GeneratedFieldPaths []string          `json:"generatedFieldPaths"`
	Status              string            `json:"status"`
	Source              string            `json:"source"`
	Issues              []ConstraintIssue `json:"issues"`
}

// CompileConstraintIR 编译字段域、静态依赖 DAG 和日期绑定；动态语义只记录问题，不猜测执行。
func CompileConstraintIR(fields []Field, constraints []Constraint, bindings []DateRangeBinding, editablePaths map[string]bool) ConstraintIR {
	stableFields := append([]Field(nil), fields...)
	sort.SliceStable(stableFields, func(left, right int) bool { return stableFields[left].Path < stableFields[right].Path })
	ir := ConstraintIR{
		Fields: stableFields, Constraints: append([]Constraint(nil), constraints...),
		DateRangeBindings: append([]DateRangeBinding(nil), bindings...), Domains: map[string]FieldDomain{},
		Order: []string{}, Status: ConstraintStatusReady, Issues: []ConstraintIssue{},
	}
	for _, field := range stableFields {
		editable := editablePaths == nil || editablePaths[field.Path]
		domain := FieldDomain{Field: field, Required: field.Required && editable, Editable: editable}
		if field.Type == "number" {
			domain.Numeric = &NumericDomain{Min: cloneNumber(field.Min), Max: cloneNumber(field.Max), MinInclusive: true, MaxInclusive: true}
		}
		if field.Type == "cascader" && len(field.OptionPaths) > 0 {
			domain.Allowed = make([]any, 0, len(field.OptionPaths))
			for _, path := range field.OptionPaths {
				domain.Allowed = append(domain.Allowed, cloneValueList(path))
			}
		} else if len(field.Options) > 0 {
			domain.Allowed = cloneValueList(field.Options)
		}
		ir.Domains[field.Path] = domain
	}
	dependencies := make(map[string][]string)
	for _, constraint := range constraints {
		compileConstraintDomain(&ir, constraint, dependencies)
	}
	for _, binding := range bindings {
		if _, durationOK := ir.Domains[binding.DurationField]; !durationOK {
			ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("date_binding_field_missing", ConstraintStatusNeedsAttention, "compile", binding.DurationField, "", "date_binding", "已声明的天数字段", nil, []string{binding.RangeField}, "日期绑定的天数字段不存在", false))
			continue
		}
		if _, rangeOK := ir.Domains[binding.RangeField]; !rangeOK {
			ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("date_binding_field_missing", ConstraintStatusNeedsAttention, "compile", binding.RangeField, "", "date_binding", "已声明的日期区间字段", nil, []string{binding.DurationField}, "日期绑定的区间字段不存在", false))
			continue
		}
		dependencies[binding.DurationField] = append(dependencies[binding.DurationField], binding.RangeField)
	}
	order, cycle := topologicalConstraintOrder(ir.Domains, dependencies)
	ir.Order = order
	if len(cycle) > 0 {
		ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("dependency_cycle", ConstraintStatusBlocked, "compile", cycle[0], fieldDomainLabel(ir.Domains, cycle[0]), "dependency", "静态无环依赖", cycle, cycle, "字段依赖形成循环，无法确定安全传播顺序", false))
	}
	for path, domain := range ir.Domains {
		if numericDomainEmpty(domain.Numeric) {
			ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("empty_numeric_interval", ConstraintStatusBlocked, "compile", path, domain.Field.Name, "interval", numericDomainText(domain.Numeric), nil, nil, "数值约束交集为空", false))
		}
		if domain.Allowed != nil && len(domain.Allowed) == 0 {
			ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("empty_enum_set", ConstraintStatusBlocked, "compile", path, domain.Field.Name, "set", "至少一个可用枚举值", []any{}, nil, "枚举约束交集为空", false))
		}
	}
	ir.Status = constraintIssuesStatus(ir.Issues)
	return ir
}

// PropagateConstraintIR 按稳定 seed 在静态候选域内传播值，并使用同一 IR 复验结果。
func PropagateConstraintIR(ir ConstraintIR, base map[string]any, seed int64) ConstraintPropagationResult {
	values := cloneMap(base)
	result := ConstraintPropagationResult{Values: values, GeneratedFieldPaths: []string{}, Status: ir.Status, Source: "propagation", Issues: append([]ConstraintIssue(nil), ir.Issues...)}
	if ir.Status == ConstraintStatusBlocked {
		return result
	}
	for index, path := range ir.Order {
		domain, exists := ir.Domains[path]
		if !exists || !domain.Editable {
			continue
		}
		current, hasCurrent := getPath(values, path)
		candidate, changed, ok := propagateFieldCandidate(domain, current, hasCurrent, seed+int64(index))
		if ok && changed {
			setPath(values, path, candidate)
			result.GeneratedFieldPaths = append(result.GeneratedFieldPaths, path)
		}
	}
	for _, constraint := range ir.Constraints {
		if constraint.ValueField == "" || strings.EqualFold(constraint.Op, "required_if") {
			continue
		}
		if propagateDependentConstraint(values, constraint) {
			result.GeneratedFieldPaths = append(result.GeneratedFieldPaths, constraint.Field)
		}
	}
	for _, constraint := range ir.Constraints {
		if !strings.EqualFold(constraint.Op, "required_if") {
			continue
		}
		if right, ok := getPath(values, constraint.ValueField); ok && equalValue(right, constraint.Value) {
			if domain, exists := ir.Domains[constraint.Field]; exists {
				domain.Required = true
				current, hasCurrent := getPath(values, constraint.Field)
				candidate, changed, ok := propagateRequiredCandidate(domain, current, hasCurrent, seed)
				if ok && changed {
					setPath(values, constraint.Field, candidate)
					result.GeneratedFieldPaths = append(result.GeneratedFieldPaths, constraint.Field)
				}
			}
		}
	}
	generated := make([]string, 0, len(ir.DateRangeBindings))
	applyDateRangeBindings(values, ir.DateRangeBindings, map[string]bool{}, &generated)
	result.GeneratedFieldPaths = uniqueSorted(append(result.GeneratedFieldPaths, generated...))
	validationIssues := ValidateConstraintIR(ir, values, "validation")
	result.Issues = appendConstraintIssues(result.Issues, validationIssues)
	result.Status = constraintIssuesStatus(result.Issues)
	return result
}

// ValidateConstraintIR 使用编译后的同一约束语义复验传播或回退结果。
func ValidateConstraintIR(ir ConstraintIR, values map[string]any, source string) []ConstraintIssue {
	issues := append([]ConstraintIssue(nil), ir.Issues...)
	if source == "" {
		source = "validation"
	}
	for _, domain := range ir.Domains {
		value, exists := getPath(values, domain.Field.Path)
		if domain.Required && (!exists || emptyValue(value)) {
			issues = appendConstraintIssue(issues, newConstraintIssue("required_value_missing", ConstraintStatusBlocked, source, domain.Field.Path, domain.Field.Name, "required", "非空值", value, nil, "必填字段缺少值", true))
			continue
		}
		if exists && !emptyValue(value) && !fieldDomainContains(domain, value) {
			issues = appendConstraintIssue(issues, newConstraintIssue("value_outside_domain", ConstraintStatusBlocked, source, domain.Field.Path, domain.Field.Name, "domain", fieldDomainExpected(domain), value, nil, "字段值不在约束候选域内", true))
		}
	}
	for _, constraint := range ir.Constraints {
		if strings.EqualFold(constraint.Op, "required_if") {
			right, rightOK := getPath(values, constraint.ValueField)
			left, leftOK := getPath(values, constraint.Field)
			if rightOK && equalValue(right, constraint.Value) && (!leftOK || emptyValue(left)) {
				issues = appendConstraintIssue(issues, newConstraintIssue("conditional_required_missing", ConstraintStatusBlocked, source, constraint.Field, fieldDomainLabel(ir.Domains, constraint.Field), constraint.Op, constraint.Value, left, []string{constraint.ValueField}, "条件成立时依赖字段必须有值", true))
			}
			continue
		}
		if !supportedConstraintOperator(constraint.Op) || constraint.Group > 0 {
			continue
		}
		left, _ := getPath(values, constraint.Field)
		if !constraintSatisfiedWithValues(values, left, constraint) {
			expected := constraint.Value
			if constraint.ValueField != "" {
				expected, _ = getPath(values, constraint.ValueField)
			}
			issues = appendConstraintIssue(issues, newConstraintIssue("constraint_not_satisfied", ConstraintStatusBlocked, source, constraint.Field, fieldDomainLabel(ir.Domains, constraint.Field), constraint.Op, expected, left, relatedConstraintFields(constraint), "字段值不满足约束", true))
		}
	}
	for _, reason := range ValidateDateRangeBindings(values, ir.DateRangeBindings) {
		issues = appendConstraintIssue(issues, newConstraintIssue("date_binding_mismatch", ConstraintStatusBlocked, source, "", "日期区间", "date_binding", "区间天数与条件天数一致", reason, nil, reason, true))
	}
	return issues
}

// compileConstraintDomain 把单条约束收敛到字段域，并登记静态依赖边。
func compileConstraintDomain(ir *ConstraintIR, constraint Constraint, dependencies map[string][]string) {
	domain, exists := ir.Domains[constraint.Field]
	if !exists {
		ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("constraint_field_missing", ConstraintStatusNeedsAttention, "compile", constraint.Field, "", constraint.Op, constraint.Value, nil, relatedConstraintFields(constraint), "约束字段无法与表单字段精确对应", false))
		return
	}
	op := strings.ToLower(strings.TrimSpace(constraint.Op))
	if !supportedConstraintOperator(op) && op != "required_if" {
		ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("unknown_operator", ConstraintStatusNeedsAttention, "compile", constraint.Field, domain.Field.Name, op, constraint.Value, nil, relatedConstraintFields(constraint), "存在无法静态证明的比较方式", false))
		return
	}
	if constraint.ValueField != "" {
		if _, rightExists := ir.Domains[constraint.ValueField]; !rightExists {
			ir.Issues = appendConstraintIssue(ir.Issues, newConstraintIssue("dependency_field_missing", ConstraintStatusNeedsAttention, "compile", constraint.Field, domain.Field.Name, op, constraint.ValueField, nil, []string{constraint.ValueField}, "依赖字段无法与表单字段精确对应", false))
			return
		}
		dependencies[constraint.ValueField] = append(dependencies[constraint.ValueField], constraint.Field)
		domain.Constrained = op != "required_if"
		ir.Domains[constraint.Field] = domain
		return
	}
	if constraint.Group > 0 {
		// OR 组由目标分支复验负责；不能把其中任一候选提前收敛成全局 AND。
		return
	}
	if domain.Numeric != nil {
		applyNumericConstraint(domain.Numeric, op, constraint.Value)
		domain.Constrained = true
		ir.Domains[constraint.Field] = domain
		return
	}
	applyAllowedConstraint(&domain, op, constraint.Value)
	domain.Constrained = true
	ir.Domains[constraint.Field] = domain
}

// supportedConstraintOperator 判断操作符是否属于当前 IR 与生成器共享的稳定集合。
func supportedConstraintOperator(operator string) bool {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "eq", "neq", "gt", "gte", "lt", "lte", "contains", "in", "default":
		return true
	default:
		return false
	}
}

// applyNumericConstraint 计算数值区间交集，非法常量交给后续复验报告。
func applyNumericConstraint(domain *NumericDomain, operator string, raw any) {
	value, ok := strictNumberValue(raw)
	if !ok {
		return
	}
	switch operator {
	case "eq":
		intersectNumericLower(domain, value, true)
		intersectNumericUpper(domain, value, true)
	case "gt", "gte":
		intersectNumericLower(domain, value, operator == "gte")
	case "lt", "lte":
		intersectNumericUpper(domain, value, operator == "lte")
	}
}

// intersectNumericLower 收紧数值下界，同值时开区间比闭区间更严格。
func intersectNumericLower(domain *NumericDomain, value float64, inclusive bool) {
	if domain.Min == nil || value > *domain.Min || (value == *domain.Min && !inclusive && domain.MinInclusive) {
		domain.Min = numberPointer(value)
		domain.MinInclusive = inclusive
	}
}

// intersectNumericUpper 收紧数值上界，同值时开区间比闭区间更严格。
func intersectNumericUpper(domain *NumericDomain, value float64, inclusive bool) {
	if domain.Max == nil || value < *domain.Max || (value == *domain.Max && !inclusive && domain.MaxInclusive) {
		domain.Max = numberPointer(value)
		domain.MaxInclusive = inclusive
	}
}

// applyAllowedConstraint 计算枚举集合交集；无静态选项的普通字段只收敛 eq/in。
func applyAllowedConstraint(domain *FieldDomain, operator string, raw any) {
	switch operator {
	case "eq":
		domain.Allowed = intersectAllowed(domain.Allowed, []any{raw})
	case "in":
		values, ok := raw.([]any)
		if !ok {
			values = []any{raw}
		}
		domain.Allowed = intersectAllowed(domain.Allowed, values)
	case "neq":
		if domain.Allowed != nil {
			domain.Allowed = removeAllowed(domain.Allowed, raw)
		}
	}
}

// propagateFieldCandidate 为字段选择稳定候选，已有合法值也参与 seed 轮换以支持“换一组”。
func propagateFieldCandidate(domain FieldDomain, current any, hasCurrent bool, seed int64) (any, bool, bool) {
	if domain.Numeric != nil && domain.Constrained {
		candidates := numericDomainCandidates(domain.Numeric)
		if hasCurrent && fieldDomainContains(domain, current) {
			candidates = append([]any{current}, candidates...)
		}
		candidates = uniqueValues(candidates)
		if len(candidates) == 0 {
			return nil, false, false
		}
		chosen := candidates[stableOffset(seed, len(candidates))]
		return chosen, !hasCurrent || !equalValue(current, chosen), true
	}
	if len(domain.Allowed) > 0 && domain.Constrained {
		chosen := domain.Allowed[stableOffset(seed, len(domain.Allowed))]
		return cloneValue(chosen), !hasCurrent || !equalValue(current, chosen), true
	}
	if hasCurrent && !emptyValue(current) {
		return current, false, true
	}
	return propagateRequiredCandidate(domain, current, hasCurrent, seed)
}

// propagateRequiredCandidate 只使用模板默认值或真实枚举补齐必填字段，不编造外部对象。
func propagateRequiredCandidate(domain FieldDomain, current any, hasCurrent bool, seed int64) (any, bool, bool) {
	if hasCurrent && !emptyValue(current) {
		return current, false, true
	}
	if !domain.Required {
		return nil, false, false
	}
	if len(domain.Allowed) > 0 {
		chosen := domain.Allowed[stableOffset(seed, len(domain.Allowed))]
		return cloneValue(chosen), true, true
	}
	if !emptyValue(domain.Field.Default) && usableValue(domain.Field, domain.Field.Default) {
		return cloneValue(domain.Field.Default), true, true
	}
	return nil, false, false
}

// propagateDependentConstraint 在依赖字段已稳定后传播可证明的字段比较。
func propagateDependentConstraint(values map[string]any, constraint Constraint) bool {
	right, exists := getPath(values, constraint.ValueField)
	if !exists || emptyValue(right) {
		return false
	}
	left, leftExists := getPath(values, constraint.Field)
	if leftExists && constraintSatisfiedWithValues(values, left, constraint) {
		return false
	}
	candidate := right
	switch strings.ToLower(strings.TrimSpace(constraint.Op)) {
	case "gt":
		candidate = numberValue(right) + 1
	case "lt":
		candidate = numberValue(right) - 1
	case "neq":
		candidate = fmt.Sprintf("其他值-%v", right)
	case "contains":
		candidate = fmt.Sprint(right) + "内容"
	}
	setPath(values, constraint.Field, cloneValue(candidate))
	return true
}

// fieldDomainContains 判断值是否仍位于字段传播后的候选域。
func fieldDomainContains(domain FieldDomain, value any) bool {
	if domain.Numeric != nil {
		number, ok := strictNumberValue(value)
		if !ok || !numericDomainContains(domain.Numeric, number) {
			return false
		}
	}
	if domain.Allowed != nil && !containsValue(domain.Allowed, value) {
		return false
	}
	return true
}

// numericDomainContains 判断数值是否满足区间的闭开边界。
func numericDomainContains(domain *NumericDomain, value float64) bool {
	if domain == nil {
		return true
	}
	if domain.Min != nil && (value < *domain.Min || (value == *domain.Min && !domain.MinInclusive)) {
		return false
	}
	if domain.Max != nil && (value > *domain.Max || (value == *domain.Max && !domain.MaxInclusive)) {
		return false
	}
	return true
}

// numericDomainEmpty 判断区间交集是否已经无解。
func numericDomainEmpty(domain *NumericDomain) bool {
	if domain == nil || domain.Min == nil || domain.Max == nil {
		return false
	}
	return *domain.Min > *domain.Max || (*domain.Min == *domain.Max && (!domain.MinInclusive || !domain.MaxInclusive))
}

// numericDomainCandidates 构造有限稳定候选，不扩大为无限搜索。
func numericDomainCandidates(domain *NumericDomain) []any {
	if domain == nil || numericDomainEmpty(domain) {
		return nil
	}
	result := make([]any, 0, 5)
	if domain.Min != nil {
		value := *domain.Min
		if !domain.MinInclusive {
			value++
		}
		if numericDomainContains(domain, value) {
			result = append(result, value)
		}
	}
	if domain.Max != nil {
		value := *domain.Max
		if !domain.MaxInclusive {
			value--
		}
		if numericDomainContains(domain, value) {
			result = append(result, value)
		}
	}
	if domain.Min != nil && domain.Max != nil {
		middle := math.Floor((*domain.Min + *domain.Max) / 2)
		if numericDomainContains(domain, middle) {
			result = append(result, middle)
		}
	}
	if len(result) == 0 {
		for _, value := range []float64{0, 1, -1, 100} {
			if numericDomainContains(domain, value) {
				result = append(result, value)
			}
		}
	}
	return uniqueValues(result)
}

// topologicalConstraintOrder 返回稳定拓扑序；剩余节点即为循环依赖。
func topologicalConstraintOrder(domains map[string]FieldDomain, dependencies map[string][]string) ([]string, []string) {
	indegree := make(map[string]int, len(domains))
	for path := range domains {
		indegree[path] = 0
	}
	for _, targets := range dependencies {
		seen := map[string]bool{}
		for _, target := range targets {
			if seen[target] {
				continue
			}
			seen[target] = true
			indegree[target]++
		}
	}
	ready := make([]string, 0)
	for path, degree := range indegree {
		if degree == 0 {
			ready = append(ready, path)
		}
	}
	sort.Strings(ready)
	order := make([]string, 0, len(domains))
	for len(ready) > 0 {
		path := ready[0]
		ready = ready[1:]
		order = append(order, path)
		targets := uniqueSorted(dependencies[path])
		for _, target := range targets {
			indegree[target]--
			if indegree[target] == 0 {
				ready = append(ready, target)
				sort.Strings(ready)
			}
		}
	}
	cycle := make([]string, 0)
	for path, degree := range indegree {
		if degree > 0 {
			cycle = append(cycle, path)
		}
	}
	sort.Strings(cycle)
	return order, cycle
}

// constraintIssuesStatus 以 blocked 优先、needs_attention 次之汇总问题状态。
func constraintIssuesStatus(issues []ConstraintIssue) string {
	status := ConstraintStatusReady
	for _, issue := range issues {
		if issue.Status == ConstraintStatusBlocked {
			return ConstraintStatusBlocked
		}
		if issue.Status == ConstraintStatusNeedsAttention {
			status = ConstraintStatusNeedsAttention
		}
	}
	return status
}

// newConstraintIssue 建立字段完整且数组非空指针的稳定结构化问题。
func newConstraintIssue(code, status, source, fieldPath, fieldLabel, operator string, expected, actual any, related []string, message string, canRetry bool) ConstraintIssue {
	return ConstraintIssue{Code: code, Status: status, Source: source, FieldPath: fieldPath, FieldLabel: fieldLabel, Operator: operator, Expected: expected, Actual: actual, RelatedFields: uniqueSorted(related), Message: message, CanRetry: canRetry}
}

// appendConstraintIssue 按稳定业务键去重问题。
func appendConstraintIssue(issues []ConstraintIssue, candidate ConstraintIssue) []ConstraintIssue {
	for _, issue := range issues {
		if issue.Code == candidate.Code && issue.Source == candidate.Source && issue.FieldPath == candidate.FieldPath && issue.Message == candidate.Message {
			return issues
		}
	}
	return append(issues, candidate)
}

// appendConstraintIssues 合并结构化问题并保持首次出现顺序。
func appendConstraintIssues(issues []ConstraintIssue, candidates []ConstraintIssue) []ConstraintIssue {
	for _, candidate := range candidates {
		issues = appendConstraintIssue(issues, candidate)
	}
	return issues
}

// relatedConstraintFields 返回字段比较涉及的右侧字段。
func relatedConstraintFields(constraint Constraint) []string {
	if strings.TrimSpace(constraint.ValueField) == "" {
		return []string{}
	}
	return []string{constraint.ValueField}
}

// fieldDomainLabel 返回字段公开名称，缺失时保留空值。
func fieldDomainLabel(domains map[string]FieldDomain, path string) string {
	if domain, ok := domains[path]; ok {
		return domain.Field.Name
	}
	return ""
}

// fieldDomainExpected 返回用于问题展示的区间或枚举集合。
func fieldDomainExpected(domain FieldDomain) any {
	if domain.Numeric != nil {
		return numericDomainText(domain.Numeric)
	}
	if domain.Allowed != nil {
		return domain.Allowed
	}
	return "符合字段值形态"
}

// numericDomainText 返回稳定可读的闭开区间。
func numericDomainText(domain *NumericDomain) string {
	if domain == nil {
		return "任意数值"
	}
	left, right := "(-∞", "+∞)"
	if domain.Min != nil {
		bracket := "("
		if domain.MinInclusive {
			bracket = "["
		}
		left = fmt.Sprintf("%s%v", bracket, *domain.Min)
	}
	if domain.Max != nil {
		bracket := ")"
		if domain.MaxInclusive {
			bracket = "]"
		}
		right = fmt.Sprintf("%v%s", *domain.Max, bracket)
	}
	return left + ", " + right
}

// strictNumberValue 仅接受可解析为有限数值的值。
func strictNumberValue(value any) (float64, bool) {
	var number float64
	var err error
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		number = numberValue(value)
	case string:
		number, err = strconv.ParseFloat(strings.TrimSpace(typed), 64)
	default:
		return 0, false
	}
	return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
}

// cloneNumber 复制可选数值指针，防止修改模板字段边界。
func cloneNumber(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return numberPointer(*value)
}

// numberPointer 返回独立数值指针。
func numberPointer(value float64) *float64 { return &value }

// cloneValueList 深复制候选列表。
func cloneValueList(values []any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = cloneValue(value)
	}
	return result
}

// intersectAllowed 计算候选集合交集；nil 表示此前没有集合边界。
func intersectAllowed(existing, incoming []any) []any {
	if existing == nil {
		return uniqueValues(incoming)
	}
	result := make([]any, 0)
	for _, value := range existing {
		if containsValue(incoming, value) {
			result = append(result, cloneValue(value))
		}
	}
	return result
}

// removeAllowed 从候选集合删除单个值。
func removeAllowed(existing []any, removed any) []any {
	result := make([]any, 0, len(existing))
	for _, value := range existing {
		if !equalValue(value, removed) {
			result = append(result, cloneValue(value))
		}
	}
	return result
}

// uniqueValues 按现有值比较语义去重候选。
func uniqueValues(values []any) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		if !containsValue(result, value) {
			result = append(result, cloneValue(value))
		}
	}
	return result
}

// stableOffset 把 seed 稳定映射到候选下标。
func stableOffset(seed int64, size int) int {
	if size <= 0 {
		return 0
	}
	offset := int(seed % int64(size))
	if offset < 0 {
		offset += size
	}
	return offset
}
