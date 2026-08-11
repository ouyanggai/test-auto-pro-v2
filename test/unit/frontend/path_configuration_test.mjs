import assert from 'node:assert/strict'
import test from 'node:test'

import {
  allEditableFieldsFilled,
  canSavePathConfiguration,
  applyPathConfigDraft,
  bindPathConfigurationNodes,
  buildPathConfigNodeSavePayload,
  buildPathConfigSavePayload,
  encodePathConfigValue,
  hasPathConfigDraftChanges,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  currentNodeConfigurationComplete,
  hasCurrentNodeDraftChanges,
  nextFormGenerationSeed,
  pathConfigNodeKey,
  pathConfigurationNodesByGraphID,
  parsePathConfigValue,
  projectPathConfigurationNodeStates,
  reconcilePathConfigDraft,
  resolveConfirmedNodeSaveDestination,
} from '../../../web/src/features/path-configuration/logic.ts'

const configuration = {
  path: { sequenceNo: 1, name: '财务路径' },
  revision: 2,
  status: 'configured',
  progress: { total: 2, completed: 2, pending: 0 },
  nextNodeKey: '',
  warnings: [],
  groups: [
    {
      title: '主线',
      kind: 'main',
      nodes: [
        {
          key: 'e6900f7404ce5bff4c1835f949b38c86',
          name: '发起',
          typeName: '发起',
          kind: 'start',
          status: 'configured',
          statusName: '已完成',
          lineBlocked: false,
          fields: [],
          persons: [],
          gaps: [],
          requirements: [],
          actions: [
            { key: 'action-submit', kind: 'submit', label: '发起动作', current: 'submit', default: 'submit', options: [{ value: 'submit', label: '提交' }], disagreeWarning: '' },
          ],
        },
        {
          key: 'cdba0a9513cd968ebdc32f2cbb516267',
          name: '财务审批',
          typeName: '审批',
          kind: 'common',
          status: 'configured',
          statusName: '已完成',
          lineBlocked: false,
          gaps: [],
          requirements: [{ category: 'person', title: '处理人员', detail: '发起时指定', status: 'configured' }],
          fields: [
            { key: 'field-amount', name: '申请金额', type: 'number', required: true, value: '2500', options: [], editable: true, affected: false, note: '' },
            { key: 'field-type', name: '类型', type: 'singleSelect', required: false, value: '"a"', options: [{ label: 'A', value: 'a' }, { label: 'B', value: 'b' }], editable: true, affected: false, note: '' },
            { key: 'field-tags', name: '标签', type: 'multiSelect', required: false, value: '["a"]', options: [{ label: 'A', value: 'a' }], editable: true, affected: false, note: '' },
            { key: 'field-subform', name: '明细', type: 'text', required: false, value: '', options: [], editable: false, affected: false, note: '' },
          ],
          persons: [
            {
              key: 'person-approve', title: '审批人', mode: 'select', detail: '从模板合法候选中选择', editable: true,
              multiple: false, required: true, minCount: 1, selected: ['person-a'],
              options: [{ label: '张三', value: 'person-a' }, { label: '李四', value: 'person-b' }], affected: false, note: '',
            },
          ],
          actions: [
            { key: 'action-approve', kind: 'agree_disagree', label: '处理结果', current: 'agree', default: 'agree', options: [{ value: 'agree', label: '同意' }, { value: 'disagree', label: '不同意' }], disagreeWarning: '会改变后续线路' },
          ],
        },
      ],
    },
  ],
}

test('初始化草稿只包含可编辑字段与全部动作', () => {
  const draft = initPathConfigDraft(configuration)
  assert.deepEqual(draft.fields, {
    'field-amount': '2500',
    'field-type': '"a"',
    'field-tags': '["a"]',
  })
  assert.equal(Object.prototype.hasOwnProperty.call(draft.fields, 'field-subform'), false)
  assert.deepEqual(draft.actions, { 'action-submit': 'submit', 'action-approve': 'agree' })
  assert.deepEqual(draft.persons, { 'person-approve': ['person-a'] })
})

test('草稿无变化不提示保存，修改后提示', () => {
  const draft = initPathConfigDraft(configuration)
  assert.equal(hasPathConfigDraftChanges(configuration, draft), false)
  draft.fields['field-amount'] = '3000'
  assert.equal(hasPathConfigDraftChanges(configuration, draft), true)
  draft.fields['field-amount'] = '2500'
  draft.actions['action-approve'] = 'disagree'
  assert.equal(hasPathConfigDraftChanges(configuration, draft), true)
  draft.actions['action-approve'] = 'agree'
  draft.persons['person-approve'] = ['person-b']
  assert.equal(hasPathConfigDraftChanges(configuration, draft), true)
})

