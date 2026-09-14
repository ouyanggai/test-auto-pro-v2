package step

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/fieldpower"
	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/model"
)

// 目标保存表单数据是整份覆盖而不是合并（语义清单第 16 条：FormDataServiceImpl 直接 setData 后 save，
// 请求不带 dataId 时还会新建文档并把实例 currentDataId 指向它），而目标自己的审批页每次提交
// generateForm.getValues() 的整份表单模型。因此工具的每一次写请求都必须提交完整表单数据，
// 只带部分字段等于把其余字段清空。
//
// 同时目标按节点声明字段权限（语义清单第 11 条），真实用户在一个节点上只能改该节点声明为 edit 的字段。
// 两条合起来给出本文件唯一的构造规则：
//
//	发起：没有实例，基线是我们在发起态渲染出来的完整表单模型，
//	      只去掉"只有后续节点才能编辑"的字段——那些值真实发起人填不出来。
//	其余动作：基线是目标实例当前的完整表单数据（只读读取），
//	      只覆盖本节点声明可编辑且我们配置过的字段，其余字段保持实例现状，
//	      绝不用历史快照覆盖上游处理人已经填过的内容。

// nodeEditableFields 取某个节点声明的可编辑字段；节点信息缺失时回落到上下文里的同源映射。
func nodeEditableFields(runCtx RunContext, nodeKey string) []string {
	if info, ok := runCtx.Nodes[nodeKey]; ok && len(info.EditableFields) > 0 {
		return info.EditableFields
	}
	return runCtx.NodeEditableFields[nodeKey]
}

// ownedByOtherNodeOnly 判断这个键只有路线上的其他节点才能编辑，本节点不能。
// 只看本条路线上的节点：路线外节点这次运行不会执行，它的声明与本次提交无关。
func ownedByOtherNodeOnly(runCtx RunContext, currentNodeKey, key string) bool {
	if fieldpower.Covers(nodeEditableFields(runCtx, currentNodeKey), key) {
		return false
	}
	for nodeKey, editable := range runCtx.NodeEditableFields {
		if nodeKey == currentNodeKey {
			continue
		}
		if fieldpower.Covers(editable, key) {
			return true
		}
	}
	return false
}

// FormDataPlan 是一次写请求的表单数据构造结果。
// Withheld 与 Overlaid 只进日志与门禁快照，让"少带了什么、覆盖了什么"可追溯，不静默丢字段。
type FormDataPlan struct {
	// Payload 是最终提交的完整表单数据 JSON；为空表示本动作不携带表单数据。
	Payload json.RawMessage
	// Overlaid 是本节点用配置值覆盖掉的键。
	Overlaid []string
	// Withheld 是被节点权限挡住、没有随本次请求提交的配置键。
	Withheld []string
	// BaseFromInstance 为真表示基线来自目标实例当前数据。
	BaseFromInstance bool
	// BaseEmptyInstance 表示实例存在但表单数据为空（空基线，不是发起态历史配置）。
	BaseEmptyInstance bool
	// Decision 是本次构造的完整节点决策记录（F-035/T05）：落日志与写后核对都消费它。
	Decision *NodeFormDataDecision
}

// NodeFormDataDecision 是一次表单写入的完整决策记录（F-035/T05）：
// 每个节点的目标字段权限、配置值、实例基线与最终载荷都能追溯到这一行。
// 后续节点只能消费该决策和最新实例事实，禁止直接把全局 EffectiveFormData 当作最终载荷。
type NodeFormDataDecision struct {
	StepNo       int    `json:"stepNo"`
	NodeKey      string `json:"nodeKey"`
	TargetNodeID string `json:"targetNodeId"`
	Action       string `json:"action"`
	// BaselineSource 是基线来源：initiation（发起态）/ instance（实例当前）/ instance_empty。
	BaselineSource string `json:"baselineSource"`
	// EditableFields 是当前节点声明可编辑（fieldPower=edit）的字段。
	EditableFields []string `json:"editableFields"`
	// OverlaidFields 是本次用配置值覆盖的字段。
	OverlaidFields []string `json:"overlaidFields"`
	// OverlaidValues 是本次实际写入的字段值（F-035 评审补充）：写后核对与跨节点核对
	// 深度比较用，字段名存在但值被改写同样算丢失。
	OverlaidValues map[string]any `json:"overlaidValues,omitempty"`
	// BaseValues 是本次构造时的完整基线快照（发起态或实例当前数据）：
	// 写后逐字段核对"上游字段保持原值"以它为准。
	BaseValues map[string]any `json:"baseValues,omitempty"`
	// PreservedFields 是基线中保留未动的字段（上一节点/目标已填值）。
	PreservedFields []string `json:"preservedFields"`
	// WithheldFields 是禁止进入本次载荷的后续节点专属/权限外配置字段。
	WithheldFields []string `json:"withheldFields"`
	// FinalPayload 是最终提交的表单数据 JSON（供指纹与写后逐字段核对）。
	FinalPayload json.RawMessage `json:"finalPayload"`
	// PayloadFingerprint 是最终表单数据 JSON 的 SHA-256 指纹（hex），可稳定比对版本。
	PayloadFingerprint string `json:"payloadFingerprint"`
	// ValidationIssues 是构造期校验问题；非空表示本次写请求必须阻塞。
	ValidationIssues []string `json:"validationIssues,omitempty"`
}

