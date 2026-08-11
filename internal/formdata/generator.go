package formdata

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Constraint 是当前已选路径对表单字段的最小可满足约束。
type Constraint struct {
	Field string
	Op    string
	Value any
	Avoid []any
	Group int
}

// GenerateInput 汇总模板、样本、身份、路径约束和人工覆盖边界。
type GenerateInput struct {
	Template            map[string]any
	Base                map[string]any
	Samples             []map[string]any
	Seed                int64
	Initiator           string
	Constraints         []Constraint
	ManualOverridePaths []string
	EditablePaths       map[string]bool
}

// GenerateResult 是一次稳定智能填充的完整 values 与所有权摘要。
type GenerateResult struct {
	Values              map[string]any
	GeneratedFieldPaths []string
	ManualOverridePaths []string
	Defaults            int
	Recent              int
	Fallback            int
	Pending             int
	Unsupported         []string
}

// Field 是从 FormMaking 模板递归提取的生成与校验元数据。
type Field struct {
	Path        string
	Name        string
	Type        string
	Mode        string
	Required    bool
	Default     any
	Options     []any
	OptionNames map[string]string
	Unsupported bool
}

var supportedTypes = map[string]bool{
	"input": true, "textarea": true, "number": true, "date": true, "time": true,
	"select": true, "radio": true, "checkbox": true, "switch": true,
}

var containerTypes = map[string]bool{
	"grid": true, "report": true, "table": true, "subform": true, "inline": true,
	"dialog": true, "card": true, "group": true, "tabs": true, "collapse": true,
}

// ParseTemplate 递归解析 list、grid/report 的行列、tableColumns 与嵌套容器。
func ParseTemplate(template map[string]any) ([]Field, []string) {
	fields := make([]Field, 0)
	unsupported := make([]string, 0)
	collectList(anySlice(template["list"]), "", &fields, &unsupported)
	return fields, uniqueSorted(unsupported)
}

// collectList 深度优先收集字段；复杂组件仅记录缺口，绝不降级成普通文本值。
func collectList(list []any, prefix string, fields *[]Field, unsupported *[]string) {
	for _, raw := range list {
		component, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := strings.TrimSpace(anyText(component["type"]))
		model := strings.TrimSpace(anyText(component["model"]))
		name := strings.TrimSpace(anyText(component["name"]))
		path := joinPath(prefix, model)
		if supportedTypes[typeName] && model != "" {
			options, _ := component["options"].(map[string]any)
			values, names := optionValues(options["options"])
			*fields = append(*fields, Field{
				Path: path, Name: firstText(name, model), Type: typeName, Mode: strings.TrimSpace(anyText(options["type"])),
				Required: anyBool(options["required"]), Default: options["defaultValue"],
				Options: values, OptionNames: names,
			})
		} else if model != "" && (typeName == "subform" || typeName == "table") {
			// 明细/子表单的 values 是数组行结构，当前基础生成器不能把它当成点路径对象伪造。
			*unsupported = append(*unsupported, firstText(name, model)+"："+typeName+" 需要独立明细数据适配")
		} else if model != "" && !containerTypes[typeName] && typeName != "text" && typeName != "html" && typeName != "divider" && typeName != "blank" {
			*unsupported = append(*unsupported, firstText(name, model)+"："+firstText(typeName, "未知组件"))
		}
		childPrefix := prefix
		if (typeName == "subform" || typeName == "table" || typeName == "group") && model != "" {
			childPrefix = path
		}
		collectList(anySlice(component["list"]), childPrefix, fields, unsupported)
		collectList(anySlice(component["tableColumns"]), childPrefix, fields, unsupported)
		for _, column := range anySlice(component["columns"]) {
			if item, ok := column.(map[string]any); ok {
				collectList(anySlice(item["list"]), childPrefix, fields, unsupported)
			}
		}
		for _, row := range anySlice(component["rows"]) {
			item, ok := row.(map[string]any)
			if !ok {
				continue
			}
			collectList(anySlice(item["list"]), childPrefix, fields, unsupported)
			for _, column := range anySlice(item["columns"]) {
				if nested, ok := column.(map[string]any); ok {
					collectList(anySlice(nested["list"]), childPrefix, fields, unsupported)
				}
			}
		}
	}
}

