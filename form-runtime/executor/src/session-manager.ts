// BrowserContext 会话管理器（T003）：每个节点动作一个全新 Context，关闭时移除监听器并关闭 Context。
// 同账号排队由 Go 实现，这里不调度任务；但作为纵深防御，同账号已存在未关闭会话时拒绝再创建，
// 避免 Go 侧缺陷造成同账号双 Context 串线。
import { randomUUID } from 'node:crypto'
import { EventEmitter } from 'node:events'
import type { Browser } from 'playwright'
import type { CloseReason, ManagedSession } from './types.ts'

// SESSION_EVENTS 是管理器对外广播的事件名：created 在页面就绪前触发（供服务层先装拦截与桥接），
// crashed 必须被服务层转发给 Go（页面崩溃后本会话立即不可用）。
export const SESSION_EVENTS = {
  created: 'created',
  closed: 'closed',
  crashed: 'crashed'
} as const

export class SessionClosedError extends Error {
  // SessionClosedError 表示命令落在已关闭/已崩溃会话上；调用方（服务层）必须把它原样回给 Go。
  constructor (sessionId: string, status: string) {
    super(`会话 ${sessionId} 已处于 ${status} 状态，不能执行新命令`)
    this.name = 'SessionClosedError'
  }
}

export class SessionManager {
  private readonly browser: Browser
  private readonly sessions = new Map<string, ManagedSession>()
  // events 向服务层广播会话创建、关闭与崩溃；服务层据此安装拦截器、通知 Go 并落运行日志。
  readonly events = new EventEmitter()

  constructor (browser: Browser) {
    this.browser = browser
  }

  // create 为一次节点动作创建全新 BrowserContext 与页面。
  // 输入边界：accountKey 必须是非空账号唯一键；同账号存在未关闭会话时抛错（Go 负责排队，这里只兜底）。
  async create (accountKey: string): Promise<ManagedSession> {
    const key = accountKey.trim()
    if (!key) throw new Error('创建会话缺少账号唯一键')
    for (const existing of this.sessions.values()) {
      if (existing.accountKey === key && (existing.status === 'creating' || existing.status === 'ready')) {
        throw new Error(`账号 ${key} 已有未关闭会话 ${existing.sessionId}，同一账号必须由 Go 串行排队执行`)
      }
    }
    const session: ManagedSession = {
      sessionId: randomUUID(),
      accountKey: key,
      createdAt: new Date().toISOString(),
      status: 'creating',
      context: null,
      page: null
    }
    this.sessions.set(session.sessionId, session)
    try {
      // 一次执行一个 Context：cookies、storage、Vuex 与请求监听都随 Context 一起生灭，绝不复用。
      const context = await this.browser.newContext()
      const page = await context.newPage()
      session.context = context
      session.page = page
      // 页面崩溃：立即置为 crashed、广播事件并释放 Context；此后任何命令都被拒绝。
      page.on('crash', () => {
        void this.handleCrash(session.sessionId)
      })
      session.status = 'ready'
      this.events.emit(SESSION_EVENTS.created, session)
      return session
    } catch (error) {
      // Context 创建失败必须释放登记并原样上抛：调用方据此写入运行日志，不产生半开会话。
      await this.release(session, 'superseded', `创建失败：${String((error as Error)?.message ?? error)}`).catch(() => undefined)
      throw error
    }
  }

  // get 按编号查询会话；不存在时返回 undefined。
  get (sessionId: string): ManagedSession | undefined {
    return this.sessions.get(sessionId)
  }

  // list 返回全部会话快照（含已关闭），供状态查询与日志核对。
  list (): ManagedSession[] {
    return [...this.sessions.values()]
  }

  // assertOpen 在执行任何命令前校验会话可用；关闭/崩溃会话一律拒绝，绝不复活。
  assertOpen (sessionId: string): ManagedSession {
    const session = this.sessions.get(sessionId)
    if (!session) throw new SessionClosedError(sessionId, '不存在')
    if (session.status !== 'ready') throw new SessionClosedError(sessionId, session.status)
    return session
  }

  // close 主动关闭会话：先移除页面监听器，再关闭 Context，最后更新状态并广播。
  async close (sessionId: string, reason: CloseReason = 'destroyed'): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session) return
    await this.release(session, reason, reason)
  }

  // closeAll 在服务停止时释放全部会话；单个失败不阻断其余清理。
  async closeAll (reason: CloseReason = 'shutdown'): Promise<void> {
    for (const session of [...this.sessions.values()]) {
      await this.close(session.sessionId, reason).catch(() => undefined)
    }
  }

  // handleCrash 处理页面崩溃：状态置为 crashed、广播 crashed 事件（服务层转发给 Go）并释放 Context。
  private async handleCrash (sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session || session.status !== 'ready') return
    session.status = 'crashed'
    this.events.emit(SESSION_EVENTS.crashed, session)
    await this.release(session, 'crashed', '页面崩溃').catch(() => undefined)
  }

  // release 是唯一的落地路径：移除监听器、关闭 Context、清空句柄并广播 closed。
  // 状态已是 closed 时跳过，保证 close 与 crash 竞争时只落地一次；
  // 只有创建成功宣告过的会话才广播 closed，创建失败的半开登记静默清理。
  private async release (session: ManagedSession, reason: CloseReason, message: string): Promise<void> {
    if (session.status === 'closed') return
    const announced = session.status === 'ready' || session.status === 'crashed'
    session.status = 'closed'
    session.closedAt = new Date().toISOString()
    session.closeReason = `${reason}：${message}`
    const { context, page } = session
    session.page = null
    session.context = null
    if (page) page.removeAllListeners('crash')
    if (announced) this.events.emit(SESSION_EVENTS.closed, session)
    if (context) await context.close().catch(() => undefined)
  }
}
