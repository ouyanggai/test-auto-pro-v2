import assert from 'node:assert/strict'
import test from 'node:test'

import { buildPathConfigNodeSavePayload, copyPathConfigActions, initPathConfigDraft, validPathConfigActions } from '../../../web/src/features/path-configuration/logic.ts'

const node = {
  key: 'node-approval', name: '审批', kind: 'common', typeName: '审批', status: 'pending', statusName: '待配置', lineBlocked: false,
  fields: [], persons: [], gaps: [], requirements: [],
  actionConfiguration: { catalog: [
    { kind: 'approve_pass', label: '同意', description: '', enabled: true, disabledReason: '', requiresPerson: false },
    { kind: 'reject_no_pass', label: '不同意', description: '', enabled: true, disabledReason: '', requiresPerson: false },
  ], actions: [], affected: false, note: '' },
}

// TestF008FrontendUsesIndependentActionConfiguration 验证动作草稿只含新动作契约。
test('F-008 动作草稿按顺序保存且不包含旧计划字段', () => {
  const configuration = { path: { sequenceNo: 1, name: '测试' }, revision: 0, nodeRevision: 0, status: 'pending', progress: { total: 1, completed: 0, pending: 1 }, nextNodeKey: '', groups: [{ title: '主线', kind: 'main', nodes: [node] }], warnings: [], form: { revision: 0, status: 'empty', statusName: '空', readOnly: false, template: {}, permissions: [], values: {}, seed: 1, generatedFieldPaths: [], manualOverridePaths: [], sampleSummary: { saved: false, defaults: 0, recent: 0, fallback: 0 }, validated: false, unsupported: [], affected: [], autoFilled: 0, manualPending: 0, conditionHints: [], fieldRules: [] }, actionCycles: [], preparation: { preparedNodes: 0, pendingItems: 1, included: false } }
  const draft = initPathConfigDraft(configuration)
  draft.actionConfigurations[node.key] = [{ key: 'action-1', kind: 'approve_pass', count: 2 }]
  const payload = buildPathConfigNodeSavePayload(node, draft)
  assert.deepEqual(payload.actions, [{ key: 'action-1', kind: 'approve_pass', count: 2 }])
  assert.equal('actionPlan' in payload, false)
  assert.equal(validPathConfigActions(node, payload.actions), true)
})

// TestF008FrontendCopiesActionRows 验证动作行复制后不携带任意目标节点。
test('F-008 动作复制保持次数与顺序', () => {
  const copied = copyPathConfigActions([{ key: 'a', kind: 'reject_no_pass', count: 3 }])
  assert.deepEqual(copied, [{ key: 'a', kind: 'reject_no_pass', count: 3 }])
  assert.equal(Object.prototype.hasOwnProperty.call(copied[0], 'target'), false)
})
