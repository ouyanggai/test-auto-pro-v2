package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/jsonvalues"
)

const (
	historyCandidateRemotePageSize = 100
	historyCandidateMaxRemotePages = 20
)

// HistoryTargetReader 是历史数据服务依赖的目标只读边界，不暴露目标会话或内部标识。
type HistoryTargetReader interface {
	HistoryIdentity(context.Context, string, string, string) (target.HistoryIdentity, error)
	HistoryCandidates(context.Context, string, string, string, string, int, int) (target.Page[target.HistoryInstance], error)
	ReadHistorySnapshot(context.Context, string, string, string, string, string) (target.HistorySnapshotSource, error)
}

// HistoryIdentity 按计划来源读取目标返回的流程编码、表单名称和运行时摘要。
func (s *TargetReadService) HistoryIdentity(ctx context.Context, account, source, targetObjectID string) (target.HistoryIdentity, error) {
	snapshot, err := s.PathConfigurationSnapshot(ctx, account, source, targetObjectID)
	if err != nil {
		return target.HistoryIdentity{}, err
	}
	flowName := strings.TrimSpace(snapshot.FlowName)
	formName := ""
	if len(snapshot.Forms) > 0 {
		formName = strings.TrimSpace(snapshot.Forms[0].Name)
	}
	// 历史身份只使用目标详情和目标表单原文；工具侧页面规则不能参与 F-012。
	templateSummary := historyTemplateSummary(snapshot)
	return target.HistoryIdentity{
		FlowCode: strings.TrimSpace(snapshot.FlowCode), FormName: formName,
		FlowName: flowName, RenderType: snapshot.RenderType, TemplateSummary: templateSummary,
	}, nil
}

// historyFormNames 仅保留目标表单 ID/名称摘要，模板正文由快照链路按需读取。
func historyFormNames(forms []target.FormRuntimeTemplate) []map[string]string {
	result := make([]map[string]string, 0, len(forms))
	for _, form := range forms {
		result = append(result, map[string]string{
			"id": strings.TrimSpace(form.ID), "name": strings.TrimSpace(form.Name),
		})
	}
	return result
}

