// 原生动作分发的权威实现（S4/S6）：同一份函数在浏览器内被 page.evaluate 执行，
// 也被 Node 侧单元测试直接调用（§8.2：断言必须打在生产分发路径上）。
// 约定：函数只依赖传入的 native 捕获对象（window.__executorNative）与输入，不触碰其它全局。

// NativeCaptured 是注入脚本捕获的原生宿主集合（initFlow=发起弹窗实例，flowDetail=详情弹窗实例）。
export interface NativeCaptured {
  initFlow?: unknown
  flowDetail?: unknown
  forwardList?: unknown
}

// DispatchInput 是动作分发输入：action 必填，其余按动作取用。
export interface DispatchInput {
  action: string
  loadMode?: string
  auditDesc?: string
  nextNodeAuditors?: Array<{ nodeProxyId?: string, bizId?: string, name?: string, auditDetailType?: string }>
}

// DispatchResult 回传调用事实：哪个组件哪个方法被调用（证据断言用）。
export interface DispatchResult {
  component: string
  method: string
}

interface VNode {
  $options?: { name?: string }
  $children?: VNode[]
  [key: string]: unknown
}

// findUniqueComponent 从捕获实例递归按 $options.name 找唯一组件；找不到/多命中抛错。
export function findUniqueComponent (container: VNode, want: string): VNode {
  const found: VNode[] = []
  const walk = (node: VNode): void => {
    if (String(node?.$options?.name ?? '') === want) { found.push(node); return }
    for (const child of node.$children ?? []) walk(child)
  }
  walk(container)
  if (found.length !== 1) throw new Error(`组件 ${want} 命中 ${found.length} 个（应为唯一），拒绝执行`)
  return found[0]!
}

