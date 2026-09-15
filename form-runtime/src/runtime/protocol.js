export const FORM_RUNTIME_VERSION = 'f007-form-runtime/v1'

const COMMAND_TYPES = new Set(['load', 'setData', 'refresh', 'getValues', 'validateAndGetValues', 'restore', 'destroy', 'cancel'])
const RESPONSE_TYPES = new Set(['state', 'result', 'error'])

// ===== 执行模式（执行引擎改造，docs/s）协议口径 =====
// 与执行服务协议（form-runtime/executor/src/protocol.ts、internal/runtimeexec/protocol.go）逐字镜像：
// 版本不一致或运行编号缺失的命令直接拒绝，不静默兼容。
export const EXECUTION_PROTOCOL_VERSION = 'exec-engine/v1'

// EXECUTION_ACTIONS 是 execute 命令允许的动作全集；Go 直连动作（add_sign 等）绝不允许出现。
export const EXECUTION_ACTIONS = Object.freeze([
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
])

// isExecutionCommand 校验执行命令信封：当前协议版本 + 四个运行编号非空；
// sessionId 由桥接按方法语义补充校验（首次 load 前允许为空，其余命令必须匹配当前会话）。
export function isExecutionCommand (value) {
  const ids = ['requestId', 'runId', 'pathRunId', 'stepId']
  return Boolean(value && typeof value === 'object'
    && value.version === EXECUTION_PROTOCOL_VERSION
    && ids.every(field => typeof value[field] === 'string' && value[field].trim())
    && (typeof value.sessionId === 'string' || value.sessionId === undefined))
}

// isRuntimeCommand 严格校验协议版本、会话和请求标识，旧 iframe 消息不会污染当前表单。
export function isRuntimeCommand (value) {
  return Boolean(value && typeof value === 'object'
    && value.version === FORM_RUNTIME_VERSION
    && typeof value.sessionId === 'string' && value.sessionId
    && typeof value.requestId === 'string' && value.requestId
    && COMMAND_TYPES.has(value.type))
}

// isRuntimeResponse 固定运行时输出的 boot、状态、结果与错误消息结构，父页仍需继续核对真实 origin、source 和当前待处理请求。
export function isRuntimeResponse (value) {
  if (!value || typeof value !== 'object' || value.version !== FORM_RUNTIME_VERSION) return false
  if (value.type === 'ready') return value.sessionId === '' && value.requestId === 'boot'
  return Boolean(typeof value.sessionId === 'string' && value.sessionId
    && typeof value.requestId === 'string' && value.requestId
    && RESPONSE_TYPES.has(value.type))
}
