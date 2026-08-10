export const FORM_RUNTIME_VERSION = 'f007-form-runtime/v1'

const COMMAND_TYPES = new Set(['load', 'setData', 'refresh', 'getValues', 'validateAndGetValues', 'restore', 'destroy'])

// isRuntimeCommand 严格校验协议版本、会话和请求标识，旧 iframe 消息不会污染当前表单。
export function isRuntimeCommand (value) {
  return Boolean(value && typeof value === 'object'
    && value.version === FORM_RUNTIME_VERSION
    && typeof value.sessionId === 'string' && value.sessionId
    && typeof value.requestId === 'string' && value.requestId
    && COMMAND_TYPES.has(value.type))
}
