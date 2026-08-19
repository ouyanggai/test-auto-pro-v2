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
	Field      string
	Op         string
	Value      any
	ValueField string
	Avoid      []any
	Group      int
}

// DateRangeBinding 是已由调用方精确证明的天数与日期区间字段对应关系。
// 两个字段均使用模板模型键，生成器不会根据字段名称或中文标签自行推断。
type DateRangeBinding struct {
	DurationField string
	RangeField    string
}

// GenerateInput 汇总模板、样本、身份、路径约束和人工覆盖边界。
type GenerateInput struct {
	Template            map[string]any
	Base                map[string]any
	Samples             []map[string]any
	Seed                int64
	Initiator           string
	Constraints         []Constraint
	DateRangeBindings   []DateRangeBinding
	ManualOverridePaths []string
	ProtectedPaths      map[string]bool
	EditablePaths       map[string]bool
	Identity            IdentityContext
}

// GenerateResult 是一次稳定智能填充的完整 values 与所有权摘要。
type GenerateResult struct {
	Values              map[string]any
	GeneratedFieldPaths []string
	ManualOverridePaths []string
	Defaults            int
	Recent              int
	Fallback            int
	Identity            int
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
	// OptionPaths 保存级联组件从根到叶子的真实候选路径，生成值必须使用完整路径数组。
	OptionPaths [][]any
	// OptionVirtualUsesValue 表示目标模板显式关闭 showLabel 时，__virtualName 使用选项值而不是展示标签。
	OptionVirtualUsesValue bool
	// CollectionRoot 标记字段位于子表单或表格行中，值读取和写入必须保持数组行结构。
	CollectionRoot string
	// ManualOnly 表示附件或外部对象需要真实页面人工提供，生成器不得伪造引用。
	ManualOnly    bool
	DataSourceURL string
	Unsupported   bool
	El            string
}

// IdentityNode 是当前账号在公司目录树中的节点上下文，用于自定义人员/公司组件自动填充。
type IdentityNode struct {
	ID        string
	Name      string
	Type      string
	ParentID  string
	CompanyID string
}

// IdentityContext 汇总当前账号的公司、部门与本人节点，字段缺失时对应项保持空。
type IdentityContext struct {
	Company    IdentityNode
	Department IdentityNode
	User       IdentityNode
}

// TemplateDataSource 描述模板字段声明的只读数据源，不执行请求也不猜测返回值。
type TemplateDataSource struct {
	FieldPath string
	URL       string
	Method    string
}

// TemplateRuleInventory 是全模板规则盘点结果；未分类能力必须进入 NeedsAttention。
type TemplateRuleInventory struct {
	Fields             []Field
	Unsupported        []string
	ComponentTypes     map[string]int
	DataSources        []TemplateDataSource
	ScriptCapabilities []string
	NeedsAttention     []string
}

var supportedTypes = map[string]bool{
	"input": true, "textarea": true, "number": true, "date": true, "time": true,
	"select": true, "radio": true, "checkbox": true, "switch": true, "cascader": true,
}

var containerTypes = map[string]bool{
	"grid": true, "report": true, "table": true, "subform": true, "inline": true,
	"dialog": true, "card": true, "group": true, "tabs": true, "collapse": true,
}

// ParseTemplate 递归解析所有 FormMaking 容器、标准组件、级联和信息选择组件；外部对象保留人工边界。
func ParseTemplate(template map[string]any) ([]Field, []string) {
	fields := make([]Field, 0)
	unsupported := make([]string, 0)
	pendingLabel := ""
	collectList(anySlice(template["list"]), "", "", &pendingLabel, &fields, &unsupported)
	return fields, uniqueSorted(unsupported)
}

