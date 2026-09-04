package verdict

import (
	"fmt"
	"sort"
	"strings"
)

// OptimisticLockMessage 是目标流程实例乐观锁失败的固定提示，来自中心
// FlowInstanceServiceImpl 的 CONCURRENT_UPDATE_MESSAGE 常量，证据见语义清单第 1.8 节。
const OptimisticLockMessage = "流程状态已发生变化，请刷新后重试"

// optimisticLockEndpoints 登记能证明会走到中心 FlowInstanceServiceImpl.update 或 .save
// 从而可能返回乐观锁提示的端点。与前置拒绝清单一样按「端点 + 精确文案」全等匹配。
// 清单刻意保守：没有登记的端点即使返回同一文案也落不可解释失败，结论只会更保守而不会更乐观。
var optimisticLockEndpoints = map[string]bool{
	// 中心 FlowSubmitServiceImpl:320 调 flowInstanceService.save，save 内部走乐观锁保存。
	"/web/flowInstanceApi/submit": true,
	// 中心 FlowAuditServiceImpl 经 updateFLowInstance:348 调 flowInstanceService.update。
	"/flowInstanceApi/audit": true,
}

// isOptimisticLock 判断「端点 + 精确文案」是否命中乐观锁提示，全等匹配。
func isOptimisticLock(endpoint, message string) bool {
	return message == OptimisticLockMessage && optimisticLockEndpoints[normalizeMessage(endpoint)]
}

// OptimisticLockEndpoints 返回登记过的端点，按字典序排列，供测试与文档核对使用。
func OptimisticLockEndpoints() []string {
	endpoints := make([]string, 0, len(optimisticLockEndpoints))
	for endpoint := range optimisticLockEndpoints {
		endpoints = append(endpoints, endpoint)
	}
	sort.Strings(endpoints)
	return endpoints
}

// AuthRejectedCodes 是会话失效的两个目标错误码。HTTP 401 单独识别。
// AUTH_401 是现有只读路径漏认的那一个，本包必须认，依据见语义清单第 1.5 节。
var AuthRejectedCodes = []string{"RESP401", "AUTH_401"}

// ForbiddenWriteField 是写请求禁止携带的字段名。
// 目标平台的 @Consistency 只在请求带 batchCode 时生效，一旦生效，失败会触发注解声明的
// deleteMethodName 回滚同批次已登记数据；/web/flowInstanceApi/submit 的回滚动作就是删除实例。
// 证据见语义清单第 2.2 节。
const ForbiddenWriteField = "batchCode"

// preRejections 是前置拒绝清单：键为目标端点，值为该端点上能证明发生在任何写之前的精确文案。
// 匹配规则是「端点 + 文案全等」，禁止模糊匹配、关键字包含或跨端点复用文案。
// 清单来自目标源码枚举，逐条证据见语义清单第 1.7 节；清单外的失败一律落不可解释失败。
var preRejections = map[string][]string{
	"/web/flowInstanceApi/revocation": {
		"当前实例不存在",
		"当前流程不在运行中,无法撤销",
		"非流程发起人不能撤销",
		submitVerifyRejection,
	},
	"/web/flowInstanceApi/approverAppend": {
		"该待办记录不存在",
		"当前任务已处理",
	},
	"/web/flowInstanceApi/retrieveProcess": {
		"该待办记录不存在",
		"流程已完结,不支持取回",
		"起始节点,不支持取回",
		// 目标源码里这条文案的逗号后带一个空格，全等匹配必须原样保留。
		"当前已办任务, 不支持取回",
		"当前环节不支持取回",
		submitVerifyRejection,
	},
	"/web/flowInstanceApi/rollBackThePreviousLevel": {
		submitVerifyRejection,
	},
	"/flowInstanceApi/audit": {
		"该待办记录不存在",
	},
}

// submitVerifyRejection 是提交校验服务未注册该 auditWay 时由 FlowSubmitVerifyBaseController
// 返回的文案。它只出现在带 @FlowSubmitVerify 的端点上，且依赖目标环境的动态注册状态：
// 旧部署上同一场景抛 IllegalArgumentException，会变成不可解释失败。证据见语义清单第 1.7 节。
const submitVerifyRejection = "未发现实例"

// isPreRejection 判断「端点 + 精确文案」是否命中前置拒绝清单，全等匹配。
func isPreRejection(endpoint, message string) bool {
	for _, candidate := range preRejections[normalizeMessage(endpoint)] {
		if candidate == message {
			return true
		}
	}
	return false
}

// PreRejectionEndpoints 返回清单里登记过的端点，按字典序排列，供测试与文档核对使用。
func PreRejectionEndpoints() []string {
	endpoints := make([]string, 0, len(preRejections))
	for endpoint := range preRejections {
		endpoints = append(endpoints, endpoint)
	}
	sort.Strings(endpoints)
	return endpoints
}

// PreRejectionMessages 返回指定端点登记的全部精确文案副本，调用方修改不影响清单。
func PreRejectionMessages(endpoint string) []string {
	source := preRejections[normalizeMessage(endpoint)]
	messages := make([]string, len(source))
	copy(messages, source)
	return messages
}

// ValidateWritePayload 检查写请求载荷没有携带禁止字段。
// 这是语义清单第 2.2 节 batchCode 禁令的代码化：目标平台没有幂等键，
// batchCode 不是幂等键而是批次补偿开关，带上它会把一次失败放大成额外的删除写入。
func ValidateWritePayload(fieldNames []string) error {
	for _, name := range fieldNames {
		if normalizeMessage(name) == ForbiddenWriteField {
			return fmt.Errorf("写请求禁止携带字段 %s：它会触发目标平台的批次补偿回滚，见 docs/TARGET_SEMANTICS.md 第 2.2 节", ForbiddenWriteField)
		}
	}
	return nil
}

// normalizeMessage 只去掉首尾空白，不做大小写折叠也不做内部空格归并，
// 因为清单匹配是文案全等：目标文案里的逗号、空格差异本身就是区分依据。
func normalizeMessage(value string) string {
	return strings.TrimSpace(value)
}
