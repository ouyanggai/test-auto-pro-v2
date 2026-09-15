// T013 运行详情前端口径：三阶段中文视图、七值状态中文、重试入口只对确定失败开放。
import assert from 'node:assert/strict'
import test from 'node:test'

import {
  RUNTIME_STATUS_LABELS,
  runtimeRetryable,
  runtimeStageViews,
  runtimeStatusLabel
} from '../../../web/src/features/runs/runtimeDetail.ts'

test('T013 三阶段视图按固定顺序输出中文状态', () => {
  const views = runtimeStageViews([
    { name: '表单业务处理', state: 'done', requests: 3 },
    { name: '流程动作', state: 'done', requests: 1 },
    { name: '提交后业务处理', state: 'failed', requests: 2 }
  ])
  assert.deepEqual(views.map(view => view.name), ['表单业务处理', '流程动作', '提交后业务处理'])
  assert.equal(views[0]!.stateLabel, '完成')
  assert.equal(views[2]!.stateLabel, '未完成')
  assert.equal(views[2]!.tone, 'bad')
})

test('T013 七值状态中文：业务失败不显示为流程失败', () => {
  assert.equal(runtimeStatusLabel('business_failed'), '流程已推进，后置业务未完成')
  assert.equal(runtimeStatusLabel('target_rejected'), '目标平台拒绝')
  assert.equal(runtimeStatusLabel('uncertain'), '结果待确认')
  assert.equal(runtimeStatusLabel('mystery'), '结果待确认')
  // 七值全部有中文，绝不渲染原始键。
  for (const status of Object.keys(RUNTIME_STATUS_LABELS)) {
    assert.notEqual(RUNTIME_STATUS_LABELS[status], status)
  }
})

test('T013 重试入口只对确定无副作用的失败开放', () => {
  assert.equal(runtimeRetryable('failed'), true)
  assert.equal(runtimeRetryable('target_rejected'), true)
  assert.equal(runtimeRetryable('uncertain'), false, '结果待确认不提供重试')
  assert.equal(runtimeRetryable('timeout'), false, '超时不提供重试')
  assert.equal(runtimeRetryable('business_failed'), false, '业务失败说明流程已推进，不提供重试')
})
