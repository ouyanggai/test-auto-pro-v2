// T004 Go/runtime 消息协议（TS 侧权威定义，Go 侧镜像见 internal/runtimeexec/protocol.go；权威定义在本文件）。
// 纪律（docs/s2/01-执行服务设计.md）：
//   - 每条消息必须携带 requestId、runId、pathRunId、stepId；除 create 外还必须携带 sessionId；
//   - 旧协议版本直接拒绝，不静默兼容；
//   - runtime 不返回 Vue 实例，只返回 JSON 数据；
//   - 结果状态固定七值，不得扩展别名。
// 协议版本：字段形状或语义发生不兼容变化时必须递增，Go 与 runtime 同时更新。
export const PROTOCOL_VERSION = 'exec-engine/v2'

// MESSAGE_KINDS 是全部消息类型；direction 声明唯一合法方向，反方向出现即拒绝。
export const MESSAGE_KINDS = {
  create: 'go_to_runtime',
  load: 'go_to_runtime',
  execute: 'go_to_runtime',
  request_forward: 'runtime_to_go',
  request_response: 'go_to_runtime',
  event: 'runtime_to_go',
  result: 'runtime_to_go',
  destroy: 'go_to_runtime'
} as const

export type MessageKind = keyof typeof MESSAGE_KINDS

// RUNTIME_ACTIONS 是 execute 命令允许的动作全集：七个 runtime 业务动作 + 页面只读命令 + 选择闭环继续命令。
// add_sign/transfer/rollback_previous/retrieve/withdraw/urge/follow/unfollow 由 Go 直连，绝不出现在协议里。
export const RUNTIME_ACTIONS = [
  'load_initiate',
  'load_approve',
  'capture',
  'validate',
  'save_draft',
  'submit',
  'storage_form_data',
  'approve',
  'reject',
  'resubmit',
  'forward',
  'choice_continue'
] as const

export type RuntimeAction = (typeof RUNTIME_ACTIONS)[number]

// RESULT_STATUSES 固定动作结果七值：
//   success 成功；failed 未发出流程写请求即失败；target_rejected 目标明确拒绝；
//   uncertain 响应丢失/崩溃无法判断；business_failed 流程已推进但后置业务失败；
//   unsupported 原生缺少入口或数据接口；timeout 动作超时。
export const RESULT_STATUSES = [
  'success',
  'failed',
  'target_rejected',
  'uncertain',
  'business_failed',
  'unsupported',
  'timeout'
] as const

export type ResultStatus = (typeof RESULT_STATUSES)[number]

// EVENT_TYPES 是 event 消息允许的子类型：
//   choice_required 目标第一次调用返回明确选择错误（isSuccess=false 且 errorType 属于三种选择错误）；
//   request_traced 一笔目标请求已完成转发的只读摘要（正文走 request_forward，这里只供完成检测计数）；
//   page_error 页面异常；business_done 业务完成事件；log 阶段日志。
export const EVENT_TYPES = ['choice_required', 'request_traced', 'page_error', 'business_done', 'log'] as const

export type EventType = (typeof EVENT_TYPES)[number]

// MessageEnvelope 是全部消息的公共信封；payload 形状由 kind 决定。
export interface MessageEnvelope {
  version: string
  kind: MessageKind
  requestId: string
  sessionId: string
  runId: string
  pathRunId: string
  stepId: string
  payload?: unknown
}

// ExecutePayload 是 execute 命令的载荷：action 必填，其余字段按动作取用（T008-T010 逐动作落地）。
// branchSelections：分支节点 ID → 选中分支入口节点代理 ID（路径已保存的选择，键值均为目标节点代理 ID）。
// nextNodeAuditors：本次动作会跨过的 run_node_choose 节点人员（nodeProxyId/bizId/name/auditDetailType=personnel）。
export interface ExecutePayload {
  action: RuntimeAction
  loadMode?: string
  instanceId?: string
  jobTaskId?: string
  nodeProxyId?: string
  toolFlowName?: string
  auditDesc?: string
  formValues?: Record<string, unknown>
  fieldPermissions?: unknown
  branchSelections?: Record<string, string>
  nextNodeAuditors?: NextAuditorEntry[]
  choice?: ChoicePayload
}

// NextAuditorEntry 与目标 FlowNodeAuditDetailConfigTemplateVo 对齐；
// auditDetailType 固定 personnel，绝不允许用姓名代替 bizId 或发送空人员。
export interface NextAuditorEntry {
  nodeProxyId: string
  bizId: string
  name: string
  auditDetailType: 'personnel'
}

// ChoicePayload 是 choice_continue 的载荷：runtime 把它写入目标组件上下文后调用原继续方法。
export interface ChoicePayload {
  branchEntryNodeProxyIds?: string[]
  nextAuditors?: NextAuditorEntry[]
}

// RequestForwardPayload 是 runtime 拦截到目标请求后发往 Go 的完整事实（不脱敏）。
export interface RequestForwardPayload {
  method: string
  url: string
  headers: Record<string, string>
  body?: string
  action: RuntimeAction | ''
  writeSequence?: number
}

