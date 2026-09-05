package target

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"test-auto-pro-v2/internal/jsonvalues"
)

// 本文件承接目标客户端的实例事实类只读读取（模板树、表单元数据、实例当前数据与转换器），
// 与 client.go 同属 target 包；拆分只为满足纲领第 10 节的单文件行数上限，不改任何行为。
// FindVisibleTemplate 通过顶层 ids 精确核对保存模板，不能依赖目标端不会筛选的 data.id。
func (c *Client) FindVisibleTemplate(ctx context.Context, active Session, templateID string) (bool, error) {
	body := map[string]any{
		"data": map[string]any{
			"useScope":     "invest",
			"customerCode": firstNonEmpty(active.CustomerCode, c.config.CustomerCode),
		},
		"ids":                                []string{strings.TrimSpace(templateID)},
		"showMe":                             true,
		"ignoreFormTemplateBizRelevanceData": true,
		"formTemplateBizRelevanceList":       []any{},
		"notFormTemplateBizRelevanceList":    []map[string]any{{"otherBiz": "isProject", "otherBizId": "isProject"}},
		"ignoreTemplateData":                 true,
		"pagination":                         true,
		"pages":                              1,
		"size":                               100,
		"projectId":                          "",
		"platformCode":                       c.config.TemplatePlatformCodes,
		"notAuditWayList":                    []string{"staff_annual_assessment"},
	}
	resp, err := c.call(ctx, "/web/flowTemplateApi/list", active.SID, body)
	if err != nil {
		return false, err
	}
	if !responseSucceeded(resp) {
		return false, responseError(resp)
	}
	var raw []rawFlowTemplate
	if err := decodeArray(resp.Data, &raw); err != nil {
		return false, err
	}
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(templateID) {
			return true, nil
		}
	}
	return false, nil
}

