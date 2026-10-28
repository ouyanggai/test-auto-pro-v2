// 动作分发的单元测试：直接调用生产代码 native-dispatch.ts 的 nativeDispatch /
// nativeChoiceContinue（server.ts 经 Function 源串把同一份实现送进页面执行），
// 断言打在生产分发路径上，不再测被生产弃用的平行实现（§8.2）。
import assert from 'node:assert/strict'
import test from 'node:test'

import { nativeChoiceContinue, nativeDispatch, type NativeCaptured } from '../../../executor-service/src/native-dispatch.ts'

interface VNodeLike {
  $options?: { name?: string }
  $children?: VNodeLike[]
  [key: string]: unknown
}

// makeTree 构造最小组件树：捕获实例（initFlow=InitFlowDialog 根 / flowDetail=匿名根）
// 下挂指定命名的业务组件，与真实组件树层级一致。
function makeTree (rootName: string, componentName: string, component: Record<string, unknown>): NativeCaptured & { calls: string[] } {
  const calls: string[] = []
  const leaf: VNodeLike = { $options: { name: componentName }, ...component }
  const root: VNodeLike = { $options: { name: rootName }, $children: [leaf] }
  const captured: NativeCaptured = rootName === 'InitFlowDialog' ? { initFlow: root } : { flowDetail: root }
  return { ...captured, calls } as NativeCaptured & { calls: string[] }
}

test('保存草稿走 FlowDialog.sumbitFlow("draft")，不提前 submit', () => {
  const tree = makeTree('InitFlowDialog', 'FlowDialog', { sumbitFlow: (_: unknown) => undefined })
  const result = nativeDispatch(tree, { action: 'save_draft' })
  assert.equal(result.component, 'FlowDialog')
  assert.match(result.method, /draft/)
})

test('已有实例再次保存草稿走 ExamineOpinion.saveSubmit("save")', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', { saveSubmit: (kind: unknown) => { calls.push(String(kind)) } })
  const result = nativeDispatch(tree, { action: 'save_draft', loadMode: 'edit' })
  assert.equal(result.component, 'ExamineOpinion')
  assert.deepEqual(calls, ['save'])
})

test('提交走 sumbitFlow("submit")', () => {
  const calls: string[] = []
  const tree = makeTree('InitFlowDialog', 'FlowDialog', { sumbitFlow: (kind: unknown) => { calls.push(String(kind)) } })
  nativeDispatch(tree, { action: 'submit' })
  assert.deepEqual(calls, ['submit'])
})

test('暂存走 temporaryStorage，不走同意或提交', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    temporaryStorage: () => { calls.push('storage') },
    formMakingFormBusiness: () => { calls.push('audit') },
    saveSubmit: () => { calls.push('submit') }
  })
  nativeDispatch(tree, { action: 'storage_form_data' })
  assert.deepEqual(calls, ['storage'])
})

test('无表单暂存入口回退到 temporarySaveBizData', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', { temporarySaveBizData: () => { calls.push('custom-storage') } })
  nativeDispatch(tree, { action: 'storage_form_data' })
  assert.deepEqual(calls, ['custom-storage'])
})

test('转发调用列表组件 confirmTranspond', () => {
  const calls: string[] = []
  const host = { $options: { name: 'Backlog' }, confirmTranspond: () => { calls.push('forward') } }
  const result = nativeDispatch({ forwardList: host }, { action: 'forward', loadMode: 'forward' })
  assert.equal(result.method, 'confirmTranspond()')
  assert.deepEqual(calls, ['forward'])
})

test('转发保留接收人与历史勾选，并在调用原生方法前写入当前附言，包括空文本', () => {
  for (const text of ['', '本次附言']) {
    for (const checkList of [[], ['fuyan']]) {
      const values: unknown[] = []
      const host = {
        $options: { name: 'Backlog' },
        approveForm: { approveMessage: '旧文本', personId: 'receiver-1', checkList },
        confirmTranspond () { values.push({ ...this.approveForm }) }
      }
      nativeDispatch({ forwardList: host }, { action: 'forward', loadMode: 'forward', auditDesc: text })
      assert.deepEqual(values, [{ approveMessage: text, personId: 'receiver-1', checkList }])
    }
  }
})

