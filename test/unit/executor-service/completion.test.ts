import assert from 'node:assert/strict'
import test from 'node:test'

import { canSendForwardStep, choiceErrorTypeFromResponse, settleCompletion, WriteGate } from '../../../executor-service/src/completion.ts'
import { RequestTraceRecorder } from '../../../executor-service/src/request-trace.ts'

type Seed = { method: string, url: string, state: 'fulfilled' | 'aborted' | 'blocked', httpStatus?: number, body?: string, requestBody?: string, writeSequence?: number }

function seedTrace (entries: Seed[]): RequestTraceRecorder {
  const trace = new RequestTraceRecorder()
  for (const item of entries) {
    const entry = trace.begin({ method: item.method, url: item.url, headers: {}, action: '', writeSequence: item.writeSequence, body: item.requestBody, bodyEncoding: 'utf8' })
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
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', state: 'fulfilled', body: ok({ id: 'x' }) }]) })
  assert.equal(verdict.status, 'success')
})

test('旧 submit 权限预检不能冒充正式提交，新端点成功才判 success', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'submit',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok({ message: '权限校验通过' }), requestBody: JSON.stringify({ data: { checkPermissions: 'first' } }) },
      { method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }) }
    ])
  })
  assert.equal(verdict.status, 'success')
})

test('保存草稿仍使用旧 submit 端点，正式提交不再接受旧端点', async () => {
  const draft = await settleCompletion({ ...base, action: 'save_draft', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok() }]) })
  assert.equal(draft.status, 'success')
  const submit = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/submit', state: 'fulfilled', body: ok() }]) })
  assert.equal(submit.status, 'failed')
  assert.match(String(submit.message), /newWorkflowCenter/)
})

test('重新提交使用新工作流端点且响应丢失按待确认处理', async () => {
  const success = await settleCompletion({ ...base, action: 'resubmit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/reSubmit', state: 'fulfilled', body: ok() }]) })
  assert.equal(success.status, 'success')
  const writeGate = new WriteGate()
  const uncertain = await settleCompletion({ ...base, action: 'resubmit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/reSubmit', state: 'aborted' }]), writeGate })
  assert.equal(uncertain.status, 'uncertain')
  assert.equal(writeGate.isLocked, true)
})

test('完成结果保留原始请求出现时间和序号，供动作接口流转排序', async () => {
  const trace = seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', state: 'fulfilled', body: ok() }])
  const original = trace.list()[0]!
  const result = await settleCompletion({ ...base, action: 'submit', trace })
  assert.equal(result.requestTrace[0]!.seq, original.seq)
  assert.equal(result.requestTrace[0]!.requestAt, original.requestAt)
})

test('SID 失效判 target_rejected', async () => {
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', state: 'fulfilled', body: encoded({ isSuccess: false, code: 'RESP401', message: '会话无效' }) }]) })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /SID/)
})

test('普通业务拒绝判 target_rejected 并保留目标消息', async () => {
  const verdict = await settleCompletion({ ...base, action: 'approve', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: rejected('任务已经处理') }]) })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /任务已经处理/)
})

test('选择要求被后续同端点成功覆盖时判 success', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: rejected('手动条件分支,请选择', { errorType: 'custom_choose' }) },
      { method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: ok() }
    ])
  })
  assert.equal(verdict.status, 'success')
})

test('选择要求未闭环时判 target_rejected', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: rejected('请选择流程分支', { errorType: 'run_node_choose' }) }])
  })
  assert.equal(verdict.status, 'target_rejected')
  assert.match(String(verdict.message), /选择要求未闭环/)
})

test('缺少明确成功标记的 HTTP 200 写响应不得冒充成功', async () => {
  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: encoded({ message: '响应未包含成功标记' }) }])
  })
  assert.equal(verdict.status, 'failed')
  assert.match(String(verdict.message), /没有成功响应/)
})