// FindSubmittedFlow 精确重查已发实例并返回代理树标识、活动入口、真实状态和代理表单。
func (c *Client) FindSubmittedFlow(ctx context.Context, active Session, instanceID string) (string, []string, string, []string, bool, error) {
	// 按实例 ID 精确复查事实时绝不附加业务关联过滤。
	// 实测（2026-09-05，实例 6bd617f3069d462d8bfe63ba12b35739）：带上
	// flowInstanceBizRelevanceList=[{otherBiz:company,otherBizId:""}] 时目标返回空集，
	// 去掉它立刻命中——本工具发起的实例不带公司业务关联，那个过滤会把它整条排除。
	// 后果是核验重读把"确实已生效的发起"读成"实例不可见"，判成不确定并把路径推进待对账，
	// 对账五维也随之全部读不到。这里问的是"这条实例现在什么状态"，与它挂在哪个公司列表无关。
	resp, err := c.call(ctx, "/web/flowInstanceApi/list", active.SID, map[string]any{
		"data": map[string]any{
			"useScope":     "invest",
			"auditWayList": []string{},
			"statusList":   []string{"draft", "await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"},
		},
		"ids": []string{strings.TrimSpace(instanceID)}, "pagination": true, "pages": 1, "size": 100,
	})
	if err != nil {
		return "", nil, "", nil, false, err
	}
	if !responseSucceeded(resp) {
		return "", nil, "", nil, false, responseError(resp)
	}
	var raw []struct {
		ID                   string          `json:"id"`
		FlowProxyID          string          `json:"flowProxyId"`
		FormProxyID          string          `json:"formProxyId"`
		Status               string          `json:"status"`
		CurrentNodeProxyID   string          `json:"currentNodeProxyId"`
		CurrentAuditUserInfo json.RawMessage `json:"currentAuditUserInfo"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return "", nil, "", nil, false, err
	}
	for _, item := range raw {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(instanceID) && strings.TrimSpace(item.FlowProxyID) != "" {
			// 活动节点集合优先于单一 currentNodeProxyId，避免并行入口被压缩成一个节点。
			entries := auditNodeIDs(item.CurrentAuditUserInfo)
			if len(entries) == 0 && strings.TrimSpace(item.CurrentNodeProxyID) != "" {
				entries = []string{strings.TrimSpace(item.CurrentNodeProxyID)}
			}
			formProxyIDs := make([]string, 0, 1)
			if formID := strings.TrimSpace(item.FormProxyID); formID != "" {
				formProxyIDs = append(formProxyIDs, formID)
			}
			return strings.TrimSpace(item.FlowProxyID), entries, strings.TrimSpace(item.Status), formProxyIDs, true, nil
		}
	}
	return "", nil, "", nil, false, nil
}

// FindDueFlow 精确重查实例全部 waiting_send 任务并汇总其代理节点入口和代理表单。
func (c *Client) FindDueFlow(ctx context.Context, active Session, instanceID string) (string, []string, []string, bool, error) {
	proxyID := ""
	entries := make([]string, 0)
	seen := make(map[string]struct{})
	formProxyIDs := make([]string, 0)
	seenForms := make(map[string]struct{})
	const pageSize = 100
	const maxPages = 20
	for page := 1; page <= maxPages; page++ {
		// 待发实例可能同时存在多个并行任务，必须遍历目标分页，不能只保留第一页入口。
		resp, err := c.call(ctx, "/web/flowJobTaskLink/list", active.SID, map[string]any{
			"data": map[string]any{
				"flowInstanceId":               strings.TrimSpace(instanceID),
				"taskStatus":                   "waiting_send",
				"auditWayList":                 []string{},
				"useScope":                     "invest",
				"flowInstanceBizRelevance":     map[string]any{},
				"flowInstanceBizRelevanceList": []any{},
			},
			"pagination": true, "pages": page, "size": pageSize,
		})
		if err != nil {
			return "", nil, nil, false, err
		}
		if !responseSucceeded(resp) {
			return "", nil, nil, false, responseError(resp)
		}
		var raw []struct {
			FlowInstanceID  string `json:"flowInstanceId"`
			FlowProxyID     string `json:"flowProxyId"`
			FlowNodeProxyID string `json:"flowNodeProxyId"`
			FormProxyID     string `json:"formProxyId"`
		}
		if err := decodeArray(resp.Data, &raw); err != nil {
			return "", nil, nil, false, err
		}
		for _, item := range raw {
			if strings.TrimSpace(item.FlowInstanceID) != strings.TrimSpace(instanceID) || strings.TrimSpace(item.FlowProxyID) == "" {
				continue
			}
			currentProxyID := strings.TrimSpace(item.FlowProxyID)
			// 同一实例任务若指向不同代理树，无法证明入口归属，必须拒绝而不是任选一个。
			if proxyID != "" && proxyID != currentProxyID {
				return "", nil, nil, false, invalidResponse("due tasks reference different flow proxies")
			}
			proxyID = currentProxyID
			entryID := strings.TrimSpace(item.FlowNodeProxyID)
			if entryID == "" {
				continue
			}
			if _, exists := seen[entryID]; !exists {
				seen[entryID] = struct{}{}
				entries = append(entries, entryID)
			}
			formID := strings.TrimSpace(item.FormProxyID)
			if formID != "" {
				if _, exists := seenForms[formID]; !exists {
					seenForms[formID] = struct{}{}
					formProxyIDs = append(formProxyIDs, formID)
				}
			}
		}
		hasMore := false
		if resp.Pages > 0 {
			if resp.Pages > maxPages {
				return "", nil, nil, false, invalidResponse("due task pagination exceeds safe limit")
			}
			hasMore = page < resp.Pages
		} else {
			hasMore = len(raw) >= pageSize
		}
		if !hasMore {
			break
		}
		// 目标可能省略 pages 且持续返回满页；硬上限必须独立于目标元数据，避免异常响应造成无界读取。
		if page == maxPages {
			return "", nil, nil, false, invalidResponse("due task pagination exceeds safe limit")
		}
	}
	if proxyID == "" {
		return "", nil, nil, false, nil
	}
	return proxyID, entries, formProxyIDs, true, nil
}

// ReadTemplateTree 按模板 ID 读取新发起的真实节点树。
func (c *Client) ReadTemplateTree(ctx context.Context, active Session, templateID string) (*FlowNodeTemplate, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	return tree, err
}

// ReadProxyTree 按已核实的 flowProxyId 读取既有实例代理树。
func (c *Client) ReadProxyTree(ctx context.Context, active Session, proxyID string) (*FlowNodeTemplate, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	return tree, err
}

// ReadTemplateRequirements 读取模板树及其关联表单字段，供路径要求核对内部使用。
func (c *Client) ReadTemplateRequirements(ctx context.Context, active Session, templateID string) (*FlowNodeTemplate, []FormFieldMetadata, error) {
	tree, forms, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	if err != nil {
		return nil, nil, err
	}
	fields, err := c.readFormFields(ctx, active, "/web/formTemplateApi/findById", forms)
	return tree, fields, err
}

// ReadProxyRequirements 读取代理树及实例代理表单字段，不回退到模板表单猜测运行态字段。
func (c *Client) ReadProxyRequirements(ctx context.Context, active Session, proxyID string, formProxyIDs []string) (*FlowNodeTemplate, []FormFieldMetadata, error) {
	tree, _, _, _, _, _, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	if err != nil {
		return nil, nil, err
	}
	forms := make([]rawFormReference, 0, len(formProxyIDs))
	for _, rawID := range formProxyIDs {
		if id := strings.TrimSpace(rawID); id != "" {
			forms = append(forms, rawFormReference{ID: id})
		}
	}
	fields, err := c.readFormFields(ctx, active, "/web/formProxy/findById", forms)
	return tree, fields, err
}

// ReadTemplateConfiguration 读取模板树、表单字段详情和模板默认值，供新发起路径配置使用。
func (c *Client) ReadTemplateConfiguration(ctx context.Context, active Session, templateID string) (PathConfigurationSnapshot, error) {
	tree, forms, flowCode, flowName, auditWay, formExist, err := c.readFlowDetail(ctx, active, "/web/flowTemplateApi/findById", templateID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	c.resolveFlowAuditMetadata(ctx, active, tree)
	fields, runtimeForms, err := c.readFormFieldDetails(ctx, active, "/web/formTemplateApi/findById", forms)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	renderType := NormalizeFormRenderType(formExist, len(runtimeForms))
	return PathConfigurationSnapshot{Tree: tree, FlowCode: flowCode, FlowName: flowName, AuditWay: auditWay, RenderType: renderType, VuePage: ResolveVueCustomPage(renderType, auditWay, flowName), FormFields: fields, Forms: runtimeForms}, nil
}

// ReadProxyConfiguration 读取代理树、实例代理表单字段详情和实例当前表单数据，供已发/待发路径配置使用。
func (c *Client) ReadProxyConfiguration(ctx context.Context, active Session, proxyID string, formProxyIDs []string, instanceID string) (PathConfigurationSnapshot, error) {
	tree, _, flowCode, flowName, auditWay, formExist, err := c.readFlowDetail(ctx, active, "/web/flowProxy/findById", proxyID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	c.resolveFlowAuditMetadata(ctx, active, tree)
	forms := make([]rawFormReference, 0, len(formProxyIDs))
	for _, rawID := range formProxyIDs {
		if id := strings.TrimSpace(rawID); id != "" {
			forms = append(forms, rawFormReference{ID: id})
		}
	}
	fields, runtimeForms, err := c.readFormFieldDetails(ctx, active, "/web/formProxy/findById", forms)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	values, err := c.readInstanceCurrentData(ctx, active, instanceID)
	if err != nil {
		return PathConfigurationSnapshot{}, err
	}
	renderType := NormalizeFormRenderType(formExist, len(runtimeForms))
	return PathConfigurationSnapshot{Tree: tree, FlowCode: flowCode, FlowName: flowName, AuditWay: auditWay, RenderType: renderType, VuePage: ResolveVueCustomPage(renderType, auditWay, flowName), FormFields: fields, Forms: runtimeForms, InstanceValues: values}, nil
}

// readFlowDetail 调用目标详情端点并转换同一棵流程树和关联表单引用。
func (c *Client) readFlowDetail(ctx context.Context, active Session, path, id string) (*FlowNodeTemplate, []rawFormReference, string, string, string, string, error) {
	resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": strings.TrimSpace(id)}})
	if err != nil {
		return nil, nil, "", "", "", "", err
	}
	if !responseSucceeded(resp) {
		return nil, nil, "", "", "", "", responseError(resp)
	}
	var data struct {
		FlowNodeTemplate *rawFlowNodeTemplate `json:"flowNodeTemplate"`
		FormTemplateList []rawFormReference   `json:"formTemplateList"`
		Code             string               `json:"code"`
		FlowCode         string               `json:"flowCode"`
		FlowName         string               `json:"flowName"`
		AuditWay         string               `json:"auditWay"`
		FormExist        string               `json:"formExist"`
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil, nil, "", "", "", "", nil
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, nil, "", "", "", "", invalidResponse("invalid flow tree data")
	}
	return convertFlowNode(data.FlowNodeTemplate), data.FormTemplateList, firstNonEmpty(data.FlowCode, data.Code), strings.TrimSpace(data.FlowName), strings.TrimSpace(data.AuditWay), data.FormExist, nil
}

// readFormFields 逐个读取已核实表单详情，只保留中文展示所需的名称字典。
func (c *Client) readFormFields(ctx context.Context, active Session, path string, forms []rawFormReference) ([]FormFieldMetadata, error) {
	result := make([]FormFieldMetadata, 0)
	seen := make(map[string]struct{}, len(forms))
	for _, form := range forms {
		formID := strings.TrimSpace(form.ID)
		if formID == "" {
			continue
		}
		if _, exists := seen[formID]; exists {
			continue
		}
		seen[formID] = struct{}{}
		resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": formID}})
		if err != nil {
			return nil, err
		}
		if !responseSucceeded(resp) {
			return nil, responseError(resp)
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			continue
		}
		var data struct {
			ID     string         `json:"id"`
			Name   string         `json:"name"`
			Fields []rawFormField `json:"fieldsTemplateList"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, invalidResponse("invalid form field data")
		}
		resolvedFormID := firstNonEmpty(data.ID, formID)
		resolvedFormName := firstNonEmpty(data.Name, form.Name)
		for _, field := range data.Fields {
			result = append(result, FormFieldMetadata{
				FormID: resolvedFormID, FormName: resolvedFormName, FieldID: field.ID,
				Name: field.Name, EnglishName: field.EnglishName,
			})
		}
	}
	return result, nil
}