// Generate 从保存值或最佳近期样本开始，按当前模板、路径约束和稳定种子重建完整 values。
func Generate(input GenerateInput) GenerateResult {
	fields, skipped := ParseTemplate(input.Template)
	seed := input.Seed
	if seed == 0 {
		seed = 1
	}
	rng := rand.New(rand.NewSource(seed))
	values := cloneMap(input.Base)
	manual := make(map[string]bool, len(input.ManualOverridePaths))
	for _, path := range input.ManualOverridePaths {
		manual[strings.TrimSpace(path)] = true
	}
	generated := make([]string, 0, len(fields))
	// 复杂组件由真实 FormMaking 负责渲染和校验；生成器只跳过它们并提示人工填写，不能把整张表单误判为不支持。
	result := GenerateResult{Values: values, ManualOverridePaths: uniqueSorted(input.ManualOverridePaths), Pending: len(skipped), Unsupported: []string{}}
	for _, field := range fields {
		if manual[field.Path] {
			continue
		}
		if input.EditablePaths != nil && !input.EditablePaths[field.Path] {
			// 目标运行时会禁用未授权字段；生成器只允许保留已有值或模板默认值，不能替用户写入样本/兜底值。
			if _, exists := getPath(values, field.Path); !exists && usableValue(field, field.Default) && !emptyValue(field.Default) {
				setPath(values, field.Path, cloneValue(field.Default))
				generated = append(generated, field.Path)
				result.Defaults++
			}
			manual[field.Path] = true
			continue
		}
		if sample, ok := sampleValue(input.Samples, field); ok {
			setPath(values, field.Path, sample)
			generated = append(generated, field.Path)
			result.Recent++
			continue
		}
		if usableValue(field, field.Default) {
			setPath(values, field.Path, cloneValue(field.Default))
			generated = append(generated, field.Path)
			result.Defaults++
			continue
		}
		if generatedValue, ok := safeFallback(field, input.Initiator, rng); ok {
			setPath(values, field.Path, generatedValue)
			generated = append(generated, field.Path)
			result.Fallback++
			continue
		}
		if field.Required {
			result.Pending++
		}
	}
	applyConstraints(values, input.Constraints, manual, rng)
	addVirtualValues(values, fields, &generated)
	result.GeneratedFieldPaths = uniqueSorted(generated)
	return result
}

// MergeGenerated 保留人工覆盖路径，只替换仍由生成器拥有的字段。
func MergeGenerated(current, generated map[string]any, generatedPaths, manualPaths []string) map[string]any {
	result := cloneMap(current)
	manual := make(map[string]bool, len(manualPaths))
	for _, path := range manualPaths {
		manual[path] = true
	}
	for _, path := range generatedPaths {
		if manual[path] {
			continue
		}
		if value, ok := getPath(generated, path); ok {
			setPath(result, path, cloneValue(value))
		}
	}
	return result
}

// Validate 使用当前模板重新校验数据形状、必填、选项和路径条件。
func Validate(template, values map[string]any, constraints []Constraint) []string {
	return ValidateEditable(template, values, constraints, nil)
}

// ValidateEditable 按当前路径可编辑字段复验 required；只读/隐藏字段仍校验已有值形状，但不强制必填。
func ValidateEditable(template, values map[string]any, constraints []Constraint, editablePaths map[string]bool) []string {
	fields, _ := ParseTemplate(template)
	errors := make([]string, 0)
	// 未被基础生成器识别的组件已经由真实运行时校验；服务端这里只复验可证明的基础字段与路径条件。
	for _, field := range fields {
		value, exists := getPath(values, field.Path)
		required := field.Required && (editablePaths == nil || editablePaths[field.Path])
		if required && (!exists || emptyValue(value)) {
			errors = append(errors, field.Name+"：必填值为空")
			continue
		}
		if exists && !emptyValue(value) && !usableValue(field, value) {
			errors = append(errors, field.Name+"：值类型或选项不符合当前模板")
		}
	}
	orGroups := make(map[int][]Constraint)
	for _, constraint := range constraints {
		if constraint.Group > 0 {
			orGroups[constraint.Group] = append(orGroups[constraint.Group], constraint)
			continue
		}
		value, _ := getPath(values, constraint.Field)
		if !constraintSatisfied(value, constraint) {
			errors = append(errors, "表单数据不满足当前已选路径条件")
		}
	}
	for _, group := range orGroups {
		matched := false
		for _, constraint := range group {
			value, _ := getPath(values, constraint.Field)
			matched = matched || constraintSatisfied(value, constraint)
		}
		if !matched {
			errors = append(errors, "表单数据不满足当前已选路径条件")
		}
	}
	return uniqueSorted(errors)
}

