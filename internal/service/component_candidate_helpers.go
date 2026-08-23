package service

import (
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
)

// componentCandidateTypes 返回当前模板实际使用且已有真实只读入口的组件类型。
func componentCandidateTypes(template map[string]any) []string {
	fields, _ := formdata.ParseTemplate(template)
	values := make([]string, 0)
	for _, field := range fields {
		if field.Type != "custom" || !verifiedRemoteCandidateComponent(field.Capability) {
			continue
		}
		values = append(values, field.Capability)
	}
	return target.SortedComponentCandidateTypes(values)
}

// verifiedRemoteCandidateComponent 只允许参考组件源码、宿主 API 常量和 Java DTO 已共同证实的读取入口。
func verifiedRemoteCandidateComponent(componentType string) bool {
	switch strings.TrimSpace(componentType) {
	case "custome-select-project", "in-bound-material-select", "out-bound-material-select", "city-select":
		return true
	default:
		return false
	}
}

// buildComponentCandidatesMap 把组件级候选绑定到模板中实际出现的字段路径。
func buildComponentCandidatesMap(template map[string]any, set target.ComponentCandidateSet) map[string][]any {
	fields, _ := formdata.ParseTemplate(template)
	result := make(map[string][]any)
	for _, field := range fields {
		if field.Type != "custom" || strings.TrimSpace(field.Capability) == "" {
			continue
		}
		values := set.ByComponent[field.Capability]
		if len(values) == 0 {
			continue
		}
		result[field.Path] = cloneAnyCandidates(values)
	}
	return result
}