// readFormFieldDetails 逐个读取已核实表单详情，并把 FormMaking 组件配置合并为字段类型、必填、默认值和选项。
func (c *Client) readFormFieldDetails(ctx context.Context, active Session, path string, forms []rawFormReference) ([]FormFieldDetail, []FormRuntimeTemplate, error) {
	result := make([]FormFieldDetail, 0)
	runtimeForms := make([]FormRuntimeTemplate, 0, len(forms))
	seen := make(map[string]struct{}, len(forms))
	for _, form := range forms {
		formID := strings.TrimSpace(form.ID)
		if formID == "" {
			continue
		}
		if _, exists := seen[formID]; exists {
			continue
		}
		seen[formID] = struct{}{}
		resp, err := c.call(ctx, path, active.SID, map[string]any{"data": map[string]any{"id": formID}})
		if err != nil {
			return nil, nil, err
		}
		if !responseSucceeded(resp) {
			return nil, nil, responseError(resp)
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			continue
		}
		var data struct {
			ID           string               `json:"id"`
			Name         string               `json:"name"`
			Fields       []rawFormFieldDetail `json:"fieldsTemplateList"`
			TemplateData string               `json:"templateData"`
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, nil, invalidResponse("invalid form field data")
		}
		components := parseFormMakingComponents(data.TemplateData)
		resolvedFormID := firstNonEmpty(data.ID, formID)
		resolvedFormName := firstNonEmpty(data.Name, form.Name)
		if strings.TrimSpace(data.TemplateData) != "" {
			runtimeForms = append(runtimeForms, FormRuntimeTemplate{ID: resolvedFormID, Name: resolvedFormName, TemplateData: data.TemplateData})
		}
		for _, field := range data.Fields {
			component, hasComponent := components[strings.TrimSpace(field.EnglishName)]
			detail := FormFieldDetail{
				FormID: resolvedFormID, FormName: resolvedFormName, FieldID: field.ID,
				Name: firstNonEmpty(component.Name, field.Name), EnglishName: field.EnglishName,
				FieldType: strings.TrimSpace(field.FieldType), DefaultValue: strings.TrimSpace(field.DefaultValue),
				ValueOrigin: strings.TrimSpace(field.ValueOrigin), FieldStatus: strings.TrimSpace(field.FieldStatus),
				ComponentType: component.Type, ComponentName: formMakingComponentName(component),
				DateMode: strings.TrimSpace(component.Options.Type),
			}
			if hasComponent {
				detail.Required = component.Options.Required
				detail.Multiple = component.Options.Multiple
				detail.Options = formMakingOptions(component.Options.Options)
				if value, ok := componentDefaultValue(component.Options.DefaultValue); ok {
					detail.DefaultValue = value
				}
			}
			result = append(result, detail)
		}
	}
	return result, runtimeForms, nil
}

