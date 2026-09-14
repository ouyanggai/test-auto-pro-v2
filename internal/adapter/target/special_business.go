package target

import (
	"fmt"
	"sort"
	"strings"
)

// SpecialBusinessStatus 表示一条特殊业务登记的实施状态。
type SpecialBusinessStatus string

const (
	// SpecialBusinessImplemented 已按目标页面顺序实现，允许发对应写请求。
	SpecialBusinessImplemented SpecialBusinessStatus = "implemented"
	// SpecialBusinessUnimplemented 目标源码尚未勘定或实施证据不足，写前必须阻塞。
	SpecialBusinessUnimplemented SpecialBusinessStatus = "unimplemented"
)

// SpecialBusinessPhase 对应目标页面的前置/主流程/后置时机。
type SpecialBusinessPhase string

const (
	SpecialBusinessPhasePre  SpecialBusinessPhase = "pre"
	SpecialBusinessPhaseMain SpecialBusinessPhase = "main"
	SpecialBusinessPhasePost SpecialBusinessPhase = "post"
)

// SpecialBusinessSpec 是一条目标特殊业务链路的唯一登记。
// 未知分支不得猜测字段；命中未实现登记必须写前阻塞并指向源码符号。
type SpecialBusinessSpec struct {
	FlowType         string
	Status           SpecialBusinessStatus
	SourceSymbol     string
	Trigger          string
	Order            []string
	BusinessEndpoint string
	MainEndpoint     string
	FailContinues    bool
	IdempotentBy     string
	// OtherBiz 是主流程 flowInstanceBizRelevanceList 里业务项的 otherBiz；
	// 目标页面用 selectFlowType / componentName，工具不得另造键。
	OtherBiz         string
	Notes            string
	UnimplementedWhy string
}

