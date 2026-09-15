// T003 会话管理器验证：真实启动无头 Chromium，逐条核对三个验收点——
// ① 两个会话的 localStorage、sessionStorage、Vuex 挂载点与页面窗口状态互相隔离；
// ② 会话关闭后不能再执行新命令；
// ③ 页面崩溃会广播事件并释放会话。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { launchHeadlessChromium } from '../../../form-runtime/executor/src/browser.ts'
import { SESSION_EVENTS, SessionClosedError, SessionManager } from '../../../form-runtime/executor/src/session-manager.ts'
import { serveDirectory } from '../../../form-runtime/executor/src/static-origin.ts'

// serveIsolationOrigin 为两个会话提供同一个 HTTP 源的最小页面：
// 只有同源才能证明「隔离来自独立 BrowserContext，而不是不同源天然隔离」。
async function serveIsolationOrigin () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'session-isolation-'))
  fs.writeFileSync(path.join(dir, 'index.html'), '<!doctype html><title>t003</title>')
  const origin = await serveDirectory(dir)
  return {
    async open (page: import('playwright').Page) {
      await page.goto(new URL('index.html', origin.url).toString(), { waitUntil: 'load' })
    },
    async close () {
      await origin.close().catch(() => undefined)
      fs.rmSync(dir, { recursive: true, force: true })
    }
  }
}

test('T003 两个会话状态完全隔离，关闭后释放句柄', async () => {
  const browser = await launchHeadlessChromium()
  const isolation = await serveIsolationOrigin()
  try {
    const manager = new SessionManager(browser)
    const created: string[] = []
    manager.events.on(SESSION_EVENTS.created, session => created.push(session.sessionId))
    const first = await manager.create('account-A')
    const second = await manager.create('account-B')
    assert.equal(first.status, 'ready')
    assert.equal(second.status, 'ready')
    assert.deepEqual(created.sort(), [first.sessionId, second.sessionId].sort())

    // 同一 HTTP 源、不同 Context：各自写入 storage 与窗口状态后必须互不可见。
    await isolation.open(first.page!)
    await isolation.open(second.page!)
    await first.page!.evaluate(() => {
      localStorage.setItem('probe-local', 'from-A')
      sessionStorage.setItem('probe-session', 'from-A')
      ;(window as any).__sessionMarker = 'A'
      ;(window as any).__vuexMountPoint = { state: { account: 'A' } }
    })
    const seenBySecond = await second.page!.evaluate(() => ({
      local: localStorage.getItem('probe-local'),
      session: sessionStorage.getItem('probe-session'),
      marker: (window as any).__sessionMarker ?? null,
      vuex: (window as any).__vuexMountPoint ?? null
    }))
    assert.deepEqual(seenBySecond, { local: null, session: null, marker: null, vuex: null })
    const stillInFirst = await first.page!.evaluate(() => ({
      local: localStorage.getItem('probe-local'),
      marker: (window as any).__sessionMarker
    }))
    assert.deepEqual(stillInFirst, { local: 'from-A', marker: 'A' })

    // 关闭第一个会话：句柄清空、状态落地，第二个会话不受影响。
    let closedEvent: string | null = null
    manager.events.on(SESSION_EVENTS.closed, session => { closedEvent = session.sessionId })
    await manager.close(first.sessionId, 'destroyed')
    assert.equal(first.status, 'closed')
    assert.equal(first.page, null)
    assert.equal(first.context, null)
    assert.ok(first.closeReason!.includes('destroyed'))
    assert.equal(closedEvent, first.sessionId)
    assert.equal(manager.get(second.sessionId)!.status, 'ready')
    await manager.closeAll()
    assert.equal(manager.list().every(session => session.status === 'closed'), true)
  } finally {
    await browser.close().catch(() => undefined)
    await isolation.close()
  }
})

test('T003 已关闭会话拒绝执行新命令', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const session = await manager.create('account-closed')
    await manager.close(session.sessionId)
    assert.throws(() => manager.assertOpen(session.sessionId), (error: unknown) => error instanceof SessionClosedError)
    assert.throws(() => manager.assertOpen('not-exists'), (error: unknown) => error instanceof SessionClosedError)
    // 同账号在会话关闭后允许创建新 Context（崩溃/结束后的下一次动作走全新现场）。
    const next = await manager.create('account-closed')
    assert.equal(next.status, 'ready')
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})

test('T003 同账号并发创建被拒绝（同账号串行的纵深防御）', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const session = await manager.create('account-serial')
    await assert.rejects(
      () => manager.create('account-serial'),
      (error: Error) => error.message.includes('串行排队')
    )
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})

test('T003 页面崩溃广播事件并释放会话', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const session = await manager.create('account-crash')
    let crashedWith: string | null = null
    let closedAfterCrash = false
    manager.events.on(SESSION_EVENTS.crashed, event => { crashedWith = event.sessionId })
    manager.events.on(SESSION_EVENTS.closed, event => {
      if (event.sessionId === session.sessionId) closedAfterCrash = true
    })
    // Playwright Page 继承事件源：直接发出 crash 事件等价于浏览器进程崩溃通知，
    // 管理器必须捕获并释放，不能留下半开的 Context。
    session.page!.emit('crash', session.page)
    await new Promise(resolve => setImmediate(resolve))
    assert.equal(session.status, 'closed')
    assert.equal(session.page, null)
    assert.equal(crashedWith, session.sessionId)
    assert.equal(closedAfterCrash, true)
    assert.throws(() => manager.assertOpen(session.sessionId), (error: unknown) => error instanceof SessionClosedError)
    // 崩溃释放后同账号允许新会话，供 Go 决定是否重试或置为结果待确认。
    const replacement = await manager.create('account-crash')
    assert.equal(replacement.status, 'ready')
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})
