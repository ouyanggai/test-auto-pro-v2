package target

import (
	"strings"
	"unicode"
)

// FieldOwnership 是表单字段的唯一所有权分类（F-035 第四轮）。
// 配置、运行构造与写后核对必须共用本登记处，禁止在执行器里再写一套前缀特例。
type FieldOwnership string

const (
	// FieldOwnershipToolOwned 当前节点配置明确覆盖的业务字段：目标保存后必须等于本次实际发送值。
	FieldOwnershipToolOwned FieldOwnership = "tool_owned"
	// FieldOwnershipPreservedBusiness 上游节点已经填写、当前节点无权限覆盖的业务字段：丢失或被工具改写则阻塞。
	FieldOwnershipPreservedBusiness FieldOwnership = "preserved_business"
	// FieldOwnershipTargetDerived 目标引擎根据审批事实自动生成的字段：不与历史空值严格相等；禁止工具删除、清空或回写历史旧值。
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
// 目标衍生字段优先于覆盖/扣留判定：auto_audit_info_* 即使出现在配置覆盖清单里，写后核对仍按 target_derived 处理。
func ClassifyFormField(field string, overlaid, withheld bool) FieldOwnershipRecord {
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