// formMakingComponentName 按目标模板真实优先级保留自定义组件注册名，供诊断与兼容投影使用。
func formMakingComponentName(component rawFormMakingComponent) string {
	return firstNonEmpty(component.El, component.Options.ComponentName, component.Options.Component, component.ComponentName, component.Component)
}

// BaseURL 返回目标网关公开地址，供短期 iframe 会话请求必要的表单只读数据。
func (c *Client) BaseURL() string {
	if c == nil || c.baseURL == nil {
		return ""
	}
	return strings.TrimRight(c.baseURL.String(), "/")
}

// ReadInstanceCurrentData 读取已核实实例的当前表单值，供受限近期样本链复用。
func (c *Client) ReadInstanceCurrentData(ctx context.Context, active Session, instanceID string) (map[string]any, error) {
	return c.readInstanceCurrentData(ctx, active, instanceID)
}

// parseFormMakingComponents 递归解析 FormMaking 的 list、grid、report、tableColumns 与嵌套容器。
func parseFormMakingComponents(raw string) map[string]rawFormMakingComponent {
	result := make(map[string]rawFormMakingComponent)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}
	var data rawFormMakingData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return result
	}
	for _, component := range data.List {
		collectFormMakingComponent(result, component)
	}
	return result
}

// collectFormMakingComponent 深度优先收集组件及其所有真实容器子列表，避免嵌套字段因漏读而降级伪造。
func collectFormMakingComponent(result map[string]rawFormMakingComponent, component rawFormMakingComponent) {
	key := strings.TrimSpace(component.Model)
	if key != "" {
		if _, exists := result[key]; !exists {
			result[key] = component
		}
	}
	for _, child := range component.List {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.Columns {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.Rows {
		collectFormMakingComponent(result, child)
	}
	for _, child := range component.TableColumns {
		collectFormMakingComponent(result, child)
	}
}

// formMakingOptions 把组件选项收敛为标签与值；值优先取 value，缺省回退 id，标签回退值本身。
func formMakingOptions(raw []rawFormMakingOption) []FormFieldOption {
	result := make([]FormFieldOption, 0, len(raw))
	for _, option := range raw {
		value := anyString(option.Value)
		if value == "" {
			value = anyString(option.ID)
		}
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = value
		}
		if value == "" {
			continue
		}
		result = append(result, FormFieldOption{Label: label, Value: value})
	}
	return result
}

// componentDefaultValue 读取组件默认值并把标量值统一为字符串；数组等复杂默认值保持空。
func componentDefaultValue(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case float64:
		return formatNumber(typed), true
	default:
		return "", false
	}
}

