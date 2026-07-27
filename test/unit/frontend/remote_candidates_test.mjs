import assert from 'node:assert/strict'
import test from 'node:test'

import { fetchTargetCandidates, TargetApiError, verifyTargetAccount } from '../../../web/src/features/plans/api.ts'
import {
  REMOTE_SEARCH_DEBOUNCE_MS,
  createDebouncedRunner,
  invalidatesVerification,
  isCurrentRemoteRequest,
  mergeCandidatePages,
  retryPageFor,
} from '../../../web/src/features/plans/remote.ts'

const template = (id, name = id) => ({
  key: `template:${id}`,
  kind: 'template',
  accountName: 'account-a',
  templateId: id,
  flowName: name,
  typeName: '测试类型',
  groupName: '测试分组',
  statusText: '正常',
  updateTime: '2026-07-27 10:00',
})

test('搜索防抖只执行最后一次输入且可取消', async () => {
  assert.equal(REMOTE_SEARCH_DEBOUNCE_MS, 250)
  const values = []
  const runner = createDebouncedRunner((value) => values.push(value), 20)
  runner.schedule('a')
  runner.schedule('ab')
  runner.schedule('abc')
  await new Promise((resolve) => setTimeout(resolve, 40))
  assert.deepEqual(values, ['abc'])
  runner.schedule('cancelled')
  runner.cancel()
  await new Promise((resolve) => setTimeout(resolve, 30))
  assert.deepEqual(values, ['abc'])
})

test('请求版本与账号来源查询共同阻止迟到结果', () => {
  const current = { version: 4, account: 'account-b', source: 'pending', query: 'flow' }
  assert.equal(isCurrentRemoteRequest(current, { ...current }), true)
  assert.equal(isCurrentRemoteRequest(current, { ...current, version: 3 }), false)
  assert.equal(isCurrentRemoteRequest(current, { ...current, account: 'account-a' }), false)
  assert.equal(isCurrentRemoteRequest(current, { ...current, source: 'new' }), false)
  assert.equal(isCurrentRemoteRequest(current, { ...current, query: 'old' }), false)
})

test('分页追加按来源真实 ID 去重并保留顺序', () => {
  const merged = mergeCandidatePages(
    [template('one'), template('two', '旧名称')],
    [template('two', '新名称'), template('three')],
  )
  assert.deepEqual(merged.map((item) => item.templateId), ['one', 'two', 'three'])
  assert.equal(merged[1].flowName, '新名称')
})

test('追加错误优先重试失败页且会话错误使验证失效', () => {
  assert.equal(retryPageFor([], 0, null), 1)
  assert.equal(retryPageFor([template('one')], 1, null), 2)
  assert.equal(retryPageFor([template('one')], 1, 2), 2)
  assert.equal(invalidatesVerification('TARGET_SESSION_EXPIRED'), true)
  assert.equal(invalidatesVerification('TARGET_LOGIN_REJECTED'), true)
  assert.equal(invalidatesVerification('TARGET_TIMEOUT'), false)
})

test('账号验证请求支持 AbortController 取消', async () => {
  const originalFetch = globalThis.fetch
  globalThis.fetch = (_input, init) => new Promise((_resolve, reject) => {
    init.signal.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')), { once: true })
  })
  try {
    const controller = new AbortController()
    const pending = verifyTargetAccount('account-a', controller.signal)
    controller.abort()
    await assert.rejects(pending, (error) => error.name === 'AbortError')
  }
  finally {
    globalThis.fetch = originalFetch
  }
})

test('三类真实 API 数据映射为各自候选且稳定错误可恢复', async () => {
  const originalFetch = globalThis.fetch
  const responses = [
    {
      success: true,
      data: { account: 'account-a', page: 1, pageSize: 20, total: 1, hasMore: false, items: [{ id: 't1', flowName: '模板', code: '', groupName: '', flowStatus: 'enable', statusText: '正常', typeName: '', updateDate: '', createDate: '', remark: '', flowCreateType: '' }] },
    },
    {
      success: true,
      data: { account: 'account-a', page: 1, pageSize: 20, total: 1, hasMore: false, items: [{ id: 's1', name: '', formName: '已发表单', title: '已发表单', status: 'run', createDate: '', currentNodeName: '', currentAuditUserNames: '' }] },
    },
    {
      success: true,
      data: { account: 'account-a', page: 1, pageSize: 20, total: 1, hasMore: false, items: [{ flowInstanceId: 'd1', flowInstanceName: '', formName: '待发表单', title: '待发表单', flowStatus: 'draft', statusName: '草稿', initiator: '', initiatorDate: '' }] },
    },
  ]
  let index = 0
  globalThis.fetch = async () => new Response(JSON.stringify(responses[index++]), { status: 200, headers: { 'Content-Type': 'application/json' } })
  const controller = new AbortController()
  try {
    const common = { account: 'account-a', query: '', page: 1, pageSize: 20, signal: controller.signal }
    const templates = await fetchTargetCandidates({ ...common, source: 'new' })
    const submitted = await fetchTargetCandidates({ ...common, source: 'started' })
    const due = await fetchTargetCandidates({ ...common, source: 'pending' })
    assert.equal(templates.items[0].kind, 'template')
    assert.equal(submitted.items[0].kind, 'submitted')
    assert.equal(due.items[0].kind, 'due')

    globalThis.fetch = async () => new Response(JSON.stringify({
      success: false,
      error: { code: 'TARGET_TIMEOUT', message: '目标平台响应超时，请重试', retryable: true },
    }), { status: 504, headers: { 'Content-Type': 'application/json' } })
    await assert.rejects(
      fetchTargetCandidates({ ...common, source: 'new' }),
      (error) => error instanceof TargetApiError && error.code === 'TARGET_TIMEOUT' && error.retryable,
    )
  }
  finally {
    globalThis.fetch = originalFetch
  }
})