// decisionFingerprint 计算最终载荷的稳定指纹；空载荷按空串处理。
func decisionFingerprint(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// BuildNodeFormData 按上述规则构造本步写请求的表单数据并产出完整节点决策（F-035/T05）。
// hasInstance 区分「发起（主实例还不存在）」与「实例存在但表单数据为空」：
// 后者必须以空对象为基线走实例分支，绝不能退回发起分支把整份历史配置当载荷——
// 那正是本函数要根治的"历史快照覆盖上游已填内容"（评审 P2：读空静默换基线）。
// instanceCurrent 已由适配层以 json.Number 解码，重新编码不会改写数字字面量。
// previous 决策非空时执行跨节点核对：上一节点覆盖字段必须仍在实例基线里，
// 丢失或被改写一律阻塞（ValidationIssues 非空，写请求禁止发出）。
func BuildNodeFormData(runCtx RunContext, compiled model.CompiledActionStep, instanceCurrent map[string]any, hasInstance bool, previous *NodeFormDataDecision) (FormDataPlan, error) {
	configured, err := decodeConfiguredFormData(runCtx.EffectiveFormData)
	if err != nil {
		return FormDataPlan{}, err
	}
	info := runCtx.Nodes[compiled.NodeKey]
	plan := FormDataPlan{Overlaid: []string{}, Withheld: []string{}}
	editable := nodeEditableFields(runCtx, compiled.NodeKey)
	decision := &NodeFormDataDecision{
		StepNo: compiled.Sequence, NodeKey: compiled.NodeKey, TargetNodeID: info.TargetNodeID,
		Action: string(compiled.Action), EditableFields: append([]string(nil), editable...),
		OverlaidFields: []string{}, PreservedFields: []string{}, WithheldFields: []string{},
	}
	plan.Decision = decision

	var merged map[string]any
	if hasInstance {
		copied, err := jsonvalues.DeepCopyObject(instanceCurrent)
		if err != nil {
			return FormDataPlan{}, err
		}
		merged = copied
		plan.BaseFromInstance = true
		if len(merged) == 0 {
			plan.BaseEmptyInstance = true
			decision.BaselineSource = "instance_empty"
		} else {
			decision.BaselineSource = "instance"
		}
		overlaidValues := map[string]any{}
		for key, value := range configured {
			if target.IsTargetDerivedField(key) {
				// 审批后目标会机械生成衍生意见：当前节点不得用配置阶段清空值覆盖目标现值。
				continue
			}
			if fieldpower.Covers(editable, key) {
				merged[key] = value
				plan.Overlaid = append(plan.Overlaid, key)
				decision.OverlaidFields = append(decision.OverlaidFields, key)
				overlaidValues[key] = value
				continue
			}
			if _, exists := merged[key]; !exists && !ownedByOtherNodeOnly(runCtx, compiled.NodeKey, key) {
				merged[key] = value
				plan.Overlaid = append(plan.Overlaid, key)
				decision.OverlaidFields = append(decision.OverlaidFields, key)
				overlaidValues[key] = value
				continue
			}
			plan.Withheld = append(plan.Withheld, key)
			decision.WithheldFields = append(decision.WithheldFields, key)
		}
		decision.OverlaidValues = overlaidValues
		decision.BaseValues = cloneForDecision(merged)
		if previous != nil && decision.BaselineSource == "instance" {
			for _, field := range previous.OverlaidFields {
				if target.IsTargetDerivedField(field) {
					current, exists := merged[field]
					if !exists {
						if written, ok := previous.OverlaidValues[field]; ok && !target.IsEmptyDerivedValue(written) {
							decision.ValidationIssues = append(decision.ValidationIssues,
								fmt.Sprintf("目标已有非空审批意见 %s 从实例中删除；若工具请求删除该值则阻塞", field))
						}
						continue
					}
					if written, ok := previous.OverlaidValues[field]; ok && target.DerivedFieldClearedByTool(written, current) {
						decision.ValidationIssues = append(decision.ValidationIssues,
							fmt.Sprintf("目标已有非空审批意见 %s 被工具写空，已阻塞", field))
					}
					continue
				}
				if fieldpower.Covers(editable, field) {
					continue
				}
				current, exists := merged[field]
				if !exists {
					decision.ValidationIssues = append(decision.ValidationIssues,
						fmt.Sprintf("上一节点写入的字段 %s 在目标实例当前数据中丢失，不能继续提交", field))
					continue
				}
				if written, ok := previous.OverlaidValues[field]; ok {
					if !deepEqualNormalized(written, current) {
						decision.ValidationIssues = append(decision.ValidationIssues,
							fmt.Sprintf("上一节点写入的字段 %s 的值在目标实例上被改写（期望 %v，实际 %v），不能继续提交", field, written, current))
					}
				}
			}
		}
		// 保留字段清单：基线里存在且本次未覆盖的键。
		for key := range merged {
			if !containsString(decision.OverlaidFields, key) {
				decision.PreservedFields = append(decision.PreservedFields, key)
			}
		}
	} else {
		merged = make(map[string]any, len(configured))
		decision.BaselineSource = "initiation"
		// 发起：基线就是发起态渲染出来的完整表单模型，只去掉只有后续节点才能编辑的字段。
		for key, value := range configured {
			if ownedByOtherNodeOnly(runCtx, compiled.NodeKey, key) {
				plan.Withheld = append(plan.Withheld, key)
				decision.WithheldFields = append(decision.WithheldFields, key)
				continue
			}
			if target.IsTargetDerivedField(key) {
				// 新发起不得把历史审批意见带进新实例：保留键，清空文本/对象/列表。
				merged[key] = emptyDerivedValue(key)
				decision.PreservedFields = append(decision.PreservedFields, key)
				continue
			}
			merged[key] = value
			if fieldpower.Covers(editable, key) {
				plan.Overlaid = append(plan.Overlaid, key)
				decision.OverlaidFields = append(decision.OverlaidFields, key)
			} else {
				decision.PreservedFields = append(decision.PreservedFields, key)
			}
		}
	}
	// T05 断言已由构造结构保证：扣留字段不会出现在 Overlaid 清单（本节点不写它们），
	// 它们若存在于合并结果，只能是实例基线保留（目标已填值）——这恰恰是“保留上游值”的语义。
	sort.Strings(plan.Overlaid)
	sort.Strings(plan.Withheld)
	sort.Strings(decision.OverlaidFields)
	sort.Strings(decision.PreservedFields)
	sort.Strings(decision.WithheldFields)
	if len(merged) == 0 {
		decision.FinalPayload = nil
		decision.PayloadFingerprint = ""
		return plan, nil
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return FormDataPlan{}, err
	}
	plan.Payload = encoded
	decision.FinalPayload = encoded
	decision.PayloadFingerprint = decisionFingerprint(encoded)
	return plan, nil
}

// decodeConfiguredFormData 以数字保真方式解码路径生效表单数据；空数据按空对象处理。
func decodeConfiguredFormData(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	values, err := jsonvalues.DecodeObject(raw)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return map[string]any{}, nil
	}
	return values, nil
}

