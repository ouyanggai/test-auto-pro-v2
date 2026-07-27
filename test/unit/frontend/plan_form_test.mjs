import assert from 'node:assert/strict'
import test from 'node:test'

import { getMockFlowCandidates } from '../../fixtures/f001_flow_candidates.ts'
import {
  FLOW_CANDIDATE_BATCH_SIZE,
  calculateNearestScrollDelta,
  filterFlowCandidates,
  isFlowSourceAvailable,
  resolvePostSelectionGuidance,
  takeCandidateBatches,
} from '../../../web/src/features/plans/selection.ts'

test('未验证账号时只允许新发起来源', () => {
  assert.equal(isFlowSourceAvailable('new', false), true)
  assert.equal(isFlowSourceAvailable('started', false), false)
  assert.equal(isFlowSourceAvailable('pending', false), false)
  assert.equal(isFlowSourceAvailable('started', true), true)
  assert.equal(isFlowSourceAvailable('pending', true), true)
})

test('三个来源返回三类语义不同的本地候选', () => {
  const templates = getMockFlowCandidates('new', 'zhangmin')
  const submitted = getMockFlowCandidates('started', 'zhangmin')
  const due = getMockFlowCandidates('pending', 'zhangmin')

  assert.equal(templates[0].kind, 'template')
  assert.equal(submitted[0].kind, 'submitted')
  assert.equal(due[0].kind, 'due')
  assert.equal(templates[0].accountName, 'zhangmin')
  assert.equal(submitted[0].accountName, 'zhangmin')
  assert.equal(due[0].accountName, 'zhangmin')
})

test('搜索只过滤候选列表且支持各来源字段语义', () => {
  assert.ok(filterFlowCandidates(getMockFlowCandidates('new', 'zhangmin'), '采购').length > 0)
  assert.ok(filterFlowCandidates(getMockFlowCandidates('started', 'zhangmin'), '当前节点').length === 0)
  assert.ok(filterFlowCandidates(getMockFlowCandidates('started', 'zhangmin'), '部门负责人').length > 0)
  assert.ok(filterFlowCandidates(getMockFlowCandidates('pending', 'zhangmin'), '草稿').length > 0)
  assert.deepEqual(filterFlowCandidates(getMockFlowCandidates('pending', 'zhangmin'), '不存在的流程'), [])
})

test('候选列表按固定批次增量追加直到全部显示', () => {
  const candidates = getMockFlowCandidates('new', 'zhangmin')
  assert.equal(takeCandidateBatches(candidates, 1).length, FLOW_CANDIDATE_BATCH_SIZE)
  assert.equal(takeCandidateBatches(candidates, 2).length, FLOW_CANDIDATE_BATCH_SIZE * 2)
  assert.equal(takeCandidateBatches(candidates, 99).length, candidates.length)
})

test('选中候选后按未完成字段决定下一步', () => {
  assert.equal(resolvePostSelectionGuidance({
    scheduleEnabled: true,
    scheduledAt: null,
    runMode: 'parallel',
    maxConcurrency: 1,
  }), 'scheduledAt')
  assert.equal(resolvePostSelectionGuidance({
    scheduleEnabled: false,
    scheduledAt: null,
    runMode: 'parallel',
    maxConcurrency: 1,
  }), 'maxConcurrency')
  assert.equal(resolvePostSelectionGuidance({
    scheduleEnabled: false,
    scheduledAt: null,
    runMode: 'parallel',
    maxConcurrency: 2,
  }), 'submit')
})

test('仅在目标区域未完整可见时计算最小滚动距离', () => {
  assert.equal(calculateNearestScrollDelta(120, 180, 100, 300), 0)
  assert.equal(calculateNearestScrollDelta(80, 150, 100, 300), -36)
  assert.equal(calculateNearestScrollDelta(220, 320, 100, 300), 36)
})
