// T011 完成检测单测：用可控的请求轨迹逐条核对终局判定——
// 写丢失 → uncertain；流程拒绝 → target_rejected；后置失败 → business_failed；
// 无写请求 → failed；硬超时 → timeout；全部收敛 → success。
import assert from 'node:assert/strict'
import test from 'node:test'

import { settleCompletion } from '../../../form-runtime/executor/src/completion.ts'
import { RequestTraceRecorder } from '../../../form-runtime/executor/src/request-trace.ts'

// seedTrace 构造一个带预置轨迹的记录器：begin 出固定请求并直接落定。
function seedTrace (entries: Array<{ method: string; url: string; state: 'fulfilled' | 'aborted' | 'blocked'; httpStatus?: number; body?: string }>): RequestTraceRecorder {
  const trace = new RequestTraceRecorder()
  for (const item of entries) {
    const entry = trace.begin({ method: item.method, url: item.url, headers: {}, action: '' })
    if (item.state === 'fulfilled') {
      trace.markFulfilled(entry, {
        httpStatus: item.httpStatus ?? 200,
        headers: {},
        bodyEncoding: item.body ? 'base64' : 'utf8',
        body: item.body,
        completedAt: new Date().toISOString(),
        durationMs: 5
      })
    } else {
      trace.markAborted(entry, item.state, item.state === 'blocked' ? '未登记' : '写响应丢失')
    }
  }
  return trace
}

const ok = (body: Record<string, unknown>) => Buffer.from(JSON.stringify({ isSuccess: true, ...body })).toString('base64')
const rejected = (body: Record<string, unknown>) => Buffer.from(JSON.stringify({ isSuccess: false, message: '手动条件分支,请选择', data: body })).toString('base64')

test('T011 流程成功 + 业务完成事件 → success', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ id: 'x' }) }
  ])
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 0,
    businessDone: async () => true,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'success')
})

test('T011 流程接口成功不能代替全部业务成功：后置请求丢失 → business_failed', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ id: 'x' }) },
    { method: 'POST', url: 'https://t/api/web/file/api/relationFile/saveBatch', state: 'aborted' }
  ])
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'business_failed')
  assert.ok(verdict.message!.includes('relationFile'))
})

test('T011 写响应丢失 → uncertain（不区分是提交还是附言）', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'aborted' }
  ])
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'uncertain')
})

test('T011 转发附言丢失 → business_failed（辅助实例已创建、附言未确认）', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ id: 'aux' }) },
    { method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', state: 'aborted' }
  ])
  const verdict = await settleCompletion({
    action: 'forward', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'business_failed')
  assert.ok(verdict.message!.includes('Postscript'))
})

test('T011 目标明确拒绝 → target_rejected', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: rejected({ errorType: 'run_node_choose' }) }
  ])
  const verdict = await settleCompletion({
    action: 'approve', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'target_rejected')
  assert.ok(verdict.message!.includes('未设置审批人') || verdict.message!.includes('选择'))
})

test('T011 没有任何写请求且业务未完成 → failed（零写入不判不确定）', async () => {
  const trace = seedTrace([
    { method: 'GET', url: 'https://t/api/web/some/list', state: 'fulfilled' }
  ])
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'failed')
})

test('T011 硬超时 → timeout（在途请求挂起时收敛，不永远等待）', async () => {
  const trace = new RequestTraceRecorder()
  const entry = trace.begin({ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', headers: {}, action: '' })
  void entry
  const startedAt = Date.now()
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 300, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'timeout')
  assert.ok(Date.now() - startedAt >= 250)
})

test('T011 基线序号之前的请求不参与本动作判定', async () => {
  const trace = seedTrace([
    { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'aborted' }
  ])
  const verdict = await settleCompletion({
    action: 'submit', trace, baselineSeq: 2,
    businessDone: async () => true,
    hardTimeoutMs: 5000, idleWindowMs: 50
  })
  assert.equal(verdict.status, 'success')
})
