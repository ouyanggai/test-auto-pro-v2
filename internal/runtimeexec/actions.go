// T010 动作入口登记：runtime 动作 → 执行宿主与目标组件入口方法的唯一映射。
// 该登记是 Go 侧路由与契约的共同事实源：
//   - 七个表单动作 + 选择闭环由 runtime 宿主执行（docs/s/03-审批页面全量动作对照.md 第 2/3 节）；
//   - 每个动作写明它在目标页面的真实入口方法（桥接调用，绝不点击）与允许写出的端点（按序）；
//   - Go 直连动作（add_sign 等）绝不出现在本登记中，防止同一动作两条写出口。
package runtimeexec

import (
	"fmt"

	"test-auto-pro-v2/internal/adapter/target"
)

// ActionEntry 是一个 runtime 动作的入口契约。
type ActionEntry struct {
	// Action 是协议动作键（与 runtimeActions 一致）。
	Action string
	// Host 是执行宿主（initiate=FlowDialog 链；approve=审批组件链；forward=flowTypeMixin 转发链）。
	Host string
	// EntryMethod 是宿主暴露的真实目标入口方法名。
	EntryMethod string
	// WriteEndpoints 是该动作按序允许写出的目标端点；转发固定两个写请求。
	WriteEndpoints []string
	// ContinueMethod 是选择闭环的继续入口（仅 submit/resubmit/approve 登记）。
	ContinueMethod string
}

// 写端点直接引用 adapter 的唯一常量，避免散落字面量。
// 正式提交端点已被全过程 test 回退为旧 submit，与 save_draft 同端点、按载荷语义区分。
var (
	endpointSubmit          = target.WriteEndpointSubmit
	endpointRuntimeReSubmit = target.RuntimeReSubmitEndpoint
	endpointRuntimeAudit    = target.RuntimeAuditEndpoint
	endpointStorageForm     = target.WriteEndpointStorageForm
	endpointTranspond       = target.WriteEndpointTranspond
	endpointFlowTracking    = target.WriteEndpointFlowTracking
	endpointUpdateProxy     = target.WriteEndpointUpdateFlowProxy
	endpointAppend          = target.WriteEndpointApproverAppend
	endpointRollBack        = target.WriteEndpointRollBack
	endpointRetrieve        = target.WriteEndpointRetrieve
	endpointRevocation      = target.WriteEndpointRevocation
	endpointUrge            = target.WriteEndpointUrge
	endpointPostscript      = target.FlowPostscriptEndpoint
)

// runtimeActionEntries 是唯一登记表。
var runtimeActionEntries = map[string]ActionEntry{
	"save_draft": {
		Action: "save_draft", Host: "initiate", EntryMethod: "sumbitFlow('draft')",
		WriteEndpoints: []string{endpointSubmit},
	},
	"submit": {
		Action: "submit", Host: "initiate", EntryMethod: "sumbitFlow('submit')",
		WriteEndpoints: []string{endpointSubmit},
		// 提交可能在首次调用后要求选择分支/人员：继续方法仍是提交入口（formMakingFormBusiness→enterpriseHandleSubmit）。
		ContinueMethod: "handleSaveBranchChooseNode/handleSaveParallelChooseNode/getSelectPerson",
	},
	"storage_form_data": {
		Action: "storage_form_data", Host: "approve", EntryMethod: "temporaryStorage()",
		WriteEndpoints: []string{endpointStorageForm},
	},
	"approve": {
		Action: "approve", Host: "approve", EntryMethod: "formMakingFormBusiness(true,'pass')",
		WriteEndpoints: []string{endpointRuntimeAudit},
		ContinueMethod: "handleSaveBranchChooseNode/handleSaveParallelChooseNode/getSelectPerson",
	},
	"reject": {
		Action: "reject", Host: "approve", EntryMethod: "formMakingFormBusiness(null,'no_pass')",
		WriteEndpoints: []string{endpointRuntimeAudit},
	},
	"resubmit": {
		Action: "resubmit", Host: "approve", EntryMethod: "reSubmitFormMakingFormBusiness(true)（isReInitiate）",
		WriteEndpoints: []string{endpointRuntimeReSubmit},
		ContinueMethod: "handleSaveBranchChooseNode/handleSaveParallelChooseNode/getSelectPerson",
	},
	"forward": {
		Action: "forward", Host: "forward", EntryMethod: "confirmTranspond()",
		// 转发是唯一复合动作：先建辅助实例，成功后保存附言；第一步未确认不得发第二步。
		WriteEndpoints: []string{endpointTranspond, endpointPostscript},
	},
	"choice_continue": {
		Action: "choice_continue", Host: "同原动作", EntryMethod: "与原动作同一入口（见 ContinueMethod）",
		WriteEndpoints: []string{},
	},
}

// EntryOf 返回动作入口登记；未登记动作返回错误。
func EntryOf(action string) (ActionEntry, error) {
	entry, ok := runtimeActionEntries[action]
	if !ok {
		return ActionEntry{}, fmt.Errorf("动作不在 runtime 登记内：%s", action)
	}
	return entry, nil
}

// GoDirectEndpoints 是 Go 直连动作的全部写端点：runtime 登记绝不允许涉及。
var GoDirectEndpoints = []string{
	endpointUpdateProxy, endpointAppend, endpointRollBack, endpointRetrieve,
	endpointRevocation, endpointUrge, endpointFlowTracking,
}

// RuntimeActionNames 返回登记的全部动作键（稳定排序由调用方处理）。
func RuntimeActionNames() []string {
	names := make([]string, 0, len(runtimeActionEntries))
	for name := range runtimeActionEntries {
		names = append(names, name)
	}
	return names
}
