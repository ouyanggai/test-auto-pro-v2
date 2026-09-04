package service

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

var pathFormWriteSegmentPrefixes = []string{
	"submit", "draft", "save", "update", "delete", "remove", "upload", "import",
	"create", "insert", "modify", "edit", "bind", "unbind", "operate", "approve",
	"reject", "withdraw", "terminate", "abandon", "complete", "execute", "publish",
	"cancel", "trigger", "send", "reset", "copy", "move", "rename", "change",
	"switch", "close", "replace",
}

var pathFormReadSegmentPrefixes = []string{
	"find", "get", "list", "query", "search", "read", "load", "fetch", "select",
	"lookup", "count", "check", "validate", "preview", "download", "statistical", "statistics",
}

var pathFormReadSegmentSuffixes = []string{"list", "tree", "detail", "details", "options", "children", "page"}

// uniquePublicStrings 按首次出现顺序去除空白摘要，避免把目标端重复问题暴露给工作区。
func uniquePublicStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

// runtimeTemplate 合并目标返回的 FormMaking 原始模板片段，保留既有 runtime 的 list/config 协议。
// 这里只做 JSON 结构拼接，不推断字段、生成值或建立工具侧字段映射。
func runtimeTemplate(forms []target.FormRuntimeTemplate) (map[string]any, []string) {
	template := map[string]any{"list": []any{}, "config": map[string]any{}}
	if len(forms) == 0 {
		return template, []string{}
	}
	issues := make([]string, 0)
	for index, form := range forms {
		var fragment map[string]any
		if err := json.Unmarshal([]byte(form.TemplateData), &fragment); err != nil || fragment == nil {
			issues = append(issues, "第 "+itoa(index+1)+" 个 FormMaking 模板无法解析")
			continue
		}
		list, ok := fragment["list"].([]any)
		if !ok {
			issues = append(issues, "第 "+itoa(index+1)+" 个 FormMaking 模板缺少字段列表")
			continue
		}
		template["list"] = append(template["list"].([]any), list...)
		if index == 0 {
			if config, ok := fragment["config"].(map[string]any); ok {
				template["config"] = config
			}
		}
	}
	return template, uniquePublicStrings(issues)
}

// itoa 将小范围模板序号转换为文本，保持运行时投影只依赖标准库。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	const digits = "0123456789"
	buf := [20]byte{}
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = digits[value%10]
		value /= 10
	}
	return string(buf[pos:])
}

// projectVueCustomPage 投影目标页面已声明的字段和校验元数据，保持页面原始路径并不生成表单值。
func projectVueCustomPage(rule *target.VueCustomPageRule) *model.PathVueCustomPageRule {
	if rule == nil {
		return nil
	}
	result := &model.PathVueCustomPageRule{
		Status: vuePageRuleStatus(rule), PageName: rule.PageName, ComponentName: rule.ComponentName, Route: rule.Route,
		Fields: make([]model.PathVueCustomFieldRule, 0, len(rule.Fields)), Issues: uniquePublicStrings(rule.Issues),
	}
	for _, field := range rule.Fields {
		options := make([]model.PathVueCustomFieldOption, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, model.PathVueCustomFieldOption{Label: option.Label, Value: option.Value})
		}
		result.Fields = append(result.Fields, model.PathVueCustomFieldRule{
			Path: field.Path, Name: field.Name, ValueType: field.ValueType, ValueShape: field.ValueShape, Serialization: field.Serialization,
			Required: field.Required, ReadOnly: field.ReadOnly, Hidden: field.Hidden, Disabled: field.Disabled,
			Nested: field.Nested, Collection: field.Collection, CandidateKind: field.CandidateKind, CandidateSource: field.CandidateSource,
			DefaultValue: field.DefaultValue, DataSource: field.DataSource, Format: field.Format,
			Validation: append([]string(nil), field.Validation...), ValidationCapability: append([]string(nil), field.ValidationCapability...),
			Evidence: field.Evidence, Options: options,
		})
	}
	return result
}

