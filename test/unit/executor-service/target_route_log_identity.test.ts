import assert from 'node:assert/strict'
import test from 'node:test'
import type { Page, Route } from 'playwright'
import { installTargetRoute } from '../../../executor-service/src/target-route.ts'

test('页面转发携带原始轨迹序号，两笔转发写各一次且载荷不变', async () => {
  // 用路由桩直接执行真实拦截函数，不启动 Chromium，也不连接真实目标平台。
  let handler: ((route: Route) => Promise<void>) | undefined
  const page = {
    route: async (_pattern: string, callback: typeof handler) => { handler = callback },
    unroute: async () => {},
  } as unknown as Page
  const forwarded: Array<{ requestSeq: number, url: string, body?: string }> = []
  const handle = await installTargetRoute(page, {
    runtimeOrigins: ['http://runtime.local'],
    forward: async payload => {
      forwarded.push(payload)
      return { httpStatus: 200, headers: { 'content-type': 'application/json' }, bodyEncoding: 'utf8', body: JSON.stringify({ isSuccess: true, data: { id: 'aux-new' } }) }
    },
  })
  const send = async (path: string, body: string) => {
    let fulfilled = false
    const route = {
      request: () => ({ url: () => `https://target.local${path}`, method: () => 'POST', allHeaders: async () => ({ 'content-type': 'application/json' }), postDataBuffer: () => Buffer.from(body) }),
      fulfill: async () => { fulfilled = true },
      abort: async () => { throw new Error('合法请求不应被拦截') },
    } as unknown as Route
    await handler!(route)
    assert.equal(fulfilled, true)
  }
  const main = JSON.stringify({ data: { personId: 'receiver' } })
  const postscript = JSON.stringify({ data: { id: 'post-new', flowInstanceId: 'aux-new', text: '当前附言' } })
  await send('/api/web/flowInstanceApi/transpond', main)
  await send('/api/web/flowInstancePostscript/save', postscript)
  assert.deepEqual(forwarded.map(request => request.requestSeq), [1, 2])
  assert.deepEqual(handle.trace.list().map(request => request.seq), [1, 2])
  assert.deepEqual(forwarded.map(request => Buffer.from(request.body!, 'base64').toString('utf8')), [main, postscript])
  await handle.uninstall()
})
