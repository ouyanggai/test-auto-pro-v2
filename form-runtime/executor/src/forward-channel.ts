// T007 转发通道：把页面拦截到的目标请求以 request_forward 消息发给 Go，并等待配对的 request_response。
// 每个会话一条 WebSocket；Go 未连接或响应超时都视为转发失败，路由层据此本地中止请求，
// 请求绝不会被直接发往目标平台。
import { randomUUID } from 'node:crypto'
import type { WebSocket } from 'ws'
import { PROTOCOL_VERSION } from './protocol.ts'

export interface ForwardOutcome {
  httpStatus: number
  headers: Record<string, string>
  bodyEncoding: 'utf8' | 'base64'
  body?: string
}

// ForwardFailure 是 Go 无法转发（断连、路径拒绝、目标传输失败）的稳定错误；路由层只关心失败这一事实。
export class ForwardFailure extends Error {}

interface PendingForward {
  resolve: (outcome: ForwardOutcome) => void
  reject: (error: ForwardFailure) => void
  timer: NodeJS.Timeout
}

export class ForwardChannel {
  private socket: WebSocket | null = null
  private readonly pending = new Map<string, PendingForward>()

  // attach 在 Go 的 WebSocket 接入时调用；同一会话同时只允许一条连接。
  attach (socket: WebSocket): void {
    this.detachAll('Go 通道被新连接替换')
    this.socket = socket
  }

  get connected (): boolean {
    return this.socket !== null && this.socket.readyState === this.socket.OPEN
  }

  // socketForEvents 供会话层在通道上主动推送事件（页面异常等）；未连接时返回 null。
  get socketForEvents (): WebSocket | null {
    return this.connected ? this.socket : null
  }

  // send 发送 request_forward 并等待配对响应；httpStatus=0 表示 Go 侧转发失败。
  // 通道未连接时先等待短暂宽限：装载先于 Go 的命令到达，页面装载期读取必须等
  // Go 预热的通道接上，不能立即按失败中止（2026-09-16 验收实测表单装载不完的根因）。
  async send (envelope: {
    sessionId: string
    runId: string
    pathRunId: string
    stepId: string
    payload: Record<string, unknown>
  }, timeoutMs = 120000, connectGraceMs = 15000): Promise<ForwardOutcome> {
    if (!this.connected) {
      const startedAt = Date.now()
      while (!this.connected && Date.now() - startedAt < connectGraceMs) {
        await new Promise(resolve => setTimeout(resolve, 50))
      }
    }
    if (!this.connected) throw new ForwardFailure('Go 通道未连接，目标请求已在本地中止')
    const requestId = randomUUID()
    const message = {
      version: PROTOCOL_VERSION,
      kind: 'request_forward',
      requestId,
      sessionId: envelope.sessionId,
      runId: envelope.runId,
      pathRunId: envelope.pathRunId,
      stepId: envelope.stepId,
      payload: envelope.payload
    }
    const promise = new Promise<ForwardOutcome>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(requestId)
        reject(new ForwardFailure(`Go 转发响应超时（${timeoutMs}ms）`))
      }, timeoutMs)
      this.pending.set(requestId, { resolve, reject, timer })
    })
    this.socket!.send(JSON.stringify(message))
    return promise
  }

  // resolve 处理 Go 回传的 request_response 载荷；httpStatus=0 转成失败。
  resolve (requestId: string, payload: { httpStatus?: number; headers?: Record<string, string>; body?: string; bodyEncoding?: 'utf8' | 'base64' }): void {
    const pending = this.pending.get(requestId)
    if (!pending) return
    this.pending.delete(requestId)
    clearTimeout(pending.timer)
    const status = Number(payload.httpStatus ?? 0)
    if (!status) {
      pending.reject(new ForwardFailure('Go 侧目标请求转发失败'))
      return
    }
    pending.resolve({
      httpStatus: status,
      headers: payload.headers ?? {},
      bodyEncoding: payload.bodyEncoding ?? 'utf8',
      body: payload.body
    })
  }

  // detachAll 在断连/替换连接时让全部在途转发失败；路由层会把对应请求本地中止。
  detachAll (reason: string): void {
    this.socket = null
    for (const [requestId, pending] of this.pending) {
      clearTimeout(pending.timer)
      pending.reject(new ForwardFailure(reason))
      this.pending.delete(requestId)
    }
  }
}