test('新工作流端点省略 isSuccess 但返回选择错误时仍进入自动续办闭环', async () => {
  const response = { httpStatus: 200, bodyEncoding: 'base64', body: encoded({ message: '手动条件分支,请选择', data: { errorType: 'custom_choose' } }) }
  const endpoint = 'https://t/api/newWorkflowCenter/flowInstance/flowAudit'
  assert.equal(choiceErrorTypeFromResponse(endpoint, response), 'custom_choose')
  const blocked = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/flowAudit', state: 'fulfilled', body: response.body }])
  })
  assert.equal(blocked.status, 'target_rejected')
  assert.match(String(blocked.message), /选择要求未闭环/)

  const verdict = await settleCompletion({
    ...base,
    action: 'approve',
    trace: seedTrace([
      { method: 'POST', url: endpoint, state: 'fulfilled', body: response.body },
      { method: 'POST', url: endpoint, state: 'fulfilled', body: ok() }
    ])
  })
  assert.equal(verdict.status, 'success')
})

test('明确成功包络中的同名字段不触发选择续办', () => {
  assert.equal(choiceErrorTypeFromResponse('https://t/api/newWorkflowCenter/flowInstance/flowAudit', {
    httpStatus: 200,
    bodyEncoding: 'base64',
    body: encoded({ isSuccess: true, data: { errorType: 'custom_choose' } })
  }), null)
})

test('三类明确选择错误识别不依赖 isSuccess，其他 HTTP 错误不触发续办', () => {
  for (const errorType of ['custom_choose', 'parallel_choose', 'run_node_choose']) {
    assert.equal(choiceErrorTypeFromResponse('https://t/api/newWorkflowCenter/flowInstance/flowAudit', {
      httpStatus: 200,
      bodyEncoding: 'base64',
      body: encoded({ data: { errorType } })
    }), errorType)
  }
  assert.equal(choiceErrorTypeFromResponse('https://t/api/newWorkflowCenter/flowInstance/flowAudit', {
    httpStatus: 500,
    bodyEncoding: 'base64',
    body: encoded({ data: { errorType: 'custom_choose' } })
  }), null)
  assert.equal(choiceErrorTypeFromResponse('https://t/api/web/flowTemplateApi/list', {
    httpStatus: 200,
    bodyEncoding: 'base64',
    body: encoded({ data: { errorType: 'custom_choose' } })
  }), null)
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
  const verdict = await settleCompletion({ ...base, action: 'submit', trace: seedTrace([{ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', state: 'aborted' }]), writeGate })
  assert.equal(verdict.status, 'uncertain')
  assert.equal(writeGate.isLocked, true)
})

test('硬超时判 timeout 且锁死后续写', async () => {
  const writeGate = new WriteGate()
  const trace = new RequestTraceRecorder()
  trace.begin({ method: 'POST', url: 'https://t/api/newWorkflowCenter/flowInstance/submit', headers: {}, action: '' })
  const verdict = await settleCompletion({ action: 'submit', trace, baselineSeq: 0, hardTimeoutMs: 250, idleWindowMs: 20, writeGate })
  assert.equal(verdict.status, 'timeout')
  assert.equal(writeGate.isLocked, true)
})

test('转发主请求成功但附言响应丢失时判 uncertain，锁住后续写', async () => {
  const writeGate = new WriteGate()
  const verdict = await settleCompletion({
    ...base,
    action: 'forward',
    trace: seedTrace([
      { method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }), writeSequence: 1 },
      { method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', state: 'aborted', requestBody: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'postscript-1', text: '' } }), writeSequence: 2 }
    ]),
    writeGate
  })
  assert.equal(verdict.status, 'uncertain')
  assert.equal(writeGate.isLocked, true)
})

test('转发第一步未确认不得发第二步，确认后不得重复发', () => {
  const main = { state: 'fulfilled', url: 'https://t/api/web/flowInstanceApi/transpond', writeSequence: 1, response: { httpStatus: 200, bodyEncoding: 'base64', body: ok({ data: { id: 'new-instance' } }) } }
  const request = { body: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'postscript-1', text: '' } }) }
  assert.equal(canSendForwardStep([], 1), true)
  assert.equal(canSendForwardStep([{ ...main, state: 'forwarding' }], 2, request), false)
  assert.equal(canSendForwardStep([main], 2, request), true)
  assert.equal(canSendForwardStep([
    main,
    { state: 'fulfilled', url: 'https://t/api/web/flowInstancePostscript/save', writeSequence: 2 }
  ], 2, request), false)
})

test('转发有无历史附言勾选、空或非空当前附言，只有两笔完整成功才通过', async () => {
  for (const checkList of [[], ['fuyan'], ['yijian'], ['fuyan', 'yijian']]) {
    for (const text of ['', '本次转发附言']) {
      const trace = seedTrace([
        { method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }), requestBody: JSON.stringify({ checkList }), writeSequence: 1 },
        { method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', state: 'fulfilled', body: ok(), requestBody: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'postscript-1', text } }), writeSequence: 2 }
      ])
      assert.equal((await settleCompletion({ ...base, action: 'forward', trace })).status, 'success')
    }
  }
})

