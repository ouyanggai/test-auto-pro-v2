package target

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

const (
	// FormSubmissionReady 表示请求体已通过纯编译预检，但尚未发送。
	FormSubmissionReady = "ready"
	// FormSubmissionBlocked 表示现有规则不足以安全编译目标请求体。
	FormSubmissionBlocked = "blocked"
)

// FormSubmissionIdentity 是编译目标表单请求所需的当前发起人身份，不包含 SID。
type FormSubmissionIdentity struct {
	UserID         string
	UserName       string
	CompanyID      string
	CompanyName    string
	DepartmentID   string
	DepartmentName string
}

// FormSubmissionCompileInput 汇总纯编译所需的当前规则和值，不提供网络客户端。
type FormSubmissionCompileInput struct {
	FlowSource string
	RenderType FormRenderType
	Values     map[string]any
	Template   map[string]any
	Forms      []FormRuntimeTemplate
	VuePage    *VueCustomPageRule
	Identity   FormSubmissionIdentity
}

// FormSubmissionIssue 说明目标请求体无法安全编译的具体原因。
type FormSubmissionIssue struct {
	Code      string
	Source    string
	FieldPath string
	Message   string
	CanRetry  bool
}

// CompiledFormSubmission 是尚未发送的目标请求计划；Payload 只在后端运行边界内使用。
type CompiledFormSubmission struct {
	Status        string
	Method        string
	Path          string
	Payload       map[string]any
	PayloadKeys   []string
	PayloadDigest string
	SuccessChecks []string
	Issues        []FormSubmissionIssue
}

// CompileFormSubmission 按渲染协议纯编译目标请求体；函数不持有客户端，也不会发起网络请求。
func CompileFormSubmission(input FormSubmissionCompileInput) CompiledFormSubmission {
	if strings.TrimSpace(input.FlowSource) != "new" {
		return blockedFormSubmission("unsupported_flow_source", "run_input", "当前仅证明了新发起表单的目标请求协议", true)
	}
	switch input.RenderType {
	case FormRenderTypeFormMaking:
		return compileFormMakingSubmission(input)
	case FormRenderTypeVueCustom:
		return compileVueSubmission(input)
	default:
		return blockedFormSubmission("unknown_render_type", "rule_catalog", "当前表单渲染协议尚未识别", true)
	}
}

// compileFormMakingSubmission 按 GroupApproveManage 新发起协议编译完整 getValues 数据。
func compileFormMakingSubmission(input FormSubmissionCompileInput) CompiledFormSubmission {
	if len(input.Forms) != 1 || strings.TrimSpace(input.Forms[0].ID) == "" {
		return blockedFormSubmission("form_proxy_unresolved", "formmaking", "FormMaking 表单代理标识未唯一确定", true)
	}
	if formTemplateHasDynamicSubmitHook(input.Template) {
		return blockedFormSubmission("dynamic_submit_hook", "formmaking", "表单包含运行前业务钩子，当前无法静态证明其目标写入副作用", false)
	}
	identity := input.Identity
	if strings.TrimSpace(identity.UserID) == "" || strings.TrimSpace(identity.CompanyID) == "" {
		return blockedFormSubmission("initiator_identity_missing", "identity", "当前发起人用户或公司身份不完整", true)
	}
	values := cloneSubmissionMap(input.Values)
	values["global_user_basic_information"] = map[string]any{
		"userId": identity.UserID, "userName": identity.UserName,
		"companyId": identity.CompanyID, "companyName": identity.CompanyName,
		"departmentId": identity.DepartmentID, "departmentName": identity.DepartmentName,
	}
	payload := map[string]any{
		"data": map[string]any{
			"formProxyId":                  input.Forms[0].ID,
			"companyId":                    identity.CompanyID,
			"flowInstanceBizRelevanceList": []any{map[string]any{"otherBiz": "company", "otherBizId": identity.CompanyID}},
		},
		"formDataMongoVo": map[string]any{"data": values},
		"nextAuditorList": []any{},
	}
	return readyFormSubmission("POST", "/web/flowInstanceApi/submit", payload, []string{"isSuccess"})
}

