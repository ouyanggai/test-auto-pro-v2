package target

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrComponentCandidatesUnsupported 表示组件没有经过参考源码和 Java 协议共同证实的只读候选入口。
var ErrComponentCandidatesUnsupported = errors.New("组件没有已验证的只读候选入口")

// ComponentCandidateProvider 只按当前发起人会话读取单个真实组件类型的候选。
type ComponentCandidateProvider interface {
	ComponentCandidates(ctx context.Context, account, flowCode, componentType string) ([]any, error)
}

// ComponentCandidateSet 保存本次模板实际使用组件的候选，不预取全局注册表。
type ComponentCandidateSet struct {
	Account     string
	FlowCode    string
	TemplateID  string
	RuleVersion string
	ByComponent map[string][]any
}

type candidateCompanyNode struct {
	ID           string                 `json:"id"`
	ChildrenList []candidateCompanyNode `json:"childrenList"`
}

// ListComponentCandidates 按宿主组件真实读取协议分派候选；未知入口明确拒绝，不能猜 URL。
func (c *Client) ListComponentCandidates(ctx context.Context, session Session, componentType string) ([]any, error) {
	switch strings.TrimSpace(componentType) {
	case "custome-select-project":
		return c.listProjectCandidates(ctx, session)
	case "in-bound-material-select", "out-bound-material-select":
		return c.listMaterialCandidates(ctx, session, componentType)
	case "city-select":
		return c.listCityCandidates(ctx, session)
	default:
		return nil, fmt.Errorf("%w：%s", ErrComponentCandidatesUnsupported, strings.TrimSpace(componentType))
	}
}

// listProjectCandidates 复现宿主项目组件的公司树与公司/集团项目两段只读查询。
func (c *Client) listProjectCandidates(ctx context.Context, session Session) ([]any, error) {
	if strings.TrimSpace(session.CompanyID) == "" {
		return nil, invalidResponse("project candidates require initiator company")
	}
	companyResponse, err := c.call(ctx, "/web/user/api/company/children", session.SID, map[string]any{
		"data": map[string]any{"id": session.CompanyID, "flag": 7},
	})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(companyResponse) {
		return nil, responseError(companyResponse)
	}
	var roots []candidateCompanyNode
	if err := decodeArray(companyResponse.Data, &roots); err != nil {
		return nil, err
	}
	// 宿主组件只保留返回根节点下与当前公司相同的分支；扩大树会越过发起人页面实际可选范围。
	if len(roots) > 0 {
		filtered := make([]candidateCompanyNode, 0, 1)
		for _, child := range roots[0].ChildrenList {
			if strings.TrimSpace(child.ID) == strings.TrimSpace(session.CompanyID) {
				filtered = append(filtered, child)
			}
		}
		roots[0].ChildrenList = filtered
	}
	companyIDs := collectCandidateCompanyIDs(roots)
	result := make([]any, 0)
	seen := make(map[string]struct{})
	for _, companyID := range companyIDs {
		projectResponse, readErr := c.call(ctx, "/web/project/api/getProjectVosOfCompanyAndGroup", session.SID, map[string]any{
			"data": map[string]any{"companyId": companyID},
		})
		if readErr != nil {
			return nil, readErr
		}
		if !responseSucceeded(projectResponse) {
			return nil, responseError(projectResponse)
		}
		items, decodeErr := decodeCandidateMaps(projectResponse.Data)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = appendUniqueCandidateMaps(result, items, seen)
	}
	return result, nil
}

// collectCandidateCompanyIDs 按宿主树顺序收集可见公司并去重，防止异常循环重复请求。
func collectCandidateCompanyIDs(roots []candidateCompanyNode) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	var visit func([]candidateCompanyNode)
	visit = func(nodes []candidateCompanyNode) {
		for _, node := range nodes {
			id := strings.TrimSpace(node.ID)
			if id != "" {
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				result = append(result, id)
			}
			visit(node.ChildrenList)
		}
	}
	visit(roots)
	return result
}

// listMaterialCandidates 复现出入库组件的台账查询，并保留出库库存大于零的真实限制。
func (c *Client) listMaterialCandidates(ctx context.Context, session Session, componentType string) ([]any, error) {
	response, err := c.call(ctx, "/web/warehouse/center/api/w2/goodsLedger/getSetLedgerGoods", session.SID, map[string]any{
		"data": map[string]any{"goodsName": "", "warehouse": map[string]any{"id": ""}}, "pagination": false,
	})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(response) {
		return nil, responseError(response)
	}
	items, err := decodeCandidateMaps(response.Data)
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(fmt.Sprint(item["id"])) == "" {
			continue
		}
		if componentType == "out-bound-material-select" && candidateNumber(item["totalCount"]) <= 0 {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

// listCityCandidates 使用宿主 CitySelect 和 Java DTO 已核实的本地城市第一页协议。
func (c *Client) listCityCandidates(ctx context.Context, session Session) ([]any, error) {
	response, err := c.call(ctx, "/web/hesi/city/local/list", session.SID, map[string]any{
		"pagination": true, "current": 1, "size": 100,
		"data": map[string]any{"name": "", "cityType": ""},
	})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(response) {
		return nil, responseError(response)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Data, &page); err != nil {
		return nil, invalidResponse("invalid city candidate page")
	}
	result := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		if strings.TrimSpace(fmt.Sprint(item["name"])) == "" {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

// decodeCandidateMaps 只接受候选接口已核实的对象数组响应。
func decodeCandidateMaps(data json.RawMessage) ([]map[string]any, error) {
	var items []map[string]any
	if err := decodeArray(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// appendUniqueCandidateMaps 按真实对象 id 去重；缺 id 的外部对象不能进入生成候选。
func appendUniqueCandidateMaps(result []any, items []map[string]any, seen map[string]struct{}) []any {
	for _, item := range items {
		id := strings.TrimSpace(fmt.Sprint(item["id"]))
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, item)
	}
	return result
}

// candidateNumber 把目标 JSON 数值转换为比较用浮点数，其他形态按无库存处理。
func candidateNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		return 0
	}
}

// SortedComponentCandidateTypes 返回稳定去重的组件候选类型，供缓存键和并发加载共用。
func SortedComponentCandidateTypes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
