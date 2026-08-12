import assert from 'node:assert/strict'
import test from 'node:test'
import { isProxy, reactive } from '../../../web/node_modules/vue/index.mjs'

import {
  allEditableFieldsFilled,
  canSavePathConfiguration,
  applyPathConfigDraft,
  bindPathConfigurationNodes,
  buildPathConfigNodeSavePayload,
  buildPathConfigSavePayload,
  copyPathConfigArrivals,
  encodePathConfigValue,
  hasPathConfigDraftChanges,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  currentNodeConfigurationComplete,
  hasCurrentNodeDraftChanges,
  nextFormGenerationSeed,
  normalizedPathConfigSeed,
  pathConfigNodeKey,
  pathConfigurationNodesByGraphID,
  parsePathConfigValue,
  pathConfigSupplementaryActions,
  projectPathConfigurationNodeStates,
  reconcilePathConfigDraft,
  resolveConfirmedNodeSaveDestination,
  resolvedPersonStrategySelection,
  resizePathConfigArrivals,
  summarizePathConfigPersonItems,
  validPathConfigArrivals,
} from '../../../web/src/features/path-configuration/logic.ts'
import { savePathConfigurationNode } from '../../../web/src/features/path-configuration/api.ts'

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
          actionPlan: {
            catalog: [{ kind: 'submit', label: '提交', description: '提交发起节点', allowsOpinion: false, requiresTarget: false, requiresPerson: false }],
            rollbackTargets: [], arrivals: [{ visit: 1, steps: [{ kind: 'submit', label: '提交', opinion: '', target: '' }] }],
            maxArrivals: 10, maxPathSteps: 100, affected: false, note: '',
          },
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
              items: [{ category: '人员', name: '张三', count: 1 }],
              multiple: false, required: true, minCount: 1, maxCount: 1, selected: ['person-a'], defaultSelected: [],
              strategy: 'manual', strategySeed: 7, strategies: [{ value: 'manual', label: '手动选择' }, { value: 'random', label: '确定性随机' }],
              options: [{ label: '张三', value: 'person-a' }, { label: '李四', value: 'person-b' }], affected: false, note: '',
            },
          ],
          actions: [
            { key: 'action-approve', kind: 'agree_disagree', label: '处理结果', current: 'agree', default: 'agree', options: [{ value: 'agree', label: '同意' }, { value: 'disagree', label: '不同意' }], disagreeWarning: '会改变后续线路' },
          ],
          actionPlan: {
            catalog: [
              { kind: 'approve_pass', label: '同意', description: '继续当前路径', allowsOpinion: true, requiresTarget: false, requiresPerson: false },
              { kind: 'reject_no_pass', label: '不同意', description: '结束当前线路', allowsOpinion: true, requiresTarget: false, requiresPerson: false },
              { kind: 'draft_save', label: '暂存', description: '不推进流程', allowsOpinion: false, requiresTarget: false, requiresPerson: false },
              { kind: 'rollback_previous', label: '回退上一级', description: '回退更早节点', allowsOpinion: true, requiresTarget: true, requiresPerson: false },
            ],
            rollbackTargets: [{ label: '发起', value: 'rollback-start' }],
            arrivals: [{ visit: 1, steps: [{ kind: 'approve_pass', label: '同意', opinion: '', target: '' }] }],
            maxArrivals: 10, maxPathSteps: 100, affected: false, note: '',
          },
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
  assert.equal(draft.personStrategies['person-approve'].strategy, 'manual')
  assert.equal(draft.arrivals['cdba0a9513cd968ebdc32f2cbb516267'][0].steps[0].kind, 'approve_pass')
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
  draft.personStrategies['person-approve'].selected = ['person-b']
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
  draft.personStrategies['person-approve'].selected = []
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
  draft.personStrategies[person.key].selected = []
  assert.equal(allEditableFieldsFilled(optionalConfiguration, draft).complete, true)

  draft.persons[person.key] = ['person-a']
  draft.personStrategies[person.key].selected = ['person-a']
  assert.deepEqual(allEditableFieldsFilled(optionalConfiguration, draft).missing, ['审批人'])

  draft.persons[person.key] = ['person-a', 'person-b']
  draft.personStrategies[person.key].selected = ['person-a', 'person-b']
  assert.equal(allEditableFieldsFilled(optionalConfiguration, draft).complete, true)

  person.required = true
  draft.persons[person.key] = []
  draft.personStrategies[person.key].selected = []
  assert.deepEqual(allEditableFieldsFilled(optionalConfiguration, draft).missing, ['审批人'])
})