// specialBusinessRegistry 是已知 FormMaking 特殊业务的唯一登记处。
// 证据：FlowDialog.formMakingFormBusiness、EnterpriseExamineOpinion.handleSubmitCheck。
var specialBusinessRegistry = map[string]SpecialBusinessSpec{
	"contract_compliance_review": {
		FlowType:         "contract_compliance_review",
		Status:           SpecialBusinessUnimplemented,
		SourceSymbol:     "LegalContractDocTable.contractBusiness / saveOrModifyContractInfo",
		Trigger:          "selectFlowType==contract_compliance_review",
		Order:            []string{"pre:contractReview.increase|modify", "pre:contractReviewLog.save", "pre:relationFile.saveBatch", "main:flowInstanceApi/submit|audit"},
		BusinessEndpoint: "/web/measuring/api/contractReview/increase",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "businessId+batchCode",
		UnimplementedWhy: "合同文件列表、在线编辑保存、手续文件绑定与 batchCode 生命周期依赖自定义组件内部状态，源码无法在无组件实例时完整复现；写前阻塞",
	},
	"contract_seal_review": {
		FlowType:         "contract_seal_review",
		Status:           SpecialBusinessUnimplemented,
		SourceSymbol:     "ContractSealReviewBusiness.contractBusiness / saveOrModifyContractInfo",
		Trigger:          "selectFlowType==contract_seal_review",
		Order:            []string{"pre:contractReview.increase|modify", "pre:legalProcedureBind", "main:flowInstanceApi/submit|audit"},
		BusinessEndpoint: "/web/measuring/api/contractReview/increase",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "businessId+batchCode",
		UnimplementedWhy: "盖章评审还依赖合同文件、手续文件绑定与集成快照；无组件实例不能发通用 submit 顶替",
	},
	"cost_funds_transactions": {
		FlowType:         "cost_funds_transactions",
		Status:           SpecialBusinessImplemented,
		SourceSymbol:     "FlowDialog.saveCostFundsBusiness",
		Trigger:          "selectFlowType==cost_funds_transactions",
		Order:            []string{"pre:/web/measuring/api/costFundsTransactions/save", "main:/web/flowInstanceApi/submit"},
		BusinessEndpoint: "/web/measuring/api/costFundsTransactions/save",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "businessId",
		OtherBiz:         "cost_funds_transactions",
		Notes:            "业务保存失败不得继续主流程；成功后把返回 id 写入 flowInstanceBizRelevanceList",
	},
	"cost_funds_invest": {
		FlowType:         "cost_funds_invest",
		Status:           SpecialBusinessImplemented,
		SourceSymbol:     "FlowDialog.saveCostFundsBusiness",
		Trigger:          "selectFlowType==cost_funds_invest",
		Order:            []string{"pre:/web/measuring/api/costFundsTransactions/save", "main:/web/flowInstanceApi/submit"},
		BusinessEndpoint: "/web/measuring/api/costFundsTransactions/save",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "businessId",
		OtherBiz:         "cost_funds_invest",
		Notes:            "type=2；其余与资金往来相同",
	},
	"publication_commission": {
		FlowType:      "publication_commission",
		Status:        SpecialBusinessImplemented,
		SourceSymbol:  "EnterpriseExamineOpinion.handleSubmitCheck",
		Trigger:       "selectFlowType==publication_commission 且审批同意",
		Order:         []string{"main:formData.manageUserDate", "main:/flowInstanceApi/audit"},
		MainEndpoint:  WriteEndpointAudit,
		FailContinues: false,
		Notes:         "无独立业务 URL；同意且当前用户是项目经理时写入 manageUserDate=YYYY-MM-DD",
	},
	"profession_indirect_provide": {
		FlowType:      "profession_indirect_provide",
		Status:        SpecialBusinessImplemented,
		SourceSymbol:  "EnterpriseExamineOpinion.handleSubmitCheck",
		Trigger:       "selectFlowType==profession_indirect_provide 且审批同意",
		Order:         []string{"main:formData.proposeLeaderDate/receiveLeaderDate", "main:/flowInstanceApi/audit"},
		MainEndpoint:  WriteEndpointAudit,
		FailContinues: false,
		Notes:         "无独立业务 URL；按当前用户是否为提资/接收专业负责人改写日期",
	},
	"staff_annual_performance": {
		FlowType:      "staff_annual_performance",
		Status:        SpecialBusinessImplemented,
		SourceSymbol:  "FlowDialog.traverseFlowNode / handleSubmitCheck.opinion",
		Trigger:       "selectFlowType==staff_annual_performance",
		Order:         []string{"pre:traverseFlowNode full tree", "main:submit|audit opinion"},
		MainEndpoint:  WriteEndpointSubmit,
		FailContinues: false,
		Notes:         "发起全树 form_person；审批同意把审批意见写入表单 opinion 字段",
	},
	"staff_annual_assessment": {
		FlowType:         "staff_annual_assessment",
		Status:           SpecialBusinessImplemented,
		SourceSymbol:     "AnnualAssessmentNoForm.saveBusinessData / FlowDialog.submitFinal",
		Trigger:          "selectFlowType==staff_annual_assessment",
		Order:            []string{"pre:/web/plan/api/annualPerformance/save", "main:/web/flowInstanceApi/submit"},
		BusinessEndpoint: "/web/plan/api/annualPerformance/save",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "bizId",
		OtherBiz:         "staff_annual_assessment",
		Notes:            "无表单考核页先保存业务再发 flowProxyId 主流程",
	},
	"expense_budget": {
		FlowType:         "expense_budget",
		Status:           SpecialBusinessImplemented,
		SourceSymbol:     "ExpensesClaimForm.postData / FlowDialog.submitFinal",
		Trigger:          "selectFlowType==expense_budget",
		Order:            []string{"pre:/web/expenseReimbursement/submit", "pre:relationFile.saveBatch", "main:/web/flowInstanceApi/submit"},
		BusinessEndpoint: "/web/expenseReimbursement/submit",
		MainEndpoint:     WriteEndpointSubmit,
		FailContinues:    false,
		IdempotentBy:     "businessId+batchCode",
		OtherBiz:         "expense_budget",
		Notes:            "无表单费用报销先保存业务与附件绑定，再发 flowProxyId 主流程；金额合计不一致阻塞",
	},
}

// LookupSpecialBusiness 返回流程类型的登记；未登记表示走通用 FormMaking submit。
func LookupSpecialBusiness(flowType string) (SpecialBusinessSpec, bool) {
	spec, ok := specialBusinessRegistry[strings.TrimSpace(flowType)]
	return spec, ok
}

// HasSpecialBusinessLifecycle 判断流程类型存在特殊业务钩子。
func HasSpecialBusinessLifecycle(flowType string) bool {
	_, ok := LookupSpecialBusiness(flowType)
	return ok
}

