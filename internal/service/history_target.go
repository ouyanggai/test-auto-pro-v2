package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/repository"
)

const (
	historyCandidateRemotePageSize = 100
	historyCandidateMaxRemotePages = 20
)

// historyCandidateExcludedNameKeywords 剔除历史自动化回归产生的实例：这些数据来自旧版本，
// 结构与当前目标表单不一致，不能作为基础表单数据。
var historyCandidateExcludedNameKeywords = []string{"自动"}

// historyCandidateNameAllowed 判定实例名称是否属于可用的真实业务数据。
func historyCandidateNameAllowed(name string) bool {
	name = strings.TrimSpace(name)
	for _, keyword := range historyCandidateExcludedNameKeywords {
		if strings.Contains(name, keyword) {
			return false
		}
	}
	return true
}

// historyCandidateStatusGroups 按目标状态枚举分组读取候选：已完成实例整体排在前面，
// 其余状态保持目标列表返回顺序；分组读取让"已完成优先"不再依赖把全部历史读进内存。
var historyCandidateStatusGroups = [][]string{
	{"end"},
	{"run", "await_sent", "draft", "withdraw", "rejected", "termination", "abandon"},
}

// HistoryTargetReader 是历史数据服务依赖的目标只读边界，不暴露目标会话或内部标识。
type HistoryTargetReader interface {
	HistoryIdentity(context.Context, string, string, string) (target.HistoryIdentity, error)
	HistoryCandidates(context.Context, string, string, string, string, string, int, int) (target.Page[target.HistoryInstance], error)
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

// HistoryCandidates 读取同流程的全部业务实例，已完成实例整体优先于其他状态。
// 配置了目标业务库只读连接时走一次联表查询；否则回落到目标只读 API 的分组分页读取。
func (s *TargetReadService) HistoryCandidates(ctx context.Context, account, flowCode, formName, flowName, query string, page, pageSize int) (target.Page[target.HistoryInstance], error) {
	if s.candidates == nil {
		if err := s.ready(); err != nil {
			return target.Page[target.HistoryInstance]{}, err
		}
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
	// 实例列表行不返回 flowCode，身份只能由目标返回的表单/流程名称构成；两者都缺失时不猜测。
	if formName == "" && flowName == "" {
		return target.Page[target.HistoryInstance]{Page: page, PageSize: pageSize, Items: []target.HistoryInstance{}}, nil
	}
	if s.candidates != nil {
		return s.candidatesFromBizDB(ctx, flowCode, formName, flowName, query, page, pageSize)
	}
	need := page * pageSize
	matched := make([]target.HistoryInstance, 0, need)
	total := 0
	seen := make(map[string]struct{})
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		for index := range historyCandidateStatusGroups {
			group, groupTotal, groupErr := s.collectHistoryInstances(callContext, active, flowName, index, need-len(matched), seen, func(item target.HistoryInstance) bool {
				return historyCandidateIdentityMatch(item, flowCode, formName, flowName)
			})
			if groupErr != nil {
				return groupErr
			}
			total += groupTotal
			matched = append(matched, group...)
		}
		return nil
	})
	if err != nil {
		return target.Page[target.HistoryInstance]{}, err
	}
	start := (page - 1) * pageSize
	if start >= len(matched) {
		return target.Page[target.HistoryInstance]{Items: []target.HistoryInstance{}, Page: page, PageSize: pageSize, Total: total, HasMore: false}, nil
	}
	end := start + pageSize
	if end > len(matched) {
		end = len(matched)
	}
	items := append([]target.HistoryInstance(nil), matched[start:end]...)
	return target.Page[target.HistoryInstance]{
		Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: end < total,
	}, nil
}

// historyCandidateStatusAccepted 判定目标返回行属于哪个状态分组；最后一个分组兼容目标新增状态，
// 保证任何状态的实例都仍然可选，只是不会抢占已完成实例的位置。
func historyCandidateStatusAccepted(status string, groupIndex int) bool {
	status = strings.TrimSpace(status)
	if groupIndex == len(historyCandidateStatusGroups)-1 {
		for index := 0; index < groupIndex; index++ {
			for _, value := range historyCandidateStatusGroups[index] {
				if value == status {
					return false
				}
			}
		}
		return true
	}
	for _, value := range historyCandidateStatusGroups[groupIndex] {
		if value == status {
			return true
		}
	}
	return false
}

