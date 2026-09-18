// 转发通道：页面拦截到的目标请求以 request_forward 发给 Go，等待配对 request_response。
// T005：通道未连接不得导航，也不得等待宽限后才失败。

import { randomUUID } from 'node:crypto'
import type { WebSocket } from 'ws'
import { PROTOCOL_VERSION } from './protocol.ts'

export interface ForwardOutcome {
  httpStatus: number
  headers: Record<string, string>
  bodyEncoding: 'utf8' | 'base64'
  body?: string
}

// ForwardChannelStats 是通道健康状况的可机读事实（F-036 门禁 G5）。
// 两个计数都必须能被测试与运行详情直接读取，而不是只留一句日志：
// 「通道被新连接替换」与「在途转发被本地中止」是两类不同的可用性事件，
// 前者说明有并发连接争用（旧实现会因此在途请求变成本地 abort），
// 后者说明已经有请求真的没发出去。二者为 0 才是「通道复用正常」的证据。
export interface ForwardChannelStats {
  // attachRejected 是被拒绝的新连接数（同一会话已有活动连接时要求新连接等待）。
  attachRejected: number
  // abortedInFlight 是因断连/替换而本地中止的在途转发数。
  abortedInFlight: number
  // pending 是当前仍在等待 Go 响应的转发数。
  pending: number
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
  // G5 计数：只增不减，供运行期间与运行结束后的门禁断言读取。
  private attachRejectedCount = 0
  private abortedInFlightCount = 0

  // attach 在 Go 的 WebSocket 接入时调用；同一会话同时只允许一条连接。
  attach (socket: WebSocket): boolean {
    if (this.socket && this.socket.readyState === this.socket.OPEN) {
      // 预热连接可能正在承载页面读取；替换它会让在途请求变成本地 abort，
      // 进而把可重试的连接问题伪装成写结果不明。新连接必须等待旧连接自然释放。
      this.attachRejectedCount++
      socket.close(1013, '已有活动通道')
      return false
    }
    this.socket = socket
    return true
  }

  // stats 返回通道健康事实；未接日志/未连接时也始终可读。
  stats (): ForwardChannelStats {
    return {
      attachRejected: this.attachRejectedCount,
      abortedInFlight: this.abortedInFlightCount,
      pending: this.pending.size
    }
  }

  get connected (): boolean {
    return this.socket !== null && this.socket.readyState === this.socket.OPEN
  }

  // socketForEvents 供会话层在通道上主动推送事件（页面异常等）；未连接时返回 null。
  get socketForEvents (): WebSocket | null {
    return this.connected ? this.socket : null
  }

  // send 发送 request_forward 并等待配对响应。通道未连接立即失败，调用方不得 page.goto。
  async send (envelope: {
    sessionId: string
    runId: string
    pathRunId: string
    stepId: string
    payload: Record<string, unknown>
  }, timeoutMs = 120000): Promise<ForwardOutcome> {
    if (!this.connected) throw new ForwardFailure('Go 通道未连接，禁止导航或转发目标请求')

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
    const { promise, resolve, reject } = Promise.withResolvers<ForwardOutcome>()
    const timer = setTimeout(() => {
      this.pending.delete(requestId)
      reject(new ForwardFailure(`Go 转发响应超时（${timeoutMs}ms）`))
    }, timeoutMs)
    this.pending.set(requestId, { resolve, reject, timer })
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
      // 本次中止是真实发生的可用性事件：计数后门禁才能断言「运行期间为零」。
      this.abortedInFlightCount++
      pending.reject(new ForwardFailure(reason))
      this.pending.delete(requestId)
    }
  }

  // detach 只允许当前连接清理会话；旧连接的 close 事件不能误伤后来建立的连接。
  detach (socket: WebSocket, reason: string): void {
    if (this.socket !== socket) return
    this.detachAll(reason)
  }
}