// ActionCarriesFormData 判断该动作的目标协议是否携带 formDataMongoVo。
// 只有带表单数据的动作才需要读实例当前数据；其余动作多读一次没有意义，也会白白拉长一步的耗时。
// 名单与 target.BuildSubmitBody/BuildAuditBody/BuildActionBody 的实际形状一一对应。
func ActionCarriesFormData(action model.ActionKey) bool {
	switch action {
	case model.ActionSaveDraft, model.ActionSubmit, model.ActionApprove, model.ActionReject,
		model.ActionResubmit, model.ActionStorageFormData, model.ActionForward:
		return true
	default:
		return false
	}
}

// cloneForDecision 为决策记录做基线深拷贝（避免后续修改污染记录）。
func cloneForDecision(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil
	}
	return cloned
}

// deepEqualNormalized 把两个值按 JSON 序列化后比较（json.Number 原样保留），
// 覆盖嵌套对象、数组与表格字段；字符串比较不做排序扰动，键序差异由 map 序列化的确定性吸收。
func deepEqualNormalized(a, b any) bool {
	encodedA, errA := json.Marshal(a)
	encodedB, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
	return string(encodedA) == string(encodedB)
}

// emptyDerivedValue 按目标衍生字段种类返回空形状：文本空串、对象空 map、列表空数组。
func emptyDerivedValue(field string) any {
	record := target.ClassifyTargetDerivedField(field)
	switch record.DerivedKind {
	case target.DerivedFieldKindObject:
		return map[string]any{}
	case target.DerivedFieldKindList:
		return []any{}
	default:
		return ""
	}
}
