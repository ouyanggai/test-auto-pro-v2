package step

import (
	"encoding/json"
	"sort"
	"strings"

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

// companionFieldSuffixes 是目标表单为选项型与人员型控件维护的伴生键后缀。
// 目标只对控件本体声明权限，这些伴生键不单独声明，但必须跟随本体一起提交：
// 例如条件求值读的就是 classificationId__virtualName（语义清单第 15 条），漏掉它分支就算不出来。
var companionFieldSuffixes = []string{"__virtualName", "__condition", "__formPersonId"}

// fieldIdentities 返回一个表单数据键在权限判断上等价的字段身份集合：
// 键本身、去掉伴生后缀的本体，以及名称字段对应的同前缀 Id 字段（目标只对 Id 控件声明权限）。
func fieldIdentities(key string) []string {
	identities := []string{key}
	body := key
	for _, suffix := range companionFieldSuffixes {
		if strings.HasSuffix(body, suffix) && len(body) > len(suffix) {
			body = strings.TrimSuffix(body, suffix)
			identities = append(identities, body)
			break
		}
	}
	switch {
	case strings.HasSuffix(body, "Names") && len(body) > 5:
		identities = append(identities, strings.TrimSuffix(body, "Names")+"Ids")
	case strings.HasSuffix(body, "Name") && len(body) > 4:
		identities = append(identities, strings.TrimSuffix(body, "Name")+"Id")
	}
	return identities
}

// editableCoversKey 判断一个可编辑字段集合是否覆盖某个表单数据键。
// 除等值匹配外还认两种目标约定：伴生键跟随本体；子表单容器键由它的列权限（`容器.列`）覆盖。
func editableCoversKey(editable []string, key string) bool {
	if key == "" {
		return false
	}
	identities := fieldIdentities(key)
	prefix := key + "."
	for _, field := range editable {
		for _, identity := range identities {
			if field == identity {
				return true
			}
		}
		if strings.HasPrefix(field, prefix) {
			return true
		}
	}
	return false
}

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
	if editableCoversKey(nodeEditableFields(runCtx, currentNodeKey), key) {
		return false
	}
	for nodeKey, editable := range runCtx.NodeEditableFields {
		if nodeKey == currentNodeKey {
			continue
		}
		if editableCoversKey(editable, key) {
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
}

// BuildNodeFormData 按上述规则构造本步写请求的表单数据。
// instanceCurrent 是目标实例当前的完整表单数据（发起时为 nil）；它已由适配层以 json.Number 解码，
// 重新编码不会改写数字字面量，因此可以安全地合并后再序列化。
func BuildNodeFormData(runCtx RunContext, compiled model.CompiledActionStep, instanceCurrent map[string]any) (FormDataPlan, error) {
	configured, err := decodeConfiguredFormData(runCtx.EffectiveFormData)
	if err != nil {
		return FormDataPlan{}, err
	}
	plan := FormDataPlan{Overlaid: []string{}, Withheld: []string{}}
	editable := nodeEditableFields(runCtx, compiled.NodeKey)

	var merged map[string]any
	if len(instanceCurrent) > 0 {
		copied, err := jsonvalues.DeepCopyObject(instanceCurrent)
		if err != nil {
			return FormDataPlan{}, err
		}
		merged = copied
		plan.BaseFromInstance = true
		// 实例已存在：只覆盖本节点声明可编辑的配置字段，其余保持实例现状。
		for key, value := range configured {
			if editableCoversKey(editable, key) {
				merged[key] = value
				plan.Overlaid = append(plan.Overlaid, key)
				continue
			}
			if _, exists := merged[key]; !exists && !ownedByOtherNodeOnly(runCtx, compiled.NodeKey, key) {
				// 实例里还没有、且没有任何其他节点声明能编辑它：属于表单自身维护的伴生键，
				// 缺了会让目标少一份数据，按配置值补上。
				merged[key] = value
				plan.Overlaid = append(plan.Overlaid, key)
				continue
			}
			plan.Withheld = append(plan.Withheld, key)
		}
	} else {
		merged = make(map[string]any, len(configured))
		// 发起：基线就是发起态渲染出来的完整表单模型，只去掉只有后续节点才能编辑的字段。
		for key, value := range configured {
			if ownedByOtherNodeOnly(runCtx, compiled.NodeKey, key) {
				plan.Withheld = append(plan.Withheld, key)
				continue
			}
			merged[key] = value
			if editableCoversKey(editable, key) {
				plan.Overlaid = append(plan.Overlaid, key)
			}
		}
	}
	sort.Strings(plan.Overlaid)
	sort.Strings(plan.Withheld)
	if len(merged) == 0 {
		return plan, nil
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return FormDataPlan{}, err
	}
	plan.Payload = encoded
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
	case model.ActionSubmit, model.ActionApprove, model.ActionReject,
		model.ActionResubmit, model.ActionStorageFormData, model.ActionForward:
		return true
	default:
		return false
	}
}