// SpecialBusinessFlowTypes 返回全部登记过的特殊业务流程类型（排序副本）。
func SpecialBusinessFlowTypes() []string {
	result := make([]string, 0, len(specialBusinessRegistry))
	for flowType := range specialBusinessRegistry {
		result = append(result, flowType)
	}
	sort.Strings(result)
	return result
}

// SpecialBusinessBlockReason 返回未实现分支的写前阻塞文案；已实现返回空串。
func SpecialBusinessBlockReason(flowType, action string) string {
	spec, ok := LookupSpecialBusiness(flowType)
	if !ok {
		return ""
	}
	if spec.Status != SpecialBusinessUnimplemented {
		return ""
	}
	return fmt.Sprintf("流程类型「%s」的特殊业务链路尚未实现（源码符号 %s；动作 %s）：%s；已阻塞，请在目标平台手工完成，不能发通用请求顶替",
		spec.FlowType, spec.SourceSymbol, action, spec.UnimplementedWhy)
}

// NeedsSpecialBusinessPreSave 判断该动作是否必须先发独立业务保存。
// 只有已实现且登记了业务端点的分支才返回 true；审批日期改写不算前置保存。
func NeedsSpecialBusinessPreSave(flowType, action string) bool {
	spec, ok := LookupSpecialBusiness(flowType)
	if !ok || spec.Status != SpecialBusinessImplemented {
		return false
	}
	if strings.TrimSpace(spec.BusinessEndpoint) == "" {
		return false
	}
	switch strings.TrimSpace(action) {
	case "submit", "save_draft", "resubmit":
		return true
	default:
		return false
	}
}

// NeedsNoFormFlowPreSave 判断无表单页面该动作是否必须先走 mixin.saveData。
func NeedsNoFormFlowPreSave(pageKey, action string) bool {
	spec, ok := LookupNoFormFlow(pageKey)
	if !ok || spec.Status != SpecialBusinessImplemented || spec.Chain == NoFormFlowChainBlocked {
		return false
	}
	if strings.TrimSpace(spec.BusinessEndpoint) == "" {
		return false
	}
	switch strings.TrimSpace(action) {
	case "submit", "save_draft", "resubmit":
		return true
	default:
		return false
	}
}

// SpecialBusinessOtherBiz 返回主流程关联应使用的 otherBiz；未登记返回流程类型本身。
func SpecialBusinessOtherBiz(flowType string) string {
	spec, ok := LookupSpecialBusiness(flowType)
	if ok && strings.TrimSpace(spec.OtherBiz) != "" {
		return spec.OtherBiz
	}
	return strings.TrimSpace(flowType)
}

// NoFormFlowOtherBiz 返回无表单主流程关联应使用的 otherBiz；目标页面用 componentName。
func NoFormFlowOtherBiz(pageKey string) string {
	spec, ok := LookupNoFormFlow(pageKey)
	if ok && strings.TrimSpace(spec.ComponentName) != "" {
		return spec.ComponentName
	}
	return strings.TrimSpace(pageKey)
}

// NoFormFlowChain 是无表单页面的专用写链路分类。
type NoFormFlowChain string

const (
	NoFormFlowChainUnknown   NoFormFlowChain = ""
	NoFormFlowChainMeasuring NoFormFlowChain = "measuring"
	NoFormFlowChainBudget    NoFormFlowChain = "budget"
	NoFormFlowChainBlocked   NoFormFlowChain = "blocked"
)

// NoFormFlowSpec 是一条 vue_custom/NoFormFlow 页面登记。
type NoFormFlowSpec struct {
	PageKey          string
	Chain            NoFormFlowChain
	Status           SpecialBusinessStatus
	ComponentName    string
	SubmitUses       string
	BusinessEndpoint string
	SourceSymbol     string
	UnimplementedWhy string
}

