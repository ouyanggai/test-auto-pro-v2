// BrowserContext 会话管理器（T003）：每个节点动作一个全新 Context，关闭时移除监听器并关闭 Context。
// 同账号排队由 Go 实现；这里作为纵深防御，同账号已存在未关闭会话时拒绝再创建。
import { randomUUID } from 'node:crypto'
import { EventEmitter } from 'node:events'
import type { Browser } from 'playwright'
import type { CloseReason, ManagedSession } from './types.ts'

export const SESSION_EVENTS = {
  created: 'created',
  closed: 'closed',
  crashed: 'crashed'
} as const

export class SessionClosedError extends Error {
  // SessionClosedError 表示命令落在已关闭/已崩溃会话上；调用方必须把它原样回给 Go。
  constructor (sessionId: string, status: string) {
    super(`会话 ${sessionId} 已处于 ${status} 状态，不能执行新命令`)
    this.name = 'SessionClosedError'
  }
}

export class SessionManager {
  private readonly browser: Browser
  private readonly sessions = new Map<string, ManagedSession>()
  readonly events = new EventEmitter()

  constructor (browser: Browser) {
    this.browser = browser
  }

  // create 为一次节点动作创建全新 BrowserContext 与页面。
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
      const context = await this.browser.newContext()
      const page = await context.newPage()
      session.context = context
      session.page = page
      page.on('crash', () => {
        void this.handleCrash(session.sessionId)
      })
      session.status = 'ready'
      this.events.emit(SESSION_EVENTS.created, session)
      return session
    } catch (error) {
      await this.release(session, 'superseded', `创建失败：${String((error as Error)?.message ?? error)}`).catch(() => undefined)
      throw error
    }
  }

  get (sessionId: string): ManagedSession | undefined {
    return this.sessions.get(sessionId)
  }

  list (): ManagedSession[] {
    return [...this.sessions.values()]
  }

  assertOpen (sessionId: string): ManagedSession {
    const session = this.sessions.get(sessionId)
    if (!session) throw new SessionClosedError(sessionId, '不存在')
    if (session.status !== 'ready') throw new SessionClosedError(sessionId, session.status)
    return session
  }

  async close (sessionId: string, reason: CloseReason = 'destroyed'): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session) return
    await this.release(session, reason, reason)
  }

  async closeAll (reason: CloseReason = 'shutdown'): Promise<void> {
    for (const session of [...this.sessions.values()]) {
      await this.close(session.sessionId, reason).catch(() => undefined)
    }
  }

  private async handleCrash (sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId)
    if (!session || session.status !== 'ready') return
    session.status = 'crashed'
    this.events.emit(SESSION_EVENTS.crashed, session)
    await this.release(session, 'crashed', '页面崩溃').catch(() => undefined)
  }

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
