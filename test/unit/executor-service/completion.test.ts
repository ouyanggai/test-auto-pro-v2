import assert from 'node:assert/strict'
import test from 'node:test'

import { canSendForwardStep, settleCompletion, WriteGate } from '../../../executor-service/src/completion.ts'
import { RequestTraceRecorder } from '../../../executor-service/src/request-trace.ts'

function seedTrace (entries: Array<{ method: string, url: string, state: 'fulfilled' | 'aborted' | 'blocked', httpStatus?: number, body?: string, writeSequence?: number }>): RequestTraceRecorder {
  const trace = new RequestTraceRecorder()
  for (const item of entries) {
    const entry = trace.begin({ method: item.method, url: item.url, headers: {}, action: '', writeSequence: item.writeSequence })
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
const sidLost = () => Buffer.from(JSON.stringify({ isSuccess: false, code: 'RESP401', message: 'SID已失效' })).toString('base64')

test('流程成功 + 业务完成事件 → success', async () => {
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ id: 'x' }) }]),
    baselineSeq: 0,
    businessDone: async () => true,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'success')
})

test('流程接口成功不能代替全部业务成功：后置请求丢失 → business_failed', async () => {
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ id: 'x' }) },
      { method: 'POST', url: 'https://t/api/web/file/api/relationFile/saveBatch', state: 'aborted' }
    ]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'business_failed')
})

test('写响应丢失 → uncertain 且锁死后续写', async () => {
  const writeGate = new WriteGate()
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'aborted' }]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20,
    writeGate
  })
  assert.equal(verdict.status, 'uncertain')
  assert.equal(writeGate.isLocked, true)
  assert.throws(() => writeGate.assertOpen())
})

test('SID 失效 → target_rejected', async () => {
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: sidLost() }]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /SID/)
})

test('选择要求未闭环 → target_rejected', async () => {
  const verdict = await settleCompletion({
    action: 'approve',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: rejected({ errorType: 'run_node_choose' }) }]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /选择/)
})

test('没有任何写请求且业务未完成 → failed', async () => {
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([{ method: 'GET', url: 'https://t/api/web/some/list', state: 'fulfilled' }]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'failed')
})

test('硬超时 → timeout 且锁死后续写', async () => {
  const writeGate = new WriteGate()
  const trace = new RequestTraceRecorder()
  trace.begin({ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', headers: {}, action: '' })
  const verdict = await settleCompletion({
    action: 'submit',
    trace,
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 250,
    idleWindowMs: 20,
    writeGate
  })
  assert.equal(verdict.status, 'timeout')
  assert.equal(writeGate.isLocked, true)
})

test('转发第一步未确认不得发第二步，确认后不得重复发', () => {
  assert.equal(canSendForwardStep([], 1), true)
  assert.equal(canSendForwardStep([{ state: 'forwarding', url: 'https://t/api/web/flowInstanceApi/transpond', writeSequence: 1 }], 2), false)
  assert.equal(canSendForwardStep([{ state: 'fulfilled', url: 'https://t/api/web/flowInstanceApi/transpond', writeSequence: 1 }], 2), true)
  assert.equal(canSendForwardStep([
    { state: 'fulfilled', url: 'https://t/api/web/flowInstanceApi/transpond', writeSequence: 1 },
    { state: 'fulfilled', url: 'https://t/api/web/flowInstancePostscript/save', writeSequence: 2 }
  ], 2), false)
})

test('装载失败无写请求 → failed', async () => {
  const verdict = await settleCompletion({
    action: 'load_initiate',
    trace: seedTrace([]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20
  })
  assert.equal(verdict.status, 'failed')
})

test('服务断开导致写请求 aborted → uncertain 且不重发', async () => {
  const writeGate = new WriteGate()
  const verdict = await settleCompletion({
    action: 'submit',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'aborted' }]),
    baselineSeq: 0,
    businessDone: async () => false,
    hardTimeoutMs: 5000,
    idleWindowMs: 20,
    writeGate
  })
  assert.equal(verdict.status, 'uncertain')
  assert.throws(() => writeGate.assertOpen())
})