var noFormFlowRegistry = map[string]NoFormFlowSpec{
	"buy_plan":                {PageKey: "buy_plan", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "buy_plan", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuring/api/procurePlan/save", SourceSymbol: "NoFormFLow/BuyPlan.vue + Flow.submitFinal"},
	"buy_demand":              {PageKey: "buy_demand", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "buy_demand", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuringApi/materialDeviceDemandTable/save", SourceSymbol: "NoFormFLow/BuyDemand.vue + Flow.submitFinal"},
	"buy_order":               {PageKey: "buy_order", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "buy_order", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuringApi/purchaseOrder/save", SourceSymbol: "NoFormFLow/BuyOrder.vue + Flow.submitFinal"},
	"contract_review":         {PageKey: "contract_review", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "contract_review", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuring/api/contractReview/save", SourceSymbol: "NoFormFLow/ContractReview.vue + Flow.submitFinal"},
	"contract_pay_request":    {PageKey: "contract_pay_request", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "contract_pay_request", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuring/api/contractPayment/save", SourceSymbol: "NoFormFLow/ContractPayRequest.vue + Flow.submitFinal"},
	"invoice":                 {PageKey: "invoice", Chain: NoFormFlowChainMeasuring, Status: SpecialBusinessImplemented, ComponentName: "invoice", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuringApi/orderReceipt/save", SourceSymbol: "NoFormFLow/Invoice.vue + Flow.submitFinal"},
	"expense_budget":          {PageKey: "expense_budget", Chain: NoFormFlowChainBudget, Status: SpecialBusinessImplemented, ComponentName: "expense_budget", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/expenseReimbursement/submit", SourceSymbol: "FlowDialog.submitFinal + ExpensesClaimForm.postData"},
	"staff_annual_assessment": {PageKey: "staff_annual_assessment", Chain: NoFormFlowChainBudget, Status: SpecialBusinessImplemented, ComponentName: "staff_annual_assessment", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/plan/api/annualPerformance/save", SourceSymbol: "AnnualAssessmentNoForm.saveBusinessData + FlowDialog.submitFinal"},
	"company_annual_budget":   {PageKey: "company_annual_budget", Chain: NoFormFlowChainBudget, Status: SpecialBusinessImplemented, ComponentName: "company_annual_budget", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/measuring/api/costBudget/saveTask", SourceSymbol: "addNewBudget.postData + FlowDialog.submitFinal"},
	"annual_perf":             {PageKey: "annual_perf", Chain: NoFormFlowChainBudget, Status: SpecialBusinessImplemented, ComponentName: "annual_perf", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/plan/api/kpiGroup/save", SourceSymbol: "WorkTarget.submitData + FlowDialog.submitFinal"},
	"monthly_perf":            {PageKey: "monthly_perf", Chain: NoFormFlowChainBudget, Status: SpecialBusinessImplemented, ComponentName: "monthly_perf", SubmitUses: "flowProxyId", BusinessEndpoint: "/web/plan/api/kpiGroup/save", SourceSymbol: "MonthlyPerf.postData + FlowDialog.submitFinal"},
	"NoFormFlow":              {PageKey: "NoFormFlow", Chain: NoFormFlowChainBlocked, Status: SpecialBusinessUnimplemented, ComponentName: "HostNoFormPage", SubmitUses: "flowProxyId", SourceSymbol: "HOST_VUE_PAGES fallback", UnimplementedWhy: "通用无表单入口没有可证明的业务保存接口"},
}

// LookupNoFormFlow 按 auditWay/pageKey 查找无表单专用链路。
func LookupNoFormFlow(pageKey string) (NoFormFlowSpec, bool) {
	spec, ok := noFormFlowRegistry[strings.TrimSpace(pageKey)]
	return spec, ok
}

// NoFormFlowBlockReason 返回未知或未实现无表单页面的阻塞文案；已实现返回空。
func NoFormFlowBlockReason(pageKey, action string) string {
	key := strings.TrimSpace(pageKey)
	if key == "" {
		key = "NoFormFlow"
	}
	spec, ok := LookupNoFormFlow(key)
	if !ok {
		return fmt.Sprintf("无表单页面「%s」未登记专用业务链路（动作 %s），禁止用 FormMaking 的 formProxyId 协议顶替；已阻塞", key, action)
	}
	if spec.Status == SpecialBusinessUnimplemented || spec.Chain == NoFormFlowChainBlocked {
		why := spec.UnimplementedWhy
		if why == "" {
			why = "源码无法确认业务写接口"
		}
		return fmt.Sprintf("无表单页面「%s」尚未实现专用链路（源码符号 %s；动作 %s）：%s；已阻塞", spec.PageKey, spec.SourceSymbol, action, why)
	}
	return ""
}