// anyString 把选项 id/value 标量转换为字符串；对象或数组等复杂值保持空。
func anyString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(typed)
	default:
		return ""
	}
}

// formatNumber 把 JSON 数字格式化为无冗余尾数的十进制字符串。
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// readInstanceCurrentData 按精确实例 ID 读取当前 formDataMongoVo.data，作为已发/待发路径初始值。
func (c *Client) readInstanceCurrentData(ctx context.Context, active Session, instanceID string) (map[string]any, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil, nil
	}
	resp, err := c.call(ctx, "/web/flowInstanceApi/getCurrentFromData", active.SID, map[string]any{
		"data": map[string]any{"id": instanceID},
	})
	if err != nil {
		return nil, err
	}
	if !responseSucceeded(resp) {
		return nil, responseError(resp)
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return nil, nil
	}
	var data struct {
		Data map[string]any `json:"data"`
	}
	// 使用 UseNumber 保留目标数字字面量的小数位，条件 eq 依赖 BigDecimal 的 scale。
	if err := jsonvalues.Decode(resp.Data, &data); err != nil {
		return nil, invalidResponse("invalid instance form data")
	}
	if data.Data == nil {
		return nil, nil
	}
	return data.Data, nil
}

// convertFlowNode 递归转换目标节点；内部配置只供要求分析，公开图分析器不会序列化这些字段。
func convertFlowNode(raw *rawFlowNodeTemplate) *FlowNodeTemplate {
	if raw == nil {
		return nil
	}
	node := &FlowNodeTemplate{
		ID: raw.ID, Name: firstNonEmpty(raw.NodeName, raw.Name), Type: raw.Type,
		BranchExecuteType: raw.BranchExecuteType, Child: convertFlowNode(raw.Child),
		AuditConfig: convertFlowAuditConfig(raw.AuditConfig), FieldPowers: convertFlowFieldPowers(raw.FieldPowers),
		IsSkip: raw.IsSkip, Delay: raw.Delay, Unit: raw.Unit, DeadlineType: raw.DeadlineType,
	}
	node.ConditionNodes = convertFlowBranches(raw.ConditionNodes)
	node.ParallelNodes = convertFlowBranches(raw.ParallelNodes)
	return node
}

