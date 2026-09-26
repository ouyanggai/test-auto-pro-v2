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
//   - 优先处理显示字段被补丁改过的控件，回填绑定值；
//   - 其次检查显示字段未被补丁改的控件，若当前绑定值不在合法选项里（可能是模板漂移导致选项 ID 变化），
//     落 STATIC_OPTION_STALE 阻断并告知用户字段名、当前值和可用选项，让用户清楚地知道该如何调整；
//   - 远程控件（remote:true）没有静态选项来源，不在此解析，仍按既有链路处理；
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
		before, beforeExisted := original[virtualField]
		virtualChanged := desiredOK && strings.TrimSpace(desired) != "" &&
			(!beforeExisted || strings.TrimSpace(fmt.Sprint(before)) != strings.TrimSpace(desired))

		current, currentOK := values[pair.valueField]
		if !currentOK || current == nil {
			continue
		}
		currentStr := fmt.Sprint(current)

		if virtualChanged {
			// 显示名被补丁改过：把新显示名解析为绑定值。
			if option, ok := pair.findByValue(currentStr); ok && option.label == desired {
				// 绑定值已与新显示名对应，无需解析。
				continue
			}
			matched, found := pair.matchByLabel(desired)
			if !found {
				issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{
					Code: "STATIC_OPTION_UNRESOLVED", Path: pair.valueField, Blocking: true,
					Message: fmt.Sprintf("静态选项里找不到「%s」，控件仍会显示并提交历史值：%v；可选项：%s",
						desired, currentStr, pair.optionLabels()),
				}})
				continue
			}
			if len(matched) != 1 {
				issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{
					Code: "STATIC_OPTION_AMBIGUOUS", Path: pair.valueField, Blocking: true,
					Message: fmt.Sprintf("静态选项里存在 %d 个「%s」，无法唯一确定绑定值，保留历史值：%v；可选项：%s",
						len(matched), desired, currentStr, pair.optionLabels()),
				}})
				continue
			}
			// 按目标渲染口径回填：值字段写入选项绑定值，虚拟显示字段保持补丁目标名。
			values[pair.valueField] = matched[0].value
			values[virtualField] = desired
		} else {
			// 显示名未被补丁改：检查历史绑定值是否还在当前模板的合法选项里（模板漂移检测）。
			// 值字段为空时不检查（表示未填写，不是非法值）。
			if strings.TrimSpace(currentStr) == "" {
				continue
			}
			if _, ok := pair.findByValue(currentStr); ok {
				// 绑定值仍在合法选项里，不做处理。
				continue
			}
			// 绑定值已不在合法选项里：模板漂移导致选项变化，告知用户需要重新选择。
			// 显示名有值时一并展示，帮助用户知道历史上选的是什么。
			historyDisplay := currentStr
			if desiredOK && strings.TrimSpace(desired) != "" {
				historyDisplay = desired + "（历史值 " + currentStr + "）"
			}
			issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{
				Code: "STATIC_OPTION_STALE", Path: pair.valueField, Blocking: true,
				Message: fmt.Sprintf("历史数据「%s」不在当前模板的合法选项里，需要重新选择；当前可选：%s",
					historyDisplay, pair.optionLabels()),
			}})
		}
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

// optionLabels 返回所有选项的显示名列表，用于在错误信息里告知用户可选值。
// 超过八条时截断并标注省略数量，避免错误信息过长。
func (p staticOptionPair) optionLabels() string {
	const maxDisplay = 8
	labels := make([]string, 0, len(p.options))
	for _, entry := range p.options {
		labels = append(labels, "「"+entry.label+"」")
	}
	if len(labels) <= maxDisplay {
		return strings.Join(labels, " / ")
	}
	visible := labels[:maxDisplay]
	return strings.Join(visible, " / ") + fmt.Sprintf(" 等共 %d 项", len(labels))
}
