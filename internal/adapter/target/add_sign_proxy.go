package target

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// InsertAddSignNode 在完整流程代理树的指定节点之后插入加签审批节点，保留目标返回的其余字段。
// 目标平台手动加签（FormMulBranch/commonFlow 设计器 + updateFlowProxy）的语义是在当前节点与
// 它的后继之间插入一个新审批节点（styleType/currentDelete 标记、auditType=assign + personnel
// 人员明细、字段权限随当前节点复制），后端所有权复制后为节点生成新 ID；只在既有节点上追加
// 人员明细不会产生新节点，「查看流程」因此看不到加签节点。输入必须是目标 /web/flowProxy/findById
// 返回的完整 FlowProxyVo；锚点节点和人员均要求唯一、非空，任何结构不明确的情况都在发出写请求前失败，
// 避免用简化树覆盖目标代理配置。
func InsertAddSignNode(flowProxyTree json.RawMessage, nodeProxyID string, userIDs []string) (json.RawMessage, error) {
	nodeProxyID = strings.TrimSpace(nodeProxyID)
	if nodeProxyID == "" {
		return nil, errors.New("加签锚点节点代理标识为空，拒绝发送")
	}
	ids := uniqueNonEmpty(userIDs)
	if len(ids) == 0 {
		return nil, errors.New("加签目标人员为空，拒绝发送")
	}

	root, err := decodeFlowProxyTree(flowProxyTree)
	if err != nil {
		return nil, err
	}

	found := 0
	var insertErr error
	var walk func(any)
	walk = func(value any) {
		object, ok := value.(map[string]any)
		if ok {
			if strings.TrimSpace(stringValue(object["id"])) == nodeProxyID {
				found++
				if insertErr == nil {
					insertErr = insertNodeAfter(object, ids)
				}
			}
			for _, child := range object {
				walk(child)
			}
			return
		}
		if list, ok := value.([]any); ok {
			for _, child := range list {
				walk(child)
			}
		}
	}
	walk(root)
	if insertErr != nil {
		return nil, insertErr
	}
	if found == 0 {
		return nil, fmt.Errorf("加签锚点节点 %s 不在完整流程代理树中，拒绝发送", nodeProxyID)
	}
	if found > 1 {
		return nil, fmt.Errorf("加签锚点节点 %s 在流程代理树中出现多次，拒绝发送", nodeProxyID)
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("加签流程代理树重新编码失败：%w", err)
	}
	return encoded, nil
}

// insertNodeAfter 在锚点节点与其现有后继之间插入加签审批节点。
// 新节点不携带 ID，由目标后端在所有权复制时生成；后继子树整体成为新节点的后继。
func insertNodeAfter(anchor map[string]any, userIDs []string) error {
	// 人员名称候选目录挂在锚点的审批配置上（userVoList/defaultUserVoList）。
	auditConfig, _ := anchor["flowNodeAuditConfig"].(map[string]any)
	names := candidateNames(auditConfig)
	details := make([]any, 0, len(userIDs))
	displayNames := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		detail := map[string]any{
			"bizId":           userID,
			"id":              userID,
			"auditDetailType": "personnel",
		}
		if name := names[userID]; name != "" {
			detail["name"] = name
			displayNames = append(displayNames, name)
		}
		details = append(details, detail)
	}
	nodeName := "审核人"
	if len(displayNames) > 0 {
		nodeName = strings.Join(displayNames, "、")
	}
	inserted := map[string]any{
		// 目标设计器新建审批节点的基础形状：无 ID、类型 common，加签节点带 styleType/currentDelete 标记。
		"nodeName":              nodeName,
		"type":                  "common",
		"styleType":             true,
		"currentDelete":         true,
		"nodeUserList":          []any{},
		"childFlowNodeTemplate": anchor["childFlowNodeTemplate"],
		"flowNodeAuditConfig": map[string]any{
			// assign + personnel 明细与目标模板中指定人员节点的持久化形状一致；countersignNum=1
			// 表示签到即过，多人时按会签明细全部签到。
			"auditType":                "assign",
			"type":                     "scramble",
			"countersignNum":           1,
			"flowNodeDetailConfigList": details,
		},
	}
	fieldPower, ok := anchor["flowNodeFieldPowerTemplateList"].([]any)
	if !ok || fieldPower == nil {
		fieldPower = []any{}
	} else {
		clone, err := deepCopyJSON(fieldPower)
		if err != nil {
			return fmt.Errorf("加签节点字段权限复制失败：%w", err)
		}
		fieldPower = clone
	}
	// 字段权限随当前节点复制：目标 commonFlow 设计器的加签节点同样复制当前节点权限列表，
	// 保证新节点对表单字段的可见/可编辑范围与发起加签的审批节点一致。
	inserted["flowNodeFieldPowerTemplateList"] = fieldPower
	anchor["childFlowNodeTemplate"] = inserted
	return nil
}