// vueCustomFormPermissions 将目标页面明确声明的只读、隐藏和可编辑字段直接传给 form-runtime。
func vueCustomFormPermissions(page *target.VueCustomPageRule) []model.PathFormPermission {
	if page == nil {
		return []model.PathFormPermission{}
	}
	permissions := make([]model.PathFormPermission, 0, len(page.Fields))
	for _, field := range page.Fields {
		path := strings.TrimSpace(field.Path)
		if path == "" {
			continue
		}
		power := "edit"
		if field.Hidden {
			power = "hide"
		} else if field.ReadOnly || field.Disabled {
			power = "only_read"
		}
		permissions = append(permissions, model.PathFormPermission{Field: path, Power: power})
	}
	return permissions
}

// formPermissions 合并目标流程可达节点的字段权限；不在目标节点声明的字段不被工具侧猜测。
func formPermissions(tree *target.FlowNodeTemplate, reachable []string) []model.PathFormPermission {
	reachableSet := make(map[string]bool, len(reachable))
	for _, id := range reachable {
		reachableSet[id] = true
	}
	powers := make(map[string]string)
	var visit func(*target.FlowNodeTemplate, map[string]bool)
	visit = func(node *target.FlowNodeTemplate, visited map[string]bool) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		if reachableSet[node.ID] {
			for _, power := range node.FieldPowers {
				field := normalizeRuntimeFieldPath(power.EnglishName)
				if field == "" {
					continue
				}
				if power.Power == "edit" || powers[field] == "" {
					powers[field] = power.Power
				}
			}
		}
		for _, branch := range node.ConditionNodes {
			visit(branch.Child, visited)
		}
		for _, branch := range node.ParallelNodes {
			visit(branch.Child, visited)
		}
		visit(node.Child, visited)
	}
	visit(tree, map[string]bool{})
	// 配置阶段的表单永远处于发起态：发起节点声明了字段权限时只用它，
	// 审批节点放开的编辑权限不能提前生效，否则发起时不可填的字段会被渲染成可填。
	if initiator := initiatorFieldPowers(tree); len(initiator) > 0 {
		powers = initiator
	}
	result := make([]model.PathFormPermission, 0, len(powers))
	for field, power := range powers {
		if power != "edit" && power != "hide" {
			power = "only_read"
		}
		result = append(result, model.PathFormPermission{Field: field, Power: power})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Field < result[right].Field })
	return result
}

// initiatorFieldPowers 只返回目标发起节点声明的字段权限；没有发起节点或没有声明时返回空。
func initiatorFieldPowers(tree *target.FlowNodeTemplate) map[string]string {
	start := findInitiatorNode(tree, map[string]bool{})
	if start == nil {
		return nil
	}
	powers := make(map[string]string, len(start.FieldPowers))
	for _, power := range start.FieldPowers {
		field := normalizeRuntimeFieldPath(power.EnglishName)
		if field == "" {
			continue
		}
		powers[field] = power.Power
	}
	return powers
}

// findInitiatorNode 按目标节点类型定位发起节点，不按名称或位置猜测。
func findInitiatorNode(node *target.FlowNodeTemplate, visited map[string]bool) *target.FlowNodeTemplate {
	if node == nil || visited[node.ID] {
		return nil
	}
	visited[node.ID] = true
	if strings.TrimSpace(node.Type) == "start" {
		return node
	}
	for _, branch := range node.ConditionNodes {
		if found := findInitiatorNode(branch.Child, visited); found != nil {
			return found
		}
	}
	for _, branch := range node.ParallelNodes {
		if found := findInitiatorNode(branch.Child, visited); found != nil {
			return found
		}
	}
	return findInitiatorNode(node.Child, visited)
}

// normalizeRuntimeFieldPath 仅归一目标节点返回的嵌套字段分隔符，不按名称相似度映射。
func normalizeRuntimeFieldPath(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "_$$_", "."))
}

// vuePageRuleStatus 把目标页面返回的问题收敛为工作区可读状态，不把缺失字段伪装为可编辑。
func vuePageRuleStatus(page *target.VueCustomPageRule) string {
	if page == nil {
		return "blocked"
	}
	if len(page.Issues) > 0 {
		return "partial"
	}
	return "complete"
}