// nativeDispatch 把动作落到真实原生组件。纪律：
//   发起族 → FlowDialog.sumbitFlow；审批族 → ExamineOpinion 的原生前缀方法；
//   审批意见先写 approveForm.approveMessage；人员先并入 nextAuditorList（空人员拒绝）；
//   未知动作/缺失方法/组件不唯一一律抛错，绝不静默跳过。
export function nativeDispatch (native: NativeCaptured | undefined, input: DispatchInput): DispatchResult {
  // 函数会以源码串注入页面执行，必须自包含，不能依赖模块闭包里的辅助函数。
  const find = (root: VNode, want: string): VNode => {
    const found: VNode[] = []
    const walk = (node: VNode): void => {
      if (String(node?.$options?.name ?? '') === want) { found.push(node); return }
      for (const child of node.$children ?? []) walk(child)
    }
    walk(root)
    if (found.length !== 1) throw new Error(`组件 ${want} 命中 ${found.length} 个（应为唯一），拒绝执行`)
    return found[0]!
  }
  const container = (input.action === 'forward' ? native?.forwardList : (native?.initFlow || native?.flowDetail)) as VNode | undefined
  if (!container) throw new Error('原生宿主未捕获，不能执行动作')
  const action = input.action
  let component: VNode
  let method = ''
  if ((action === 'save_draft' && input.loadMode !== 'edit') || action === 'submit') {
    component = find(container, 'FlowDialog')
    if (typeof component.sumbitFlow !== 'function') throw new Error('FlowDialog 缺少 sumbitFlow 方法')
  } else if (action === 'forward') {
    component = container
  } else {
    component = find(container, 'ExamineOpinion')
  }
  const auditDesc = String(input.auditDesc ?? '')
  if ((action === 'forward' || auditDesc !== '') && component.approveForm && typeof component.approveForm === 'object') {
    // 转发允许明确的空附言，不能留下宿主上一次填写的文本。
    (component.approveForm as { approveMessage?: string }).approveMessage = auditDesc
  }
  if (Array.isArray(input.nextNodeAuditors) && input.nextNodeAuditors.length > 0) {
    const entries = input.nextNodeAuditors.map(item => ({
      name: String(item.name ?? ''),
      bizId: String(item.bizId ?? ''),
      auditDetailType: 'personnel',
      nodeProxyId: String(item.nodeProxyId ?? '')
    }))
    if (entries.some(item => item.bizId === '' || item.name === '')) {
      throw new Error('下一节点人员存在空 bizId/name，拒绝写入目标组件')
    }
    component.nextAuditorList = entries
  }
  switch (action) {
    case 'save_draft':
      if (input.loadMode === 'edit') {
        if (typeof component.saveSubmit !== 'function') throw new Error('ExamineOpinion 缺少 saveSubmit 方法')
        method = 'saveSubmit("save")'
        ;(component.saveSubmit as (kind: string) => unknown)('save')
      } else {
        method = 'sumbitFlow("draft")'
        ;(component.sumbitFlow as (kind: string) => unknown)('draft')
      }
      break
    case 'submit':
      method = 'sumbitFlow("submit")'
      ;(component.sumbitFlow as (kind: string) => unknown)('submit')
      break
    case 'storage_form_data':
      if (typeof component.temporaryStorage === 'function') {
        method = 'temporaryStorage()'
        ;(component.temporaryStorage as () => unknown)()
      } else if (typeof component.temporarySaveBizData === 'function') {
        method = 'temporarySaveBizData()'
        ;(component.temporarySaveBizData as () => unknown)()
      } else {
        throw new Error('ExamineOpinion 缺少 temporaryStorage/temporarySaveBizData 方法')
      }
      break
    case 'approve':
    case 'reject':
      // 合同业务是流程写请求的前置条件；组件缺失或业务方法异常时必须阻断，
      // 不能用普通审批写请求顶替，否则会留下合同未保存但流程已经推进的脏状态。
      {
        const rawComp = component as unknown as { dealFormBusiness?: (func: (flag: unknown, type: unknown) => unknown, flag: unknown, type: unknown) => unknown }
        const selectFlowType = String((component as { selectFlowType?: unknown }).selectFlowType ?? '')
        const requiresBusinessComponent = selectFlowType === 'contract_seal_review' || selectFlowType === 'contract_compliance_review'
        if (requiresBusinessComponent && typeof rawComp.dealFormBusiness !== 'function') {
          throw new Error(`流程类型 ${selectFlowType} 缺少 dealFormBusiness，已阻止普通流程审批`)
        }
        if (typeof rawComp.dealFormBusiness === 'function' && !(rawComp.dealFormBusiness as { __patched?: boolean }).__patched) {
          const patchedDeal = function (this: unknown, func: (flag: unknown, type: unknown) => unknown, flag: unknown, type: unknown) {
            try {
              const self = this as { selectFlowType?: string, $parent?: { $parent?: { $refs?: { generateForm?: { getComponent?: (name: string) => unknown } } }, $refs?: { generateForm?: { getComponent?: (name: string) => unknown } } } }
              const generateForm = self?.$parent?.$parent?.$refs?.generateForm || self?.$parent?.$refs?.generateForm
              const fieldName = self?.selectFlowType === 'contract_seal_review' ? 'custom_contractSealField'
                : self?.selectFlowType === 'contract_compliance_review' ? 'custom_contractLegalField'
                  : null
              const customField = fieldName && typeof generateForm?.getComponent === 'function' ? generateForm.getComponent(fieldName) : null
              if (customField && typeof (customField as { contractBusiness?: unknown }).contractBusiness === 'function') {
                return (customField as { contractBusiness: (form: unknown, that: unknown, flag: unknown, type: unknown) => unknown }).contractBusiness(generateForm, self, flag, type)
              }
              throw new Error(`流程类型 ${self?.selectFlowType ?? ''} 缺少自定义业务组件，已阻止普通流程审批`)
            } catch (error) {
              throw new Error(`自定义业务前置处理失败：${String((error as Error)?.message ?? error)}`)
            }
          }
          ;(patchedDeal as { __patched?: boolean }).__patched = true
          rawComp.dealFormBusiness = patchedDeal
        }
      }
      // 走原生按钮入口 submitCheckBefore：内含 $confirm → approveForm.validate → formMakingFormBusiness；
      // $confirm 的确认按钮由服务端在执行后于页面内点击（已知差异，登记于设计文档）。
      if (typeof component.submitCheckBefore !== 'function') throw new Error('ExamineOpinion 缺少 submitCheckBefore 方法')
      method = `submitCheckBefore("${action === 'approve' ? 'pass' : 'no_pass'}")`
      ;(component.submitCheckBefore as (type: string) => unknown)(action === 'approve' ? 'pass' : 'no_pass')
      break
    case 'resubmit':
      if (typeof component.saveSubmit !== 'function') throw new Error('ExamineOpinion 缺少 saveSubmit 方法')
      method = 'saveSubmit("submit")'
      ;(component.saveSubmit as (type: string) => unknown)('submit')
      break
    case 'forward':
      if (typeof component.confirmTranspond !== 'function') throw new Error('转发列表组件缺少 confirmTranspond 方法')
      method = 'confirmTranspond()'
      ;(component.confirmTranspond as () => unknown)()
      break
    default:
      throw new Error(`动作 ${action} 尚未接入原生组件分发`)
  }
  return { component: String(component.$options?.name ?? ''), method }
}

// ChoiceContinueChoice 是 Go 回传的选择继续载荷（协议 ChoicePayload 镜像）。
export interface ChoiceContinueChoice {
  kind?: string
  branchEntryNodeProxyIds?: string[]
  nextAuditors?: Array<{ nodeProxyId?: string, bizId?: string, name?: string, auditDetailType?: string }>
}

