// T006 网络拦截行为验证：真实无头 Chromium 内由页面发起 XHR / Fetch / 二进制上传，
// 逐条核对转发、回填、未登记请求本地拒绝与顺序回放。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { launchHeadlessChromium } from '../../../form-runtime/executor/src/browser.ts'
import { installTargetRoute, TARGET_PATH_PREFIXES } from '../../../form-runtime/executor/src/target-route.ts'
import { serveDirectory } from '../../../form-runtime/executor/src/static-origin.ts'

// serveProbePage 提供一个能在页面内发起 XHR/Fetch/二进制请求的静态源。
async function serveProbePage () {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'target-route-'))
  fs.writeFileSync(path.join(dir, 'index.html'), `<!doctype html><title>t006</title><script>
    window.doFetch = (url, options) => fetch(url, options).then(async response => ({
      status: response.status,
      text: await response.text()
    }))
    window.doXHR = (url, body) => new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', url)
      xhr.onload = () => resolve({ status: xhr.status, text: xhr.responseText })
      xhr.onerror = () => reject(new Error('xhr network error'))
      xhr.send(body)
    })
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

test('T006 目标请求转发 Go 并原样回填，自身资源正常放行，未登记请求被拒绝', async () => {
  const browser = await launchHeadlessChromium()
  const probe = await serveProbePage()
  const forwarded: Array<{ method: string; url: string; bodyEncoding: string; body?: string }> = []
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [probe.origin.url.replace(/\/$/, '')],
      forward: async payload => {
        forwarded.push({ method: payload.method, url: payload.url, bodyEncoding: payload.bodyEncoding, body: payload.body })
        // 与真实 Go 行为一致：转发载荷是 base64 保真，Go 解码后以 utf8 原文回填页面。
        const decoded = payload.body ? Buffer.from(payload.body, 'base64').toString('utf8') : ''
        return {
          httpStatus: 200,
          headers: { 'content-type': 'application/json' },
          bodyEncoding: 'utf8',
          body: JSON.stringify({ isSuccess: true, echo: decoded })
        }
      }
    })
    await page.goto(new URL('index.html', probe.origin.url).toString(), { waitUntil: 'load' })

    // ① Fetch 命中 /web/ 前缀：转发 Go 后把响应原样交回页面。
    const fetched = await page.evaluate(`window.doFetch('${new URL('/web/flowInstanceApi/submit', probe.origin.url)}', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ data: { name: 'T006' } })
    })`)
    assert.equal(fetched.status, 200)
    const echo = JSON.parse(fetched.text)
    assert.equal(echo.isSuccess, true)
    assert.ok(echo.echo.includes('T006'))

    // ② XHR 命中 /flowInstanceApi 前缀：同样转发并回填。
    const xhr = await page.evaluate(`window.doXHR('${new URL('/flowInstanceApi/audit', probe.origin.url)}', 'audit-body')`)
    assert.equal(xhr.status, 200)
    assert.ok(xhr.text.includes('audit-body'))

    // ③ 自身静态资源：不转发，正常加载（index.html 可再次取得）。
    const ownAsset = await page.evaluate(`window.doFetch('${new URL('index.html', probe.origin.url)}')`)
    assert.equal(ownAsset.status, 200)

    // ④ 未登记外部请求：本地拒绝，fetch 直接失败，不会出现任何网络访问。
    await assert.rejects(() => page.evaluate(`window.doFetch('http://unregistered.example.com/anything')`))

    // ⑤ 轨迹事实：只有目标请求与被拒请求进轨迹（自身资源不算目标请求），顺序与响应顺序可回放。
    assert.equal(forwarded.length, 2)
    assert.deepEqual(forwarded.map(item => item.method), ['POST', 'POST'])
    assert.ok(forwarded.every(item => item.bodyEncoding === 'base64'))
    const entries = handle.trace.list()
    assert.equal(entries.length, 3)
    assert.deepEqual(entries.map(entry => entry.seq), [1, 2, 3])
    const targetEntries = entries.filter(entry => entry.state === 'fulfilled' && entry.response)
    assert.equal(targetEntries.length, 2)
    const responseSeqs = targetEntries.map(entry => entry.response!.responseSeq)
    assert.deepEqual(responseSeqs, [...responseSeqs].sort((a, b) => a - b))
    const blocked = entries.find(entry => entry.state === 'blocked')
    assert.ok(blocked, '未登记请求必须留下 blocked 轨迹')
    const stats = handle.trace.stats()
    assert.equal(stats.fulfilled, 2)
    assert.equal(stats.blocked, 1)
    await handle.uninstall()
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
    await probe.close()
  }
})

test('T006 二进制上传请求按 base64 保真转发并可回放', async () => {
  const browser = await launchHeadlessChromium()
  const probe = await serveProbePage()
  let capturedBody = ''
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [probe.origin.url.replace(/\/$/, '')],
      forward: async payload => {
        capturedBody = payload.body ?? ''
        assert.equal(payload.bodyEncoding, 'base64')
        return { httpStatus: 200, headers: { 'content-type': 'text/plain' }, bodyEncoding: 'utf8', body: 'uploaded' }
      }
    })
    await page.goto(new URL('index.html', probe.origin.url).toString(), { waitUntil: 'load' })
    // [1,2,3,0,255] 的二进制上传：buffer 必须逐字节保真。
    const status = await page.evaluate(`window.doBinaryUpload('${new URL('/web/file/upload', probe.origin.url)}', [1,2,3,0,255])`)
    assert.equal(status, 200)
    const decoded = Buffer.from(capturedBody, 'base64')
    assert.deepEqual([...decoded], [1, 2, 3, 0, 255])
    assert.equal(handle.trace.list()[0]!.state, 'fulfilled')
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
    await probe.close()
  }
})

test('T006 Go 不可达时请求本地中止，绝不直连目标平台', async () => {
  const browser = await launchHeadlessChromium()
  const probe = await serveProbePage()
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    const handle = await installTargetRoute(page, {
      runtimeOrigins: [probe.origin.url.replace(/\/$/, '')],
      forward: async () => {
        throw new Error('Go 通道已断开')
      }
    })
    await page.goto(new URL('index.html', probe.origin.url).toString(), { waitUntil: 'load' })
    await assert.rejects(() => page.evaluate(`window.doFetch('${new URL('/web/flowInstanceApi/submit', probe.origin.url)}', { method: 'POST', body: '{}' })`))
    const entry = handle.trace.list()[0]!
    assert.equal(entry.state, 'aborted')
    assert.ok(entry.abortReason!.includes('Go 通道已断开'))
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
    await probe.close()
  }
})

test('T006 目标路径前缀与 Go 侧允许清单镜像', () => {
  // 前缀清单在两侧必须同步维护：这里锁定 TS 侧形状，Go 侧由 request-route_test.go 锁定。
  assert.deepEqual(TARGET_PATH_PREFIXES, ['/web/', '/flowInstanceApi'])
  assert.equal(TARGET_PATH_PREFIXES.includes('/web/'), true)
})