test('同意/不同意走 submitCheckBefore 原生入口且互不误触', () => {
  const calls: string[] = []
  const make = () => makeTree('flowDetail', 'ExamineOpinion', {
    approveForm: { approveMessage: '' },
    submitCheckBefore: (type: unknown) => { calls.push(String(type)) }
  })
  nativeDispatch(make(), { action: 'approve' })
  nativeDispatch(make(), { action: 'reject' })
  const tree = make()
  nativeDispatch(tree, { action: 'approve', auditDesc: '同意意见' })
  assert.equal(((tree.flowDetail ?? tree.initFlow) as unknown as { $children: Array<{ approveForm: { approveMessage: string } }> }).$children[0]!.approveForm.approveMessage, '同意意见')
  assert.deepEqual(calls, ['pass', 'no_pass', 'pass'])
})
test('合同盖章审批缺失自定义业务组件时，阻止普通审核写请求', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    approveForm: { approveMessage: '' },
    selectFlowType: 'contract_seal_review',
    dealFormBusiness: function (this: unknown, func: (flag: unknown, type: unknown) => unknown, flag: unknown, type: unknown) {
      // 原生实现没有判空直接访问未挂载的自定义组件：
      const generateForm = (this as { $parent?: { $refs?: { generateForm?: { getComponent: (name: string) => unknown } } } })?.$parent?.$refs?.generateForm
      const custom = generateForm?.getComponent('custom_contractSealField') as { contractBusiness: unknown }
      custom.contractBusiness // 必定报错 TypeError: Cannot read properties of undefined
      func(flag, type)
    },
    submitCheckBefore: function (this: unknown, type: unknown) {
      (this as { dealFormBusiness: (func: (flag: unknown, type: unknown) => unknown, flag: unknown, type: unknown) => unknown }).dealFormBusiness(
        (flag, t) => { calls.push(`audited-${String(t)}`) },
        true,
        type
      )
    }
  })
  assert.throws(() => nativeDispatch(tree, { action: 'approve' }), /自定义业务前置处理失败|缺少自定义业务组件/)
  assert.deepEqual(calls, [])
})

test('重提走 saveSubmit("submit")', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', { saveSubmit: (type: unknown) => { calls.push(String(type)) } })
  nativeDispatch(tree, { action: 'resubmit' })
  assert.deepEqual(calls, ['submit'])
})

test('审批意见先写入 approveForm.approveMessage 再分发', () => {
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    approveForm: { approveMessage: '' },
    temporaryStorage: () => undefined
  })
  nativeDispatch(tree, { action: 'storage_form_data', auditDesc: '暂存意见' })
  const opinion = ((tree.flowDetail ?? tree.initFlow) as unknown as { $children: Array<{ approveForm: { approveMessage: string } }> }).$children[0]!
  assert.equal(opinion.approveForm.approveMessage, '暂存意见')
})

test('人员注入：空 bizId/name 拒绝写入', () => {
  const tree = makeTree('flowDetail', 'ExamineOpinion', { temporaryStorage: () => undefined })
  assert.throws(
    () => nativeDispatch(tree, { action: 'storage_form_data', nextNodeAuditors: [{ nodeProxyId: 'n1', bizId: '', name: '张三' }] }),
    /空 bizId\/name/
  )
})

test('组件不唯一或缺失时抛错，绝不静默跳过', () => {
  const dialog: VNodeLike = { $options: { name: 'InitFlowDialog' }, $children: [] }
  assert.throws(() => nativeDispatch({ initFlow: dialog }, { action: 'save_draft' }), /命中 0 个/)
})

