import assert from 'node:assert/strict'
import test from 'node:test'

import { ActionBlockedError, canContinueChoice, dispatchChoiceContinue } from '../../../executor-service/src/native-actions.ts'

test('三种选择错误只允许继续一次', () => {
  const state = { continued: false }
  assert.equal(canContinueChoice('custom_choose', state, true), true)
  assert.throws(
    () => canContinueChoice('custom_choose', state, true),
    (error: unknown) => error instanceof ActionBlockedError && String(error.message).includes('一次')
  )
})

test('选择数据不完整时阻塞', () => {
  assert.throws(
    () => canContinueChoice('run_node_choose', { continued: false }, false),
    (error: unknown) => error instanceof ActionBlockedError && String(error.message).includes('不完整')
  )
})

test('选择继续分发到原生方法', () => {
  const calls: string[] = []
  dispatchChoiceContinue({ handleSaveBranchChooseNode: () => calls.push('branch') }, 'custom_choose')
  dispatchChoiceContinue({ handleSaveParallelChooseNode: () => calls.push('parallel') }, 'parallel_choose')
  dispatchChoiceContinue({ getSelectPerson: () => calls.push('person') }, 'run_node_choose')
  assert.deepEqual(calls, ['branch', 'parallel', 'person'])
})
