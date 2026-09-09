package target

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// AppendAddSignUsers 在完整流程代理树的指定节点追加人员明细，保留目标返回的其余字段。
// 输入必须是目标 /web/flowProxy/findById 返回的完整 FlowProxyVo；节点和人员均要求唯一、非空，
// 任何结构不明确的情况都在发出写请求前失败，避免用简化树覆盖目标代理配置。
func AppendAddSignUsers(flowProxyTree json.RawMessage, nodeProxyID string, userIDs []string) (json.RawMessage, error) {
	nodeProxyID = strings.TrimSpace(nodeProxyID)
	if nodeProxyID == "" {
		return nil, errors.New("加签节点代理标识为空，拒绝发送")
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
	var walk func(any)
	walk = func(value any) {
		if found < 0 {
			return
		}
		object, ok := value.(map[string]any)
		if ok {
			if strings.TrimSpace(stringValue(object["id"])) == nodeProxyID {
				if config, configOK := object["flowNodeAuditConfig"].(map[string]any); configOK {
					found++
					if err := appendPersonnelDetails(config, ids); err != nil {
						found = -1
						return
					}
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
	if found < 0 {
		return nil, errors.New("加签节点人员配置结构无效，拒绝发送")
	}
	if found == 0 {
		return nil, fmt.Errorf("加签节点 %s 不在完整流程代理树中，拒绝发送", nodeProxyID)
	}
	if found > 1 {
		return nil, fmt.Errorf("加签节点 %s 在流程代理树中出现多次，拒绝发送", nodeProxyID)
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("加签流程代理树重新编码失败：%w", err)
	}
	return encoded, nil
}

// HasAddSignUsers 核对目标当前完整代理树中指定节点是否已包含本次加签人员。
// updateFlowProxy 成功响应只说明目标已受理；这里读取写后的完整代理树，逐项确认人员明细，
// 让加签和其他动作一样以目标持久化状态作为成功依据。
func HasAddSignUsers(flowProxyTree json.RawMessage, nodeProxyID string, userIDs []string) (bool, error) {
	nodeProxyID = strings.TrimSpace(nodeProxyID)
	if nodeProxyID == "" {
		return false, errors.New("加签节点代理标识为空，无法核对")
	}
	ids := uniqueNonEmpty(userIDs)
	if len(ids) == 0 {
		return false, errors.New("加签目标人员为空，无法核对")
	}
	root, err := decodeFlowProxyTree(flowProxyTree)
	if err != nil {
		return false, err
	}

	found := 0
	invalid := false
	matched := false
	var walk func(any)
	walk = func(value any) {
		object, ok := value.(map[string]any)
		if ok {
			if strings.TrimSpace(stringValue(object["id"])) == nodeProxyID {
				found++
				config, configOK := object["flowNodeAuditConfig"].(map[string]any)
				if !configOK {
					invalid = true
				} else if details, detailsOK := config["flowNodeDetailConfigList"].([]any); detailsOK {
					present := make(map[string]struct{}, len(details))
					for _, item := range details {
						if detail, detailOK := item.(map[string]any); detailOK {
							if id := firstDetailID(detail); id != "" {
								present[id] = struct{}{}
							}
						}
					}
					matched = true
					for _, id := range ids {
						if _, exists := present[id]; !exists {
							matched = false
							break
						}
					}
				} else {
					invalid = true
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
	if invalid {
		return false, errors.New("加签节点人员配置结构无效，无法核对")
	}
	if found == 0 {
		return false, fmt.Errorf("加签节点 %s 不在完整流程代理树中，无法核对", nodeProxyID)
	}
	if found > 1 {
		return false, fmt.Errorf("加签节点 %s 在流程代理树中出现多次，无法核对", nodeProxyID)
	}
	return matched, nil
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

// appendPersonnelDetails 将候选人员变成目标 FlowNodeAuditDetailConfigTemplateVo，并保持已有人员顺序。
func appendPersonnelDetails(config map[string]any, userIDs []string) error {
	details, ok := config["flowNodeDetailConfigList"]
	if !ok || details == nil {
		details = []any{}
		config["flowNodeDetailConfigList"] = details
	}
	list, ok := details.([]any)
	if !ok {
		return errors.New("flowNodeDetailConfigList 不是数组")
	}
	seen := make(map[string]struct{}, len(list)+len(userIDs))
	for _, item := range list {
		if detail, ok := item.(map[string]any); ok {
			if id := firstDetailID(detail); id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	names := candidateNames(config)
	for _, userID := range userIDs {
		if _, exists := seen[userID]; exists {
			continue
		}
		detail := map[string]any{
			"bizId":           userID,
			"id":              userID,
			"auditDetailType": "personnel",
		}
		if name := names[userID]; name != "" {
			detail["name"] = name
		}
		list = append(list, detail)
		seen[userID] = struct{}{}
	}
	config["flowNodeDetailConfigList"] = list
	return nil
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