test('未知动作抛错', () => {
  // 组件树里连审批组件都没有：报组件缺失而非动作未知。
  const tree = makeTree('InitFlowDialog', 'FlowDialog', { sumbitFlow: () => undefined })
  assert.throws(() => nativeDispatch(tree, { action: 'withdraw' }), /命中 0 个/)
  // 组件齐全但动作不在协议内：报尚未接入。
  const withOpinion = makeTree('flowDetail', 'ExamineOpinion', { temporaryStorage: () => undefined })
  assert.throws(() => nativeDispatch(withOpinion, { action: 'withdraw' }), /尚未接入/)
})

test('选择继续 custom_choose：写 chooseBranchList + chooseBranchNode 后走原生继续', () => {
  const calls: string[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    chooseBranchList: [],
    handleSaveBranchChooseNode: () => { calls.push('branch-continue') }
  })
  const result = nativeChoiceContinue(tree, 'custom_choose', { kind: 'branch', branchEntryNodeProxyIds: ['entry-9'] }, {
    data: { branchNodes: [{ id: 'entry-9', flowNodeAuditConfig: { auditType: 'company' } }] }
  })
  const opinion = ((tree.flowDetail ?? tree.initFlow) as unknown as { $children: Array<Record<string, unknown>> }).$children[0]!
  assert.deepEqual(opinion.chooseBranchList, ['entry-9'])
  assert.deepEqual((opinion.chooseBranchNode as Record<string, unknown>).nextNodeTemplateId, 'entry-9')
  assert.deepEqual(result, { component: 'ExamineOpinion', method: 'handleSaveBranchChooseNode' })
  assert.deepEqual(calls, ['branch-continue'])
})
test('选择继续 custom_choose 发起态（FlowDialog）：写 chooseBranchList + nextNodeProxyId + chooseBranchNode 后走原生继续', () => {
  const calls: string[] = []
  const tree = makeTree('InitFlowDialog', 'FlowDialog', {
    chooseBranchList: [],
    handleSaveBranchChooseNode: () => { calls.push('flow-dialog-branch-continue') }
  })
  const result = nativeChoiceContinue(tree, 'custom_choose', { kind: 'branch', branchEntryNodeProxyIds: ['entry-init-1'] }, {
    data: { branchNodes: [{ id: 'entry-init-1', flowNodeAuditConfig: { auditType: 'company' } }] }
  })
  const dialog = ((tree.initFlow ?? tree.flowDetail) as unknown as { $children: Array<Record<string, unknown>> }).$children[0]!
  assert.deepEqual(dialog.chooseBranchList, ['entry-init-1'])
  assert.deepEqual(dialog.nextNodeProxyId, 'entry-init-1')
  assert.deepEqual(dialog.chooseBranchNode, { nextNodeTemplateId: 'entry-init-1', auditType: 'company' })
  assert.deepEqual(result, { component: 'FlowDialog', method: 'handleSaveBranchChooseNode' })
  assert.deepEqual(calls, ['flow-dialog-branch-continue'])
})

test('选择继续 run_node_choose：以 checkboxPersonGroup 调 getSelectPerson', () => {
  const calls: unknown[] = []
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    getSelectPerson: (data: unknown) => { calls.push(data) }
  })
  nativeChoiceContinue(tree, 'run_node_choose', {
    kind: 'auditor',
    nextAuditors: [{ nodeProxyId: 'n1', bizId: 'biz-1', name: '张三', auditDetailType: 'personnel' }]
  }, null)
  assert.deepEqual(calls, [{ checkboxPersonGroup: [{ name: '张三', id: 'biz-1', nodeProxyId: 'n1' }] }])
})

test('自选分支缺已配置人员时拒绝继续（绝不带空人员提交）', () => {
  const tree = makeTree('flowDetail', 'ExamineOpinion', {
    nextAuditorList: [],
    handleSaveBranchChooseNode: () => undefined
  })
  assert.throws(
    () => nativeChoiceContinue(tree, 'custom_choose', { kind: 'branch', branchEntryNodeProxyIds: ['entry-9'] }, {
      data: { branchNodes: [{ id: 'entry-9', flowNodeAuditConfig: { auditType: 'run_node_choose' } }] }
    }),
    /未配置人员/
  )
})
