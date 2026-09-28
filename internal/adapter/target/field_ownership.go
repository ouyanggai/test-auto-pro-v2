package target

import (
	"strings"
	"unicode"
)

// FieldOwnership 是表单字段的唯一所有权分类（F-035 第四轮）。
// 配置、运行构造与写后核对必须共用本登记处，禁止在执行器里再写一套前缀特例。
type FieldOwnership string

const (
	// FieldOwnershipTargetContext 是目标平台按当前登录会话维护的上下文身份字段。
	// 这些字段会随节点处理人变化，不能参与跨节点业务值保留或写后正文核对。
	FieldOwnershipTargetContext FieldOwnership = "target_context"
	// FieldOwnershipToolOwned 当前节点配置明确覆盖的业务字段：目标保存后必须等于本次实际发送值。
	FieldOwnershipToolOwned FieldOwnership = "tool_owned"
	// FieldOwnershipPreservedBusiness 上游节点已经填写、当前节点无权限覆盖的业务字段：丢失或被工具改写则阻塞。
	FieldOwnershipPreservedBusiness FieldOwnership = "preserved_business"
	// FieldOwnershipTargetDerived 目标引擎根据审批事实自动生成的字段：普通实例动作不与历史空值严格相等，
	// 也禁止工具删除、清空或回写历史旧值；重新发起是产品明确要求清空意见的例外动作。
	FieldOwnershipTargetDerived FieldOwnership = "target_derived"
	// FieldOwnershipNodeOwnedFuture 后续节点才拥有的字段：当前请求不得提前发送。
	FieldOwnershipNodeOwnedFuture FieldOwnership = "node_owned_future"
)

// DerivedFieldKind 区分目标衍生审批意见的文本/对象/列表三种伴生结构。
type DerivedFieldKind string

const (
	// DerivedFieldKindNone 不是目标衍生审批意见。
	DerivedFieldKindNone DerivedFieldKind = ""
	// DerivedFieldKindText 对应 auto_audit_info_<n> 文本字段。
	DerivedFieldKindText DerivedFieldKind = "text"
	// DerivedFieldKindObject 对应 auto_audit_info_obj_<n> 对象字段。
	DerivedFieldKindObject DerivedFieldKind = "object"
	// DerivedFieldKindList 对应 auto_audit_info_obj_list_<n> 会签/多条意见列表。
	DerivedFieldKindList DerivedFieldKind = "list"
)

// FieldOwnershipRecord 是一次字段分类结果：分类、衍生种类和同序号伴生键。
type FieldOwnershipRecord struct {
	Ownership     FieldOwnership
	DerivedKind   DerivedFieldKind
	Index         string
	TextKey       string
	ObjectKey     string
	ListKey       string
	CompanionKeys []string
}

// ClassifyFormField 按字段名与当前节点决策返回所有权分类。
// overlaid 表示本节点实际覆盖；withheld 表示后续节点专属被扣留。
// 目标上下文和衍生字段优先于覆盖/扣留判定：会话身份变化不作业务核对，auto_audit_info_* 即使出现在配置覆盖清单里，
// 普通动作写后核对仍按 target_derived 处理；重新发起的清空语义由动作构造器额外核对。
func ClassifyFormField(field string, overlaid, withheld bool) FieldOwnershipRecord {
	if IsTargetContextField(field) {
		return FieldOwnershipRecord{Ownership: FieldOwnershipTargetContext}
	}
	record := ClassifyTargetDerivedField(field)
	if record.Ownership == FieldOwnershipTargetDerived {
		return record
	}
	if withheld {
		record.Ownership = FieldOwnershipNodeOwnedFuture
		return record
	}
	if overlaid {
		record.Ownership = FieldOwnershipToolOwned
		return record
	}
	record.Ownership = FieldOwnershipPreservedBusiness
	return record
}

// IsTargetContextField 判断字段是否由目标平台当前登录会话维护。
// 发起、重提和审批写入时，目标页面都会按实时身份覆盖这些键；节点切换后值变化是正常事实，
// 不能把它们当成上一节点写入的业务值来阻塞恢复。global_user_basic_information 虽不在逐字段规则表中，
// 但同样是目标登录上下文的整体字段，必须一并排除。
func IsTargetContextField(field string) bool {
	key := strings.TrimSpace(field)
	if key == "" {
		return false
	}
	if key == "global_user_basic_information" {
		return true
	}
	_, ok := targetLoginIdentityFieldRules[key]
	return ok
}