// InventoryTemplateRules 递归盘点模板组件、数据源与脚本能力，统一提供给生成器和覆盖报告。
func InventoryTemplateRules(template map[string]any) TemplateRuleInventory {
	fields, unsupported := ParseTemplate(template)
	result := TemplateRuleInventory{
		Fields: fields, Unsupported: unsupported, ComponentTypes: map[string]int{},
		DataSources: []TemplateDataSource{}, ScriptCapabilities: []string{}, NeedsAttention: []string{},
	}
	var walk func(any, string)
	walk = func(raw any, path string) {
		switch value := raw.(type) {
		case []any:
			for index, item := range value {
				walk(item, fmt.Sprintf("%s[%d]", path, index))
			}
		case map[string]any:
			typeName := strings.TrimSpace(anyText(value["type"]))
			if typeName != "" {
				result.ComponentTypes[typeName]++
				if !supportedTypes[typeName] && !containerTypes[typeName] && typeName != "text" && typeName != "html" && typeName != "divider" && typeName != "blank" && typeName != "custom" {
					result.NeedsAttention = append(result.NeedsAttention, "未知组件："+typeName)
				}
			}
			options, _ := value["options"].(map[string]any)
			if url := firstText(anyText(options["requestURL"]), anyText(options["url"])); url != "" {
				result.DataSources = append(result.DataSources, TemplateDataSource{FieldPath: firstText(path, anyText(value["model"])), URL: url, Method: strings.ToUpper(firstText(anyText(options["requestMethod"]), anyText(options["method"]), "GET"))})
			}
			for key, child := range value {
				if strings.Contains(strings.ToLower(key), "script") || key == "requestFunc" || key == "responseFunc" {
					text := strings.TrimSpace(anyText(child))
					if text != "" {
						result.ScriptCapabilities = append(result.ScriptCapabilities, key)
						if !safeScriptCapability(text) {
							result.NeedsAttention = append(result.NeedsAttention, "动态脚本需要人工核对："+key)
						}
					}
				}
				walk(child, path+"."+key)
			}
		}
	}
	walk(template, "")
	result.Unsupported = uniqueSorted(result.Unsupported)
	result.NeedsAttention = uniqueSorted(result.NeedsAttention)
	result.ScriptCapabilities = uniqueSorted(result.ScriptCapabilities)
	return result
}

