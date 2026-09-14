package target

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// 目标请求协议矩阵（F-035/T01）的唯一代码登记处。
// 每个实际使用的写端点必须在这里登记字段存在性规则：目标页面发送的字段必须发送，
// 目标省略的字段必须省略；空数组、空对象、null 与缺失是四种不同形状，
// 不能用 Go 的 omitempty 或「非空才发送」替代。
// 证据位置见 docs/TARGET_SEMANTICS.md 第 2.2/2.3 节（F-035 重写版）与 docs/features/F-035-*.md 协议基线。
// FieldPresence 是协议矩阵里一个字段的固定存在性取值。
type FieldPresence string

const (
	// PresenceRequired 目标必发（包括固定发送的空数组/空对象/空字符串）。
	PresenceRequired FieldPresence = "required"
	// PresenceOptional 仅目标页面条件成立时发送（如手动分支选择）。
	PresenceOptional FieldPresence = "optional"
	// PresenceEmpty 目标固定发送空值形状。
	PresenceEmpty FieldPresence = "empty"
	// PresenceForbidden 目标明确不发送。
	PresenceForbidden FieldPresence = "forbidden"
)

// endpointFieldMatrix 是「端点 -> 字段 -> 存在性」的登记表；
// 供契约测试逐端点核对，构造器实现必须与本表一致。
// 端点键与 write.go / write_actions.go 的 WriteEndpoint* 常量一一对应。
var endpointFieldMatrix = map[string]map[string]FieldPresence{
	// FormMaking 新建提交/保存草稿：参考 rsh-flow-components FlowDialog.enterpriseHandleSubmit，
	// 人工成功 curl（运行 7 同计划）逐字段确认。
	WriteEndpointSubmit: {
		"data.name":                         PresenceRequired,
		"data.formProxyId":                  PresenceOptional, // FormMaking 必发；无表单时目标页面不发送
		"data.flowProxyId":                  PresenceOptional, // 无表单必发；FormMaking 不发送（与 formProxyId 互斥）
		"data.status":                       PresenceOptional, // 仅保存草稿固定 draft
		"data.companyId":                    PresenceRequired,
		"data.customerCode":                 PresenceRequired, // 目标 axios 拦截器对全部 POST 统一注入 data
		"data.flowInstanceBizRelevanceList": PresenceRequired, // 至少携带公司关联的数组
		"formDataMongoVo.data":              PresenceRequired, // 完整表单对象
		"nextAuditorList":                   PresenceRequired, // 顶层；无显式选人固定 []
		"batchCode":                         PresenceRequired, // 顶层；FlowDialog 打开时生成的 32 位批次号
		"sid":                               PresenceRequired, // 顶层；与会话同值
		"projectId":                         PresenceRequired, // 顶层；目标注入 store 的项目 ID，本项目固定空字符串
	},
	// 重新提交：参考 examineOpinion.submitFinal（无表单）与 handleSaveDraft/草稿重提页面。
	WriteEndpointReSubmit: {
		"data.id":                           PresenceRequired,
		"data.formProxyId":                  PresenceOptional, // FormMaking 必发（实例实时代理）
		"data.flowProxyId":                  PresenceOptional, // 无表单必发
		"data.companyId":                    PresenceRequired,
		"data.flowInstanceBizRelevanceList": PresenceRequired,
		"formDataMongoVo.data":              PresenceRequired,
		"nextAuditorList":                   PresenceRequired,  // 顶层；重提页面无条件 map 出数组
		"batchCode":                         PresenceForbidden, // 重提页面不生成批次号
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 审批（同意/不同意）：参考 examineOpinion.handleSubmitCheck。
	WriteEndpointAudit: {
		"data.id":                           PresenceRequired,
		"data.jobTaskId":                    PresenceRequired,
		"data.flowProxyId":                  PresenceOptional, // FormMaking 审批页同源携带；无表单页不带
		"data.name":                         PresenceOptional, // FormMaking 审批按 getBackFlowName 携带，空时 delete
		"data.flowInstanceBizRelevanceList": PresenceOptional, // 无表单页直传；FormMaking 仅公共流程/案件时设置
		"data.auditRecord":                  PresenceRequired,
		"formDataMongoVo.data":              PresenceRequired,
		"nextAuditorList":                   PresenceOptional, // 仅 pass 显式人员时发送；no_pass 不发送
		"tracking":                          PresenceRequired, // 顶层布尔；页面无条件携带
		"batchCode":                         PresenceForbidden,
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 当前节点暂存：参考 EnterpriseExamineOpinion.temporaryStorage。
	WriteEndpointStorageForm: {
		"data.id":                PresenceRequired,
		"data.currentNodeProxyId": PresenceRequired,
		"data.auditRecord":       PresenceRequired, // executeDesc 必在（页面 approveMessage）
		"formDataMongoVo.data":   PresenceRequired, // editData+getValues 整份覆盖
		"batchCode":              PresenceForbidden,
		"sid":                    PresenceRequired,
		"projectId":              PresenceRequired,
	},
	// 移交：参考 Backlog/index.vue handleHandOver（Api.schedule.handOver = approverAppend）。
	WriteEndpointApproverAppend: {
		"data.id":                          PresenceRequired,
		"data.jobTaskId":                   PresenceRequired,
		"data.batchNo":                     PresenceRequired, // 来自当前待办快照行，绝不从路径配置复制
		"data.auditRecord":                 PresenceRequired, // auditStatus=transfer + executeDesc
		"approverAppendVo.flowNodeProxyId": PresenceRequired,
		"approverAppendVo.userIds":         PresenceRequired, // 真实用户 ID 列表
		"batchCode":                        PresenceForbidden,
		"sid":                              PresenceRequired,
		"projectId":                        PresenceRequired,
	},
	// 加签：Api.schedule.updateFlowProxy（FlowOperateServiceImpl 校验完整代理树）。
	WriteEndpointUpdateFlowProxy: {
		"data.id":                PresenceRequired,
		"flowProxyProtocol.data": PresenceRequired, // 目标读取的完整 FlowProxyVo 树
		"batchCode":              PresenceForbidden,
		"sid":                    PresenceRequired,
		"projectId":              PresenceRequired,
	},
	// 回退上一审批节点：参考 GroupApproveManage/components/mixin.js clickRollBack 与 Backlog clickRollBack。
	WriteEndpointRollBack: {
		"data.id":           PresenceRequired,
		"data.jobTaskId":    PresenceRequired,
		"data.withdrawDesc": PresenceOptional, // mixin 版携带审批意见，Backlog 版不带
		"batchCode":         PresenceForbidden,
		"sid":               PresenceRequired,
		"projectId":         PresenceRequired,
	},
	// 取回：参考 Finished/index.vue retrieveProcess。
	WriteEndpointRetrieve: {
		"data.jobTaskId": PresenceRequired,
		"data.id":        PresenceRequired,
		"batchCode":      PresenceForbidden,
		"sid":            PresenceRequired,
		"projectId":      PresenceRequired,
	},
	// 撤回：参考 Submitted/index.vue withDrawFlow。
	WriteEndpointRevocation: {
		"data.id":           PresenceRequired,
		"data.withdrawDesc": PresenceRequired, // 页面发送空字符串也保留字段
		"batchCode":         PresenceForbidden,
		"sid":               PresenceRequired,
		"projectId":         PresenceRequired,
	},
	// 转发：参考 flowTypeMixin.js transpond（顶层 receiverId + data 容器 + formDataMongoVo）。
	WriteEndpointTranspond: {
		"data.name":                         PresenceRequired,
		"data.flowInstanceBizRelevanceList": PresenceOptional, // 二次转发复用旧关联；首次转发全量构造
		"receiverId":                        PresenceRequired, // 顶层，不在 data 内
		"formDataMongoVo.data":              PresenceRequired,
		"batchCode":                         PresenceForbidden,
		"sid":                               PresenceRequired,
		"projectId":                         PresenceRequired,
	},
	// 关注/取消关注：参考 GroupApproveManage/components/mixin.js setTracking。
	WriteEndpointFlowTracking: {
		"data.id":   PresenceRequired,
		"tracking":  PresenceRequired, // 顶层布尔
		"batchCode": PresenceForbidden,
		"sid":       PresenceRequired,
		"projectId": PresenceRequired,
	},
	// 催办：参考 Submitted/index.vue urgeFlow（顶层 flowInstanceId，不走 data 容器）。
	WriteEndpointUrge: {
		"flowInstanceId": PresenceRequired, // 顶层
		"data":           PresenceRequired, // 固定空对象
		"batchCode":      PresenceForbidden,
		"sid":            PresenceRequired,
		"projectId":      PresenceRequired,
	},
}

// NewBatchCode 生成目标 FlowDialog 同形状的 32 位十六进制批次号。
// 它只在 submit/draft（FlowDialog enterpriseHandleSubmit）顶层携带，生命周期与目标页面
// 「打开弹窗生成、随提交发送」一致；绝不是工具幂等键：写请求仍只发送一次，
// 响应丢失先按目标事实对账，绝不以 batchCode 为重试依据。
func NewBatchCode() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		// 批次号生成失败不得阻断写请求：退化为时间戳派生的 32 位十六进制串，
		// 长度仍与目标页面一致，只用于满足目标页面的字段形状。
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// ProtocolMatrix 返回端点字段存在性登记表的只读副本，供契约测试与文档核对。
func ProtocolMatrix() map[string]map[string]FieldPresence {
	result := make(map[string]map[string]FieldPresence, len(endpointFieldMatrix))
	for endpoint, fields := range endpointFieldMatrix {
		copied := make(map[string]FieldPresence, len(fields))
		for field, presence := range fields {
			copied[field] = presence
		}
		result[endpoint] = copied
	}
	return result
}
