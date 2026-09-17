import assert from 'node:assert/strict'
import test from 'node:test'

import { canSendForwardStep, settleCompletion, WriteGate } from '../../../executor-service/src/completion.ts'
import { RequestTraceRecorder } from '../../../executor-service/src/request-trace.ts'

type Seed = { method: string, url: string, state: 'fulfilled' | 'aborted' | 'blocked', httpStatus?: number, body?: string, writeSequence?: number }

function seedTrace (entries: Seed[]): RequestTraceRecorder {
  const trace = new RequestTraceRecorder()
  for (const item of entries) {
    const entry = trace.begin({ method: item.method, url: item.url, headers: {}, action: '', writeSequence: item.writeSequence })
    if (item.state === 'fulfilled') {
      trace.markFulfilled(entry, { httpStatus: item.httpStatus ?? 200, headers: {}, bodyEncoding: item.body ? 'base64' : 'utf8', body: item.body, completedAt: new Date().toISOString(), durationMs: 5 })
    } else {
      trace.markAborted(entry, item.state, item.state === 'blocked' ? '未登记' : '写响应丢失')
    }
  }
  return trace
}

const encoded = (body: Record<string, unknown>) => Buffer.from(JSON.stringify(body)).toString('base64')
const ok = (body: Record<string, unknown> = {}) => encoded({ isSuccess: true, ...body })
const rejected = (message: string, data: Record<string, unknown> = {}) => encoded({ isSuccess: false, message, data })
const base = { baselineSeq: 0, hardTimeoutMs: 5000, idleWindowMs: 20 }

test('期望写端点 HTTP 200 且未明确拒绝时判 success', async () => {
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ id: 'x' }) }]) })
  assert.equal(verdict.status, 'success')
})

test('SID 失效判 target_rejected', async () => {
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: encoded({ isSuccess: false, code: 'RESP401', message: '会话无效' }) }]) })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /SID/)
})

test('普通业务拒绝判 target_rejected 并保留目标消息', async () => {
  const verdict = await settleCompletion({ ...base, action: 'approve', trace: seedTrace([{ method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: rejected('任务已经处理') }]) })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /任务已经处理/)
})

test('选择要求被后续同端点成功覆盖时判 success', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: rejected('手动条件分支,请选择', { errorType: 'custom_choose' }) },
      { method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: ok() }
    ])
  })
  assert.equal(verdict.status, 'success')
})

test('选择要求未闭环时判 target_rejected', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/flowInstanceApi/audit', state: 'fulfilled', body: rejected('请选择流程分支', { errorType: 'run_node_choose' }) }])
  })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /选择要求未闭环/)
})

test('没有写请求判 failed', async () => {
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'GET', url: 'https://t/api/web/some/list', state: 'fulfilled' }]) })
  assert.equal(verdict.status, 'failed')
  assert.match(String(verdict.message), /期望写请求/)
})

test('调用其他写端点但未命中动作期望端点时判 failed', async () => {
  const verdict = await settleCompletion({ ...base, action: 'approve', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/storageFormData', state: 'fulfilled', body: ok() }]) })
  assert.equal(verdict.status, 'failed')
  assert.match(String(verdict.message), /实际调用/)
})

test('期望写响应丢失判 uncertain 且锁死后续写', async () => {
  const writeGate = new WriteGate()
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'aborted' }]), writeGate })
  assert.equal(verdict.status, 'uncertain')
  assert.equal(writeGate.isLocked, true)
})

test('硬超时判 timeout 且锁死后续写', async () => {
  const writeGate = new WriteGate()
  const trace = new RequestTraceRecorder()
  trace.begin({ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', headers: {}, action: '' })
  const verdict = await settleCompletion({ action: 'submit', trace, baselineSeq: 0, hardTimeoutMs: 250, idleWindowMs: 20, writeGate })
  assert.equal(verdict.status, 'timeout')
  assert.equal(writeGate.isLocked, true)
})

test('转发主请求成功但必需附言未完成时判 business_failed', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'forward',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok(), writeSequence: 1 },
      { method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', state: 'aborted', writeSequence: 2 }
    ]),
    forwardPostscriptRequired: true
  })
  assert.equal(verdict.status, 'business_failed')
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
