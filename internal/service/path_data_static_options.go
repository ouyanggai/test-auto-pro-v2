package service

import (
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// staticOptionPair 是模板里一个静态下拉的绑定信息：值字段与它的虚拟显示字段。
// 目标约定控件绑定值字段（如 vacateType），FormMaking 为每个选项型控件维护 model+"__virtualName" 兜底显示字段。
type staticOptionPair struct {
	valueField string
	typeName   string
	multiple   bool
	options    []staticOptionEntry
}

// staticOptionEntry 是按目标渲染口径归一化后的一个选项：显示名与绑定值。
type staticOptionEntry struct {
	value string
	label string
}

// staticOptionBindingIssues 在服务端把"补丁只改了 __virtualName、绑定值仍是历史值"的静态下拉解析为真实绑定值。
// 背景：分支最小补丁按约定只改显示字段，绑定 ID/值此前只能由浏览器里的 form-runtime 选项协调回填。
// 用户裁决去掉浏览器验证环节，因此对 remote:false 且 options 声明完整的静态下拉，服务端可以直接按
// 目标渲染口径（showLabel 时显示 label，否则直接显示 value）把期望显示名唯一匹配成绑定值。
// 边界：
//   - 只处理值字段存在、虚拟显示字段被补丁改过（与原值不同）的控件，其他形态不猜测；
//   - 远程控件（remote:true）没有静态选项来源，不在此解析，仍按既有链路（远程数据源同步或浏览器）处理；
//   - 显示名在选项里找不到或同名多条时落阻断并保留原值，绝不猜测绑定值；
//   - 值字段已被补丁直接改写时视为已解析，不做覆盖。
func staticOptionBindingIssues(template map[string]any, original, values map[string]any) []model.HistoryDataIssue {
	pairs := staticOptionPairs(template)
	if len(pairs) == 0 {
		return nil
	}
	issues := []model.HistoryDataIssue{}
	for _, pair := range pairs {
		virtualField := pair.valueField + "__virtualName"
		desired, desiredOK := values[virtualField].(string)
		if !desiredOK || strings.TrimSpace(desired) == "" {
			continue
		}
		before, existed := original[virtualField]
		if !existed || strings.TrimSpace(fmt.Sprint(before)) == strings.TrimSpace(desired) {
			// 显示名没变化：可能是直接命中路径或历史已一致，绑定值不做任何改动。
			continue
		}
		current, currentOK := values[pair.valueField]
		if !currentOK || current == nil {
			continue
		}
		// 绑定值已与新显示名对应时无需解析（例如值字段本身就是显示值，如“事假”这类 value==显示名场景）。
		if option, ok := pair.findByValue(fmt.Sprint(current)); ok && option.label == desired {
			continue
		}
		matched, found := pair.matchByLabel(desired)
		if !found {
			issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{
				Code: "STATIC_OPTION_UNRESOLVED", Path: pair.valueField, Blocking: true,
				Message: fmt.Sprintf("静态选项里找不到%s，控件仍会显示并提交历史值：%v", desired, current),
			}})
			continue
		}
		if len(matched) != 1 {
			issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{
				Code: "STATIC_OPTION_AMBIGUOUS", Path: pair.valueField, Blocking: true,
				Message: fmt.Sprintf("静态选项里存在 %d 个%s，无法唯一确定绑定值，保留历史值：%v", len(matched), desired, current),
			}})
			continue
		}
		// 按目标渲染口径回填：值字段写入选项绑定值，虚拟显示字段保持补丁目标名，保证回显与提交一致。
		values[pair.valueField] = matched[0].value
		values[virtualField] = desired
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

// staticOptionPairs 从 FormMaking 模板提取静态下拉配对；与运行时 buildStaticOptions 同一套渲染口径。
// showLabel 声明为 true（或远程控件）时显示名取 option.label；否则目标渲染层直接显示 value，显示名即 value。
func staticOptionPairs(template map[string]any) []staticOptionPair {
	if len(template) == 0 {
		return nil
	}
	pairs := []staticOptionPair{}
	var visit func(node any)
	visit = func(node any) {
		switch typed := node.(type) {
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]any:
			options, _ := typed["options"].(map[string]any)
			typeName := templateString(typed["type"])
			modelName := templateString(typed["model"])
			if options != nil && modelName != "" && isOptionWidgetType(typeName) && options["remote"] != true && options["dataBind"] != false {
				if entries, ok := staticOptionEntries(options, typeName); ok {
					pairs = append(pairs, staticOptionPair{
						valueField: modelName, typeName: typeName, multiple: options["multiple"] == true, options: entries,
					})
				}
			}
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(template["list"])
	return pairs
}

// isOptionWidgetType 判断控件是否为值型选项控件；与运行时 OPTION_WIDGET_TYPES 同一口径。
func isOptionWidgetType(typeName string) bool {
	switch typeName {
	case "select", "radio", "checkbox":
		return true
	}
	return false
}

// staticOptionEntries 把模板静态选项按目标渲染口径归一化；与运行时 buildStaticOptions 行为一致。
// 级联不做服务端解析（树形匹配依赖运行时挂载后的真实数据源），返回 false 跳过。
func staticOptionEntries(options map[string]any, typeName string) ([]staticOptionEntry, bool) {
	if typeName == "cascader" {
		return nil, false
	}
	raw, ok := options["options"].([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	showLabel := options["showLabel"] == true
	entries := make([]staticOptionEntry, 0, len(raw))
	for _, item := range raw {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		value := templateString(option["value"])
		if value == "" {
			continue
		}
		label := value
		if showLabel {
			if declared := templateString(option["label"]); declared != "" {
				label = declared
			}
		}
		entries = append(entries, staticOptionEntry{value: value, label: label})
	}
	if len(entries) == 0 {
		return nil, false
	}
	return entries, true
}

// findByValue 按绑定值找选项，用于判断当前值是否已与新显示名一致。
func (p staticOptionPair) findByValue(value string) (staticOptionEntry, bool) {
	for _, entry := range p.options {
		if entry.value == value {
			return entry, true
		}
	}
	return staticOptionEntry{}, false
}

// matchByLabel 按显示名找全部选项；同名多条时由调用方阻断，不猜测。
func (p staticOptionPair) matchByLabel(label string) ([]staticOptionEntry, bool) {
	matched := []staticOptionEntry{}
	for _, entry := range p.options {
		if entry.label == label {
			matched = append(matched, entry)
		}
	}
	return matched, len(matched) > 0
}
