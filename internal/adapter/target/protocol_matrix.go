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
	// 资金往来/投资款业务保存：FlowDialog.saveCostFundsBusiness，主流程前置。
	"/web/measuring/api/costFundsTransactions/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	// 年度考核业务保存：AnnualAssessmentNoForm.saveBusinessData。
	"/web/plan/api/annualPerformance/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	// 费用报销业务保存：ExpensesClaimForm.postData。
	"/web/expenseReimbursement/submit": {
		"data":      PresenceRequired,
		"batchCode": PresenceOptional, // 页面明细可能携带，不是主流程批次号
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	// 无表单计量页业务保存：mixin.saveData / apiList.save。
	"/web/measuring/api/procurePlan/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuringApi/materialDeviceDemandTable/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuringApi/purchaseOrder/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuring/api/contractReview/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuring/api/contractPayment/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuringApi/orderReceipt/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/measuring/api/costBudget/saveTask": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	"/web/plan/api/kpiGroup/save": {
		"data":      PresenceRequired,
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
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

// 目标业务生命周期登记已迁到 special_business.go（F-035 第四轮）：
// 已知分支必须按目标页面顺序实现，未知分支继续写前阻塞，禁止“命中即阻塞”当作完成。

// FlowLifecycleMeta 是流程生命周期元数据快照（F-035 评审补充）：
// FlowType 是目标页面分派业务钩子的键（auditWay/selectFlowType）；
// RenderType 是表单渲染类型；FormPersonFields 是目标流程树逐节点声明的
// form_person 表单人员选择器字段（递归收集，逐节点携带目标节点 ID）。
// Tree 保留完整目标树，供审批按 nextNodeProxyId 收窄入口，不得在运行时再猜。
type FlowLifecycleMeta struct {
	FlowType         string
	RenderType       string
	FormPersonFields []NodeFormPersonField
	Tree             *FlowNodeTemplate
}

const (
	// FormPersonScopeInitiationFullTree 新发起页 staff_annual_performance 的全树 traverseFlowNode。
	FormPersonScopeInitiationFullTree = "initiation_full_tree"
	// FormPersonScopeInitiationNextEntry 新发起/重提页 OtherSteps2.setFormPersonFields：只收下一节点及其直接条件/并行入口。
	FormPersonScopeInitiationNextEntry = "initiation_next_entry"
	// FormPersonScopeApprovalNextEntry 审批页 getFlowDetailFindFormPerson：只收 nextNodeProxyId 及其直接条件/并行入口。
	FormPersonScopeApprovalNextEntry = "approval_next_entry"
	// FormPersonScopeResubmitPageRule 重提页沿用发起页入口规则，不是审批整树。
	FormPersonScopeResubmitPageRule = "resubmit_page_rule"
)

// NodeFormPersonField 是一个目标节点声明的表单人员选择器字段。
// 目标发起与审批使用不同遍历范围和取值规则，Scope 必须随字段一起传递，禁止全局复用整树清单。
type NodeFormPersonField struct {
	// NodeID 是声明该选择器的目标节点 ID。
	NodeID string
	// Field 是表单里的选择器字段名（formPersonFields 逗号分隔项，如 myUserName__formPersonId）。
	Field string
	// Scope 区分发起全树、发起下一入口、审批下一入口和重提页规则。
	Scope string
}

// CollectFormPersonFields 递归流程树，收集 auditType=form_person 节点声明的全部表单人员字段。
// 默认 Scope 为 initiation_full_tree，供配置期与年度绩效全树特例使用；运行时必须再按动作收窄。
func CollectFormPersonFields(tree *FlowNodeTemplate) []NodeFormPersonField {
	return collectFormPersonFields(tree, FormPersonScopeInitiationFullTree, "")
}

// CollectFormPersonFieldsForAction 按目标页面对应动作选择遍历范围：
// 发起/草稿默认只收下一节点入口；年度绩效发起才走全树；审批只收 nextNodeProxyId 入口；重提沿用发起入口规则。
func CollectFormPersonFieldsForAction(tree *FlowNodeTemplate, action, flowType, nextNodeProxyID string) []NodeFormPersonField {
	switch strings.TrimSpace(action) {
	case "approve", "reject", "storage_form_data":
		return collectFormPersonFields(tree, FormPersonScopeApprovalNextEntry, nextNodeProxyID)
	case "resubmit":
		if strings.TrimSpace(flowType) == "staff_annual_performance" {
			return collectFormPersonFields(tree, FormPersonScopeInitiationFullTree, "")
		}
		return collectFormPersonFields(tree, FormPersonScopeResubmitPageRule, nextNodeProxyID)
	case "submit", "save_draft":
		if strings.TrimSpace(flowType) == "staff_annual_performance" {
			return collectFormPersonFields(tree, FormPersonScopeInitiationFullTree, "")
		}
		return collectFormPersonFields(tree, FormPersonScopeInitiationNextEntry, nextNodeProxyID)
	default:
		return nil
	}
}

// collectFormPersonFields 按作用域收集 form_person 字段。
func collectFormPersonFields(tree *FlowNodeTemplate, scope, nextNodeProxyID string) []NodeFormPersonField {
	result := []NodeFormPersonField{}
	seen := map[string]bool{}
	appendNode := func(node *FlowNodeTemplate) {
		if node == nil || node.AuditConfig == nil || strings.TrimSpace(node.AuditConfig.AuditType) != "form_person" {
			return
		}
		nodeID := strings.TrimSpace(node.ID)
		for _, raw := range strings.Split(node.AuditConfig.FormPersonField, ",") {
			field := strings.TrimSpace(raw)
			if field == "" || nodeID == "" {
				continue
			}
			key := nodeID + "\x00" + field + "\x00" + scope
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, NodeFormPersonField{NodeID: nodeID, Field: field, Scope: scope})
		}
	}
	appendDirectEntries := func(node *FlowNodeTemplate) {
		if node == nil {
			return
		}
		appendNode(node)
		for index := range node.ConditionNodes {
			appendNode(node.ConditionNodes[index].Child)
		}
		for index := range node.ParallelNodes {
			appendNode(node.ParallelNodes[index].Child)
		}
	}
	switch scope {
	case FormPersonScopeApprovalNextEntry, FormPersonScopeInitiationNextEntry, FormPersonScopeResubmitPageRule:
		nextID := strings.TrimSpace(nextNodeProxyID)
		if nextID == "" {
			if scope == FormPersonScopeInitiationNextEntry || scope == FormPersonScopeResubmitPageRule {
				if tree != nil {
					appendDirectEntries(tree.Child)
				}
			}
			return result
		}
		var visit func(node *FlowNodeTemplate)
		visit = func(node *FlowNodeTemplate) {
			if node == nil {
				return
			}
			if strings.TrimSpace(node.ID) == nextID {
				appendDirectEntries(node)
				return
			}
			for index := range node.ConditionNodes {
				branch := node.ConditionNodes[index]
				if strings.TrimSpace(branch.ID) == nextID {
					appendNode(branch.Child)
					return
				}
				visit(branch.Child)
			}
			for index := range node.ParallelNodes {
				branch := node.ParallelNodes[index]
				if strings.TrimSpace(branch.ID) == nextID {
					appendNode(branch.Child)
					return
				}
				visit(branch.Child)
			}
			visit(node.Child)
		}
		visit(tree)
		return result
	default:
		var visit func(node *FlowNodeTemplate)
		visit = func(node *FlowNodeTemplate) {
			if node == nil {
				return
			}
			appendNode(node)
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
}

// ApplyFormPersonFieldRule 按目标页面规则把声明字段补进表单值。
// 发起全树（FlowDialog.traverseFlowNode）用 fieldKey.split('__')[0]；审批/发起入口（replace __formPersonId）用精确后缀。
// JSON 取 id，纯文本取原值；已有值不覆盖，源缺失不产出、不伪造。
func ApplyFormPersonFieldRule(values map[string]any, field NodeFormPersonField) bool {
	if values == nil {
		return false
	}
	if existing, exists := values[field.Field]; exists && existing != nil && existing != "" {
		return false
	}
	source := formPersonSourceKey(field)
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

// formPersonSourceKey 按动作选择目标页面的源字段解析规则，不得自行统一。
func formPersonSourceKey(field NodeFormPersonField) string {
	if field.Scope == FormPersonScopeInitiationFullTree {
		parts := strings.Split(field.Field, "__")
		if len(parts) == 0 {
			return field.Field
		}
		return parts[0]
	}
	return strings.TrimSuffix(field.Field, "__formPersonId")
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

// Complete 判断身份事实是否足以构造目标登录人上下文。
// 目标发起页无条件写入 userId/userName/companyId/companyName/departmentId/departmentName/dutyId/dutyName，缺任一字段都必须阻塞。
func (i UserIdentity) Complete() bool {
	return strings.TrimSpace(i.UserID) != "" && strings.TrimSpace(i.UserName) != "" &&
		strings.TrimSpace(i.CompanyID) != "" && strings.TrimSpace(i.CompanyName) != "" &&
		strings.TrimSpace(i.DepartmentID) != "" && strings.TrimSpace(i.DepartmentName) != "" &&
		strings.TrimSpace(i.DutyID) != "" && strings.TrimSpace(i.DutyName) != ""
}

// MissingFields 返回本次动作实际缺少的身份字段名，供写前阻塞文案使用。
func (i UserIdentity) MissingFields() []string {
	missing := []string{}
	check := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	check("userId", i.UserID)
	check("userName", i.UserName)
	check("companyId", i.CompanyID)
	check("companyName", i.CompanyName)
	check("departmentId", i.DepartmentID)
	check("departmentName", i.DepartmentName)
	check("dutyId", i.DutyID)
	check("dutyName", i.DutyName)
	return missing
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

// ApplyUserIdentity 用当前会话身份覆盖目标登录人上下文字段。
// 目标 FlowDialog 无条件设置 global_user_basic_information；表单没有该键时，发起/草稿/重提必须创建并注入。
// createGlobal 为假时（审批页不写该字段）不创建新键，但仍覆盖已经存在的登记字段。
func ApplyUserIdentity(values map[string]any, identity UserIdentity) {
	ApplyUserIdentityForAction(values, identity, true)
}

// ApplyUserIdentityForAction 按动作决定是否创建 global_user_basic_information。
func ApplyUserIdentityForAction(values map[string]any, identity UserIdentity, createGlobal bool) {
	if values == nil {
		return
	}
	const globalField = "global_user_basic_information"
	_, exists := values[globalField]
	if exists || createGlobal {
		values[globalField] = map[string]any{
			"userId": identity.UserID, "userName": identity.UserName,
			"companyId": identity.CompanyID, "companyName": identity.CompanyName,
			"departmentId": identity.DepartmentID, "departmentName": identity.DepartmentName,
			"dutyId": identity.DutyID, "dutyName": identity.DutyName,
		}
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
			} else if createGlobal {
				if value, ok := identityAttr(identity, idAttr); ok {
					values[field+"__formPersonId"] = value
				}
			}
			if _, exists := values[field+"__condition"]; exists {
				if value, ok := identityAttr(identity, nameAttr); ok {
					values[field+"__condition"] = value
				}
			} else if createGlobal {
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

// identityAttr 按属性名取身份值；空值不产出，避免把空字符串写成登录人 ID。
func identityAttr(identity UserIdentity, attr string) (string, bool) {
	var value string
	switch attr {
	case "userId":
		value = identity.UserID
	case "userName":
		value = identity.UserName
	case "companyId":
		value = identity.CompanyID
	case "companyName":
		value = identity.CompanyName
	case "departmentId":
		value = identity.DepartmentID
	case "departmentName":
		value = identity.DepartmentName
	case "dutyId":
		value = identity.DutyID
	case "dutyName":
		value = identity.DutyName
	default:
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
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

// ValidateBodyMatrix 在写请求发出前按矩阵强制校验最终 body 的真实路径和值。
// required 必须真实存在；empty 必须存在且为空串/空数组/空对象；forbidden 不得存在；optional 不强制。
// 不再把 sid/projectId/customerCode 伪造为 true；信封字段只在最终 body 里真实出现时才算存在。
func ValidateBodyMatrix(endpoint string, body map[string]any, customerCode string) error {
	_ = customerCode
	fields, registered := endpointFieldMatrix[endpoint]
	if !registered {
		return fmt.Errorf("端点 %s 未登记协议矩阵，禁止发送写请求", endpoint)
	}
	if body == nil {
		return fmt.Errorf("写请求 %s 缺少请求体，不能发送", endpoint)
	}
	var missingRequired []string
	var missingEmpty []string
	var forbidden []string
	var nonemptyEmpty []string
	for field, presence := range fields {
		exists, empty := lookupBodyPath(body, field)
		switch presence {
		case PresenceRequired:
			if !exists {
				missingRequired = append(missingRequired, field)
			}
		case PresenceEmpty:
			if !exists {
				missingEmpty = append(missingEmpty, field)
			} else if !empty {
				nonemptyEmpty = append(nonemptyEmpty, field)
			}
		case PresenceForbidden:
			if exists {
				forbidden = append(forbidden, field)
			}
		}
	}
	sort.Strings(missingRequired)
	sort.Strings(missingEmpty)
	sort.Strings(nonemptyEmpty)
	sort.Strings(forbidden)
	if len(missingRequired) > 0 {
		return fmt.Errorf("写请求 %s 缺少协议矩阵 required 字段 %s，不能发送", endpoint, strings.Join(missingRequired, "、"))
	}
	if len(missingEmpty) > 0 {
		return fmt.Errorf("写请求 %s 缺少协议矩阵 empty 字段 %s，不能发送", endpoint, strings.Join(missingEmpty, "、"))
	}
	if len(nonemptyEmpty) > 0 {
		return fmt.Errorf("写请求 %s 的协议矩阵 empty 字段 %s 必须为空串/空数组/空对象，不能发送", endpoint, strings.Join(nonemptyEmpty, "、"))
	}
	if len(forbidden) > 0 {
		return fmt.Errorf("写请求 %s 携带了协议矩阵 forbidden 字段 %s，不能发送", endpoint, strings.Join(forbidden, "、"))
	}
	return nil
}

// lookupBodyPath 读取最终 body 的点分路径；第二返回值表示存在且形状为空。
func lookupBodyPath(body map[string]any, path string) (exists bool, empty bool) {
	if body == nil || strings.TrimSpace(path) == "" {
		return false, false
	}
	var current any = body
	for _, part := range strings.Split(path, ".") {
		typed, ok := current.(map[string]any)
		if !ok {
			return false, false
		}
		next, found := typed[part]
		if !found {
			return false, false
		}
		current = next
	}
	return true, isMatrixEmptyValue(current)
}

// isMatrixEmptyValue 只把空串、空数组、空对象视为 empty 形状；nil 不算目标页面固定发送的空值。
func isMatrixEmptyValue(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case []NextAuditor:
		return len(typed) == 0
	case []BizRelevance:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// InjectWriteEnvelope 按目标 axios 拦截器语义生成最终写请求 body：
// 顶层 sid、空字符串 projectId，以及 data.customerCode（已有则保留）。
// 矩阵校验必须读取这份最终 body，不能在构造器阶段伪造字段存在。
func InjectWriteEnvelope(body map[string]any, sid, customerCode string) map[string]any {
	payload := map[string]any{}
	for key, value := range body {
		payload[key] = value
	}
	if strings.TrimSpace(sid) == "" {
		return payload
	}
	payload["sid"] = sid
	if _, exists := payload["projectId"]; !exists {
		payload["projectId"] = ""
	}
	if dataMap, ok := payload["data"].(map[string]any); ok {
		if _, exists := dataMap["customerCode"]; !exists {
			dataMap["customerCode"] = customerCode
		}
	}
	return payload
}

// ValidateFinalWriteBody 先注入信封再按矩阵校验真实存在性。
func ValidateFinalWriteBody(endpoint, sid, customerCode string, body map[string]any) (map[string]any, error) {
	finalBody := InjectWriteEnvelope(body, sid, customerCode)
	if err := ValidateBodyMatrix(endpoint, finalBody, customerCode); err != nil {
		return nil, err
	}
	return finalBody, nil
}
