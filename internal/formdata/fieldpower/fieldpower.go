// Package fieldpower 承载目标平台「节点级表单字段权限」的判定约定（语义清单第 11 条）。
//
// 单独成包是因为配置侧与执行侧都要用同一套判据：配置侧据此按节点渲染可填字段、
// 告诉用户某个条件字段会在哪个节点填写；执行侧据此决定写请求允许覆盖哪些字段。
// 两侧口径必须一致，否则界面说的和真正发出去的不是一回事。
package fieldpower

import "strings"

// CompanionSuffixes 是目标表单为选项型与人员型控件维护的伴生键后缀。
// 目标只对控件本体声明权限，这些伴生键不单独声明，但必须跟随本体：
// 条件求值读的就是 classificationId__virtualName（语义清单第 15 条），漏掉它分支就算不出来。
var CompanionSuffixes = []string{"__virtualName", "__condition", "__formPersonId"}

// Identities 返回一个表单数据键在权限判断上等价的字段身份集合：
// 键本身、去掉伴生后缀的本体，以及名称字段对应的同前缀 Id 字段（目标只对 Id 控件声明权限）。
func Identities(key string) []string {
	identities := []string{key}
	body := key
	for _, suffix := range CompanionSuffixes {
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

// Covers 判断一个节点声明的可编辑字段集合是否覆盖某个表单数据键。
// 除等值匹配外还认两种目标约定：伴生键跟随本体；子表单容器键由它的列权限（`容器.列`）覆盖。
func Covers(editable []string, key string) bool {
	if key == "" {
		return false
	}
	identities := Identities(key)
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

// NormalizeFieldPath 把目标权限声明里的嵌套字段分隔符归一为点号。
// 目标前端消费权限时做同样的替换（EnterpriseExamineDialog 的 replaceAll('_$$_', '.')）。
func NormalizeFieldPath(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "_$$_", "."))
}