test('节点完整性不依赖兼容响应中的表单字段和组件缺口', () => {
  const node = structuredClone(configuration.groups[0].nodes[0])
  node.status = 'partial'
  node.fields = [{ key: 'legacy-field', name: '通用信息选择', type: 'text', required: true, value: '', options: [], editable: false, affected: true, note: '旧字段' }]
  node.gaps = [{ name: '通用信息选择', reason: '旧组件缺口' }]
  const draft = initPathConfigDraft({ ...configuration, groups: [{ ...configuration.groups[0], nodes: [node] }] })
  assert.deepEqual(currentNodeConfigurationComplete(node, draft), { missing: [], complete: true })
})

test('人员规则长列表摘要保留真实总数并提供前三项预览', () => {
  const summary = summarizePathConfigPersonItems([
    { category: '人员', name: '张三', count: 1 },
    { category: '岗位', name: '主任', count: 1 },
    { category: '组织', name: '财务部', count: 1 },
    { category: '岗级', name: '二级岗', count: 12 },
  ])
  assert.equal(summary.total, 15)
  assert.equal(summary.preview.length, 3)
  assert.equal(summary.hidden, 12)
  assert.deepEqual(summary.preview.map(item => item.name), ['张三', '主任', '财务部'])
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
  draft.personStrategies['person-approve'].selected = ['person-a', 'removed-person']
  draft.fields['removed-field'] = '"secret"'
  const reconciled = reconcilePathConfigDraft(configuration, draft)
  assert.equal(reconciled.fields['field-amount'], '3800')
  assert.equal(Object.prototype.hasOwnProperty.call(reconciled.fields, 'removed-field'), false)
  assert.equal(reconciled.actions['action-approve'], 'disagree')
  assert.deepEqual(reconciled.persons['person-approve'], ['person-a'])
  assert.deepEqual(reconciled.personStrategies['person-approve'].selected, ['person-a'])
})

test('当前节点保存载荷不覆盖其他节点且保存完整性只看人员动作', () => {
  const draft = initPathConfigDraft(configuration)
  const approval = configuration.groups[0].nodes[1]
  const payload = buildPathConfigNodeSavePayload(approval, draft)
  assert.deepEqual(payload.persons.map(item => item.key), ['person-approve'])
  assert.equal(payload.arrivals[0].steps[0].kind, 'approve_pass')
  assert.equal(currentNodeConfigurationComplete(approval, draft).complete, true)
  assert.equal(hasCurrentNodeDraftChanges(approval, draft), false)
  draft.persons['person-approve'] = []
  draft.personStrategies['person-approve'].selected = []
  assert.deepEqual(currentNodeConfigurationComplete(approval, draft).missing, ['审批人'])
  draft.persons['person-approve'] = ['person-b']
  draft.personStrategies['person-approve'].selected = ['person-b']
  assert.equal(hasCurrentNodeDraftChanges(approval, draft), true)
})

test('响应式节点动作草稿转为普通载荷并真实发出节点 PUT', async (t) => {
  const approval = configuration.groups[0].nodes[1]
  const draft = reactive(initPathConfigDraft(configuration))
  draft.arrivals[approval.key][0].steps[0].opinion = '同意办理'
  draft.arrivals[approval.key][0].steps[0].person = {
    key: 'person-approve', strategy: 'manual', seed: 7, selected: ['person-a'],
  }
  assert.equal(isProxy(draft.arrivals[approval.key]), true)
  assert.equal(isProxy(draft.arrivals[approval.key][0].steps[0].person.selected), true)

  const copied = copyPathConfigArrivals(draft.arrivals[approval.key])
  const payload = buildPathConfigNodeSavePayload(approval, draft)
  assert.equal(isProxy(copied), false)
  assert.equal(isProxy(payload.arrivals), false)
  assert.equal(isProxy(payload.arrivals[0].steps[0]), false)
  assert.equal(isProxy(payload.arrivals[0].steps[0].person.selected), false)
  payload.arrivals[0].steps[0].opinion = '修改载荷'
  payload.arrivals[0].steps[0].person.selected.push('person-b')
  assert.equal(draft.arrivals[approval.key][0].steps[0].opinion, '同意办理')
  assert.deepEqual(draft.arrivals[approval.key][0].steps[0].person.selected, ['person-a'])

  let request
  const originalFetch = globalThis.fetch
  t.after(() => { globalThis.fetch = originalFetch })
  globalThis.fetch = async (input, init) => {
    request = { input, init }
    return Response.json({ success: true, data: { path: { sequenceNo: 1, name: '财务路径' }, revision: 3, nodeRevision: 4, formRevision: 1, status: 'partial' } })
  }
  await savePathConfigurationNode('7', '32', approval.key, 3, buildPathConfigNodeSavePayload(approval, draft), 'node-save-key')
  assert.equal(request.input, `/api/plans/7/execution-paths/32/configuration/nodes/${approval.key}`)
  assert.equal(request.init.method, 'PUT')
  const body = JSON.parse(request.init.body)
  assert.equal(body.revision, 3)
  assert.equal(body.arrivals[0].steps[0].opinion, '同意办理')
  assert.deepEqual(body.arrivals[0].steps[0].person.selected, ['person-a'])
})

