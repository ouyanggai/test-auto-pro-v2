import assert from 'node:assert/strict'
import test from 'node:test'

import { buildPathActionConfigurationInput, currentNodeConfigurationComplete } from '../../../../web/src/features/path-configuration/logic.ts'

const node = {
  key: 'node-review',
  persons: [],
  actionConfiguration: { catalog: [], actions: [] },
}

// TestF012ActionPayloadExpandsRepeatedRows 验证重复动作按独立记录发送，不再携带旧 count 字段。
test('F-012 动作请求把重复行展开为独立语义记录', () => {
  const payload = buildPathActionConfigurationInput(node, {
    fields: {},
    persons: {},
    personStrategies: {},
    actionConfigurations: {
      [node.key]: [
        { key: 'draft', kind: 'draft_save', count: 2 },
        { key: 'reject', kind: 'reject_no_pass', count: 1 },
      ],
    },
  }, 7)

  assert.equal(payload.revision, 7)
  assert.deepEqual(payload.persons, [])
  assert.deepEqual(payload.actions, [
    { key: 'draft#1', action: 'storage_form_data', scope: 'task', nodeKey: 'node-review', order: 1 },
    { key: 'draft#2', action: 'storage_form_data', scope: 'task', nodeKey: 'node-review', order: 2 },
    { key: 'reject', action: 'reject', scope: 'task', nodeKey: 'node-review', order: 3 },
  ])
  assert.equal('count' in payload, false)
})

// TestF012ActionPayloadKeepsStableSingleKey 验证单次动作继续使用用户已有稳定键和语义节点。
test('F-012 单次动作请求保留稳定键和节点语义', () => {
  const payload = buildPathActionConfigurationInput(node, {
    fields: {},
    persons: {},
    personStrategies: {},
    actionConfigurations: { [node.key]: [{ key: 'sign', kind: 'add_sign', count: 1 }] },
  }, 3)
  assert.deepEqual(payload.actions, [{ key: 'sign', action: 'add_sign', scope: 'task', nodeKey: node.key, order: 1 }])
})

test('F-012 动作请求合并目标参数并透传人员策略', () => {
  const personNode = {
    ...node,
    persons: [{
      key: 'approver', title: '审批人', mode: 'select', detail: '', items: [], editable: true, multiple: false,
      required: true, minCount: 1, maxCount: 1, selected: ['user-a'], defaultSelected: ['user-a'],
      options: [{ label: '用户 A', value: 'user-a' }], strategy: 'manual', strategySeed: 1,
      strategies: [{ value: 'manual', label: '手动' }], affected: false, note: '',
    }],
  }
  const payload = buildPathActionConfigurationInput(personNode, {
    fields: {},
    persons: { approver: ['user-a'] },
    personStrategies: { approver: { key: 'approver', strategy: 'manual', seed: 3, selected: ['user-a'] } },
    actionConfigurations: {
      [node.key]: [{
        key: 'sign', kind: 'add_sign', count: 1,
        person: { key: 'approver', strategy: 'manual', seed: 3, selected: ['user-a'] },
        parameters: { remark: '需要会签' },
        note: '按审批规则选择',
      }],
    },
  }, 4)

  assert.deepEqual(payload.persons, [{ key: 'approver', strategy: 'manual', seed: 3, selected: ['user-a'] }])
  assert.deepEqual(payload.actions, [{
    key: 'sign', action: 'add_sign', scope: 'task', nodeKey: node.key, order: 1,
    parameters: { remark: '需要会签', actorStrategy: 'manual' }, actorPolicy: 'manual', note: '按审批规则选择',
  }])
})

test('F-012 当前节点校验允许同 kind 独立动作记录', () => {
  const configuredNode = {
    ...node,
    actionConfiguration: {
      catalog: [{ kind: 'reject_no_pass', label: '不同意', description: '', enabled: true, disabledReason: '', requiresPerson: false }],
      actions: [], affected: false, note: '',
    },
  }
  const result = currentNodeConfigurationComplete(configuredNode, {
    fields: {}, persons: {}, personStrategies: {}, actionConfigurations: {
      [node.key]: [
        { key: 'reject-1', kind: 'reject_no_pass', count: 1 },
        { key: 'reject-2', kind: 'reject_no_pass', count: 1 },
      ],
    },
  })
  assert.equal(result.complete, true)
})