// safeScriptCapability 只认可显示隐藏、赋值和选项更新等可静态识别的脚本片段。
func safeScriptCapability(script string) bool {
	text := strings.ToLower(script)
	for _, marker := range []string{"setvisible", "setvalue", "options", "visible", "hidden", "value"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// infoSelectKind 按目标组件字段命名约定识别选公司、选部门、选人、选岗位语义。
func infoSelectKind(model string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(lower, "company"):
		return "company"
	case strings.Contains(lower, "dep"):
		return "department"
	case strings.Contains(lower, "user"):
		return "user"
	case strings.Contains(lower, "duty"), strings.Contains(lower, "position"), strings.Contains(lower, "post"):
		return "position"
	default:
		return ""
	}
}

// collectList 深度优先收集字段；复杂组件仅记录缺口，绝不降级成普通文本值。
// 目标模板的字段标题是独立 text 组件，这里把最近的前置标题作为字段展示名称。
func collectList(list []any, prefix, collectionRoot string, pendingLabel *string, fields *[]Field, unsupported *[]string) {
	for _, raw := range list {
		component, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := strings.TrimSpace(anyText(component["type"]))
		model := strings.TrimSpace(anyText(component["model"]))
		name := strings.TrimSpace(anyText(component["name"]))
		el := strings.TrimSpace(anyText(component["el"]))
		path := joinPath(prefix, model)
		options, _ := component["options"].(map[string]any)
		dataSourceURL := firstText(anyText(options["requestURL"]), anyText(options["url"]))
		switch {
		case typeName == "text" && model == "" && name != "":
			*pendingLabel = name
		case typeName == "text":
			// 带 model 的文本（审批意见占位等）不是字段标题，避免标题串到后续字段。
			*pendingLabel = ""
		case supportedTypes[typeName] && model != "":
			values, names := optionValues(options["options"])
			optionPaths := [][]any(nil)
			if typeName == "cascader" {
				values, names, optionPaths = cascaderOptionValues(options["options"])
			}
			_, hasShowLabel := options["showLabel"]
			*fields = append(*fields, Field{
				Path: path, Name: firstText(*pendingLabel, name, model), Type: typeName, Mode: strings.TrimSpace(anyText(options["type"])),
				Required: anyBool(options["required"]), Default: options["defaultValue"],
				Options: values, OptionNames: names, OptionPaths: optionPaths, OptionVirtualUsesValue: hasShowLabel && !anyBool(options["showLabel"]),
				CollectionRoot: collectionRoot, DataSourceURL: dataSourceURL, El: el,
			})
			*pendingLabel = ""
		case typeName == "fileupload" && model != "":
			*fields = append(*fields, Field{
				Path: path, Name: firstText(*pendingLabel, name, model), Type: typeName, Mode: strings.TrimSpace(anyText(options["listType"])),
				Required: anyBool(options["required"]), Default: options["defaultValue"], CollectionRoot: collectionRoot,
				ManualOnly: true, DataSourceURL: dataSourceURL, El: el,
			})
			*pendingLabel = ""
		case typeName == "custom" && el == "custome-info-select" && model != "":
			// 选公司/部门/岗位/人员组件可由当前账号身份自动填充，不再当作不可生成缺口。
			options, _ := component["options"].(map[string]any)
			*fields = append(*fields, Field{
				Path: path, Name: firstText(*pendingLabel, name, model), Type: "infoSelect", Mode: infoSelectKind(model),
				Required: anyBool(options["required"]), Default: options["defaultValue"], El: el,
			})
			*pendingLabel = ""
		case model != "" && !containerTypes[typeName] && typeName != "html" && typeName != "divider" && typeName != "blank":
			// 用 model 作主标识，避免多个同名自定义组件被去重后人工待填计数失真。
			*unsupported = append(*unsupported, firstText(model, *pendingLabel, name)+"："+firstText(typeName, "未知组件"))
			*pendingLabel = ""
		}
		childPrefix := prefix
		childCollectionRoot := collectionRoot
		if (typeName == "subform" || typeName == "table" || typeName == "group") && model != "" {
			childPrefix = path
			if typeName == "subform" || typeName == "table" {
				childCollectionRoot = path
				childPrefix += "[]"
			}
		}
		collectList(anySlice(component["list"]), childPrefix, childCollectionRoot, pendingLabel, fields, unsupported)
		collectList(anySlice(component["tableColumns"]), childPrefix, childCollectionRoot, pendingLabel, fields, unsupported)
		for _, column := range anySlice(component["columns"]) {
			if item, ok := column.(map[string]any); ok {
				collectList(anySlice(item["list"]), childPrefix, childCollectionRoot, pendingLabel, fields, unsupported)
			}
		}
		for _, row := range anySlice(component["rows"]) {
			item, ok := row.(map[string]any)
			if !ok {
				continue
			}
			collectList(anySlice(item["list"]), childPrefix, childCollectionRoot, pendingLabel, fields, unsupported)
			for _, column := range anySlice(item["columns"]) {
				if nested, ok := column.(map[string]any); ok {
					collectList(anySlice(nested["list"]), childPrefix, childCollectionRoot, pendingLabel, fields, unsupported)
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
		if field.ManualOnly {
			if field.Required && emptyFieldValue(values, field) {
				result.Pending++
			}
			continue
		}
		if input.ProtectedPaths[field.Path] {
			// 已存在的路径条件字段只能由约束投影调整，不能被普通样本或随机值覆盖。
			if _, exists := getFieldValue(values, field); exists {
				manual[field.Path] = true
				continue
			}
		}
		if input.EditablePaths != nil && !input.EditablePaths[field.Path] {
			// 目标运行时会禁用未授权字段；生成器只允许保留已有值或模板默认值，不能替用户写入样本/兜底值。
			if _, exists := getFieldValue(values, field); !exists && usableValue(field, field.Default) && !emptyValue(field.Default) {
				setFieldValue(values, field, cloneValue(field.Default))
				generated = append(generated, field.Path)
				result.Defaults++
			}
			manual[field.Path] = true
			continue
		}
		if field.Type == "infoSelect" {
			// 选公司/部门/人员优先使用当前账号在真实目录树中的节点，无法解析时保留已有值并计入人工待填。
			if value, ok := infoSelectValue(field.Mode, input.Identity); ok {
				setFieldValue(values, field, value)
				generated = append(generated, field.Path)
				result.Identity++
			} else if field.Required {
				result.Pending++
			}
			continue
		}
		if sample, ok := sampleValue(input.Samples, field, int(seed)); ok {
			setFieldValue(values, field, sample)
			generated = append(generated, field.Path)
			result.Recent++
			continue
		}
		if usableValue(field, field.Default) && !emptyValue(field.Default) {
			setFieldValue(values, field, cloneValue(field.Default))
			generated = append(generated, field.Path)
			result.Defaults++
			continue
		}
		if generatedValue, ok := safeFallback(field, input.Initiator, rng); ok {
			setFieldValue(values, field, generatedValue)
			generated = append(generated, field.Path)
			result.Fallback++
			continue
		}
		if field.Required {
			result.Pending++
		}
	}
	applyConstraints(values, input.Constraints, manual, input.ProtectedPaths, rng, &generated)
	applyDateRangeBindings(values, input.DateRangeBindings, manual, &generated)
	addVirtualValues(values, fields, &generated)
	result.GeneratedFieldPaths = uniqueSorted(generated)
	return result
}

// ValidateDateRangeBindings 校验已声明绑定的日期区间是否以自然日含首尾覆盖对应天数。
func ValidateDateRangeBindings(values map[string]any, bindings []DateRangeBinding) []string {
	errors := make([]string, 0)
	for _, binding := range bindings {
		duration, durationOK := formWholeDays(values, binding.DurationField)
		start, end, rangeOK := formDateRange(values, binding.RangeField)
		if !durationOK || !rangeOK {
			continue
		}
		if start.AddDate(0, 0, duration-1).Format("2006-01-02") != end.Format("2006-01-02") {
			errors = append(errors, "日期区间与条件天数不一致")
		}
	}
	return uniqueSorted(errors)
}

// SynchronizeDateRangeBindings 在条件求解改变天数后重新同步日期区间，人工覆盖的区间保持原值。
func SynchronizeDateRangeBindings(values map[string]any, bindings []DateRangeBinding, manualPaths []string) {
	manual := make(map[string]bool, len(manualPaths))
	for _, path := range manualPaths {
		manual[strings.TrimSpace(path)] = true
	}
	generated := make([]string, 0, len(bindings))
	applyDateRangeBindings(values, bindings, manual, &generated)
}

// ApplyVirtualValues 根据模板真实选项和身份组件值重建目标条件使用的虚拟字段。
func ApplyVirtualValues(template, values map[string]any) {
	fields, _ := ParseTemplate(template)
	generated := make([]string, 0)
	addVirtualValues(values, fields, &generated)
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
		value, exists := getFieldValue(values, field)
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
		if !constraintSatisfiedWithValues(values, value, constraint) {
			errors = append(errors, "表单数据不满足当前已选路径条件")
		}
	}
	for _, group := range orGroups {
		matched := false
		for _, constraint := range group {
			value, _ := getPath(values, constraint.Field)
			matched = matched || constraintSatisfiedWithValues(values, value, constraint)
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

// sampleValue 按稳定种子轮转近期样本，换一组时只从同一已验证样本来源选择下一可用值。
func sampleValue(samples []map[string]any, field Field, offset int) (any, bool) {
	for index := range samples {
		sample := samples[(index+offset)%len(samples)]
		if value, ok := getFieldValue(sample, field); ok && !emptyValue(value) && usableValue(field, value) {
			return cloneValue(value), true
		}
	}
	return nil, false
}

// infoSelectValue 把账号目录节点编码为 custome-info-select 组件约定的 JSON 文本值。
func infoSelectValue(kind string, identity IdentityContext) (string, bool) {
	var node IdentityNode
	switch kind {
	case "company":
		node = identity.Company
	case "department":
		node = identity.Department
	case "user":
		node = identity.User
	default:
		return "", false
	}
	if strings.TrimSpace(node.ID) == "" || strings.TrimSpace(node.Name) == "" {
		return "", false
	}
	encoded, err := json.Marshal(map[string]any{
		"id": node.ID, "name": node.Name, "type": node.Type,
		"companyId": node.CompanyID, "parentId": node.ParentID,
	})
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// smartTextValue 按字段中文标题生成可读的占位内容，避免出现“组件名-编号”这类无意义文本。
func smartTextValue(label, initiator string) string {
	switch {
	case strings.Contains(label, "发起人"), strings.Contains(label, "申请人"), strings.Contains(label, "姓名"),
		strings.Contains(label, "联系人"), strings.Contains(label, "负责人"):
		if strings.TrimSpace(initiator) != "" {
			return strings.TrimSpace(initiator)
		}
	case strings.Contains(label, "原因"):
		return "个人事务需要处理"
	case strings.Contains(label, "说明"), strings.Contains(label, "描述"), strings.Contains(label, "备注"):
		return "无特殊说明"
	case strings.Contains(label, "电话"), strings.Contains(label, "手机"), strings.Contains(label, "联系方式"):
		return "13800000000"
	case strings.Contains(label, "邮箱"):
		return "test@example.com"
	case strings.Contains(label, "地址"):
		return "测试地址（请核对）"
	}
	return strings.TrimSpace(label) + "（自动填写，请核对）"
}

// safeFallback 只为基础组件生成确定性安全值，未知复杂组件永远不编造。
func safeFallback(field Field, initiator string, rng *rand.Rand) (any, bool) {
	switch field.Type {
	case "input", "textarea":
		return smartTextValue(field.Name, initiator), true
	case "number":
		return rng.Intn(90) + 10, true
	case "date":
		value := time.Date(2024, 1, 1, rng.Intn(8)+9, rng.Intn(12)*5, 0, 0, time.UTC).AddDate(0, 0, rng.Intn(365))
		if field.Mode == "daterange" {
			// 目标日期范围组件要求 [开始, 结束] 数组，单日期字符串不会被渲染。
			return []any{value.Format("2006-01-02"), value.AddDate(0, 0, 3).Format("2006-01-02")}, true
		}
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
	case "cascader":
		if len(field.OptionPaths) > 0 {
			return cloneValue(field.OptionPaths[rng.Intn(len(field.OptionPaths))]), true
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

// formWholeDays 读取正整数天数；小数、零和缺失值不参与区间同步或校验。
func formWholeDays(values map[string]any, path string) (int, bool) {
	value, exists := getPath(values, path)
	if !exists {
		return 0, false
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	if err != nil || number <= 0 || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}

// formDateRange 读取 FormMaking daterange 的开始和结束日期，仅接受标准日期字符串。
func formDateRange(values map[string]any, path string) (time.Time, time.Time, bool) {
	value, exists := getPath(values, path)
	if !exists {
		return time.Time{}, time.Time{}, false
	}
	rangeValues, ok := value.([]any)
	if !ok || len(rangeValues) != 2 {
		return time.Time{}, time.Time{}, false
	}
	start, startErr := time.Parse("2006-01-02", strings.TrimSpace(fmt.Sprint(rangeValues[0])))
	end, endErr := time.Parse("2006-01-02", strings.TrimSpace(fmt.Sprint(rangeValues[1])))
	if startErr != nil || endErr != nil || end.Before(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

// applyConstraints 只覆盖生成器拥有的字段；命中条件字段也纳入生成所有权，供运行时统计准确识别自动值。
func applyConstraints(values map[string]any, constraints []Constraint, manual, protected map[string]bool, rng *rand.Rand, generated *[]string) {
	appliedGroups := make(map[int]bool)
	for _, constraint := range constraints {
		if constraint.Field == "" || (manual[constraint.Field] && !protected[constraint.Field]) {
			continue
		}
		// OR 组只需选择一个可满足分支；稳定选择列表中的第一项，避免同组后续约束互相覆盖。
		if constraint.Group > 0 && appliedGroups[constraint.Group] {
			continue
		}
		if current, exists := getPath(values, constraint.Field); exists && constraintSatisfiedWithValues(values, current, constraint) {
			// 近期样本或已有草稿已经满足路径条件时必须保留，不能为了“生成”篡改有效候选。
			if constraint.Group > 0 {
				appliedGroups[constraint.Group] = true
			}
			continue
		}
		value := constraint.Value
		if constraint.ValueField != "" {
			var exists bool
			value, exists = getPath(values, constraint.ValueField)
			if !exists || emptyValue(value) {
				continue
			}
		}
		switch strings.ToLower(constraint.Op) {
		case "eq", "in":
			setPath(values, constraint.Field, cloneValue(value))
		case "neq":
			current, _ := getPath(values, constraint.Field)
			if equalValue(current, value) {
				setPath(values, constraint.Field, fmt.Sprintf("其他值-%d", rng.Intn(900)+100))
			}
		case "gt":
			setPath(values, constraint.Field, numberValue(value)+1)
		case "gte":
			setPath(values, constraint.Field, numberValue(value))
		case "lt":
			setPath(values, constraint.Field, numberValue(value)-1)
		case "lte":
			setPath(values, constraint.Field, numberValue(value))
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
		*generated = append(*generated, constraint.Field)
	}
}

// applyDateRangeBindings 在条件值稳定后同步日期结束日，日期区间按自然日含首尾计算。
func applyDateRangeBindings(values map[string]any, bindings []DateRangeBinding, manual map[string]bool, generated *[]string) {
	for _, binding := range bindings {
		if manual[binding.RangeField] {
			continue
		}
		duration, durationOK := formWholeDays(values, binding.DurationField)
		start, _, rangeOK := formDateRange(values, binding.RangeField)
		if !durationOK || !rangeOK {
			continue
		}
		// FormMaking daterange 的两端均计入请假天数，不能沿用通用兜底的固定三天。
		setPath(values, binding.RangeField, []any{start.Format("2006-01-02"), start.AddDate(0, 0, duration-1).Format("2006-01-02")})
		*generated = append(*generated, binding.RangeField)
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

// constraintSatisfiedWithValues 在字段对字段比较时从当前完整表单读取右侧值，其余比较沿用固定值约束。
func constraintSatisfiedWithValues(values map[string]any, value any, constraint Constraint) bool {
	if constraint.ValueField == "" {
		return constraintSatisfied(value, constraint)
	}
	right, exists := getPath(values, constraint.ValueField)
	if !exists {
		return false
	}
	constraint.Value = right
	return constraintSatisfied(value, constraint)
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
		label, exists := field.OptionNames[fmt.Sprint(value)]
		if !exists {
			continue
		}
		if field.OptionVirtualUsesValue {
			label = fmt.Sprint(value)
		}
		virtualPath := field.Path + "__virtualName"
		setPath(values, virtualPath, label)
		*generated = append(*generated, virtualPath)
	}
	for _, field := range fields {
		if field.Type != "infoSelect" {
			continue
		}
		value, ok := getPath(values, field.Path)
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		var selected map[string]any
		if json.Unmarshal([]byte(text), &selected) != nil {
			continue
		}
		if name := strings.TrimSpace(fmt.Sprint(selected["name"])); name != "" {
			virtualPath := field.Path + "__condition"
			if _, exists := getPath(values, virtualPath); !exists {
				setPath(values, virtualPath, name)
				*generated = append(*generated, virtualPath)
			}
		}
		if id := strings.TrimSpace(fmt.Sprint(selected["id"])); id != "" {
			setPath(values, field.Path+"__formPersonId", id)
			*generated = append(*generated, field.Path+"__formPersonId")
		}
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
	case "infoSelect":
		text, ok := value.(string)
		return ok && strings.TrimSpace(text) != ""
	case "date":
		if field.Mode == "daterange" {
			list, ok := value.([]any)
			if !ok || len(list) != 2 {
				return false
			}
			for _, item := range list {
				text, ok := item.(string)
				if !ok {
					return false
				}
				if _, err := time.Parse("2006-01-02", text); err != nil {
					return false
				}
			}
			return true
		}
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
	case "cascader":
		list, ok := value.([]any)
		if !ok || len(list) == 0 {
			return false
		}
		if len(field.OptionPaths) == 0 {
			return true
		}
		for _, path := range field.OptionPaths {
			if equalValue(path, list) {
				return true
			}
		}
		return false
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
	case "fileupload":
		_, ok := value.([]any)
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

// cascaderOptionValues 展平真实级联候选，同时保留从根到叶子的完整值路径。
func cascaderOptionValues(raw any) ([]any, map[string]string, [][]any) {
	values := make([]any, 0)
	names := make(map[string]string)
	paths := make([][]any, 0)
	var walk func([]any, []any)
	walk = func(items []any, prefix []any) {
		for _, item := range items {
			node, ok := item.(map[string]any)
			if !ok {
				continue
			}
			value := node["value"]
			if value == nil {
				value = node["id"]
			}
			if value == nil {
				continue
			}
			current := append(append([]any{}, prefix...), value)
			children := anySlice(node["children"])
			if len(children) > 0 {
				walk(children, current)
				continue
			}
			values = append(values, value)
			paths = append(paths, current)
			names[fmt.Sprint(value)] = firstText(anyText(node["label"]), anyText(node["name"]), fmt.Sprint(value))
		}
	}
	walk(anySlice(raw), nil)
	return values, names, paths
}

// getFieldValue 读取普通字段或子表单首行字段；集合值只处理真实已有首行，不伪造外部对象。
func getFieldValue(values map[string]any, field Field) (any, bool) {
	if !strings.Contains(field.Path, "[]") {
		return getPath(values, field.Path)
	}
	return getPath(values, field.Path)
}

// setFieldValue 写入普通字段或子表单首行字段，保持 FormMaking 数组行结构。
func setFieldValue(values map[string]any, field Field, value any) {
	setPath(values, field.Path, value)
}

// emptyFieldValue 判断字段在集合路径中是否缺失，供附件和子表单保持人工边界。
func emptyFieldValue(values map[string]any, field Field) bool {
	value, exists := getFieldValue(values, field)
	return !exists || emptyValue(value)
}

// getPath 读取点分隔对象路径并支持 [] 标记的首个真实数组行。
func getPath(values map[string]any, path string) (any, bool) {
	current := any(values)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if strings.HasSuffix(part, "[]") {
			part = strings.TrimSuffix(part, "[]")
		}
		object, ok := current.(map[string]any)
		if !ok {
			if list, listOK := current.([]any); listOK && len(list) > 0 {
				current = list[0]
				object, ok = current.(map[string]any)
			}
			if !ok {
				return nil, false
			}
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
	for _, rawPart := range parts[:len(parts)-1] {
		part := strings.TrimSuffix(rawPart, "[]")
		next, ok := current[part].(map[string]any)
		if strings.HasSuffix(rawPart, "[]") {
			list, listOK := current[part].([]any)
			if !listOK || len(list) == 0 {
				list = []any{map[string]any{}}
				current[part] = list
			}
			row, rowOK := list[0].(map[string]any)
			if !rowOK {
				row = map[string]any{}
				list[0] = row
			}
			current = row
			continue
		}
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		current[strings.TrimSuffix(parts[len(parts)-1], "[]")] = value
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
func anyText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

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
