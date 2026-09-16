// T006 请求轨迹：按发出顺序记录每笔被拦截的目标请求与其响应/中止结果。
// 完成检测（T011）与运行详情（T013）都从这里取事实；顺序可回放是硬要求：
// seq 按请求出现顺序递增，responseSeq 按响应完成顺序递增，二者独立计数。
export interface TraceResponse {
  responseSeq: number
  httpStatus: number
  headers: Record<string, string>
  bodyEncoding?: 'utf8' | 'base64'
  body?: string
  completedAt: string
  durationMs: number
}

export interface TraceEntry {
  seq: number
  method: string
  url: string
  headers: Record<string, string>
  bodyEncoding?: 'utf8' | 'base64'
  body?: string
  action: string
  writeSequence?: number
  requestAt: string
  // state：forwarding 已转发 Go 等待响应；fulfilled 已回填页面；aborted 拒绝发出（Go 不可达或未登记请求）。
  state: 'forwarding' | 'fulfilled' | 'aborted' | 'blocked'
  abortReason?: string
  response?: TraceResponse
}

export class RequestTraceRecorder {
  private readonly entries: TraceEntry[] = []
  private nextSeq = 1
  private nextResponseSeq = 1

  // begin 登记一笔刚拦截到的请求；调用即分配发出顺序号。
  begin (input: Pick<TraceEntry, 'method' | 'url' | 'headers' | 'action'> & Partial<Pick<TraceEntry, 'body' | 'bodyEncoding' | 'writeSequence'>>): TraceEntry {
    const entry: TraceEntry = {
      ...input,
      seq: this.nextSeq++,
      requestAt: new Date().toISOString(),
      state: 'forwarding'
    }
    this.entries.push(entry)
    return entry
  }

  // markFulfilled 记录 Go 响应已回填页面；完成顺序号独立于请求顺序号。
  markFulfilled (entry: TraceEntry, response: Omit<TraceResponse, 'responseSeq'>): void {
    entry.state = 'fulfilled'
    entry.response = { ...response, responseSeq: this.nextResponseSeq++ }
  }

  // markAborted 记录请求未发往目标（转发失败）或被本地拒绝（未登记请求）。
  markAborted (entry: TraceEntry, state: 'aborted' | 'blocked', reason: string): void {
    entry.state = state
    entry.abortReason = reason
  }

  // list 返回按发出顺序排列的完整轨迹快照；响应内含 responseSeq 可重放完成顺序。
  list (): TraceEntry[] {
    return this.entries.map(entry => ({ ...entry }))
  }

  // stats 返回计数摘要，供完成检测判断在途请求是否归零。
  stats (): { total: number; forwarding: number; fulfilled: number; aborted: number; blocked: number } {
    const stats = { total: this.entries.length, forwarding: 0, fulfilled: 0, aborted: 0, blocked: 0 }
    for (const entry of this.entries) stats[entry.state]++
    return stats
  }

  // lastSeq 返回当前已分配的最大序列号；用于完成检测的基线记录。
  get lastSeq (): number {
    return this.nextSeq - 1
  }
}
