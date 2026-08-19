import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildCreatePlanRequest,
  createPlan,
  fetchPlan,
  fetchPlans,
  PlanApiError,
  toPlanRow,
} from '../../../web/src/features/plans/persistence.ts'

const baseForm = {
  name: '  采购回归  ',
  account: 'tester01',
  flowSource: 'new',
  templateId: 'template-id',
  submittedFlowId: null,
  dueFlowId: null,
  runMode: 'serial',
  maxConcurrency: null,
  scheduleEnabled: false,
  scheduledAt: null,
}

const verifiedAccount = { account: 'tester01', displayName: '测试专员', companyName: '集团公司' }

test('创建请求始终使用当前候选展示名而非候选 key 或编码', () => {
  const templates = buildCreatePlanRequest(baseForm, verifiedAccount, {
    kind: 'template', key: 'template:internal-key', templateId: 'template-id', flowName: '采购申请',
    code: 'SHOULD-NOT-BE-NAME', accountName: 'tester01', typeName: '', groupName: '', statusText: '',
    updateTime: '', remark: '', flowCreateType: '', formExist: '', formTemplateCount: 0,
  })
  assert.equal(templates.targetObjectId, 'template-id')
  assert.equal(templates.targetObjectName, '采购申请')
  assert.equal(templates.name, '采购回归')
  assert.equal(templates.maxConcurrency, null)
  assert.equal(templates.scheduledAt, null)

  const started = buildCreatePlanRequest({ ...baseForm, flowSource: 'started' }, verifiedAccount, {
    kind: 'submitted', key: 'submitted:internal-key', id: 'submitted-id', name: '已发采购流程',
    accountName: 'tester01', status: 'run', statusName: '审批中', createDate: '', currentNodeName: '', currentAuditUserNames: '',
  })
  assert.equal(started.targetObjectName, '已发采购流程')

  const pending = buildCreatePlanRequest({ ...baseForm, flowSource: 'pending' }, verifiedAccount, {
    kind: 'due', key: 'due:internal-key', flowInstanceId: 'due-id', flowInstanceName: '待发采购流程',
    accountName: 'tester01', statusName: '草稿', initiator: '', initiatorDate: '',
  })
  assert.equal(pending.targetObjectName, '待发采购流程')
})

test('并行和定时字段按已验收表单边界转换', () => {
  const timestamp = Date.UTC(2026, 6, 29, 9, 30)
  const request = buildCreatePlanRequest({
    ...baseForm,
    runMode: 'parallel',
    maxConcurrency: 5,
    scheduleEnabled: true,
    scheduledAt: timestamp,
  }, verifiedAccount, {
    kind: 'template', key: 'template:key', templateId: 'template-id', flowName: '采购申请', code: '',
    accountName: 'tester01', typeName: '', groupName: '', statusText: '', updateTime: '', remark: '',
    flowCreateType: '', formExist: '', formTemplateCount: 0,
  })
  assert.equal(request.maxConcurrency, 5)
  assert.equal(request.scheduledAt, '2026-07-29T09:30:00.000Z')
})

test('持久化计划映射真实列表语言且拒绝未知状态', () => {
  const row = toPlanRow({
    id: '42', name: '采购回归', account: 'tester01', accountDisplayName: '测试专员', flowSource: 'new',
    targetObjectId: 'template-id', targetObjectName: '采购申请', runMode: 'serial', maxConcurrency: null,
    scheduledAt: null, status: 'not_started', pathCount: 0, lastRunResult: '',
    createdAt: '2026-07-28T00:00:00Z', updatedAt: '2026-07-28T00:00:00Z',
  })
  assert.equal(row.accountName, '测试专员（tester01）')
  assert.equal(row.flowName, '采购申请')
  assert.equal(row.pathCount, 0)
  assert.equal(row.lastRunResult, '暂无运行记录')
  assert.throws(() => toPlanRow({ ...row, account: 'a', accountDisplayName: '', flowSource: 'new', targetObjectId: 'x', targetObjectName: 'x', maxConcurrency: null, createdAt: '', updatedAt: '', status: 'unknown' }), PlanApiError)
})

test('创建请求携带原幂等键，列表和详情读取真实 API', async (t) => {
  const calls = []
  const originalFetch = globalThis.fetch
  t.after(() => { globalThis.fetch = originalFetch })
  globalThis.fetch = async (input, init) => {
    calls.push({ input: String(input), init })
    const plan = {
      id: '42', name: '采购回归', account: 'tester01', accountDisplayName: '测试专员', flowSource: 'new',
      targetObjectId: 'template-id', targetObjectName: '采购申请', runMode: 'serial', maxConcurrency: null,
      scheduledAt: null, status: 'not_started', pathCount: 0, lastRunResult: '', createdAt: '', updatedAt: '',
    }
    const data = String(input).startsWith('/api/plans?') ? { items: [plan] } : plan
    return new Response(JSON.stringify({ success: true, data }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }

  const controller = new AbortController()
  const payload = { name: '采购回归', account: 'tester01', accountDisplayName: '测试专员', flowSource: 'new', targetObjectId: 'template-id', targetObjectName: '采购申请', runMode: 'serial', maxConcurrency: null, scheduledAt: null }
  await createPlan(payload, '123e4567-e89b-12d3-a456-426614174000', controller.signal)
  const rows = await fetchPlans({ name: '采购', status: 'not_started' }, controller.signal)
  const detail = await fetchPlan('42', controller.signal)

  assert.equal(calls[0].init.headers['Idempotency-Key'], '123e4567-e89b-12d3-a456-426614174000')
  assert.equal(calls[1].input, '/api/plans?name=%E9%87%87%E8%B4%AD&status=not_started')
  assert.equal(calls[2].input, '/api/plans/42')
  assert.equal(rows[0].id, '42')
  assert.equal(detail.targetObjectName, '采购申请')
})

test('失败响应保留稳定错误码和重试属性', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => { globalThis.fetch = originalFetch })
  globalThis.fetch = async () => new Response(JSON.stringify({
    success: false,
    error: { code: 'PLAN_STORAGE_UNAVAILABLE', message: '计划存储暂不可用，请重试', retryable: true },
  }), { status: 503, headers: { 'Content-Type': 'application/json' } })
  await assert.rejects(
    fetchPlans({ name: '', status: null }, new AbortController().signal),
    (error) => error instanceof PlanApiError && error.code === 'PLAN_STORAGE_UNAVAILABLE' && error.retryable,
  )
})