// convertFlowBranches 保留真实分支 ID、顺序、名称和子节点。
func convertFlowBranches(raw []rawFlowBranchTemplate) []FlowBranchTemplate {
	result := make([]FlowBranchTemplate, 0, len(raw))
	for _, branch := range raw {
		result = append(result, FlowBranchTemplate{
			ID: firstNonEmpty(branch.StrategyID, branch.ID), Name: firstNonEmpty(branch.NodeName, branch.Name),
			Sort: branch.Sort, Conditions: convertFlowConditions(branch.ConditionList), Child: convertFlowNode(branch.Child),
		})
	}
	return result
}

// convertFlowConditions 复制条件内部关联键，后续分析必须翻译后才能公开。
func convertFlowConditions(raw []rawFlowCondition) []FlowCondition {
	result := make([]FlowCondition, 0, len(raw))
	for _, condition := range raw {
		result = append(result, FlowCondition{
			FieldA: condition.FieldA, FieldB: condition.FieldB, ValueB: condition.ValueB,
			ValueType: condition.ValueType, Judge: condition.Judge, ConditionType: condition.ConditionType,
		})
	}
	return result
}

// convertFlowAuditConfig 去除审批业务 ID，仅保留分类、数量和可展示名称。
func convertFlowAuditConfig(raw *rawFlowNodeAuditConfig) *FlowNodeAuditConfig {
	if raw == nil {
		return nil
	}
	result := &FlowNodeAuditConfig{
		AuditType: raw.AuditType, Mode: raw.Mode, CountersignNum: raw.CountersignNum,
		FormPersonField: raw.FormPersonField, AuditCondition: raw.AuditCondition,
		PlatformCode: strings.TrimSpace(raw.PlatformCode),
	}
	for _, detail := range raw.Details {
		result.Details = append(result.Details, FlowAuditDetail{ID: strings.TrimSpace(detail.BizID), Name: detail.Name, Type: detail.Type})
	}
	for _, scope := range raw.Scopes {
		result.Scopes = append(result.Scopes, FlowAuditScope{ID: strings.TrimSpace(scope.BizID), Type: scope.Type})
	}
	seenCandidates := make(map[string]bool, len(raw.Candidates))
	for _, candidate := range raw.Candidates {
		id := strings.TrimSpace(candidate.ID)
		name := firstNonEmpty(candidate.Name, candidate.RealName, candidate.DisplayName)
		if id == "" || strings.TrimSpace(name) == "" || seenCandidates[id] {
			continue
		}
		seenCandidates[id] = true
		result.Candidates = append(result.Candidates, FlowAuditCandidate{ID: id, Name: strings.TrimSpace(name)})
	}
	seenDefaults := make(map[string]bool, len(raw.DefaultCandidates))
	for _, candidate := range raw.DefaultCandidates {
		id := strings.TrimSpace(candidate.ID)
		name := firstNonEmpty(candidate.Name, candidate.RealName, candidate.DisplayName)
		if id == "" || strings.TrimSpace(name) == "" || seenDefaults[id] {
			continue
		}
		seenDefaults[id] = true
		result.DefaultCandidates = append(result.DefaultCandidates, FlowAuditCandidate{ID: id, Name: strings.TrimSpace(name)})
	}
	return result
}

