import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import type { Page } from 'playwright'

import { launchHeadlessChromium } from '../../../executor-service/src/browser.ts'
import { ForwardChannel, ForwardFailure } from '../../../executor-service/src/forward-channel.ts'
import { assertForwardReady } from '../../../executor-service/src/navigate.ts'
import { installTargetRoute, TARGET_PATH_PREFIXES } from '../../../executor-service/src/target-route.ts'
import { serveDirectory } from '../../../executor-service/src/static-origin.ts'

async function serveProbePage () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'target-route-'))
  fs.writeFileSync(path.join(dir, 'index.html'), `<!doctype html><title>t005</title><script>
    window.doFetch = (url, options) => fetch(url, options).then(async response => ({
      status: response.status,
      text: await response.text()
    }))
    window.doXHR = (url, body) => {
      const { promise, resolve, reject } = Promise.withResolvers()
      const xhr = new XMLHttpRequest()
      xhr.open('POST', url)
      xhr.onload = () => resolve({ status: xhr.status, text: xhr.responseText })
      xhr.onerror = () => reject(new Error('xhr network error'))
      xhr.send(body)
      return promise
    }
    window.doBinaryUpload = (url, bytes) => fetch(url, { method: 'POST', body: new Uint8Array(bytes) }).then(r => r.status)
  </script>`)
  const origin = await serveDirectory(dir)
  return {
    origin,
    close: async () => {
      await origin.close().catch(() => undefined)
      fs.rmSync(dir, { recursive: true, force: true })
    }
  }
}

async function withPage (run: (page: Page, originURL: string) => Promise<void>) {
  const browser = await launchHeadlessChromium()
  const probe = await serveProbePage()
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    await run(page, probe.origin.url)
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
    await probe.close()
  }
}

test('目标请求转发并回填，自身资源放行，未登记请求拒绝', async () => {
  const forwarded: Array<{ method: string, url: string, bodyEncoding: string }> = []
  await withPage(async (page, originURL) => {
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [originURL.replace(/\/$/, '')],
      forward: async payload => {
        forwarded.push({ method: payload.method, url: payload.url, bodyEncoding: payload.bodyEncoding })
        const decoded = payload.body ? Buffer.from(payload.body, 'base64').toString('utf8') : ''
        return {
          httpStatus: 200,
          headers: { 'content-type': 'application/json' },
          bodyEncoding: 'utf8',
          body: JSON.stringify({ isSuccess: true, echo: decoded })
        }
      }
    })
    await page.goto(new URL('index.html', originURL).toString(), { waitUntil: 'load' })
    const fetched = await page.evaluate(`window.doFetch('${new URL('/web/flowInstanceApi/submit', originURL)}', {
      method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ data: { name: 'T005' } })
    })`) as { status: number, text: string }
    assert.equal(fetched.status, 200)
    assert.equal(JSON.parse(fetched.text).echo.includes('T005'), true)
    const xhr = await page.evaluate(`window.doXHR('${new URL('/flowInstanceApi/audit', originURL)}', 'audit-body')`) as { status: number, text: string }
    assert.equal(xhr.status, 200)
    await assert.rejects(() => page.evaluate(`window.doFetch('http://unregistered.example.com/anything')`))
    const entries = handle.trace.list()
    assert.equal(forwarded.length, 2)
    assert.equal(entries.filter(entry => entry.state === 'fulfilled').length, 2)
    assert.equal(entries.some(entry => entry.state === 'blocked'), true)
    await handle.uninstall()
  })
})

test('二进制上传按 base64 保真', async () => {
  let capturedBody = ''
  await withPage(async (page, originURL) => {
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [originURL.replace(/\/$/, '')],
      forward: async payload => {
        capturedBody = payload.body ?? ''
        return { httpStatus: 200, headers: { 'content-type': 'text/plain' }, bodyEncoding: 'utf8', body: 'uploaded' }
      }
    })
    await page.goto(new URL('index.html', originURL).toString(), { waitUntil: 'load' })
    const status = await page.evaluate(`window.doBinaryUpload('${new URL('/web/file/upload', originURL)}', [1,2,3,0,255])`)
    assert.equal(status, 200)
    assert.deepEqual([...Buffer.from(capturedBody, 'base64')], [1, 2, 3, 0, 255])
    await handle.uninstall()
  })
})

test('转发失败时本地中止，不直连目标', async () => {
  await withPage(async (page, originURL) => {
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [originURL.replace(/\/$/, '')],
      forward: async () => {
        throw new Error('Go 通道已断开')
      }
    })
    await page.goto(new URL('index.html', originURL).toString(), { waitUntil: 'load' })
    await assert.rejects(() => page.evaluate(`window.doFetch('${new URL('/web/flowInstanceApi/submit', originURL)}', { method: 'POST', body: '{}' })`))
    assert.equal(handle.trace.list()[0]?.state, 'aborted')
    await handle.uninstall()
  })
})

test('目标路径前缀与 Go 侧允许清单镜像', () => {
  assert.deepEqual(TARGET_PATH_PREFIXES, ['/web/', '/flowInstanceApi', '/api/'])
})

test('转发通道未连接禁止导航', () => {
  const channel = new ForwardChannel()
  assert.equal(channel.connected, false)
  assert.throws(() => assertForwardReady(channel), (error: unknown) => error instanceof ForwardFailure)
})
