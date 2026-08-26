export const FORM_RUNTIME_VERSION = 'f007-form-runtime/v1'

const COMMAND_TYPES = new Set(['load', 'setData', 'refresh', 'getValues', 'validateAndGetValues', 'restore', 'destroy'])
const RESPONSE_TYPES = new Set(['state', 'result', 'error'])

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