// TemplateVersion 对完整模板生成稳定轻量版本，用于识别保存后的目标结构变化。
func TemplateVersion(template map[string]any) string {
	data, _ := json.Marshal(template)
	var hash uint64 = 1469598103934665603
	for _, value := range data {
		hash ^= uint64(value)
		hash *= 1099511628211
	}
	return strconv.FormatUint(hash, 16)
}

// sampleValue 选择第一个与当前组件形状匹配的近期样本值。
func sampleValue(samples []map[string]any, field Field) (any, bool) {
	for _, sample := range samples {
		if value, ok := getPath(sample, field.Path); ok && !emptyValue(value) && usableValue(field, value) {
			return cloneValue(value), true
		}
	}
	return nil, false
}

// safeFallback 只为基础组件生成确定性安全值，未知复杂组件永远不编造。
func safeFallback(field Field, initiator string, rng *rand.Rand) (any, bool) {
	switch field.Type {
	case "input", "textarea":
		if strings.Contains(field.Name, "发起人") && strings.TrimSpace(initiator) != "" {
			return strings.TrimSpace(initiator), true
		}
		return fmt.Sprintf("%s-%03d", field.Name, rng.Intn(900)+100), true
	case "number":
		return rng.Intn(90) + 10, true
	case "date":
		value := time.Date(2024, 1, 1, rng.Intn(8)+9, rng.Intn(12)*5, 0, 0, time.UTC).AddDate(0, 0, rng.Intn(365))
		if field.Mode == "datetime" {
			return value.Format("2006-01-02 15:04:05"), true
		}
		return value.Format("2006-01-02"), true
	case "time":
		return fmt.Sprintf("%02d:%02d:00", rng.Intn(8)+9, rng.Intn(12)*5), true
	case "select", "radio":
		if len(field.Options) > 0 {
			return cloneValue(field.Options[rng.Intn(len(field.Options))]), true
		}
	case "checkbox":
		if len(field.Options) > 0 {
			return []any{cloneValue(field.Options[rng.Intn(len(field.Options))])}, true
		}
	case "switch":
		return rng.Intn(2) == 0, true
	}
	return nil, false
}

// applyConstraints 只覆盖生成器拥有的字段；人工值冲突留给保存校验明确提示。
func applyConstraints(values map[string]any, constraints []Constraint, manual map[string]bool, rng *rand.Rand) {
	appliedGroups := make(map[int]bool)
	for _, constraint := range constraints {
		if constraint.Field == "" || manual[constraint.Field] {
			continue
		}
		// OR 组只需选择一个可满足分支；稳定选择列表中的第一项，避免同组后续约束互相覆盖。
		if constraint.Group > 0 && appliedGroups[constraint.Group] {
			continue
		}
		switch strings.ToLower(constraint.Op) {
		case "eq", "in":
			setPath(values, constraint.Field, cloneValue(constraint.Value))
		case "neq":
			current, _ := getPath(values, constraint.Field)
			if equalValue(current, constraint.Value) {
				setPath(values, constraint.Field, fmt.Sprintf("其他值-%d", rng.Intn(900)+100))
			}
		case "gt":
			setPath(values, constraint.Field, numberValue(constraint.Value)+1)
		case "gte":
			setPath(values, constraint.Field, numberValue(constraint.Value))
		case "lt":
			setPath(values, constraint.Field, numberValue(constraint.Value)-1)
		case "lte":
			setPath(values, constraint.Field, numberValue(constraint.Value))
		case "default":
			candidate := fmt.Sprintf("默认分支-%d", rng.Intn(900)+100)
			for containsValue(constraint.Avoid, candidate) {
				candidate += "x"
			}
			setPath(values, constraint.Field, candidate)
		}
		if constraint.Group > 0 {
			appliedGroups[constraint.Group] = true
		}
	}
}

// constraintSatisfied 判断 values 是否真实满足当前路径的基本比较条件。
func constraintSatisfied(value any, constraint Constraint) bool {
	switch strings.ToLower(constraint.Op) {
	case "eq":
		return equalValue(value, constraint.Value)
	case "in":
		if list, ok := constraint.Value.([]any); ok {
			return containsValue(list, value)
		}
		return equalValue(value, constraint.Value)
	case "neq":
		return !equalValue(value, constraint.Value)
	case "gt":
		return numberValue(value) > numberValue(constraint.Value)
	case "gte":
		return numberValue(value) >= numberValue(constraint.Value)
	case "lt":
		return numberValue(value) < numberValue(constraint.Value)
	case "lte":
		return numberValue(value) <= numberValue(constraint.Value)
	case "default":
		return !containsValue(constraint.Avoid, value)
	default:
		return false
	}
}

