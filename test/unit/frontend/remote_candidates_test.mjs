import assert from 'node:assert/strict'
import test from 'node:test'

import { fetchTargetCandidates, targetApiErrorMessage, TargetApiError, verifyTargetAccount } from '../../../web/src/features/plans/api.ts'
import {
  CANDIDATE_ITEM_SIZE,
  CANDIDATE_VIEWPORT_HEIGHT,
  candidateDetail,
  candidateDetailTitle,
  candidateMeta,
  candidateStatus,
} from '../../../web/src/features/plans/presentation.ts'
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
	code: 'FLOW-CODE',
	remark: '用于验证采购审批',
	flowCreateType: 'current_platform',
	formExist: 'withForm',
	formTemplateCount: 2,
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

test('模板展示使用编码分类表单关联和备注，不把状态作为主展示', () => {
	const item = template('one', '采购审批')
	assert.equal(candidateStatus(item), '')
	assert.match(candidateMeta(item), /编码 FLOW-CODE/)
	assert.match(candidateMeta(item), /分类 测试类型/)
	assert.match(candidateMeta(item), /有表单 · 关联 2 个/)
	assert.equal(candidateDetail(item), '备注：用于验证采购审批')

	const emptyRemark = { ...item, remark: '' }
	assert.equal(candidateDetail(emptyRemark), '备注：暂无备注')
	const longRemark = { ...item, remark: '用于采购审批主流程的跨部门长文本说明，列表只截断视觉内容但保留完整提示。' }
	assert.equal(candidateDetailTitle(longRemark), candidateDetail(longRemark))
	assert.equal(CANDIDATE_ITEM_SIZE, 96)
	assert.equal(CANDIDATE_VIEWPORT_HEIGHT, 480)
})

test('稳定错误码映射为简洁业务提示', () => {
	assert.equal(targetApiErrorMessage('TARGET_LOGIN_REJECTED'), '账号验证失败，请核对账号')
	assert.equal(targetApiErrorMessage('TARGET_TIMEOUT'), '读取流程超时，请重试')
	assert.equal(targetApiErrorMessage('UNKNOWN'), '请求失败，请重试')
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
		data: { account: 'account-a', page: 1, pageSize: 20, total: 1, hasMore: false, items: [{ id: 't1', flowName: '模板', code: 'FLOW-CODE', groupName: '', flowStatus: 'enable', statusText: '正常', typeName: '经营管理', updateDate: '', createDate: '', remark: '用途说明', flowCreateType: '', formExist: 'withForm', formTemplateCount: 2 }] },
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
	assert.equal(templates.items[0].remark, '用途说明')
	assert.equal(templates.items[0].formTemplateCount, 2)
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