// pathFormRequestSegments 解码目标请求路径并按层级归一，用于先识别写语义再判断查询动作。
func pathFormRequestSegments(path string) []string {
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

// pathFormReadRequestAllowed 对齐 iframe 运行时的只读边界：安全方法直接可读，POST 只接受明确查询动作。
func pathFormReadRequestAllowed(method, path string) bool {
	segments := pathFormRequestSegments(path)
	joined := "/" + strings.Join(segments, "/")
	if strings.Contains(joined, "/web/flowoperate") ||
		strings.Contains(joined, "/web/user/api/login/user/login") ||
		strings.Contains(joined, "/web/user/api/login/user/loginout") ||
		strings.Contains(joined, "/web/user/api/login/user/switchlinkage") {
		return false
	}
	for _, segment := range segments {
		for _, prefix := range pathFormWriteSegmentPrefixes {
			if strings.Contains(segment, prefix) {
				return false
			}
		}
	}
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		return true
	}
	if method != "POST" || len(segments) == 0 {
		return false
	}
	action := segments[len(segments)-1]
	for _, prefix := range pathFormReadSegmentPrefixes {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	for _, suffix := range pathFormReadSegmentSuffixes {
		if strings.HasSuffix(action, suffix) {
			return true
		}
	}
	return false
}

// projectPathFormReadRequests 仅提取目标模板显式声明且具备明确只读语义的请求，拒绝写动作和未知 POST。
func projectPathFormReadRequests(snapshot target.PathConfigurationSnapshot, template map[string]any) []model.PathFormReadRequest {
	requests := make([]model.PathFormReadRequest, 0)
	seen := make(map[string]bool)
	appendRequest := func(method, rawPath string) {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method == "" {
			method = "GET"
		}
		path := strings.TrimSpace(rawPath)
		if parsed, err := url.Parse(path); err == nil && parsed.Path != "" {
			path = parsed.Path
		}
		if path == "" {
			return
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if !pathFormReadRequestAllowed(method, path) {
			return
		}
		key := method + "\x00" + path
		if seen[key] {
			return
		}
		seen[key] = true
		requests = append(requests, model.PathFormReadRequest{Method: method, Path: path, Source: "formmaking_template"})
	}
	// 目标自定义页面请求清单已经是目标端的只读声明，直接按原路径投影。
	if snapshot.RenderType == target.FormRenderTypeVueCustom && snapshot.VuePage != nil {
		for _, request := range snapshot.VuePage.ReadRequests {
			if request.ReadOnly {
				appendRequest(request.Method, request.Path)
			}
		}
	}
	// FormMaking 数据源只接受显式 method/url（或 path）字段；递归读取原始 config/list，不猜测组件类型。
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				visit(item)
			}
		case map[string]any:
			method, _ := typed["method"].(string)
			if path, ok := typed["url"].(string); ok {
				appendRequest(method, path)
			} else if path, ok := typed["path"].(string); ok {
				if _, explicit := typed["method"]; explicit {
					appendRequest(method, path)
				}
			}
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(template)
	sort.Slice(requests, func(left, right int) bool {
		if requests[left].Method != requests[right].Method {
			return requests[left].Method < requests[right].Method
		}
		return requests[left].Path < requests[right].Path
	})
	return requests
}

// ProjectFormPermissionsForTest 暴露发起态字段权限投影，供 test 目录下的定向用例锁定行为。
func ProjectFormPermissionsForTest(tree *target.FlowNodeTemplate, reachable []string) []model.PathFormPermission {
	return formPermissions(tree, reachable)
}

// ProjectPathFormReadRequestsForTest 暴露表单只读请求投影，供 test 目录锁定查询 POST 与写请求的安全边界。
func ProjectPathFormReadRequestsForTest(snapshot target.PathConfigurationSnapshot, template map[string]any) []model.PathFormReadRequest {
	return projectPathFormReadRequests(snapshot, template)
}