// compileVueSubmission 按规则目录中的 Vue 保存映射和 Java 路由证据编译 data 外壳。
func compileVueSubmission(input FormSubmissionCompileInput) CompiledFormSubmission {
	page := input.VuePage
	if page == nil || page.Submit == nil {
		return blockedFormSubmission("vue_submit_rule_missing", "vue_rule_catalog", "Vue 页面缺少已识别的保存协议", true)
	}
	submit := page.Submit
	// Submit.Blocked 是配置工作台禁止写目标系统的总闸；P4 只消费静态协议证据并纯编译请求，不会发送该请求。
	if strings.ToUpper(strings.TrimSpace(submit.Method)) != "POST" || !strings.HasPrefix(strings.TrimSpace(submit.Path), "/web/") {
		return blockedFormSubmission("vue_submit_endpoint_invalid", "vue_rule_catalog", "Vue 保存端点或方法未被证明", true)
	}
	if len(submit.Issues) > 0 || len(page.Issues) > 0 {
		return blockedFormSubmission("vue_submit_rule_partial", "vue_rule_catalog", "Vue 保存协议仍有未解决的静态分析问题", true)
	}
	if len(submit.Payload) != 1 || submit.Payload[0] != "data" {
		return blockedFormSubmission("vue_payload_shape_unknown", "vue_rule_catalog", "Vue 保存请求体不是已证明的 data 外壳", true)
	}
	if !javaRouteProvesSubmission(page.Java, submit.Method, submit.Path) {
		return blockedFormSubmission("java_submit_route_unverified", "java_rule_catalog", "Java 控制器未证明当前 Vue 保存端点", true)
	}
	for _, field := range page.Fields {
		if field.Required && submissionValueEmpty(submissionValueAt(input.Values, field.Path)) {
			issue := blockedFormSubmission("required_value_missing", "vue_rule_catalog", "Vue 必填字段缺少运行输入", true)
			issue.Issues[0].FieldPath = field.Path
			return issue
		}
	}
	payload := map[string]any{"data": cloneSubmissionMap(input.Values)}
	checks := append([]string(nil), submit.SuccessChecks...)
	return readyFormSubmission("POST", submit.Path, payload, checks)
}

// formTemplateHasDynamicSubmitHook 检查配置中会在真实提交前执行的业务脚本。
func formTemplateHasDynamicSubmitHook(template map[string]any) bool {
	config, _ := template["config"].(map[string]any)
	for _, key := range []string{"beforeSubmitAndDraft", "beforeSubmit", "beforeSubmitAndDraftNoBiz", "eventScript"} {
		value, exists := config[key]
		if exists && !submissionValueEmpty(value) {
			return true
		}
	}
	return false
}

// javaRouteProvesSubmission 要求 Java 摘要存在方法和路径完全匹配的控制器路由。
func javaRouteProvesSubmission(java *JavaPageRule, method, path string) bool {
	if java == nil {
		return false
	}
	for _, route := range java.Routes {
		if strings.EqualFold(strings.TrimSpace(route.Method), strings.TrimSpace(method)) && strings.TrimSpace(route.Path) == strings.TrimSpace(path) && strings.TrimSpace(route.Request) != "" {
			return true
		}
	}
	return false
}

// readyFormSubmission 构造稳定请求摘要，调用方只能在后续真实运行切片决定是否发送。
func readyFormSubmission(method, path string, payload map[string]any, checks []string) CompiledFormSubmission {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return CompiledFormSubmission{
		Status: FormSubmissionReady, Method: method, Path: path, Payload: cloneSubmissionMap(payload),
		PayloadKeys: keys, PayloadDigest: submissionDigest(payload), SuccessChecks: append([]string(nil), checks...), Issues: []FormSubmissionIssue{},
	}
}

// blockedFormSubmission 构造不含目标请求体的阻断结果。
func blockedFormSubmission(code, source, message string, canRetry bool) CompiledFormSubmission {
	return CompiledFormSubmission{Status: FormSubmissionBlocked, Payload: map[string]any{}, PayloadKeys: []string{}, SuccessChecks: []string{}, Issues: []FormSubmissionIssue{{Code: code, Source: source, Message: message, CanRetry: canRetry}}}
}

// submissionValueAt 按点路径读取嵌套值，集合字段由上游完整 values 保持原形。
func submissionValueAt(values map[string]any, path string) any {
	var current any = values
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

// submissionValueEmpty 统一判断预检中的缺失值形态。
func submissionValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
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

// cloneSubmissionMap 通过 JSON 深复制请求值，避免编译结果与配置对象共享可变引用。
func cloneSubmissionMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	encoded, _ := json.Marshal(values)
	result := make(map[string]any)
	_ = json.Unmarshal(encoded, &result)
	return result
}

// submissionDigest 为请求体生成稳定摘要，预检结果不需要向浏览器公开完整目标请求体。
func submissionDigest(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
