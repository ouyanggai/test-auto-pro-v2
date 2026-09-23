// BrowserContext 会话管理器（T003）：每个节点动作一个全新 Context，关闭时移除监听器并关闭 Context。
// 同账号排队由 Go 实现；这里作为纵深防御，同账号已存在未关闭会话时拒绝再创建。
import { randomUUID } from 'node:crypto'
import { EventEmitter } from 'node:events'
import type { Browser } from 'playwright'
import type { CloseReason, ManagedSession } from './types.ts'

export interface SessionManagerOptions {
  leaseMs?: number
  now?: () => number
  maxAccountConcurrency?: number
}

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
  private readonly leaseMs: number
  private readonly maxAccountConcurrency: number
  private readonly now: () => number
  readonly events = new EventEmitter()

  constructor (browser: Browser, options: SessionManagerOptions = {}) {
    this.browser = browser
    const configuredLeaseMs = options.leaseMs ?? Number(process.env.EXECUTOR_SESSION_LEASE_MS || 90000)
    this.leaseMs = Number.isFinite(configuredLeaseMs) && configuredLeaseMs > 0 ? configuredLeaseMs : 90000
    const configuredAccountConcurrency = options.maxAccountConcurrency ?? Number(process.env.EXECUTOR_ACCOUNT_CONCURRENCY || 20)
    this.maxAccountConcurrency = Number.isFinite(configuredAccountConcurrency) && configuredAccountConcurrency > 0 ? configuredAccountConcurrency : 20
    this.now = options.now ?? Date.now
  }

  // create 为一次节点动作创建全新 BrowserContext 与页面。
  async create (accountKey: string): Promise<ManagedSession> {
    const key = accountKey.trim()
    if (!key) throw new Error('创建会话缺少账号唯一键')
    // 创建前同步回收已过期租约，避免定时器尚未 tick 时旧会话仍占用配额。
    this.reapExpired()
    let activeCountForAccount = 0
    for (const existing of this.sessions.values()) {
      if (existing.accountKey === key && (existing.status === 'creating' || existing.status === 'ready')) {
        activeCountForAccount++
      }
    }
    if (activeCountForAccount >= this.maxAccountConcurrency) {
      throw new Error(`账号 ${key} 当前活跃会话数已达上限（${activeCountForAccount}/${this.maxAccountConcurrency}），请等待前序会话释放`)
    }
    const now = this.now()
    const session: ManagedSession = {
      sessionId: randomUUID(),
      accountKey: key,
      createdAt: new Date().toISOString(),
      lastHeartbeatAt: now,
      leaseExpiresAt: now + this.leaseMs,
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

  // heartbeat 续期由 Go 运行进程定期调用；进程消失后租约自然过期，不能靠旧进程主动 close。
  heartbeat (sessionId: string): number {
    const session = this.assertOpen(sessionId)
    const now = this.now()
    session.lastHeartbeatAt = now
    session.leaseExpiresAt = now + this.leaseMs
    return session.leaseExpiresAt
  }

  // reapExpired 只回收租约已经过期的活动会话，绝不抢占仍在续租的同账号会话。
  reapExpired (now = this.now()): string[] {
    const expired: string[] = []
    for (const session of this.sessions.values()) {
      if ((session.status === 'creating' || session.status === 'ready') && session.leaseExpiresAt <= now) {
        expired.push(session.sessionId)
        void this.release(session, 'lease_expired', `租约已过期（最后心跳 ${session.lastHeartbeatAt}）`).catch(() => undefined)
      }
    }
    return expired
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
