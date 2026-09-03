import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildPathActionConfigurationInput,
  containerActionsDraft,
  hasContainerDraftChanges,
  instanceActionContainer,
  instanceActionsComplete,
  nodeActionContainer,
  validPathConfigActions,
} from '../../../../web/src/features/path-configuration/logic.ts'

const INSTANCE_KEY = 'ae7ff038255a213ff4e8b07d6160db5b'

// catalogItem 构造一个与服务端目录同形的动作项，避免测试放宽真实门禁字段。
function catalogItem(kind, overrides = {}) {
  return {
    kind,
    category: 'instance_management',
    scope: 'instance',
    label: kind,
    description: '',
    enabled: true,
    disabledReason: '',
    requiresPerson: false,
    targetOperation: '',
    parameters: [],
    parameterDetails: [],
    preconditions: [],
    expectedEffect: '',
    requiresReload: false,
    reloadRequirements: [],
    systemOnly: false,
    systemNodeType: '',
    runtimeNote: '',
    ...overrides,
  }
}

function emptyDraft(actionConfigurations = {}) {
  return { fields: {}, persons: {}, personStrategies: {}, actionConfigurations }
}

// TestF012InstanceContainerNeedsRealCatalog 验证没有实例目录时不投影空容器，页面不会出现无门禁的编排入口。
test('F-012 实例动作容器只在服务端返回真实目录时出现', () => {
  assert.equal(instanceActionContainer({ instanceActionKey: '', instanceActions: { catalog: [], actions: [] } }), null)
  assert.equal(instanceActionContainer({ instanceActionKey: INSTANCE_KEY, instanceActions: { catalog: [], actions: [] } }), null)

  const container = instanceActionContainer({
    instanceActionKey: INSTANCE_KEY,
    instanceActions: { catalog: [catalogItem('withdraw')], actions: [], affected: false, note: '' },
  })
  assert.equal(container.key, INSTANCE_KEY)
  assert.deepEqual(container.persons, [])
})

// TestF012InstanceActionsDropNodeKey 验证实例动作以实例作用域保存且不绑定任何语义节点。
test('F-012 实例动作请求不携带语义节点键', () => {
  const container = instanceActionContainer({
    instanceActionKey: INSTANCE_KEY,
    instanceActions: { catalog: [catalogItem('withdraw'), catalogItem('urge')], actions: [], affected: false, note: '' },
  })
  const payload = buildPathActionConfigurationInput(container, emptyDraft({
    [INSTANCE_KEY]: [{ key: 'withdraw-1', kind: 'withdraw' }, { key: 'urge-1', kind: 'urge' }],
  }), 5)

  assert.equal(payload.revision, 5)
  assert.deepEqual(payload.actions, [
    { key: 'withdraw-1', action: 'withdraw', scope: 'instance', nodeKey: undefined, order: 1 },
    { key: 'urge-1', action: 'urge', scope: 'instance', nodeKey: undefined, order: 2 },
  ])
})

// TestF012ContainerDraftFallsBackToSaved 验证未编辑时回落到服务端已保存动作，避免页面把空草稿当成删除。
test('F-012 动作容器草稿未编辑时回落服务端已保存动作', () => {
  const container = nodeActionContainer({
    key: 'node-review',
    persons: [],
    actionConfiguration: { catalog: [catalogItem('approve', { scope: 'task', category: 'current_todo' })], actions: [{ key: 'approve-1', kind: 'approve', label: '同意' }], affected: false, note: '' },
  })

  assert.deepEqual(containerActionsDraft(container, emptyDraft()), [{ key: 'approve-1', kind: 'approve' }])
  assert.equal(hasContainerDraftChanges(container, emptyDraft()), false)
  assert.equal(hasContainerDraftChanges(container, emptyDraft({ 'node-review': [] })), true)
  assert.equal(hasContainerDraftChanges(null, emptyDraft()), false)
})

// TestF012ValidationRejectsGatedActions 验证被门禁禁用的动作和系统只读语义都不能保存。
test('F-012 动作校验拒绝禁用动作与系统只读语义', () => {
  const container = nodeActionContainer({
    key: 'node-review',
    persons: [],
    actionConfiguration: {
      catalog: [
        catalogItem('approve', { scope: 'task', category: 'current_todo' }),
        catalogItem('add_sign', { scope: 'task', category: 'current_todo', enabled: false, disabledReason: '当前节点不是可处理的人工审批或协同节点' }),
        catalogItem('system_automatic', { scope: 'task', category: 'system_automatic', systemOnly: true, systemNodeType: 'condition' }),
      ],
      actions: [], affected: false, note: '',
    },
  })

  assert.equal(validPathConfigActions(container, [{ key: 'a', kind: 'approve' }]), true)
  assert.equal(validPathConfigActions(container, [{ key: 'a', kind: 'add_sign' }]), false)
  assert.equal(validPathConfigActions(container, [{ key: 'a', kind: 'system_automatic' }]), false)
  assert.equal(validPathConfigActions(container, Array.from({ length: 11 }, (_, index) => ({ key: `a-${index}`, kind: 'approve' }))), false)
})

// TestF012InstanceCompletionUsesSameGate 验证实例容器与节点容器共用同一套门禁判定。
test('F-012 实例动作完成度复用同一门禁判定', () => {
  const container = instanceActionContainer({
    instanceActionKey: INSTANCE_KEY,
    instanceActions: {
      catalog: [catalogItem('withdraw'), catalogItem('forward', { enabled: false, disabledReason: '当前实例不允许转发' })],
      actions: [], affected: false, note: '',
    },
  })

  assert.equal(instanceActionsComplete(null, emptyDraft()), true)
  assert.equal(instanceActionsComplete(container, emptyDraft({ [INSTANCE_KEY]: [{ key: 'w', kind: 'withdraw' }] })), true)
  assert.equal(instanceActionsComplete(container, emptyDraft({ [INSTANCE_KEY]: [{ key: 'f', kind: 'forward' }] })), false)
})
