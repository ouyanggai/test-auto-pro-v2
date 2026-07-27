import assert from 'node:assert/strict'
import test from 'node:test'

import { getPlanAction, planActionByStatus } from '../../../web/src/features/plans/logic.ts'

test('四种状态分别映射唯一主要动作', () => {
  assert.equal(Object.keys(planActionByStatus).length, 4)
  assert.deepEqual(getPlanAction('pending_configuration'), { label: '继续配置', intent: 'configure' })
  assert.deepEqual(getPlanAction('ready'), { label: '开始运行', intent: 'start' })
  assert.deepEqual(getPlanAction('running'), { label: '查看运行', intent: 'view_running' })
  assert.deepEqual(getPlanAction('completed'), { label: '查看结果', intent: 'view_result' })
})