// candidatesFromBizDB 用一次目标业务库联表查询取回候选关键列；不读取任何表单正文。
func (s *TargetReadService) candidatesFromBizDB(ctx context.Context, flowCode, formName, flowName, query string, page, pageSize int) (target.Page[target.HistoryInstance], error) {
	rows, total, err := s.candidates.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{
		FlowCode: flowCode, FlowName: flowName, FormName: formName, Query: query,
		ExcludeNameKeywords: historyCandidateExcludedNameKeywords,
	}, page, pageSize)
	if err != nil {
		return target.Page[target.HistoryInstance]{}, target.NewError(target.ErrorResponseInvalid, err)
	}
	items := make([]target.HistoryInstance, 0, len(rows))
	for _, row := range rows {
		items = append(items, historyInstanceFromCandidateRow(row))
	}
	return target.Page[target.HistoryInstance]{
		Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: page*pageSize < total,
	}, nil
}

// historyInstanceFromCandidateRow 把业务库关键列投影为与目标只读 API 相同的实例摘要模型。
func historyInstanceFromCandidateRow(row repository.TargetHistoryCandidateRow) target.HistoryInstance {
	formProxyIDs := make([]string, 0, 1)
	if strings.TrimSpace(row.FormProxyID) != "" {
		formProxyIDs = append(formProxyIDs, strings.TrimSpace(row.FormProxyID))
	}
	return target.HistoryInstance{
		ID: row.InstanceID, FlowProxyID: row.FlowProxyID, FormProxyIDs: formProxyIDs,
		FlowCode: row.FlowCode, FlowName: row.FlowName, FormName: row.FormName,
		Title:           firstHistoryValue(row.Name, row.FormName, row.FlowName),
		BusinessSummary: strings.TrimSpace(row.Name),
		Initiator:       row.InitiatorName, CompanyName: row.CompanyName, CreatedAt: row.CreatedAt,
		Status: row.Status, StatusName: target.SubmittedStatusText(row.Status),
		ActiveNodeProxyIDs: []string{},
	}
}

// firstHistoryValue 返回第一个非空摘要字段，避免候选标题为空。
func firstHistoryValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// historyCandidateIdentityMatch 只用目标返回的原始身份字段判定候选归属，不拼装新的身份键。
func historyCandidateIdentityMatch(item target.HistoryInstance, flowCode, formName, flowName string) bool {
	// 目标只在部分实例上写入 flowCode；提供时按精确值校验，缺失时不能据此排除候选。
	if flowCode != "" && strings.TrimSpace(item.FlowCode) != "" && strings.TrimSpace(item.FlowCode) != flowCode {
		return false
	}
	itemFormName, itemFlowName := strings.TrimSpace(item.FormName), strings.TrimSpace(item.FlowName)
	// 有表单时只接受目标返回的完整表单名称；该行没有表单名称时退回同样由目标返回的流程名称。
	if formName != "" {
		if itemFormName != "" {
			return itemFormName == formName
		}
		return flowName != "" && itemFlowName == flowName
	}
	// 无表单流程使用目标返回的流程/页面名称；字段缺失时保留候选并在来源状态中提示人工核对。
	return itemFlowName == "" || itemFlowName == flowName
}

