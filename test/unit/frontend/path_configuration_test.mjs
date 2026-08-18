import assert from 'node:assert/strict'
import test from 'node:test'

import { buildPathConfigNodeSavePayload, copyPathConfigActions, currentNodeConfigurationComplete, initPathConfigDraft, normalizedPersonStrategy, validPathConfigActions } from '../../../web/src/features/path-configuration/logic.ts'

const node = {
  key: 'node-approval', name: '审批', kind: 'common', typeName: '审批', status: 'pending', statusName: '待配置', lineBlocked: false,
  fields: [], persons: [], gaps: [], requirements: [],
  actionConfiguration: { catalog: [
    { kind: 'reject_no_pass', label: '不同意', description: '', enabled: true, disabledReason: '', requiresPerson: false },
  ], actions: [], affected: false, note: '' },
}

// TestF008FrontendUsesIndependentActionConfiguration 验证动作草稿只含新动作契约。
test('F-008 动作草稿按顺序保存且不包含旧计划字段', () => {
  const configuration = { path: { sequenceNo: 1, name: '测试' }, revision: 0, nodeRevision: 0, status: 'pending', progress: { total: 1, completed: 0, pending: 1 }, nextNodeKey: '', groups: [{ title: '主线', kind: 'main', nodes: [node] }], warnings: [], form: { revision: 0, status: 'empty', statusName: '空', readOnly: false, template: {}, permissions: [], values: {}, seed: 1, generatedFieldPaths: [], manualOverridePaths: [], sampleSummary: { saved: false, defaults: 0, recent: 0, fallback: 0 }, validated: false, unsupported: [], affected: [], autoFilled: 0, manualPending: 0, conditionBindings: [], conditionReviews: [], fieldRules: [] }, actionCycles: [], preparation: { preparedNodes: 0, pendingItems: 1, included: false } }
  const draft = initPathConfigDraft(configuration)
  draft.actionConfigurations[node.key] = [{ key: 'action-1', kind: 'reject_no_pass', count: 2 }]
  const payload = buildPathConfigNodeSavePayload(node, draft)
  assert.deepEqual(payload.actions, [{ key: 'action-1', kind: 'reject_no_pass', count: 2 }])
  assert.equal('actionPlan' in payload, false)
  assert.equal(validPathConfigActions(node, payload.actions), true)
  assert.equal(validPathConfigActions(node, []), true)
  assert.equal(validPathConfigActions(node, [
    { key: 'action-1', kind: 'reject_no_pass', count: 1 },
    { key: 'action-2', kind: 'reject_no_pass', count: 1 },
  ]), false)
  assert.equal(validPathConfigActions(node, [{ key: 'approve', kind: 'approve_pass', count: 1 }]), false)
})

// TestF008FrontendCopiesActionRows 验证动作行复制后不携带任意目标节点。
test('F-008 动作复制保持次数与顺序', () => {
  const copied = copyPathConfigActions([{ key: 'a', kind: 'reject_no_pass', count: 3 }])
  assert.deepEqual(copied, [{ key: 'a', kind: 'reject_no_pass', count: 3 }])
  assert.equal(Object.prototype.hasOwnProperty.call(copied[0], 'target'), false)
})

test('F-008 人员从范围随机切换手动时保留候选人姓名对应的值', () => {
  const person = {
    key: 'person-1', title: '审批人', strategy: 'random', strategySeed: 7, selected: ['person-token-a'], defaultSelected: [],
    options: [{ label: '张三', value: 'person-token-a' }, { label: '李四', value: 'person-token-b' }],
    strategies: [{ value: 'random', label: '范围随机' }, { value: 'manual', label: '手动选择' }],
    multiple: false, required: true, minCount: 1, maxCount: 1,
  }
  const manual = normalizedPersonStrategy(person, { key: person.key, strategy: 'manual', seed: 7, selected: ['person-token-b'] })
  assert.deepEqual(manual.selected, ['person-token-b'])
})

test('F-008 批量保存不把无动作目录的节点误报为待配置', () => {
  const noActionNode = { ...node, actionConfiguration: { ...node.actionConfiguration, catalog: [], actions: [] } }
  const draft = initPathConfigDraft({ groups: [{ nodes: [noActionNode] }] })
  assert.deepEqual(currentNodeConfigurationComplete(noActionNode, draft), { missing: [], complete: true })
})