// ClassifyTargetDerivedField 识别目标审批意见衍生字段及其伴生键。
// 必须先匹配 obj_list_、再匹配 obj_、最后匹配纯文本前缀，避免 auto_audit_info_ 吞掉伴生键。
func ClassifyTargetDerivedField(field string) FieldOwnershipRecord {
	key := strings.TrimSpace(field)
	if key == "" {
		return FieldOwnershipRecord{}
	}
	switch {
	case hasDerivedPrefix(key, "auto_audit_info_obj_list_"):
		index := strings.TrimPrefix(key, "auto_audit_info_obj_list_")
		return derivedRecord(index, DerivedFieldKindList)
	case hasDerivedPrefix(key, "auto_audit_info_obj_"):
		index := strings.TrimPrefix(key, "auto_audit_info_obj_")
		return derivedRecord(index, DerivedFieldKindObject)
	case hasDerivedPrefix(key, "auto_audit_info_"):
		index := strings.TrimPrefix(key, "auto_audit_info_")
		return derivedRecord(index, DerivedFieldKindText)
	default:
		return FieldOwnershipRecord{}
	}
}

// IsTargetDerivedField 判断字段是否属于目标审批意见衍生家族。
func IsTargetDerivedField(field string) bool {
	return ClassifyTargetDerivedField(field).Ownership == FieldOwnershipTargetDerived
}

// systemStructuralFields 是目标流程引擎自己管理、会出现在 formDataMongoVo 里但绝不属于任何节点业务内容的
// 结构性标识字段。工具重发整份表单时会把它们带上，但目标按实例自身重写/规范化，读回值可能缺失或不同；
// 因此这些字段一律不纳入表单写后核对，否则会把「实例自身 ID 未原样回显」误判成写入丢失而阻塞整条路径。
// 目前只登记实测确认的 flowInstanceId（实例自身 ID）；后续如再观察到同类结构键，在此登记，禁止在执行器里另写特例。
var systemStructuralFields = map[string]bool{
	"flowInstanceId": true,
}

// IsSystemStructuralField 判断字段是否为目标引擎管理的结构性标识字段（不参与业务写后核对）。
func IsSystemStructuralField(field string) bool {
	return systemStructuralFields[strings.TrimSpace(field)]
}

// personPickerCompanionSuffixes 是目标人员选择组件自动生成的伴生字段后缀。
// 这类字段由目标平台的人员选择组件在写入时自动附带，用于记录条件、人员 ID 与下拉显示名：
//   - __condition：人员选择的条件字段，记录选择了哪个分支条件
//   - __formPersonId：人员选择的人员 ID 字段
//   - __virtualName：人员选择的虚拟显示名字段
//
// 特点：工具提交时会写入这些字段，目标平台在节点流转后，下一节点若没有同名人员组件，
// /getCurrentFromData 接口不会在当前节点视图里返回这些字段——字段没有真正消失，只是
// 不在当前节点的字段权限范围内。因此跨节点时不能以这些字段"不存在"来判定丢失。
var personPickerCompanionSuffixes = []string{
	"__condition",
	"__formPersonId",
	"__virtualName",
}

// IsPersonPickerCompanionField 判断字段是否为目标人员选择组件的伴生字段。
// 伴生字段在节点流转后不一定出现在下一节点的表单视图里，不能做跨节点丢失核对。
func IsPersonPickerCompanionField(field string) bool {
	f := strings.TrimSpace(field)
	for _, suffix := range personPickerCompanionSuffixes {
		if strings.HasSuffix(f, suffix) && len(f) > len(suffix) {
			return true
		}
	}
	return false
}

// TargetDerivedCompanionKeys 返回同序号文本/对象/列表伴生键（含自身），供清空与核对一次处理完整结构。
func TargetDerivedCompanionKeys(field string) []string {
	return ClassifyTargetDerivedField(field).CompanionKeys
}

// IsEmptyDerivedValue 判断目标衍生字段是否视为空：空串、nil、空对象、空列表都算空。
func IsEmptyDerivedValue(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// DerivedFieldClearedByTool 判断工具是否把目标已有非空衍生值删掉或写空。
func DerivedFieldClearedByTool(previous, current any) bool {
	return !IsEmptyDerivedValue(previous) && IsEmptyDerivedValue(current)
}

// hasDerivedPrefix 要求前缀命中后剩余部分是目标使用的序号（数字或字母数字），避免误伤业务字段。
func hasDerivedPrefix(field, prefix string) bool {
	if !strings.HasPrefix(field, prefix) {
		return false
	}
	index := strings.TrimPrefix(field, prefix)
	if index == "" {
		return false
	}
	for _, item := range index {
		if unicode.IsLetter(item) || unicode.IsDigit(item) || item == '_' {
			continue
		}
		return false
	}
	return true
}

// derivedRecord 按序号组装三类伴生键关系。
func derivedRecord(index string, kind DerivedFieldKind) FieldOwnershipRecord {
	textKey := "auto_audit_info_" + index
	objectKey := "auto_audit_info_obj_" + index
	listKey := "auto_audit_info_obj_list_" + index
	return FieldOwnershipRecord{
		Ownership:     FieldOwnershipTargetDerived,
		DerivedKind:   kind,
		Index:         index,
		TextKey:       textKey,
		ObjectKey:     objectKey,
		ListKey:       listKey,
		CompanionKeys: []string{textKey, objectKey, listKey},
	}
}