// HasAddSignNode 核对目标当前完整代理树中是否恰好存在一个包含全部加签人员的加签审批节点。
// updateFlowProxy 成功响应只说明目标已受理，所有权复制后所有节点都会换成新 ID，无法按写前
// 节点 ID 回查；这里按「flowNodeAuditConfig.flowNodeDetailConfigList 中 personnel 明细包含全部
// 本次人员」定位插入节点，要求全树恰好一个命中：零个说明未持久化，多个说明无法唯一证明。
func HasAddSignNode(flowProxyTree json.RawMessage, userIDs []string) (bool, error) {
	ids := uniqueNonEmpty(userIDs)
	if len(ids) == 0 {
		return false, errors.New("加签目标人员为空，无法核对")
	}
	root, err := decodeFlowProxyTree(flowProxyTree)
	if err != nil {
		return false, err
	}

	matched := 0
	var walk func(any)
	walk = func(value any) {
		object, ok := value.(map[string]any)
		if ok {
			if containsAllPersonnelDetails(object, ids) {
				matched++
			}
			for _, child := range object {
				walk(child)
			}
			return
		}
		if list, ok := value.([]any); ok {
			for _, child := range list {
				walk(child)
			}
		}
	}
	walk(root)
	if matched > 1 {
		return false, fmt.Errorf("加签人员 %v 在流程代理树中命中多个节点，无法唯一核对", ids)
	}
	return matched == 1, nil
}

// containsAllPersonnelDetails 判断节点的人员明细是否以 personnel 类型覆盖全部目标人员。
// 只认 personnel 明细：发起人、扩展属性等其他类型的命中不能证明加签节点已插入。
func containsAllPersonnelDetails(node map[string]any, userIDs []string) bool {
	config, ok := node["flowNodeAuditConfig"].(map[string]any)
	if !ok {
		return false
	}
	details, ok := config["flowNodeDetailConfigList"].([]any)
	if !ok {
		return false
	}
	present := make(map[string]struct{}, len(details))
	for _, item := range details {
		detail, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue(detail["auditDetailType"])) != "personnel" {
			continue
		}
		if id := firstDetailID(detail); id != "" {
			present[id] = struct{}{}
		}
	}
	for _, id := range userIDs {
		if _, exists := present[id]; !exists {
			return false
		}
	}
	return true
}

// decodeFlowProxyTree 严格解码目标完整代理树，保留数字字面量并拒绝拼接 JSON。
// 加签写前构造和写后核验必须使用同一解析边界，避免一个路径接受而另一个路径误判。
func decodeFlowProxyTree(flowProxyTree json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(flowProxyTree)))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("加签流程代理树不是有效 JSON：%w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("加签流程代理树包含多个 JSON 文档")
		}
		return nil, fmt.Errorf("加签流程代理树尾部不是有效 JSON：%w", err)
	}
	if _, ok := root.(map[string]any); !ok {
		return nil, errors.New("加签流程代理树顶层不是对象")
	}
	return root, nil
}

// deepCopyJSON 通过 JSON 往返深拷贝任意树片段，避免插入节点与锚点节点共享可变列表。
func deepCopyJSON(value any) ([]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var clone []any
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// candidateNames 从目标节点既有候选目录中读取可复用的中文名称，找不到时保留无名称的目标协议最小字段。
func candidateNames(config map[string]any) map[string]string {
	result := make(map[string]string)
	for _, key := range []string{"userVoList", "defaultUserVoList"} {
		list, ok := config[key].([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			candidate, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id := strings.TrimSpace(firstNonEmptyString(stringValue(candidate["id"]), stringValue(candidate["bizId"])))
			if id == "" {
				continue
			}
			name := strings.TrimSpace(firstNonEmptyString(stringValue(candidate["name"]), stringValue(candidate["realName"]), stringValue(candidate["displayName"])))
			if name != "" {
				result[id] = name
			}
		}
	}
	return result
}

// firstDetailID 取人员明细的业务 ID；id 在旧响应里也可能承担 bizId 作用。
func firstDetailID(detail map[string]any) string {
	return strings.TrimSpace(firstNonEmptyString(stringValue(detail["bizId"]), stringValue(detail["id"])))
}

// uniqueNonEmpty 去掉人员策略中的空值与重复值，避免目标端重复追加同一人员。
func uniqueNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// stringValue 将代理树中的 JSON 标量安全转换成字符串，不把复杂对象误当成业务标识。
func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if number, ok := value.(json.Number); ok {
		return number.String()
	}
	return ""
}

// firstNonEmptyString 返回首个非空文本，供代理树候选名称解析使用。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