// convertFlowFieldPowers 复制字段权限关联键，公开层只使用解析后的字段中文名。
func convertFlowFieldPowers(raw []rawFlowNodeFieldPower) []FlowNodeFieldPower {
	result := make([]FlowNodeFieldPower, 0, len(raw))
	for _, power := range raw {
		result = append(result, FlowNodeFieldPower{
			FormID: power.FormID, FieldID: power.FieldID, EnglishName: power.EnglishName, Power: power.Power,
		})
	}
	return result
}

// FindDoneTaskOnNode 读取指定实例、指定节点上是否已有本账号的已办记录（对账「已办记录」维度）。
// 只读，可安全重试。已办任务与待办同表同端点，只是 taskStatus 取 done（TaskStatusEnum.done=已办）。
// 节点标识为空时只按实例判断"这个实例上是否已经有已办"。
func (c *Client) FindDoneTaskOnNode(ctx context.Context, active Session, instanceID, nodeProxyID string) (bool, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return false, nil
	}
	resp, err := c.call(ctx, "/web/flowJobTaskLink/list", active.SID, map[string]any{
		"data": map[string]any{
			"flowInstanceId":               instanceID,
			"taskStatus":                   "done",
			"auditWayList":                 []string{},
			"useScope":                     "invest",
			"flowInstanceBizRelevance":     map[string]any{},
			"flowInstanceBizRelevanceList": []any{},
		},
		"pagination": true, "pages": 1, "size": 100,
	})
	if err != nil {
		return false, err
	}
	if !responseSucceeded(resp) {
		return false, responseError(resp)
	}
	var raw []struct {
		FlowInstanceID  string `json:"flowInstanceId"`
		FlowNodeProxyID string `json:"flowNodeProxyId"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return false, err
	}
	wantNode := strings.TrimSpace(nodeProxyID)
	for _, item := range raw {
		if strings.TrimSpace(item.FlowInstanceID) != instanceID {
			continue
		}
		if wantNode != "" && strings.TrimSpace(item.FlowNodeProxyID) != wantNode {
			continue
		}
		return true, nil
	}
	return false, nil
}

// FindAuditTraceOnNode 读取指定实例上的审核记录，判断本节点是否已留下动作痕迹（对账「动作痕迹」维度）。
// 只读，可安全重试。端点与目标自己的流程日志同源（/web/flowAuditRecord/list，请求只带 flowInstanceId）。
// 返回：本节点是否有痕迹、该实例审核记录总条数（条数进对账依据说明，便于人工核对）。
func (c *Client) FindAuditTraceOnNode(ctx context.Context, active Session, instanceID, nodeProxyID string) (bool, int, error) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return false, 0, nil
	}
	resp, err := c.call(ctx, "/web/flowAuditRecord/list", active.SID, map[string]any{
		"data": map[string]any{"flowInstanceId": instanceID},
	})
	if err != nil {
		return false, 0, err
	}
	if !responseSucceeded(resp) {
		return false, 0, responseError(resp)
	}
	var raw []struct {
		FlowInstanceID  string `json:"flowInstanceId"`
		FlowNodeProxyID string `json:"flowNodeProxyId"`
		AuditStatus     string `json:"auditStatus"`
	}
	if err := decodeArray(resp.Data, &raw); err != nil {
		return false, 0, err
	}
	wantNode := strings.TrimSpace(nodeProxyID)
	found := false
	total := 0
	for _, item := range raw {
		if strings.TrimSpace(item.FlowInstanceID) != instanceID {
			continue
		}
		total++
		if wantNode != "" && strings.TrimSpace(item.FlowNodeProxyID) == wantNode {
			found = true
		}
	}
	return found, total, nil
}