test('人员策略和有界到达动作即时投影与后端规则一致', () => {
  const approval = structuredClone(configuration.groups[0].nodes[1])
  const person = approval.persons[0]
  person.multiple = true
  person.minCount = 2
  person.maxCount = 2
  person.strategies.push({ value: 'all', label: '全部候选' })
  approval.actionPlan.catalog.push({ kind: 'add_sign', label: '加签', description: '受限候选', allowsOpinion: false, requiresTarget: false, requiresPerson: true, person })
  approval.actionPlan.catalog.push({ kind: 'transfer_approver', label: '移交', description: '结束本次动作', allowsOpinion: false, requiresTarget: false, requiresPerson: true, person })
  const random = { key: person.key, strategy: 'random', seed: 1, selected: [] }
  assert.deepEqual(resolvedPersonStrategySelection(person, random), ['person-b', 'person-a'])
  assert.deepEqual(resolvedPersonStrategySelection(person, random), resolvedPersonStrategySelection(person, random))
  assert.equal(normalizedPathConfigSeed(1), 1)
  assert.equal(normalizedPathConfigSeed(Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER)
  assert.equal(normalizedPathConfigSeed(0), 1)
  assert.equal(normalizedPathConfigSeed(-1), 1)
  assert.equal(normalizedPathConfigSeed(Number.MAX_SAFE_INTEGER + 1), 1)
  assert.deepEqual(resolvedPersonStrategySelection(person, { ...random, seed: Number.MAX_SAFE_INTEGER }), ['person-b', 'person-a'])
  assert.deepEqual(resolvedPersonStrategySelection(person, { ...random, seed: -9 }), ['person-b', 'person-a'])
  const valid = [{ visit: 1, steps: [{ kind: 'rollback_previous', opinion: '退回补充', target: 'rollback-start' }] }]
  assert.equal(validPathConfigArrivals(approval, valid), true)
  assert.equal(validPathConfigArrivals(approval, [{ visit: 2, steps: valid[0].steps }]), false)
  assert.equal(validPathConfigArrivals(approval, [{ visit: 1, steps: [{ kind: 'approve_pass', opinion: '', target: '' }, { kind: 'draft_save', opinion: '', target: '' }] }]), false)
  const partialSign = [{ visit: 1, steps: [
    { kind: 'add_sign', opinion: '', target: '', person: { key: person.key, strategy: 'manual', seed: 1, selected: ['person-a'] } },
    { kind: 'approve_pass', opinion: '', target: '' },
  ] }]
  assert.equal(validPathConfigArrivals(approval, partialSign), false)
  partialSign[0].steps[0].person.selected.push('person-b')
  assert.equal(validPathConfigArrivals(approval, partialSign), true)
  const transfer = { kind: 'transfer_approver', opinion: '', target: '', person: { key: person.key, strategy: 'all', seed: 1, selected: [] } }
  assert.equal(validPathConfigArrivals(approval, [{ visit: 1, steps: [transfer] }]), true)
  assert.equal(validPathConfigArrivals(approval, [{ visit: 1, steps: [transfer, { kind: 'approve_pass', opinion: '', target: '' }] }]), false)
  const overflow = Array.from({ length: 100 }, () => ({
    kind: 'add_sign', opinion: '', target: '', person: { key: person.key, strategy: 'all', seed: 1, selected: [] },
  }))
  overflow.push({ kind: 'approve_pass', opinion: '', target: '' })
  assert.equal(validPathConfigArrivals(approval, [{ visit: 1, steps: overflow }]), false)
  assert.deepEqual(pathConfigSupplementaryActions(approval).map(item => item.kind), ['add_sign'])

  const once = [{ visit: 1, steps: [{ kind: 'approve_pass', opinion: '保留', target: '' }] }]
  const three = resizePathConfigArrivals(reactive(once), 3, 10, 100, 'approve_pass')
  assert.equal(three.length, 3)
  assert.deepEqual(three.map(item => item.visit), [1, 2, 3])
  assert.equal(isProxy(three), false)
  three[1].steps[0].opinion = '独立修改'
  assert.equal(three[0].steps[0].opinion, '保留')
  const backToOne = resizePathConfigArrivals(three, 1, 10, 100, 'approve_pass')
  assert.equal(backToOne.length, 1)
})

test('换一组种子稳定推进并安全处理非法值', () => {
  assert.equal(nextFormGenerationSeed(73), 104802)
  assert.equal(nextFormGenerationSeed(0), 1)
  assert.equal(nextFormGenerationSeed(Number.NaN), 1)
})