// addVirtualValues 为选项型字段补齐目标条件和展示所需的虚拟名称字段。
func addVirtualValues(values map[string]any, fields []Field, generated *[]string) {
	for _, field := range fields {
		if len(field.OptionNames) == 0 {
			continue
		}
		value, ok := getPath(values, field.Path)
		if !ok {
			continue
		}
		label := field.OptionNames[fmt.Sprint(value)]
		if label == "" {
			continue
		}
		virtualPath := field.Path + "__virtualName"
		setPath(values, virtualPath, label)
		*generated = append(*generated, virtualPath)
	}
}

// usableValue 验证基础组件的数据形状与选项范围。
func usableValue(field Field, value any) bool {
	if value == nil {
		return !field.Required
	}
	switch field.Type {
	case "input", "textarea":
		_, ok := value.(string)
		return ok
	case "date":
		text, ok := value.(string)
		if !ok {
			return false
		}
		layout := "2006-01-02"
		if field.Mode == "datetime" {
			layout = "2006-01-02 15:04:05"
		}
		_, err := time.Parse(layout, text)
		return err == nil
	case "time":
		text, ok := value.(string)
		if !ok {
			return false
		}
		_, err := time.Parse("15:04:05", text)
		return err == nil
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32, uint, uint64, json.Number:
			return true
		default:
			return false
		}
	case "select", "radio":
		return len(field.Options) == 0 || containsValue(field.Options, value)
	case "checkbox":
		list, ok := value.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if len(field.Options) > 0 && !containsValue(field.Options, item) {
				return false
			}
		}
		return true
	case "switch":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

// optionValues 读取目标组件 options 的 label/value 或 id，并建立虚拟名称字典。
func optionValues(raw any) ([]any, map[string]string) {
	values := make([]any, 0)
	names := make(map[string]string)
	for _, item := range anySlice(raw) {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		value := option["value"]
		if value == nil {
			value = option["id"]
		}
		if value == nil {
			continue
		}
		values = append(values, value)
		names[fmt.Sprint(value)] = firstText(anyText(option["label"]), fmt.Sprint(value))
	}
	return values, names
}

// getPath 读取点分隔对象路径；数组与复杂明细由真实运行时维护，不在生成器中伪造。
func getPath(values map[string]any, path string) (any, bool) {
	current := any(values)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// setPath 设置点分隔对象路径，缺少的中间对象按空对象建立。
func setPath(values map[string]any, path string, value any) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	current := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		current[parts[len(parts)-1]] = value
	}
}

// cloneMap 深复制完整表单值，避免生成器修改保存基线或样本缓存。
func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	data, _ := json.Marshal(values)
	result := make(map[string]any)
	_ = json.Unmarshal(data, &result)
	return result
}

// cloneValue 深复制一个 JSON 值。
func cloneValue(value any) any {
	data, _ := json.Marshal(value)
	var result any
	_ = json.Unmarshal(data, &result)
	return result
}

// anySlice 将 JSON 数组安全转换为空数组兜底。
func anySlice(value any) []any {
	list, _ := value.([]any)
	return list
}

// anyText 将模板标量转为去空白字符串。
func anyText(value any) string { return strings.TrimSpace(fmt.Sprint(value)) }

// anyBool 读取模板布尔值。
func anyBool(value any) bool {
	result, _ := value.(bool)
	return result
}

// firstText 返回第一个非空展示值。
func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// joinPath 合并对象路径。
func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	if name == "" {
		return prefix
	}
	return prefix + "." + name
}

// uniqueSorted 去重并稳定排序公开路径或错误摘要。
func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

// emptyValue 判断 FormMaking 必填值是否为空。
func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// containsValue 用 JSON 等价判断选项成员。
func containsValue(values []any, target any) bool {
	for _, value := range values {
		if equalValue(value, target) {
			return true
		}
	}
	return false
}

// equalValue 用 JSON 标量表示比较目标模板与运行时值。
func equalValue(left, right any) bool { return fmt.Sprint(left) == fmt.Sprint(right) }

// numberValue 把模板比较值转换为数字，失败时使用零并由最终校验拒绝不满足条件。
func numberValue(value any) float64 {
	result, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
	return result
}
