import assert from 'node:assert/strict'
import test from 'node:test'

import { DetailBlockedError, resolveDetailSemantics } from '../../../executor-service/src/native-detail.ts'

test('audit/edit/view 分别映射原生语义', () => {
  const audit = resolveDetailSemantics('audit', { jobTaskId: 'task-1' })
  assert.deepEqual(audit, {
    mode: 'audit', isExamine: true, isReInitiate: false, operaType: 'examine', actionType: 'examine', taskStatus: 'pending'
  })
  const edit = resolveDetailSemantics('edit', { jobTaskId: 'task-1' })
  assert.equal(edit.isReInitiate, true)
  assert.equal(edit.operaType, 'reEdit')
  const view = resolveDetailSemantics('view', null)
  assert.equal(view.operaType, 'check')
  assert.equal(view.isExamine, false)
})

test('缺少 jobTaskId 的审批装载阻塞', () => {
  assert.throws(
    () => resolveDetailSemantics('audit', { id: 'inst-1' }),
    (error: unknown) => error instanceof DetailBlockedError && String(error.message).includes('jobTaskId')
  )
})
