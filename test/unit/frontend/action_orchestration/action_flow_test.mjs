import assert from 'node:assert/strict'
import test from 'node:test'

import {
  pathActionFlowLabels,
  pathActionFlowParameters,
  pathActionFlowSegments,
  pathActionFlowStepName,
} from '../../../../web/src/features/path-configuration/logic.ts'

const configuration = {
  instanceActionKey: 'instance-key',
  instanceActions: { catalog: [{ kind: 'withdraw', label: '撤回实例' }], actions: [] },
  groups: [
    {
      title: '审批',
      kind: 'approval',
      nodes: [
        { key: 'node-apply', name: '发起申请', actionConfiguration: { catalog: [{ kind: 'submit', label: '提交' }], actions: [] } },
        { key: 'node-review', name: '部门审批', actionConfiguration: { catalog: [], actions: [{ kind: 'approve', label: '同意' }] } },
      ],
    },
  ],
}

// 分支节点不是可配置节点，它的名字只能从画布节点取。
const graph = { nodes: [{ id: 'graph-condition', name: '金额判断' }, { id: 'graph-review', name: '部门审批' }] }
const graphNodeIDByConfigurationKey = new Map([['node-condition', 'graph-condition'], ['node-review', 'graph-review']])

const labels = pathActionFlowLabels(configuration, graph, graphNodeIDByConfigurationKey)

// 流程图上的名字必须来自服务端目录与画布，节点键和实例键都是内部标识。
test('F-012 流程图中文名来自节点目录与画布节点', () => {
  assert.deepEqual(labels.actions, { withdraw: '撤回实例', submit: '提交', approve: '同意' })
  assert.deepEqual(labels.nodes, {
    'instance-key': '实例级动作',
    'node-apply': '发起申请',
    'node-review': '部门审批',
    'node-condition': '金额判断',
  })
  assert.deepEqual(pathActionFlowLabels(null, graph, graphNodeIDByConfigurationKey), { actions: {}, nodes: {} })
})

// 步骤按 sequence 排序后按所属节点切段，当前节点必须被标出来。
test('F-012 编译步骤按语义节点分段并标出当前节点', () => {
  const steps = [
    { sequence: 3, source: 'system_navigation', action: 'system_automatic', nodeKey: 'node-condition' },
    { sequence: 1, source: 'user', action: 'submit', nodeKey: 'node-apply' },
    { sequence: 4, source: 'user', action: 'approve', nodeKey: 'node-review' },
    { sequence: 2, source: 'system_recovery', action: 'retrieve', nodeKey: 'node-apply' },
  ]

  const segments = pathActionFlowSegments(steps, labels, 'node-review')

  assert.deepEqual(segments.map(segment => [segment.title, segment.current, segment.steps.map(step => step.sequence)]), [
    ['发起申请', false, [1, 2]],
    ['金额判断', false, [3]],
    ['部门审批', true, [4]],
  ])
  assert.deepEqual(steps.map(step => step.sequence), [3, 1, 4, 2])
})

// 画布上也找不到的节点键不能直接显示出来，只能退回通用说明。
test('F-012 无法解析的节点键退回通用说明', () => {
  const segments = pathActionFlowSegments([{ sequence: 1, source: 'system_navigation', action: 'system_automatic', nodeKey: 'node-removed' }], labels, 'node-review')

  assert.equal(segments[0].title, '流程中的其他节点')
  assert.equal(segments[0].nodeKey, 'node-removed')
})

// 没有节点键的实例级步骤不能显示成某个节点上的步骤。
test('F-012 缺少节点键的步骤归入整个流程实例', () => {
  const segments = pathActionFlowSegments([{ sequence: 1, source: 'user', action: 'withdraw' }], labels, 'node-review')

  assert.equal(segments.length, 1)
  assert.equal(segments[0].title, '整个流程实例')
  assert.equal(segments[0].current, false)
  assert.deepEqual(pathActionFlowSegments([], labels, 'node-review'), [])
})

// 系统自动步骤没有用户动作语义，目录缺名时才退回动作键。
test('F-012 步骤名优先使用中文动作名', () => {
  assert.equal(pathActionFlowStepName({ action: 'system_automatic' }, labels), '目标引擎自动处理')
  assert.equal(pathActionFlowStepName({ action: 'approve' }, labels), '同意')
  assert.equal(pathActionFlowStepName({ action: 'urge' }, labels), 'urge')
})

// 参数由服务端编译给出，界面只做可读展开。
test('F-012 编译参数展开为可读文本', () => {
  assert.deepEqual(pathActionFlowParameters({ parameters: { comment: '同意', persons: ['a', 'b'], empty: null } }), [
    'comment=同意',
    'persons=["a","b"]',
    'empty=null',
  ])
  assert.deepEqual(pathActionFlowParameters({}), [])
})