// RequestResponsePayload 是 Go 把目标响应回填给页面的载荷；runtime 用它完成 route 响应。
export interface RequestResponsePayload {
  httpStatus: number
  headers: Record<string, string>
  body?: string
}

// ResultPayload 是动作终局消息：status 固定七值；requestTrace/表单前后值/摘要/异常原样携带。
export interface ResultPayload {
  status: ResultStatus
  message?: string
  requestTrace?: Array<Record<string, unknown>>
  formValuesBefore?: Record<string, unknown>
  formValuesAfter?: Record<string, unknown>
  businessSummary?: Record<string, unknown>
  errors?: Array<Record<string, unknown>>
}

// EventPayload 是 event 消息载荷；type 固定五种，数据按子类型取用。
export interface EventPayload {
  type: EventType
  data?: Record<string, unknown>
}

// ProtocolError 是协议校验失败的稳定错误；code 供 Go 与 runtime 的测试共同断言。
export class ProtocolError extends Error {
  code: string

  constructor (code: string, message: string) {
    super(message)
    this.name = 'ProtocolError'
    this.code = code
  }
}

// isNonEmptyString 只接受非空字符串编号；数字、空白串一律视为缺失。
function isNonEmptyString (value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0
}

// validateMessage 校验信封与 kind 相关规则；返回值为具体类型（收窄后的信封），失败抛 ProtocolError。
// 规则顺序固定：先版本（旧版本绝不静默兼容），再 kind，再公共编号，最后 kind 专属载荷。
export function validateMessage (value: unknown): MessageEnvelope {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new ProtocolError('message_not_object', '协议消息必须是 JSON 对象')
  }
  const message = value as Record<string, unknown>
  if (message.version !== PROTOCOL_VERSION) {
    throw new ProtocolError('version_rejected', `协议版本必须是 ${PROTOCOL_VERSION}，收到 ${String(message.version)}`)
  }
  const kind = message.kind
  if (typeof kind !== 'string' || !(kind in MESSAGE_KINDS)) {
    throw new ProtocolError('kind_unknown', `未知消息类型：${String(kind)}`)
  }
  for (const field of ['requestId', 'runId', 'pathRunId', 'stepId'] as const) {
    if (!isNonEmptyString(message[field])) {
      throw new ProtocolError('id_missing', `消息缺少 ${field}`)
    }
  }
  if (kind !== 'create' && !isNonEmptyString(message.sessionId)) {
    throw new ProtocolError('session_missing', `${kind} 消息缺少 sessionId`)
  }
  if (kind === 'load') {
    const payload = message.payload as Record<string, unknown> | undefined
    if (!payload || typeof payload !== 'object') {
      throw new ProtocolError('load_payload_invalid', 'load 载荷必须是 JSON 对象')
    }
    if (Object.prototype.hasOwnProperty.call(payload, 'template')) {
      throw new ProtocolError('template_forbidden', 'exec-engine/v2 装载不得携带 Go 渲染模板')
    }
    const mode = payload.mode
    if (mode !== 'initiate' && mode !== 'audit' && mode !== 'edit' && mode !== 'view' && mode !== 'approve' && mode !== 'forward') {
      throw new ProtocolError('load_mode_invalid', `装载模式不支持：${String(mode)}`)
    }
  }
  if (kind === 'execute') {
    const payload = message.payload as ExecutePayload | undefined
    const action = payload?.action
    if (typeof action !== 'string' || !(RUNTIME_ACTIONS as readonly string[]).includes(action)) {
      throw new ProtocolError('action_unknown', `未知 runtime 动作：${String(action)}`)
    }
  }
  if (kind === 'result') {
    const payload = message.payload as ResultPayload | undefined
    if (!payload || typeof payload.status !== 'string' || !(RESULT_STATUSES as readonly string[]).includes(payload.status)) {
      throw new ProtocolError('status_invalid', `结果状态必须是固定七值之一，收到 ${String(payload?.status)}`)
    }
  }
  if (kind === 'event') {
    const payload = message.payload as EventPayload | undefined
    if (!payload || typeof payload.type !== 'string' || !(EVENT_TYPES as readonly string[]).includes(payload.type)) {
      throw new ProtocolError('event_type_invalid', `事件子类型必须是 ${EVENT_TYPES.join('/')}`)
    }
  }
  return message as unknown as MessageEnvelope
}

// encodeEnvelope 构造发给 Go 的协议信封；调用方保证 payload 形状。
export function encodeEnvelope (kind: MessageKind, session: { sessionId: string, runId: string, pathRunId: string, stepId: string }, requestId: string, payload?: unknown): MessageEnvelope {
  return {
    version: PROTOCOL_VERSION,
    kind,
    requestId,
    sessionId: session.sessionId,
    runId: session.runId,
    pathRunId: session.pathRunId,
    stepId: session.stepId,
    payload
  }
}