test('首次没有配置记录时即使草稿未变化也允许保存，已保存状态仍需真实变化', () => {
  const draft = initPathConfigDraft({ ...configuration, status: 'pending' })
  assert.equal(canSavePathConfiguration({ ...configuration, status: 'pending' }, draft), true)
  assert.equal(canSavePathConfiguration(configuration, initPathConfigDraft(configuration)), false)
})

test('保存成功后配置模型立即成为已保存基线并保留当前结果', () => {
  const draft = initPathConfigDraft({ ...configuration, status: 'pending' })
  draft.fields['field-amount'] = '3000'
  const saved = applyPathConfigDraft({ ...configuration, status: 'pending' }, draft, 3)
  assert.equal(saved.status, 'configured')
  assert.equal(saved.revision, 3)
  assert.equal(saved.groups[0].nodes[1].fields[0].value, '3000')
  assert.equal(hasPathConfigDraftChanges(saved, draft), false)
})

test('字段值按控件类型往返编码', () => {
  const amount = configuration.groups[0].nodes[1].fields[0]
  assert.equal(parsePathConfigValue(amount, '2500'), 2500)
  assert.equal(encodePathConfigValue(amount, 3000), '3000')
  const tags = configuration.groups[0].nodes[1].fields[2]
  assert.deepEqual(parsePathConfigValue(tags, '["a","b"]'), ['a', 'b'])
  assert.equal(encodePathConfigValue(tags, []), '[]')
  assert.equal(encodePathConfigValue(tags, undefined), '[]')
})

test('保存载荷包含全部可编辑字段与动作且不含不可编辑项', () => {
  const draft = initPathConfigDraft(configuration)
  const payload = buildPathConfigSavePayload(configuration, draft)
  assert.equal(payload.fields.length, 3)
  assert.equal(payload.actions.length, 3)
  assert.deepEqual(payload.fields.map((item) => item.key).sort(), ['field-amount', 'field-tags', 'field-type'])
  assert.equal(payload.fields.some((item) => item.key === 'field-subform'), false)
  assert.deepEqual(payload.actions.map((item) => item.action).sort(), ['["person-a"]', 'agree', 'submit'])
})

test('必填字段缺失时提示具体中文名称', () => {
  const draft = initPathConfigDraft(configuration)
  draft.fields['field-amount'] = '""'
  const result = allEditableFieldsFilled(configuration, draft)
  assert.equal(result.complete, false)
  assert.deepEqual(result.missing, ['申请金额'])
  draft.fields['field-amount'] = '3500'
  assert.equal(allEditableFieldsFilled(configuration, draft).complete, true)
  draft.persons['person-approve'] = []
  assert.deepEqual(allEditableFieldsFilled(configuration, draft).missing, ['审批人'])
})

test('可跳过人员零选择合法但主动选择仍满足最低人数', () => {
  const optionalConfiguration = structuredClone(configuration)
  const person = optionalConfiguration.groups[0].nodes[1].persons[0]
  person.required = false
  person.multiple = true
  person.minCount = 2

  const draft = initPathConfigDraft(optionalConfiguration)
  draft.persons[person.key] = []
  assert.equal(allEditableFieldsFilled(optionalConfiguration, draft).complete, true)

  draft.persons[person.key] = ['person-a']
  assert.deepEqual(allEditableFieldsFilled(optionalConfiguration, draft).missing, ['审批人'])

  draft.persons[person.key] = ['person-a', 'person-b']
  assert.equal(allEditableFieldsFilled(optionalConfiguration, draft).complete, true)

  person.required = true
  draft.persons[person.key] = []
  assert.deepEqual(allEditableFieldsFilled(optionalConfiguration, draft).missing, ['审批人'])
})

