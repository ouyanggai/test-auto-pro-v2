package service

import (
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
)

// buildComponentCandidatesMap 从候选集合构建字段路径到候选的映射。
func buildComponentCandidatesMap(template map[string]any, set target.ComponentCandidateSet) map[string][]any {
	fields, _ := formdata.ParseTemplate(template)
	result := make(map[string][]any)

	for _, field := range fields {
		if field.Type != "custom" || strings.TrimSpace(field.Capability) == "" {
			continue
		}

		var candidates []any
		switch field.Capability {
		case "out-bound-material-select":
			candidates = toAnyCandidates(set.Materials["out"])
		case "in-bound-material-select":
			candidates = toAnyCandidates(set.Materials["in"])
		case "custome-select-project":
			candidates = toAnyCandidates(set.Projects)
		case "travel-order-management":
			candidates = toAnyCandidates(set.Orders)
		case "general-flow-list-mulSelect", "flow-list-mul-select":
			candidates = toAnyCandidates(set.FlowLists)
		case "custome-expense-budgetType":
			candidates = toAnyCandidates(set.ExpenseBudgetTypes)
		case "city-select":
			candidates = toAnyCandidates(set.Cities)
		case "travel-route-planning":
			candidates = toAnyCandidates(set.TravelRoutes)
		}

		if len(candidates) > 0 {
			result[field.Path] = candidates
		}
	}

	return result
}

// toAnyCandidates 将类型化候选切片转换为 []any。
func toAnyCandidates[T any](items []T) []any {
	result := make([]any, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}
