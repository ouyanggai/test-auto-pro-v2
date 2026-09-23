import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import type { Page } from 'playwright'

import { launchHeadlessChromium } from '../../../executor-service/src/browser.ts'
import { SESSION_EVENTS, SessionClosedError, SessionManager } from '../../../executor-service/src/session-manager.ts'
import { serveDirectory } from '../../../executor-service/src/static-origin.ts'

async function serveIsolationOrigin () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'session-isolation-'))
  fs.writeFileSync(path.join(dir, 'index.html'), '<!doctype html><title>t003</title>')
  const origin = await serveDirectory(dir)
  return {
    async open (page: Page) {
      await page.goto(new URL('index.html', origin.url).toString(), { waitUntil: 'load' })
    },
    async close () {
      await origin.close().catch(() => undefined)
      fs.rmSync(dir, { recursive: true, force: true })
    }
  }
}


test('两个会话状态完全隔离，关闭后释放句柄', async () => {
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
    await isolation.open(first.page!)
    await isolation.open(second.page!)
    await first.page!.evaluate(() => {
      localStorage.setItem('probe-local', 'from-A')
      sessionStorage.setItem('probe-session', 'from-A')
      Reflect.set(window, '__sessionMarker', 'A')
      Reflect.set(window, '__vuexMountPoint', { state: { account: 'A' } })
    })
    const seenBySecond = await second.page!.evaluate(() => ({
      local: localStorage.getItem('probe-local'),
      session: sessionStorage.getItem('probe-session'),
      marker: Reflect.get(window, '__sessionMarker') ?? null,
      vuex: Reflect.get(window, '__vuexMountPoint') ?? null
    }))

    assert.deepEqual(seenBySecond, { local: null, session: null, marker: null, vuex: null })
    let closedEvent: string | null = null
    manager.events.on(SESSION_EVENTS.closed, session => { closedEvent = session.sessionId })
    await manager.close(first.sessionId, 'destroyed')
    assert.equal(first.status, 'closed')
    assert.equal(first.page, null)
    assert.equal(closedEvent, first.sessionId)
    assert.equal(manager.get(second.sessionId)!.status, 'ready')
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
    await isolation.close()
  }
})

test('已关闭会话拒绝执行新命令', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const session = await manager.create('account-closed')
    await manager.close(session.sessionId)
    assert.throws(() => manager.assertOpen(session.sessionId), (error: unknown) => error instanceof SessionClosedError)
    const next = await manager.create('account-closed')
    assert.equal(next.status, 'ready')
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})

test('同账号支持并发创建独立会话，达到并发上限后被拒绝', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser, { maxAccountConcurrency: 2 })
    const first = await manager.create('account-parallel')
    const second = await manager.create('account-parallel')
    assert.equal(first.status, 'ready')
    assert.equal(second.status, 'ready')
    assert.notEqual(first.sessionId, second.sessionId)
    await assert.rejects(
      () => manager.create('account-parallel'),
      (error: Error) => error.message.includes('当前活跃会话数已达上限')
    )
    await manager.close(first.sessionId)
    const third = await manager.create('account-parallel')
    assert.equal(third.status, 'ready')
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})

test('页面崩溃广播事件并释放会话', async () => {
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const session = await manager.create('account-crash')
    let crashedWith: string | null = null
    manager.events.on(SESSION_EVENTS.crashed, event => { crashedWith = event.sessionId })
    session.page!.emit('crash', session.page)
    const { promise: drained, resolve } = Promise.withResolvers<void>()
    setImmediate(resolve)
    await drained
    assert.equal(session.status, 'closed')
    assert.equal(session.page, null)
    assert.equal(crashedWith, session.sessionId)
    assert.throws(() => manager.assertOpen(session.sessionId), (error: unknown) => error instanceof SessionClosedError)
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})

test('租约心跳保活，过期后只回收孤儿会话并释放同账号门禁配额', async () => {
  const browser = await launchHeadlessChromium()
  let now = 1000
  try {
    const manager = new SessionManager(browser, { leaseMs: 10, now: () => now, maxAccountConcurrency: 1 })
    const first = await manager.create('account-lease')
    now = 1009
    manager.reapExpired()
    await assert.rejects(() => manager.create('account-lease'), (error: Error) => error.message.includes('当前活跃会话数已达上限'))
    manager.heartbeat(first.sessionId)
    now = 1018
    manager.reapExpired()
    await assert.rejects(() => manager.create('account-lease'), (error: Error) => error.message.includes('当前活跃会话数已达上限'))
    now = 1029
    manager.reapExpired()
    assert.equal(first.status, 'closed')
    const next = await manager.create('account-lease')
    assert.notEqual(next.sessionId, first.sessionId)
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
  }
})