test('配置节点不透明键与真实图节点稳定绑定且不暴露节点 ID', async () => {
  const graph = {
    planId: '1', targetName: '流程', flowSource: 'new', entryNodeIds: ['start'], warnings: [],
    nodes: [
      { id: 'start', name: '发起', type: 'start', typeName: '发起' },
      { id: 'approve-a', name: '财务审批', type: 'common', typeName: '审批' },
      { id: 'outside', name: '路径外节点', type: 'common', typeName: '审批' },
    ],
    edges: [{ id: 'edge-1', source: 'start', target: 'approve-a', kind: 'sequence', label: '', branchId: '' }],
  }
  assert.equal(await pathConfigNodeKey('start'), 'e6900f7404ce5bff4c1835f949b38c86')
  const bindings = await bindPathConfigurationNodes(graph, configuration)
  assert.equal(bindings.byGraphNodeID.get('approve-a').name, '财务审批')
  assert.equal(bindings.graphNodeIDByKey.get('cdba0a9513cd968ebdc32f2cbb516267'), 'approve-a')
  const pendingConfiguration = { ...configuration, nextNodeKey: 'cdba0a9513cd968ebdc32f2cbb516267' }
  assert.equal(initialPathConfigurationNodeID(pendingConfiguration, bindings.graphNodeIDByKey), 'approve-a')

  const analysis = {
    complete: true,
    invalid: false,
    missingRouteNodeIds: [],
    reachableNodeIds: new Set(['start', 'approve-a']),
    reachableEdgeIds: new Set(['edge-1']),
  }
  const states = projectPathConfigurationNodeStates(graph, analysis, bindings.byGraphNodeID, 'approve-a')
  assert.equal(states.start.interactive, true)
  assert.equal(states['approve-a'].status, 'configured')
  assert.equal(states['approve-a'].selected, true)
  assert.equal(states.outside.interactive, false)
  assert.equal(states.outside.selected, false)
  const rebound = pathConfigurationNodesByGraphID(configuration, bindings.graphNodeIDByKey)
  assert.equal(rebound.get('start').key, 'e6900f7404ce5bff4c1835f949b38c86')

  const foreignConfiguration = structuredClone(configuration)
  foreignConfiguration.groups[0].nodes[0].key = 'foreign-node-key'
  await assert.rejects(
    () => bindPathConfigurationNodes(graph, foreignConfiguration),
    /路径节点配置与当前流程结构不一致/,
  )
})

test('节点保存确认后只推进同一路径节点并正确分流表单完成态', () => {
  const bindings = new Map([['next-key', 'approve-next']])
  assert.deepEqual(resolveConfirmedNodeSaveDestination('next-key', bindings, 'empty'), {
    kind: 'next-node', nodeID: 'approve-next',
  })
  assert.deepEqual(resolveConfirmedNodeSaveDestination('', bindings, 'draft'), { kind: 'form' })
  assert.deepEqual(resolveConfirmedNodeSaveDestination('', bindings, 'valid'), { kind: 'complete' })
  assert.deepEqual(resolveConfirmedNodeSaveDestination('removed-key', bindings, 'valid'), { kind: 'unmapped' })
})

test('目标结构刷新只保留仍可对应字段动作和合法人员候选', () => {
  const draft = initPathConfigDraft(configuration)
  draft.fields['field-amount'] = '3800'
  draft.actions['action-approve'] = 'disagree'
  draft.persons['person-approve'] = ['person-a', 'removed-person']
  draft.fields['removed-field'] = '"secret"'
  const reconciled = reconcilePathConfigDraft(configuration, draft)
  assert.equal(reconciled.fields['field-amount'], '3800')
  assert.equal(Object.prototype.hasOwnProperty.call(reconciled.fields, 'removed-field'), false)
  assert.equal(reconciled.actions['action-approve'], 'disagree')
  assert.deepEqual(reconciled.persons['person-approve'], ['person-a'])
})

test('当前节点保存载荷不覆盖其他节点且保存完整性只看人员动作', () => {
  const draft = initPathConfigDraft(configuration)
  const approval = configuration.groups[0].nodes[1]
  const payload = buildPathConfigNodeSavePayload(approval, draft)
  assert.deepEqual(payload.map(item => item.key).sort(), ['action-approve', 'person-approve'])
  assert.equal(payload.some(item => item.key === 'action-submit'), false)
  assert.equal(currentNodeConfigurationComplete(approval, draft).complete, true)
  assert.equal(hasCurrentNodeDraftChanges(approval, draft), false)
  draft.persons['person-approve'] = []
  assert.deepEqual(currentNodeConfigurationComplete(approval, draft).missing, ['审批人'])
  draft.persons['person-approve'] = ['person-b']
  assert.equal(hasCurrentNodeDraftChanges(approval, draft), true)
})

test('换一组种子稳定推进并安全处理非法值', () => {
  assert.equal(nextFormGenerationSeed(73), 104802)
  assert.equal(nextFormGenerationSeed(0), 1)
  assert.equal(nextFormGenerationSeed(Number.NaN), 1)
})