// nativeChoiceContinue 把选择继续落到原生组件。纪律：
//   custom_choose → 写 chooseBranchNode + chooseBranchList（提交载荷从它取分支选择）后调用
//   handleSaveBranchChooseNode；parallel_choose → 补齐 parallelNodeChooseList 后调用
//   handleSaveParallelChooseNode；run_node_choose → 以 checkboxPersonGroup 形状调用 getSelectPerson。
//   分支 auditType 取目标返回的真实值，硬编码会让原生人员校验误拦（F-036 夜间验证实测）。
export function nativeChoiceContinue (
  native: NativeCaptured | undefined,
  errorType: string,
  choice: ChoiceContinueChoice,
  choiceError: Record<string, unknown> | null
): DispatchResult {
  const find = (root: VNode, want: string): VNode => {
    const found: VNode[] = []
    const walk = (node: VNode): void => {
      if (String(node?.$options?.name ?? '') === want) { found.push(node); return }
      for (const child of node.$children ?? []) walk(child)
    }
    walk(root)
    if (found.length !== 1) throw new Error(`组件 ${want} 命中 ${found.length} 个（应为唯一），拒绝执行`)
    return found[0]!
  }
  const container = (native?.initFlow || native?.flowDetail) as VNode | undefined
  if (!container) throw new Error('原生宿主未捕获，不能继续选择')
  const isInitiate = Boolean(native?.initFlow)
  const component = isInitiate ? find(container, 'FlowDialog') : find(container, 'ExamineOpinion')
  const componentName = isInitiate ? 'FlowDialog' : 'ExamineOpinion'
  const auditors = (choice.nextAuditors ?? []) as Array<{ nodeProxyId?: string, bizId?: string, name?: string }>
  if (auditors.length > 0) {
    const list = (component.nextAuditorList as Array<Record<string, unknown>> | undefined) ?? []
    for (const auditor of auditors) {
      const entry = {
        name: String(auditor.name ?? ''),
        bizId: String(auditor.bizId ?? ''),
        auditDetailType: 'personnel',
        nodeProxyId: String(auditor.nodeProxyId ?? '')
      }
      if (!entry.bizId || !entry.name) throw new Error('选择继续携带空人员，拒绝写入')
      const exists = list.some(item => item.nodeProxyId === entry.nodeProxyId)
      if (!exists) list.push(entry)
    }
    component.nextAuditorList = list
  }
  if (errorType === 'custom_choose') {
    const entry = String(choice.branchEntryNodeProxyIds?.[0] ?? '')
    if (!entry) throw new Error('分支继续缺少入口节点 ID')
    // auditType 必须取目标返回的真实值。
    const nodes = ((choiceError as { data?: { branchNodes?: Array<{ id?: string, flowNodeAuditConfig?: { auditType?: string } }> } }).data?.branchNodes ?? [])
    const branchNode = nodes.find(item => String(item?.id ?? '') === entry)
    const auditType = String(branchNode?.flowNodeAuditConfig?.auditType ?? '')
    if (auditType === 'run_node_choose') {
      const list = (component.nextAuditorList as Array<Record<string, unknown>> | undefined) ?? []
      const has = list.some(item => item.nodeProxyId === entry)
      if (!has) throw new Error('分支入口为审批人自选且未配置人员，拒绝继续（绝不带空人员提交）')
    }
    component.chooseBranchNode = { nextNodeTemplateId: entry, auditType }
    component.chooseBranchList = Array.from(new Set([...((component.chooseBranchList as string[] | undefined) ?? []), entry]))
    if (isInitiate) {
      component.nextNodeProxyId = entry
    }
    const saveMethod = typeof component.handleSaveBranchChooseNode === 'function'
      ? 'handleSaveBranchChooseNode'
      : typeof component.handleSaveChooseBranch === 'function'
        ? 'handleSaveChooseBranch'
        : null
    if (!saveMethod) throw new Error(`${componentName} 缺少 handleSaveBranchChooseNode`)
    ;(component[saveMethod] as () => unknown)()
    return { component: componentName, method: saveMethod }
  }
  if (errorType === 'parallel_choose') {
    const entries = (choice.branchEntryNodeProxyIds ?? []).map(String)
    const targetLists = [
      component.parallelNodeChooseList as Array<Record<string, unknown>> | undefined,
      component.parallelChooseNodes as Array<Record<string, unknown>> | undefined
    ]
    for (const list of targetLists) {
      if (!Array.isArray(list)) continue
      for (const item of list) {
        const id = String(item.nextNodeTemplateId ?? item.id ?? '')
        if (id && (entries.length === 0 || entries.includes(id))) {
          item.nextNodeTemplateId = id
          const matched = auditors.filter(a => String(a.nodeProxyId ?? '') === id).map(a => ({
            id: String(a.bizId ?? ''),
            name: String(a.name ?? '')
          }))
          if (matched.length > 0) {
            item.nodeAuditList = matched
          }
        }
      }
    }
    if (typeof component.handleSaveParallelChooseNode !== 'function') throw new Error(`${componentName} 缺少 handleSaveParallelChooseNode`)
    component.handleSaveParallelChooseNode()
    return { component: componentName, method: 'handleSaveParallelChooseNode' }
  }
  // run_node_choose：原生 getSelectPerson 消费 checkboxPersonGroup（{name,id,nodeProxyId}）。
  if (typeof component.getSelectPerson !== 'function') throw new Error(`${componentName} 缺少 getSelectPerson`)
  const group = auditors.map(auditor => ({
    name: String(auditor.name ?? ''),
    id: String(auditor.bizId ?? ''),
    nodeProxyId: String(auditor.nodeProxyId ?? '')
  }))
  component.getSelectPerson({ checkboxPersonGroup: group })
  return { component: componentName, method: 'getSelectPerson' }
}