test('转发主请求成功但没有当前附言，必须失败并禁止迟到的补写', async () => {
  const writeGate = new WriteGate()
  const trace = seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }) }])
  assert.equal((await settleCompletion({ ...base, action: 'forward', trace, writeGate })).status, 'business_failed')
  assert.equal(writeGate.isLocked, true)
})

test('主请求缺少明确成功或新实例 ID 时，附言不能放行', async () => {
  for (const body of [undefined, '{}', '{broken', encoded({ data: { id: 'new-instance' } }), rejected('转发拒绝'), ok(), ok({ data: { id: 123 } })]) {
    const main = { state: 'fulfilled', url: 'https://t/api/web/flowInstanceApi/transpond', response: { httpStatus: 200, bodyEncoding: 'base64', body } }
    const request = { body: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'postscript-1', text: '' } }) }
    assert.equal(canSendForwardStep([main], 2, request), false)
    const trace = seedTrace([{ method: 'POST', url: main.url, state: 'fulfilled', body }])
    assert.notEqual((await settleCompletion({ ...base, action: 'forward', trace })).status, 'success')
  }
})

test('错实例、缺少附言 ID 或文本的保存请求在写前拒绝', () => {
  const main = { state: 'fulfilled', url: 'https://t/api/web/flowInstanceApi/transpond', response: { httpStatus: 200, bodyEncoding: 'base64', body: ok({ data: { id: 'new-instance' } }) } }
  for (const data of [{ flowInstanceId: 'old-instance', id: 'p', text: '' }, { flowInstanceId: 'new-instance', text: '' }, { flowInstanceId: 'new-instance', id: 'p' }]) {
    assert.equal(canSendForwardStep([main], 2, { body: JSON.stringify({ data }) }), false)
  }
})

test('附言明确拒绝与响应不明分别判失败和待确认，均不能成功', async () => {
  for (const [body, status] of [[rejected('附言拒绝'), 'business_failed'], [encoded({ code: 'RESP401' }), 'business_failed'], [undefined, 'uncertain'], [encoded({}), 'uncertain'], [Buffer.from('{broken').toString('base64'), 'uncertain']] as const) {
    const trace = seedTrace([
      { method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }) },
      { method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', state: 'fulfilled', body, requestBody: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'p', text: '' } }) }
    ])
    assert.equal((await settleCompletion({ ...base, action: 'forward', trace })).status, status)
  }
})

test('附言仍在途且硬超时，保持待确认并锁住后续写', async () => {
  const trace = seedTrace([{ method: 'POST', url: 'https://t/api/web/flowInstanceApi/transpond', state: 'fulfilled', body: ok({ data: { id: 'new-instance' } }) }])
  trace.begin({ method: 'POST', url: 'https://t/api/web/flowInstancePostscript/save', headers: {}, action: '', body: JSON.stringify({ data: { flowInstanceId: 'new-instance', id: 'p', text: '' } }) })
  const writeGate = new WriteGate()
  assert.equal((await settleCompletion({ ...base, hardTimeoutMs: 200, action: 'forward', trace, writeGate })).status, 'timeout')
  assert.equal(writeGate.isLocked, true)
})
