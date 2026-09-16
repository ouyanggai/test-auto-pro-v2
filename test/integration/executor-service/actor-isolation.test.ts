import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { launchHeadlessChromium } from '../../../executor-service/src/browser.ts'
import { requireNewContextOnActorChange, splitIdentities } from '../../../executor-service/src/native-detail.ts'
import { SessionManager } from '../../../executor-service/src/session-manager.ts'
import { serveDirectory } from '../../../executor-service/src/static-origin.ts'

test('当前审批人与原发起人分开保存', () => {
  const identities = splitIdentities({ userId: 'auditor-1', name: '审批人' }, { userId: 'initiator-1', name: '发起人' })
  assert.equal(identities.current.userId, 'auditor-1')
  assert.equal(identities.initiator.userId, 'initiator-1')
})

test('切换审批人必须使用新 Context', () => {
  assert.equal(requireNewContextOnActorChange('auditor-1', 'auditor-2'), true)
  assert.equal(requireNewContextOnActorChange('auditor-1', 'auditor-1'), false)
})

test('两个审批人依次执行时 storage 不串', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'actor-iso-'))
  fs.writeFileSync(path.join(dir, 'index.html'), '<!doctype html><title>iso</title>')
  const origin = await serveDirectory(dir)
  const browser = await launchHeadlessChromium()
  try {
    const manager = new SessionManager(browser)
    const first = await manager.create('auditor-1')
    await first.page!.goto(new URL('index.html', origin.url).toString(), { waitUntil: 'load' })
    await first.page!.evaluate(() => { localStorage.setItem('userId', 'auditor-1') })
    await manager.close(first.sessionId)
    const second = await manager.create('auditor-2')
    await second.page!.goto(new URL('index.html', origin.url).toString(), { waitUntil: 'load' })
    const leaked = await second.page!.evaluate(() => localStorage.getItem('userId'))
    assert.equal(leaked, null)
    await manager.closeAll()
  } finally {
    await browser.close().catch(() => undefined)
    await origin.close().catch(() => undefined)
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