// HistoryCandidates 读取计划账号可见的同流程历史实例，并按已完成状态优先排序。
func (s *TargetReadService) HistoryCandidates(ctx context.Context, account, flowCode, formName, flowName string, page, pageSize int) (target.Page[target.HistoryInstance], error) {
	if err := s.ready(); err != nil {
		return target.Page[target.HistoryInstance]{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	flowCode = strings.TrimSpace(flowCode)
	formName = strings.TrimSpace(formName)
	flowName = strings.TrimSpace(flowName)
	if flowCode == "" {
		return target.Page[target.HistoryInstance]{Page: page, PageSize: pageSize, Items: []target.HistoryInstance{}}, nil
	}
	var all []target.HistoryInstance
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		instances, readErr := s.readAllHistoryInstances(callContext, active, flowCode)
		if readErr == nil {
			all = instances
		}
		return readErr
	})
	if err != nil {
		return target.Page[target.HistoryInstance]{}, err
	}
	filtered := make([]target.HistoryInstance, 0, len(all))
	for _, item := range all {
		if strings.TrimSpace(item.FlowCode) != flowCode {
			continue
		}
		// 有表单时只接受目标返回的完整表单名称；无表单流程没有可拼装的名称约束。
		if formName != "" && strings.TrimSpace(item.FormName) != formName {
			continue
		}
		// 无表单流程使用目标返回的流程/页面名称；字段缺失时保留候选并在来源状态中提示人工核对。
		if formName == "" && flowName != "" && strings.TrimSpace(item.FlowName) != "" && strings.TrimSpace(item.FlowName) != flowName {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(left, right int) bool {
		leftRank, rightRank := historyStatusRank(filtered[left].Status), historyStatusRank(filtered[right].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return filtered[left].CreatedAt > filtered[right].CreatedAt
	})
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return target.Page[target.HistoryInstance]{Items: []target.HistoryInstance{}, Page: page, PageSize: pageSize, Total: total, HasMore: false}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := append([]target.HistoryInstance(nil), filtered[start:end]...)
	return target.Page[target.HistoryInstance]{
		Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: end < total,
	}, nil
}

// readAllHistoryInstances 在单账号会话内建立有界分页，防止目标异常分页造成无界读取。
func (s *TargetReadService) readAllHistoryInstances(ctx context.Context, active target.Session, flowCode string) ([]target.HistoryInstance, error) {
	result := make([]target.HistoryInstance, 0)
	seen := make(map[string]struct{})
	for page := 1; page <= historyCandidateMaxRemotePages; page++ {
		response, err := s.client.ListHistoryInstances(ctx, active, flowCode, page, historyCandidateRemotePageSize)
		if err != nil {
			return nil, err
		}
		for _, item := range response.Items {
			key := strings.TrimSpace(item.ID)
			if key != "" {
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
			}
			result = append(result, item)
		}
		if !response.HasMore || len(response.Items) == 0 {
			break
		}
		if page == historyCandidateMaxRemotePages {
			return nil, target.NewError(target.ErrorResponseInvalid, errors.New("历史候选分页超过安全上限"))
		}
	}
	return result, nil
}

// HistoryCandidateKey 生成只携带摘要可计算的不透明候选键，不把目标实例 ID 返回给浏览器。
func HistoryCandidateKey(account string, item target.HistoryInstance) string {
	digest := sha256.New()
	for _, value := range []string{
		strings.TrimSpace(account), strings.TrimSpace(item.ID), strings.TrimSpace(item.FlowCode),
		strings.TrimSpace(item.FormName), strings.TrimSpace(item.CreatedAt),
	} {
		digest.Write([]byte(value))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// ReadHistorySnapshot 读取候选当前原始表单数据并尽力绑定目标模板/页面摘要。
func (s *TargetReadService) ReadHistorySnapshot(ctx context.Context, account, flowCode, formName, flowName, candidateKey string) (target.HistorySnapshotSource, error) {
	if err := s.ready(); err != nil {
		return target.HistorySnapshotSource{}, err
	}
	flowCode = strings.TrimSpace(flowCode)
	formName = strings.TrimSpace(formName)
	flowName = strings.TrimSpace(flowName)
	candidateKey = strings.TrimSpace(candidateKey)
	if flowCode == "" || candidateKey == "" {
		return target.HistorySnapshotSource{}, ErrTargetFlowNotFound
	}
	var result target.HistorySnapshotSource
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		instances, readErr := s.readAllHistoryInstances(callContext, active, flowCode)
		if readErr != nil {
			return readErr
		}
		var selected *target.HistoryInstance
		for index := range instances {
			item := &instances[index]
			if strings.TrimSpace(item.FlowCode) != flowCode {
				continue
			}
			if formName != "" && strings.TrimSpace(item.FormName) != formName {
				continue
			}
			if formName == "" && flowName != "" && strings.TrimSpace(item.FlowName) != "" && strings.TrimSpace(item.FlowName) != flowName {
				continue
			}
			if HistoryCandidateKey(account, *item) == candidateKey {
				selected = item
				break
			}
		}
		if selected == nil {
			return ErrTargetFlowNotFound
		}
		rawData, rawErr := s.client.ReadInstanceCurrentData(callContext, active, selected.ID)
		if rawErr != nil {
			return rawErr
		}
		copiedRawData, copyErr := cloneMap(rawData)
		if copyErr != nil {
			return target.NewError(target.ErrorResponseInvalid, copyErr)
		}
		result = target.HistorySnapshotSource{
			Instance:    *selected,
			RawFormData: copiedRawData,
			Issues:      []string{},
		}
		if formName == "" && (flowName == "" || strings.TrimSpace(selected.FlowName) == "") {
			result.Issues = append(result.Issues, "目标无表单流程缺少稳定流程或页面名称")
		}
		selectedFormProxyIDs := append([]string(nil), selected.FormProxyIDs...)
		if selected.FlowProxyID == "" || (formName != "" && len(selectedFormProxyIDs) == 0) {
			proxyID, _, _, formProxyIDs, found, findErr := s.client.FindSubmittedFlow(callContext, active, selected.ID)
			if findErr == nil && found {
				if selected.FlowProxyID == "" {
					selected.FlowProxyID = proxyID
				}
				if len(selectedFormProxyIDs) == 0 {
					selectedFormProxyIDs = append([]string(nil), formProxyIDs...)
				}
			} else if findErr != nil {
				result.Issues = append(result.Issues, "目标流程模板摘要暂时无法读取")
			}
		}
		if selected.FlowProxyID != "" {
			snapshot, configErr := s.client.ReadProxyConfiguration(callContext, active, selected.FlowProxyID, selectedFormProxyIDs, selected.ID)
			if configErr == nil {
				result.RenderType = snapshot.RenderType
				result.TemplateSummary = historyTemplateSummary(snapshot)
				if formName != "" && len(snapshot.Forms) == 0 {
					result.Issues = append(result.Issues, "目标历史表单模板版本无法确认")
				}
				if len(result.RawFormData) == 0 && len(snapshot.InstanceValues) > 0 {
					copiedValues, valuesCopyErr := cloneMap(snapshot.InstanceValues)
					if valuesCopyErr != nil {
						return target.NewError(target.ErrorResponseInvalid, valuesCopyErr)
					}
					result.RawFormData = copiedValues
				}
			} else {
				result.Issues = append(result.Issues, "目标流程模板摘要暂时无法读取")
			}
		}
		if result.RenderType == target.FormRenderTypeUnknown {
			result.Issues = append(result.Issues, "目标表单运行时类型无法确认")
		}
		return nil
	})
	return result, err
}

// historyTemplateSummary 只从目标详情与目标表单原文生成版本摘要，不依赖旧页面规则映射。
func historyTemplateSummary(snapshot target.PathConfigurationSnapshot) map[string]any {
	result := map[string]any{
		"flowCode":   strings.TrimSpace(snapshot.FlowCode),
		"flowName":   strings.TrimSpace(snapshot.FlowName),
		"renderType": string(snapshot.RenderType),
		"formNames":  historyFormNames(snapshot.Forms),
	}
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		result["pageKey"] = strings.TrimSpace(snapshot.AuditWay)
	}
	if digest := historyRuntimeVersionDigest(snapshot); digest != "" {
		result["runtimeVersionDigest"] = digest
	}
	return result
}

// historyRuntimeVersionDigest 摘要目标原始运行时身份与表单正文，忽略模板/代理临时 ID。
func historyRuntimeVersionDigest(snapshot target.PathConfigurationSnapshot) string {
	if snapshot.RenderType == target.FormRenderTypeUnknown {
		return ""
	}
	if snapshot.RenderType == target.FormRenderTypeFormMaking && len(snapshot.Forms) == 0 {
		return ""
	}
	if snapshot.RenderType == target.FormRenderTypeVueCustom && strings.TrimSpace(snapshot.FlowName) == "" && strings.TrimSpace(snapshot.AuditWay) == "" {
		return ""
	}
	type versionForm struct {
		Name         string `json:"name"`
		TemplateData string `json:"templateData"`
	}
	forms := make([]versionForm, 0, len(snapshot.Forms))
	for _, form := range snapshot.Forms {
		forms = append(forms, versionForm{Name: strings.TrimSpace(form.Name), TemplateData: form.TemplateData})
	}
	payload, err := json.Marshal(struct {
		FlowCode   string                `json:"flowCode"`
		FlowName   string                `json:"flowName"`
		PageKey    string                `json:"pageKey"`
		RenderType target.FormRenderType `json:"renderType"`
		Forms      []versionForm         `json:"forms"`
	}{
		FlowCode: strings.TrimSpace(snapshot.FlowCode), FlowName: strings.TrimSpace(snapshot.FlowName),
		PageKey: strings.TrimSpace(snapshot.AuditWay), RenderType: snapshot.RenderType, Forms: forms,
	})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

// cloneMap 通过 JSON 编解码断开远程响应和持久化/调用方之间的对象引用，失败时拒绝降级为空数据。
func cloneMap(value map[string]any) (map[string]any, error) {
	return jsonvalues.DeepCopyObject(value)
}

// historyStatusRank 将目标状态归一为排序优先级，不改变目标状态文本或业务语义。
func historyStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "end":
		return 0
	case "rejected", "withdraw", "termination", "abandon":
		return 1
	case "run":
		return 2
	case "draft", "await_sent":
		return 3
	default:
		return 4
	}
}
