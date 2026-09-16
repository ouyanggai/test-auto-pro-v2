import assert from 'node:assert/strict'
import test from 'node:test'

import { InitiateBlockedError, selectBoundTemplate } from '../../../executor-service/src/native-initiate.ts'

test('同 auditWay 两条模板时选中计划绑定项', () => {
  const confirmed: string[] = []
  const found: string[] = []
  const result = selectBoundTemplate({
    params: { auditWay: 'employee_leave', flowTemplateId: 'bound-2' },
    flowList: [{ id: 'bound-1' }, { id: 'bound-2' }],
    confirm: id => confirmed.push(id),
    getFlowFindById: id => found.push(id)
  }, 'bound-2')
  assert.equal(result.via, 'confirm')
  assert.deepEqual(confirmed, ['bound-2'])
  assert.deepEqual(found, [])
})

test('单模板直接进入绑定项', () => {
  const found: string[] = []
  const result = selectBoundTemplate({
    params: { auditWay: 'employee_leave' },
    flowList: [{ id: 'only-1' }],
    getFlowFindById: id => found.push(id)
  }, 'only-1')
  assert.equal(result.via, 'single')
  assert.deepEqual(found, ['only-1'])
})

test('只传 flowTemplateId 不传 auditWay 时阻塞', () => {
  assert.throws(
    () => selectBoundTemplate({ params: { flowTemplateId: 'bound-1' }, flowList: [{ id: 'bound-1' }] }, 'bound-1'),
    (error: unknown) => error instanceof InitiateBlockedError && String(error.message).includes('auditWay')
  )
})

test('查询空列表直接装载（单模板场景）', () => {
  const handle = { params: { auditWay: 'approval' }, flowList: [] }
  const result = selectBoundTemplate(handle, 'tpl-999')
  assert.strictEqual(result.selectedId, 'tpl-999')
  assert.strictEqual(result.via, 'direct')
})

test('结果不含绑定模板时阻塞且不调用确认', () => {
  const confirmed: string[] = []
  assert.throws(
    () => selectBoundTemplate({
      params: { auditWay: 'employee_leave' },
      flowList: [{ id: 'other-1' }, { id: 'other-2' }],
      confirm: id => confirmed.push(id)
    }, 'bound-2'),
    (error: unknown) => error instanceof InitiateBlockedError && String(error.message).includes('不含计划绑定模板')
  )
  assert.deepEqual(confirmed, [])
})
