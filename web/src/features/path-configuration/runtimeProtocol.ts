export const FORM_RUNTIME_VERSION = 'f007-form-runtime/v1'

export type RuntimeMessageDisposition = 'boot' | 'state' | 'result' | 'error' | 'ignore'

export interface RuntimeMessage {
  version?: string
  sessionId?: string
  requestId?: string
  type?: string
  payload?: Record<string, unknown>
}

export interface RuntimeMessageContext {
  sessionId: string
  pendingRequestIds: ReadonlySet<string>
  runtimeActive: boolean
  disposed: boolean
  bootPending: boolean
}

// classifyRuntimeMessage 固定父页对版本、会话、请求和迟到响应的判定顺序，来源校验仍由消息监听器使用真实窗口完成。
export function classifyRuntimeMessage(message: RuntimeMessage, context: RuntimeMessageContext): RuntimeMessageDisposition {
  if (message.version !== FORM_RUNTIME_VERSION) return 'ignore'
  if (message.type === 'ready' && message.requestId === 'boot') {
    return context.disposed || context.runtimeActive || !context.bootPending ? 'ignore' : 'boot'
  }
  if (message.sessionId !== context.sessionId || !message.requestId) return 'ignore'
  if (message.type === 'state') {
    return context.runtimeActive && !context.disposed ? 'state' : 'ignore'
  }
  if (!context.pendingRequestIds.has(message.requestId)) return 'ignore'
  if (message.type === 'error') return 'error'
  return message.type === 'result' ? 'result' : 'ignore'
}
