package target

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// 目标请求协议矩阵（F-035/T01）的唯一代码登记处。
// 每个实际使用的写端点必须在这里登记字段存在性规则：目标页面发送的字段必须发送，
// 目标省略的字段必须省略；空数组、空对象、null 与缺失是四种不同形状，
// 不能用 Go 的 omitempty 或「非空才发送」替代。
// 证据位置见 docs/TARGET_SEMANTICS.md 第 2.2/2.3 节（F-035 重写版）与 docs/features/F-035-*.md 协议基线。
// FieldPresence 是协议矩阵里一个字段的固定存在性取值。
type FieldPresence string

const (
	// PresenceRequired 目标必发（包括固定发送的空数组/空对象/空字符串）。
	PresenceRequired FieldPresence = "required"
	// PresenceOptional 仅目标页面条件成立时发送（如手动分支选择）。
	PresenceOptional FieldPresence = "optional"
	// PresenceEmpty 目标固定发送空值形状。
	PresenceEmpty FieldPresence = "empty"
	// PresenceForbidden 目标明确不发送。
	PresenceForbidden FieldPresence = "forbidden"
)

// endpointFieldMatrix 是「端点 -> 字段 -> 存在性」的登记表；
// 供契约测试逐端点核对，构造器实现必须与本表一致。
// 端点键与 write.go / write_actions.go 的 WriteEndpoint* 常量一一对应。
var endpointFieldMatrix = map[string]map[string]FieldPresence{
	// FormMaking 新建提交/保存草稿：参考 rsh-flow-components FlowDialog.enterpriseHandleSubmit，
	// 人工成功 curl（运行 7 同计划）逐字段确认。
	WriteEndpointSubmit: {
		"data.name":                         PresenceRequired,
		"data.formProxyId":                  PresenceOptional, // FormMaking 必发；无表单时目标页面不发送
		"data.flowProxyId":                  PresenceOptional, // 无表单必发；FormMaking 不发送（与 formProxyId 互斥）
		"data.status":                       PresenceOptional, // 仅保存草稿固定 draft
		"data.companyId":                    PresenceRequired,
		"data.customerCode":                 PresenceRequired, // 目标 axios 拦截器对全部 POST 统一注入 data
		"data.flowInstanceBizRelevanceList": PresenceRequired, // 至少携带公司关联的数组
		"formDataMongoVo.data":              PresenceRequired, // 完整表单对象
		"nextAuditorList":                   PresenceRequired, // 顶层；无显式选人固定 []
		"batchCode":                         PresenceRequired, // 顶层；FlowDialog 打开时生成的 32 位批次号
		"sid":                               PresenceRequired, // 顶层；与会话同值
		"projectId":                         PresenceRequired, // 顶层；目标注入 store 的项目 ID，本项目固定空字符串
	},
	// 重新提交：参考 examineOpinion.submitFinal（无表单）与 handleSaveDraft/草稿重提页面。
	WriteEndpointReSubmit: {
		"data.id":                           PresenceRequired,
		"data.formProxyId":                  PresenceOptional, // FormMaking 必发（实例实时代理）
		"data.flowProxyId":                  PresenceOptional, // 无表单必发
		"data.companyId":                    PresenceRequired,
		"data.flowInstanceBizRelevanceList": PresenceRequired,
		"formDataMongoVo.data":              PresenceRequired,
		"nextAuditorList":                   PresenceRequired,  // 顶层；重提页面无条件 map 出数组
		"batchCode":                         PresenceForbidden, // 重提页面不生成批次号
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 审批（同意/不同意）：参考 examineOpinion.handleSubmitCheck。
	WriteEndpointAudit: {
		"data.id":                           PresenceRequired,
		"data.jobTaskId":                    PresenceRequired,
		"data.flowProxyId":                  PresenceOptional, // FormMaking 审批页同源携带；无表单页不带
		"data.name":                         PresenceOptional, // FormMaking 审批按 getBackFlowName 携带，空时 delete
		"data.flowInstanceBizRelevanceList": PresenceOptional, // 无表单页直传；FormMaking 仅公共流程/案件时设置
		"data.auditRecord":                  PresenceRequired,
		"formDataMongoVo.data":              PresenceRequired,
		"nextAuditorList":                   PresenceOptional, // 仅 pass 显式人员时发送；no_pass 不发送
		"tracking":                          PresenceRequired, // 顶层布尔；页面无条件携带
		"batchCode":                         PresenceForbidden,
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 当前节点暂存：参考 EnterpriseExamineOpinion.temporaryStorage。
	WriteEndpointStorageForm: {
		"data.id":                 PresenceRequired,
		"data.currentNodeProxyId": PresenceRequired,
		"data.auditRecord":        PresenceRequired, // executeDesc 必在（页面 approveMessage）
		"formDataMongoVo.data":    PresenceRequired, // editData+getValues 整份覆盖
		"batchCode":               PresenceForbidden,
		"sid":                     PresenceRequired,
		"projectId":               PresenceRequired,
	},
	// 移交：参考 Backlog/index.vue handleHandOver（Api.schedule.handOver = approverAppend）。
	WriteEndpointApproverAppend: {
		"data.id":                          PresenceRequired,
		"data.jobTaskId":                   PresenceRequired,
		"data.batchNo":                     PresenceRequired, // 来自当前待办快照行，绝不从路径配置复制
		"data.auditRecord":                 PresenceRequired, // auditStatus=transfer + executeDesc
		"approverAppendVo.flowNodeProxyId": PresenceRequired,
		"approverAppendVo.userIds":         PresenceRequired, // 真实用户 ID 列表
		"batchCode":                        PresenceForbidden,
		"sid":                              PresenceRequired,
		"projectId":                        PresenceRequired,
	},
	// 加签：Api.schedule.updateFlowProxy（FlowOperateServiceImpl 校验完整代理树）。
	WriteEndpointUpdateFlowProxy: {
		"data.id":                PresenceRequired,
		"flowProxyProtocol.data": PresenceRequired, // 目标读取的完整 FlowProxyVo 树
		"batchCode":              PresenceForbidden,
		"sid":                    PresenceRequired,
		"projectId":              PresenceRequired,
	},
	// 回退上一审批节点：参考 GroupApproveManage/components/mixin.js clickRollBack 与 Backlog clickRollBack。
	WriteEndpointRollBack: {
		"data.id":           PresenceRequired,
		"data.jobTaskId":    PresenceRequired,
		"data.withdrawDesc": PresenceOptional, // mixin 版携带审批意见，Backlog 版不带
		"batchCode":         PresenceForbidden,
		"sid":               PresenceRequired,
		"projectId":         PresenceRequired,
	},
	// 取回：参考 Finished/index.vue retrieveProcess。
	WriteEndpointRetrieve: {
		"data.jobTaskId": PresenceRequired,
		"data.id":        PresenceRequired,
		"batchCode":      PresenceForbidden,
		"sid":            PresenceRequired,
		"projectId":      PresenceRequired,
	},
	// 撤回：参考 Submitted/index.vue withDrawFlow。
	WriteEndpointRevocation: {
		"data.id":           PresenceRequired,
		"data.withdrawDesc": PresenceRequired, // 页面发送空字符串也保留字段
		"batchCode":         PresenceForbidden,
		"sid":               PresenceRequired,
		"projectId":         PresenceRequired,
	},
	// 转发：参考 flowTypeMixin.js transpond（顶层 receiverId + data 容器 + formDataMongoVo）。
	WriteEndpointTranspond: {
		"data.name":                         PresenceRequired,
		"data.flowInstanceBizRelevanceList": PresenceOptional, // 二次转发复用旧关联；首次转发全量构造
		"receiverId":                        PresenceRequired, // 顶层，不在 data 内
		"formDataMongoVo.data":              PresenceRequired,
		"batchCode":                         PresenceForbidden,
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 关注/取消关注：参考 GroupApproveManage/components/mixin.js setTracking。
	WriteEndpointFlowTracking: {
		"data.id":   PresenceRequired,
		"tracking":  PresenceRequired, // 顶层布尔
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	// 催办：参考 Submitted/index.vue urgeFlow（顶层 flowInstanceId，不走 data 容器）。
	WriteEndpointUrge: {
		"flowInstanceId": PresenceRequired, // 顶层
		"data":           PresenceRequired, // 固定空对象
		"batchCode":      PresenceForbidden,
		"sid":            PresenceRequired,
		"projectId":      PresenceRequired,
	},
}

// NewBatchCode 生成目标 FlowDialog 同形状的 32 位十六进制批次号。
// 它只在 submit/draft（FlowDialog enterpriseHandleSubmit）顶层携带，生命周期与目标页面
// 「打开弹窗生成、随提交发送」一致；绝不是工具幂等键：写请求仍只发送一次，
// 响应丢失先按目标事实对账，绝不以 batchCode 为重试依据。
func NewBatchCode() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		// 批次号生成失败不得阻断写请求：退化为时间戳派生的 32 位十六进制串，
		// 长度仍与目标页面一致，只用于满足目标页面的字段形状。
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// ProtocolMatrix 返回端点字段存在性登记表的只读副本，供契约测试与文档核对。
func ProtocolMatrix() map[string]map[string]FieldPresence {
	result := make(map[string]map[string]FieldPresence, len(endpointFieldMatrix))
	for endpoint, fields := range endpointFieldMatrix {
		copied := make(map[string]FieldPresence, len(fields))
		for field, presence := range fields {
			copied[field] = presence
		}
		result[endpoint] = copied
	}
	return result
}

// 目标业务生命周期登记（F-035 评审补充，唯一代码登记处）。
// 目标发起/审批页按流程类型（selectFlowType/auditWay）分派特殊业务前置与后置：
//   - submit 分派：FlowDialog.formMakingFormBusiness —— contract_compliance_review 与
//     contract_seal_review 调用自定义组件保存业务（FlowDialog.vue:367-374），
//     cost_funds_transactions / cost_funds_invest 先保存请款业务数据（FlowDialog.vue:375、:380-425），
//     其余流程类型直接走通用 enterpriseHandleSubmit（FlowDialog.vue:377-378）；
//   - submit 前后钩子顺序：checkFlowPermission → 业务保存/触发 beforeSubmitAndDraft →
//     发起 → afterSaveFlowInstance / 文件状态与业务实例绑定（FlowDialog.vue:385-905）；
//   - 审批同意的业务字段改写：EnterpriseExamineOpinion.handleSubmitCheck ——
//     publication_commission / profession_indirect_provide 审批时更新日期字段（:939-975），
//     staff_annual_performance / staff_annual_assessment 把审批意见写入表单（:946-949），
//     差旅/请款/借款等类型有专用金额一致性计算（:952-990）。
//
// 工具没有这些自定义组件与业务接口的执行能力：命中下列清单的流程类型必须在发送前阻塞，
// 绝不能发通用请求顶替目标业务变更。新类型必须在目标页面核实后先登记再放行。
var specialBusinessFlowTypes = map[string]bool{
	// submit 阶段的自定义业务组件/业务数据保存（FlowDialog.formMakingFormBusiness）。
	"contract_compliance_review": true,
	"contract_seal_review":       true,
	"cost_funds_transactions":    true,
	"cost_funds_invest":          true,
	// 审批同意阶段会改写业务字段的类型（EnterpriseExamineOpinion.handleSubmitCheck）。
	"publication_commission":      true,
	"profession_indirect_provide": true,
	"staff_annual_performance":    true,
	"staff_annual_assessment":     true,
	"expense_budget":              true,
}

// HasSpecialBusinessLifecycle 判断流程类型在目标页面存在工具无法执行的特殊业务钩子。
func HasSpecialBusinessLifecycle(flowType string) bool {
	return specialBusinessFlowTypes[strings.TrimSpace(flowType)]
}

// SpecialBusinessFlowTypes 返回登记过的特殊业务流程类型（排序副本），供测试与文档核对。
func SpecialBusinessFlowTypes() []string {
	result := make([]string, 0, len(specialBusinessFlowTypes))
	for flowType := range specialBusinessFlowTypes {
		result = append(result, flowType)
	}
	sort.Strings(result)
	return result
}

// FlowLifecycleMeta 是流程生命周期元数据快照（F-035 评审补充）：
// FlowType 是目标页面分派业务钩子的键（auditWay/selectFlowType）；
// RenderType 是表单渲染类型；FormPersonFields 是目标流程树逐节点声明的
// form_person 表单人员选择器字段（递归收集，逐节点携带目标节点 ID）。
type FlowLifecycleMeta struct {
	FlowType         string
	RenderType       string
	FormPersonFields []NodeFormPersonField
}

// NodeFormPersonField 是一个目标节点声明的表单人员选择器字段（FlowDialog.traverseFlowNode 规则）：
// 目标在提交时递归流程树，对 auditType=form_person 的节点把声明字段从
// {"id":..,"name":..} JSON 或同前缀名称字段解析成真实用户 ID；字段缺失时按源字段补齐。
// 工具必须按同一规则在发起/重提/审批前生成该字段及其伴生字段，不能用固定字段名表猜测。
type NodeFormPersonField struct {
	// NodeID 是声明该选择器的目标节点 ID。
	NodeID string
	// Field 是表单里的选择器字段名（formPersonFields 逗号分隔项，如 myUserName__formPersonId）。
	Field string
}

// CollectFormPersonFields 递归流程树，收集 auditType=form_person 节点声明的全部表单人员字段。
// 一个节点可声明多个字段（逗号分隔）；同一字段被多个节点声明时按节点去重合并。
// 树为空时返回空切片（无表单人员节点的流程没有该协议）。
func CollectFormPersonFields(tree *FlowNodeTemplate) []NodeFormPersonField {
	result := []NodeFormPersonField{}
	seen := map[string]bool{}
	var visit func(node *FlowNodeTemplate)
	visit = func(node *FlowNodeTemplate) {
		if node == nil {
			return
		}
		if config := node.AuditConfig; config != nil && strings.TrimSpace(config.AuditType) == "form_person" {
			nodeID := strings.TrimSpace(node.ID)
			for _, raw := range strings.Split(config.FormPersonField, ",") {
				field := strings.TrimSpace(raw)
				if field == "" || nodeID == "" {
					continue
				}
				key := nodeID + "\x00" + field
				if seen[key] {
					continue
				}
				seen[key] = true
				result = append(result, NodeFormPersonField{NodeID: nodeID, Field: field})
			}
		}
		visit(node.Child)
		for index := range node.ConditionNodes {
			visit(node.ConditionNodes[index].Child)
		}
		for index := range node.ParallelNodes {
			visit(node.ParallelNodes[index].Child)
		}
	}
	visit(tree)
	return result
}

// ApplyFormPersonFieldRule 按目标 traverseFlowNode 规则把声明字段补进表单值（F-035 评审补充）：
// 声明字段（如 xxx__formPersonId）为空/缺失时，从去掉 __formPersonId 后缀的源字段解析：
// 源是 JSON 文本取 id，否则取原值。源也不存在时不产出（目标同样跳过），不伪造人员。
// 返回是否写入了新值，供调用方记录协议摘要。
func ApplyFormPersonFieldRule(values map[string]any, field NodeFormPersonField) bool {
	if values == nil {
		return false
	}
	if existing, exists := values[field.Field]; exists && existing != nil && existing != "" {
		return false
	}
	source := strings.TrimSuffix(field.Field, "__formPersonId")
	if source == field.Field {
		return false
	}
	raw, exists := values[source]
	if !exists || raw == nil {
		return false
	}
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return false
		}
		// JSON 文本（人员选择器写入的 {"id":..,"name":..}）取 id；其余取原值。
		var decoded struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil && strings.TrimSpace(decoded.ID) != "" {
			values[field.Field] = strings.TrimSpace(decoded.ID)
			return true
		}
		values[field.Field] = trimmed
		return true
	default:
		values[field.Field] = raw
		return true
	}
}

// UserIdentity 是一次目标会话的当前账号身份事实（F-035 评审补充）：
// 全部来自当前会话的实时目标读取（目录树 + 人员目录），不是配置期快照。
type UserIdentity struct {
	UserID         string
	UserName       string
	CompanyID      string
	CompanyName    string
	DepartmentID   string
	DepartmentName string
	DutyID         string
	DutyName       string
}

// Complete 判断身份事实是否足以构造目标登录人上下文；岗位缺失时调用方必须阻塞。
func (i UserIdentity) Complete() bool {
	return strings.TrimSpace(i.UserID) != "" && strings.TrimSpace(i.CompanyID) != "" &&
		strings.TrimSpace(i.DutyID) != "" && strings.TrimSpace(i.DutyName) != ""
}

// targetLoginIdentityFieldRules 是目标平台登录人字段约定的唯一登记处（从 service 层迁移）：
// 键为目标表单字段模型，值为身份属性或 "json:user"/"json:department"/"json:company"（人员选择器 JSON 文本）。
// 目标 FlowDialog 提交时用登录态覆盖这些字段；新约定出现时在此补充一条即全局生效。
var targetLoginIdentityFieldRules = map[string]string{
	"handledBy":             "userId",
	"handlingCompany":       "companyId",
	"handlingDepartment":    "departmentId",
	"initiatorDepartmentId": "departmentId",
	"currentDepartment":     "departmentId",
	"currentCompanyId":      "companyId",
	"currentDepName":        "departmentName",
	"expenseUserId":         "userId",
	"expenseUserName":       "userName",
	"expenseCompanyId":      "companyId",
	"expenseCompanyName":    "companyName",
	"myUserName":            "json:user",
	"myDepName":             "json:department",
	"myCompanyName":         "json:company",
}

// ApplyUserIdentity 用当前会话身份覆盖目标登录人上下文字段（F-035 评审补充）：
// global_user_basic_information 整体覆盖（含岗位），登记字段逐项替换并同步 __formPersonId/__condition 伴生键。
// 表单没有该字段时跳过（目标同样只在有字段时覆盖）；身份属性为空不伪造。
func ApplyUserIdentity(values map[string]any, identity UserIdentity) {
	if values == nil {
		return
	}
	const globalField = "global_user_basic_information"
	// 与目标页面一致：表单没有登录人上下文字段时不做任何身份写入（历史行为保持，配置期测试也锁定该语义）。
	if _, exists := values[globalField]; !exists {
		return
	}
	values[globalField] = map[string]any{
		"userId": identity.UserID, "userName": identity.UserName,
		"companyId": identity.CompanyID, "companyName": identity.CompanyName,
		"departmentId": identity.DepartmentID, "departmentName": identity.DepartmentName,
		"dutyId": identity.DutyID, "dutyName": identity.DutyName,
	}
	for field, rule := range targetLoginIdentityFieldRules {
		if _, exists := values[field]; !exists {
			continue
		}
		if strings.HasPrefix(rule, "json:") {
			encoded, ok := identityJSONText(identity, strings.TrimPrefix(rule, "json:"))
			if !ok {
				continue
			}
			values[field] = encoded
			idAttr, nameAttr := identityPickerCompanions(rule)
			if _, exists := values[field+"__formPersonId"]; exists {
				if value, ok := identityAttr(identity, idAttr); ok {
					values[field+"__formPersonId"] = value
				}
			}
			if _, exists := values[field+"__condition"]; exists {
				if value, ok := identityAttr(identity, nameAttr); ok {
					values[field+"__condition"] = value
				}
			}
			continue
		}
		if value, ok := identityAttr(identity, rule); ok {
			values[field] = value
		}
	}
}

// identityJSONText 按人员选择器约定产出 {"id":..,"name":..} JSON 文本。
func identityJSONText(identity UserIdentity, kind string) (string, bool) {
	var id, name string
	switch kind {
	case "user":
		id, name = identity.UserID, identity.UserName
	case "department":
		id, name = identity.DepartmentID, identity.DepartmentName
	case "company":
		id, name = identity.CompanyID, identity.CompanyName
	}
	if id == "" || name == "" {
		return "", false
	}
	encoded, err := json.Marshal(map[string]string{"id": id, "name": name})
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// identityPickerCompanions 返回人员选择器伴生键对应的身份属性。
func identityPickerCompanions(rule string) (idAttr, nameAttr string) {
	switch rule {
	case "json:user":
		return "userId", "userName"
	case "json:department":
		return "departmentId", "departmentName"
	case "json:company":
		return "companyId", "companyName"
	}
	return "", ""
}

// identityAttr 按属性名取身份值；空值不产出。
func identityAttr(identity UserIdentity, attr string) (string, bool) {
	switch attr {
	case "userId":
		return identity.UserID, true
	case "userName":
		return identity.UserName, true
	case "companyId":
		return identity.CompanyID, true
	case "companyName":
		return identity.CompanyName, true
	case "departmentId":
		return identity.DepartmentID, true
	case "departmentName":
		return identity.DepartmentName, true
	case "dutyId":
		return identity.DutyID, true
	case "dutyName":
		return identity.DutyName, true
	}
	return "", false
}

// CurrentUserIdentity 用当前会话实时读取账号身份（目录树 + 岗位），
// 供执行器在每次写请求前覆盖身份字段；任何一项读取失败都返回错误，调用方必须阻塞。
func (c *Client) CurrentUserIdentity(ctx context.Context, active Session) (UserIdentity, error) {
	identityCtx, err := c.FormIdentityContext(ctx, active)
	if err != nil {
		return UserIdentity{}, err
	}
	dutyID, dutyName, err := c.CurrentUserDuty(ctx, active)
	if err != nil {
		return UserIdentity{}, err
	}
	companyName := firstNonEmpty(identityCtx.Company.Name, active.Summary.CompanyName)
	departmentID := firstNonEmpty(identityCtx.Department.ID, active.DepartmentID)
	departmentName := firstNonEmpty(identityCtx.Department.Name, "")
	return UserIdentity{
		UserID: active.UserID, UserName: active.Summary.DisplayName,
		CompanyID: active.CompanyID, CompanyName: companyName,
		DepartmentID: departmentID, DepartmentName: departmentName,
		DutyID: dutyID, DutyName: dutyName,
	}, nil
}

// ValidateBodyMatrix 在写请求发出前按矩阵强制校验载荷（F-035 评审补充）：
// required 字段必须存在（含固定发送的空数组/空对象），forbidden 字段不得出现。
// sid/projectId/customerCode 由信封注入，不在载荷构造器职责内，这里跳过；
// optional 字段由调用方按页面条件决定，不做硬性断言。返回中文结论供阻塞使用。
func ValidateBodyMatrix(endpoint string, body map[string]any, customerCode string) error {
	fields, registered := endpointFieldMatrix[endpoint]
	if !registered {
		// 未登记矩阵的端点禁止进入实现（F-035 硬性规则）。
		return fmt.Errorf("端点 %s 未登记协议矩阵，禁止发送写请求", endpoint)
	}
	flat := map[string]any{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		if typed, ok := value.(map[string]any); ok {
			for key, child := range typed {
				pathPath := key
				if prefix != "" {
					pathPath = prefix + "." + key
				}
				flat[pathPath] = child
				walk(pathPath, child)
			}
		}
	}
	walk("", body)
	// 信封层注入的字段在发送前由统一出口补齐，这里按已注入口径核对。
	if sid, ok := body["__envelope_sid__"]; ok {
		flat["sid"] = sid
	}
	flat["sid"] = true
	flat["projectId"] = true
	flat["data.customerCode"] = true
	for field, presence := range fields {
		switch presence {
		case PresenceRequired:
			if _, exists := flat[field]; !exists {
				return fmt.Errorf("写请求 %s 缺少协议矩阵 required 字段 %s，不能发送", endpoint, field)
			}
		case PresenceForbidden:
			if _, exists := flat[field]; exists {
				return fmt.Errorf("写请求 %s 携带了协议矩阵 forbidden 字段 %s，不能发送", endpoint, field)
			}
		}
	}
	return nil
}
