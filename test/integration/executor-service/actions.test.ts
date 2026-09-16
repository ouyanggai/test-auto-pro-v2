import assert from 'node:assert/strict'
import test from 'node:test'

import { dispatchNativeAction } from '../../../executor-service/src/native-actions.ts'

test('草稿走 draft，不提前 submit', () => {
  const calls: string[] = []
  dispatchNativeAction({ sumbitFlow: kind => calls.push(kind) }, 'save_draft')
  assert.deepEqual(calls, ['draft'])
})

test('提交走 submit', () => {
  const calls: string[] = []
  dispatchNativeAction({ sumbitFlow: kind => calls.push(kind) }, 'submit')
  assert.deepEqual(calls, ['submit'])
})

test('暂存不走同意或提交', () => {
  const calls: string[] = []
  dispatchNativeAction({
    sumbitFlow: kind => calls.push(`submit:${kind}`),
    storageFormData: () => calls.push('storage'),
    formMakingFormBusiness: kind => calls.push(`audit:${kind}`)
  }, 'storage_form_data')
  assert.deepEqual(calls, ['storage'])
})

test('同意和不同意不误走对方分支', () => {
  const calls: string[] = []
  dispatchNativeAction({ formMakingFormBusiness: kind => calls.push(kind) }, 'approve')
  dispatchNativeAction({ formMakingFormBusiness: kind => calls.push(kind) }, 'reject')
  assert.deepEqual(calls, ['pass', 'no_pass'])
})

test('转发走独立入口', () => {
  const calls: string[] = []
  dispatchNativeAction({
    sumbitFlow: () => calls.push('submit'),
    confirmTranspond: () => calls.push('forward')
  }, 'forward')
  assert.deepEqual(calls, ['forward'])
})