// collectHistoryInstances 在一个目标状态分组内按目标分页顺序收集匹配候选，够用即停，
// 避免账号业务实例达到数千条时把整个列表读进内存。
func (s *TargetReadService) collectHistoryInstances(ctx context.Context, active target.Session, flowName string, groupIndex, limit int, seen map[string]struct{}, match func(target.HistoryInstance) bool) ([]target.HistoryInstance, int, error) {
	query := target.HistoryInstanceQuery{FlowName: flowName, StatusList: historyCandidateStatusGroups[groupIndex]}
	if limit <= 0 {
		// 已经收集到足够候选：只读一条用于取回该分组的目标总数，保持分页器计数真实。
		response, err := s.client.ListHistoryInstances(ctx, active, query, 1, 1)
		if err != nil {
			return nil, 0, err
		}
		return nil, response.Total, nil
	}
	collected := make([]target.HistoryInstance, 0, limit)
	total := 0
	for page := 1; page <= historyCandidateMaxRemotePages; page++ {
		response, err := s.client.ListHistoryInstances(ctx, active, query, page, historyCandidateRemotePageSize)
		if err != nil {
			return nil, 0, err
		}
		if page == 1 {
			total = response.Total
		}
		for _, item := range response.Items {
			key := strings.TrimSpace(item.ID)
			if key != "" {
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
			}
			// 本地复核目标状态归组，既保证已完成优先，又让分组读取不依赖目标是否真的应用了 statusList。
			if !historyCandidateStatusAccepted(item.Status, groupIndex) {
				continue
			}
			if !historyCandidateNameAllowed(item.BusinessSummary) || !match(item) {
				continue
			}
			collected = append(collected, item)
		}
		if !response.HasMore || len(response.Items) == 0 || len(collected) >= limit {
			break
		}
	}
	sort.SliceStable(collected, func(left, right int) bool { return collected[left].CreatedAt > collected[right].CreatedAt })
	return collected, total, nil
}

// findCandidateInBizDB 在业务库候选窗口内按同一候选键定位实例，命中即停止翻页。
func (s *TargetReadService) findCandidateInBizDB(ctx context.Context, account, flowCode, formName, flowName, candidateKey string) (*target.HistoryInstance, error) {
	for page := 1; page <= historyCandidateMaxRemotePages; page++ {
		result, err := s.candidatesFromBizDB(ctx, flowCode, formName, flowName, "", page, historyCandidateRemotePageSize)
		if err != nil {
			return nil, err
		}
		for index := range result.Items {
			item := result.Items[index]
			if HistoryCandidateKey(account, item) == candidateKey {
				return &item, nil
			}
		}
		if !result.HasMore || len(result.Items) == 0 {
			break
		}
	}
	return nil, ErrTargetFlowNotFound
}

// findHistoryInstanceByKey 按同一组身份规则在目标分页内定位候选键对应实例，找到即停止读取。
func (s *TargetReadService) findHistoryInstanceByKey(ctx context.Context, active target.Session, account, flowCode, formName, flowName, candidateKey string) (*target.HistoryInstance, error) {
	if s.candidates != nil {
		return s.findCandidateInBizDB(ctx, account, flowCode, formName, flowName, candidateKey)
	}
	for groupIndex := range historyCandidateStatusGroups {
		query := target.HistoryInstanceQuery{FlowName: flowName, StatusList: historyCandidateStatusGroups[groupIndex]}
		for page := 1; page <= historyCandidateMaxRemotePages; page++ {
			response, err := s.client.ListHistoryInstances(ctx, active, query, page, historyCandidateRemotePageSize)
			if err != nil {
				return nil, err
			}
			for index := range response.Items {
				item := response.Items[index]
				if !historyCandidateStatusAccepted(item.Status, groupIndex) || !historyCandidateIdentityMatch(item, flowCode, formName, flowName) {
					continue
				}
				if HistoryCandidateKey(account, item) == candidateKey {
					return &item, nil
				}
			}
			if !response.HasMore || len(response.Items) == 0 {
				break
			}
		}
	}
	return nil, ErrTargetFlowNotFound
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
	if (formName == "" && flowName == "") || candidateKey == "" {
		return target.HistorySnapshotSource{}, ErrTargetFlowNotFound
	}
	var result target.HistorySnapshotSource
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		selected, findErr := s.findHistoryInstanceByKey(callContext, active, account, flowCode, formName, flowName, candidateKey)
		if findErr != nil {
			return findErr
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
					result.Issues = append(result.Issues, "目标业务表单模板版本无法确认")
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
