import type {
  PathConfigActionPlanInput,
  PathConfigActionCycleInput,
  PathConfigConfiguredActionInput,
  PathConfigActionStepInput,
  PathConfigActionKind,
  PathConfigActionValue,
  PathConfiguration,
  PathConfigDraft,
  PathConfigField,
  PathConfigFieldValue,
  PathConfigNode,
  PathConfigPersonDisplayItem,
  PathConfigPerson,
  PathConfigPersonStrategyInput,
  PathConfigNodeSavePayload,
  PathFormStatus,
} from './types.ts'
import type { ExecutionPathAnalysis } from '../execution-paths/types.ts'
import type { FlowConfigurationNodeState, FlowGraph } from '../flow-graph/types.ts'

// pathConfigurationStatusName 把内部状态收敛为页面仅展示的配置完成度，affected 仍只在校验链路中使用。
export function pathConfigurationStatusName(status: string): '待配置' | '部分配置' | '已配置' {
  if (status === 'configured' || status === 'valid' || status === 'not_required') return '已配置'
  if (status === 'pending' || status === 'empty' || status === 'unsupported') return '待配置'
  return '部分配置'
}

// pathConfigurationMessage 隐藏内部 affected 校验术语，页面只提示用户补齐当前配置而不暴露历史状态名。
export function pathConfigurationMessage(message: string): string {
  return String(message || '')
    .replace(/配置失效|需要重新确认|受影响需确认|需要重新核对/g, '请补充配置')
}

// pathConfigNodeKey 使用与后端相同的稳定哈希把图节点映射到配置节点；公开配置模型无需返回目标节点 ID。
export async function pathConfigNodeKey(nodeID: string): Promise<string> {
  const bytes = new TextEncoder().encode(`node:${nodeID.trim()}:configuration`)
  const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes)
  return [...new Uint8Array(digest).slice(0, 16)]
    .map((value) => value.toString(16).padStart(2, '0'))
    .join('')
}

// bindPathConfigurationNodes 把同一真实图的节点 ID 与配置不透明键绑定，并拒绝缺失或额外配置节点。
export async function bindPathConfigurationNodes(graph: FlowGraph, configuration: PathConfiguration) {
  const configByKey = new Map(configuration.groups.flatMap((group) => group.nodes).map((node) => [node.key, node]))
  const graphNodeIDByKey = new Map<string, string>()
  await Promise.all(graph.nodes.map(async (node) => {
    const key = await pathConfigNodeKey(node.id)
    graphNodeIDByKey.set(key, node.id)
  }))
  for (const key of configByKey.keys()) {
    if (!graphNodeIDByKey.has(key)) throw new Error('路径节点配置与当前流程结构不一致')
  }
  return { byGraphNodeID: pathConfigurationNodesByGraphID(configuration, graphNodeIDByKey), graphNodeIDByKey }
}

// pathConfigurationNodesByGraphID 使用已经核实过的键映射重建响应式节点索引，保存后不会继续引用旧投影。
export function pathConfigurationNodesByGraphID(configuration: PathConfiguration, graphNodeIDByKey: Map<string, string>) {
  const result = new Map<string, PathConfiguration['groups'][number]['nodes'][number]>()
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      const graphNodeID = graphNodeIDByKey.get(node.key)
      if (graphNodeID) result.set(graphNodeID, node)
    }
  }
  return result
}

// initialPathConfigurationNodeID 优先选择后端给出的下一待配置节点，否则选择投影顺序中的首个节点。
export function initialPathConfigurationNodeID(configuration: PathConfiguration, graphNodeIDByKey: Map<string, string>): string {
  const next = graphNodeIDByKey.get(configuration.nextNodeKey)
  if (next) return next
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      const graphNodeID = graphNodeIDByKey.get(node.key)
      if (graphNodeID) return graphNodeID
    }
  }
  return ''
}

export type ConfirmedNodeSaveDestination =
  | { kind: 'next-node', nodeID: string }
  | { kind: 'form' }
  | { kind: 'complete' }
  | { kind: 'unmapped' }

export interface PathConfigPersonItemSummary {
  preview: PathConfigPersonDisplayItem[]
  total: number
  hidden: number
}

// summarizePathConfigPersonItems 按目标对象数量生成侧栏摘要；聚合范围的 count 仍计入“查看全部 N 项”。
export function summarizePathConfigPersonItems(items: PathConfigPersonDisplayItem[], limit = 3): PathConfigPersonItemSummary {
  const safeItems = Array.isArray(items) ? items.filter(item => item && item.count > 0) : []
  const total = safeItems.reduce((sum, item) => sum + item.count, 0)
  let visibleCount = 0
  const preview: PathConfigPersonDisplayItem[] = []
  for (const item of safeItems) {
    if (visibleCount >= limit) break
    preview.push(item)
    visibleCount += item.count
  }
  return { preview, total, hidden: Math.max(0, total - Math.min(total, limit)) }
}

// resolveConfirmedNodeSaveDestination 只按服务端最新 nextNodeKey 推进当前路径；没有下一节点时才转入表单或完成态。
export function resolveConfirmedNodeSaveDestination(
  nextNodeKey: string,
  graphNodeIDByConfigurationKey: Map<string, string>,
  formStatus: PathFormStatus,
): ConfirmedNodeSaveDestination {
  if (nextNodeKey) {
    const nodeID = graphNodeIDByConfigurationKey.get(nextNodeKey)
    return nodeID ? { kind: 'next-node', nodeID } : { kind: 'unmapped' }
  }
  return formStatus === 'valid' ? { kind: 'complete' } : { kind: 'form' }
}

// projectPathConfigurationNodeStates 把后端节点状态投影到同一流程图；路径外节点只作弱化上下文且不可配置。
export function projectPathConfigurationNodeStates(
  graph: FlowGraph,
  analysis: ExecutionPathAnalysis,
  configByGraphNodeID: Map<string, PathConfiguration['groups'][number]['nodes'][number]>,
  selectedNodeID: string,
): Record<string, FlowConfigurationNodeState> {
  const result: Record<string, FlowConfigurationNodeState> = {}
  for (const node of graph.nodes) {
    const configNode = configByGraphNodeID.get(node.id)
    const onCurrentPath = analysis.reachableNodeIds.has(node.id)
    result[node.id] = {
      status: configNode?.status ?? 'not_required',
      statusName: configNode ? pathConfigurationStatusName(configNode.status) : '路径外上下文',
      interactive: Boolean(onCurrentPath && configNode),
      selected: Boolean(onCurrentPath && configNode && node.id === selectedNodeID),
    }
  }
  return result
}

// parsePathConfigValue 把后端 JSON 文本值按字段类型解析为前端控件值；无法解析时退回原始文本。
export function parsePathConfigValue(field: PathConfigField, raw: string): unknown {
  if (raw === '') return field.type === 'multiSelect' ? [] : ''
  try {
    return JSON.parse(raw)
  }
  catch {
    return raw
  }
}

// encodePathConfigValue 把前端控件值编码为后端 JSON 文本；多选保持数组。
export function encodePathConfigValue(field: PathConfigField, value: unknown): string {
  if (field.type === 'multiSelect' && !Array.isArray(value)) return '[]'
  if (value === '' || value === undefined || value === null) return '""'
  return JSON.stringify(value)
}

// initPathConfigDraft 从配置模型生成可编辑草稿；字段用不透明键、动作用不透明键保存当前值。
export function initPathConfigDraft(configuration: PathConfiguration): PathConfigDraft {
  const fields: Record<string, string> = {}
  const actions: Record<string, string> = {}
  const persons: Record<string, string[]> = {}
  const personStrategies: Record<string, PathConfigPersonStrategyInput> = {}
  const actionPlans: Record<string, PathConfigActionPlanInput> = {}
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (field.editable) fields[field.key] = field.value
      }
      for (const action of node.actions) {
        actions[action.key] = action.current
      }
      for (const person of node.persons) {
        if (person.editable) {
          persons[person.key] = [...person.selected]
          personStrategies[person.key] = {
            key: person.key,
            strategy: person.strategy || 'manual',
            seed: person.strategySeed || 1,
            selected: [...person.selected],
          }
        }
      }
      if (node.actionPlan.result.kind || node.actionPlan.actions?.length || node.actionPlan.combinationCount > 1) actionPlans[node.key] = pathConfigActionPlanInput(node)
    }
  }
  return { fields, actions, persons, personStrategies, actionPlans }
}

// hasPathConfigDraftChanges 判断草稿相对配置模型是否有真实变化；仅用于保存按钮可用性。
export function hasPathConfigDraftChanges(configuration: PathConfiguration, draft: PathConfigDraft): boolean {
  const baseline = initPathConfigDraft(configuration)
  const baselineFields = JSON.stringify(baseline.fields)
  const baselineActions = JSON.stringify(baseline.actions)
  const baselinePersons = JSON.stringify(baseline.persons)
  return baselineFields !== JSON.stringify(draft.fields)
    || baselineActions !== JSON.stringify(draft.actions)
    || baselinePersons !== JSON.stringify(draft.persons)
    || JSON.stringify(baseline.personStrategies) !== JSON.stringify(draft.personStrategies)
    || JSON.stringify(baseline.actionPlans) !== JSON.stringify(draft.actionPlans)
}

// reconcilePathConfigDraft 在目标结构刷新后只保留仍有相同不透明配置键的草稿，避免跨节点或跨路径串值。
export function reconcilePathConfigDraft(configuration: PathConfiguration, draft: PathConfigDraft): PathConfigDraft {
  const next = initPathConfigDraft(configuration)
  for (const key of Object.keys(next.fields)) {
    if (Object.prototype.hasOwnProperty.call(draft.fields, key)) next.fields[key] = draft.fields[key]
  }
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const action of node.actions) {
        const candidate = draft.actions[action.key]
        if (candidate && action.options.some((option) => option.value === candidate)) next.actions[action.key] = candidate
      }
      for (const person of node.persons) {
        if (!person.editable || !Object.prototype.hasOwnProperty.call(draft.persons, person.key)) continue
        const allowed = new Set(person.options.map((option) => option.value))
        // 人员候选由当前目标快照决定；刷新后只保留仍在合法候选中的值，不能回写已经失效的目标人员 ID。
        next.persons[person.key] = draft.persons[person.key].filter((value) => allowed.has(value))
        const strategy = draft.personStrategies[person.key]
        if (strategy && person.strategies.some(option => option.value === strategy.strategy)) {
          next.personStrategies[person.key] = {
            ...strategy,
            key: person.key,
            selected: strategy.selected.filter(value => allowed.has(value)),
          }
        }
      }
      if (Object.prototype.hasOwnProperty.call(draft.actionPlans, node.key)) {
        const candidate = copyPathConfigActionPlan(draft.actionPlans[node.key])
        if (validPathConfigActionPlan(node, candidate)) next.actionPlans[node.key] = candidate
      }
    }
  }
  return next
}

// canSavePathConfiguration 允许首次无记录配置在默认值/实例现值未改变时保存，同时要求必填项完整。
export function canSavePathConfiguration(configuration: PathConfiguration, draft: PathConfigDraft): boolean {
  const required = allEditableFieldsFilled(configuration, draft)
  if (!required.complete) return false
  return configuration.status === 'pending' || hasPathConfigDraftChanges(configuration, draft)
}

// applyPathConfigDraft 把成功保存后的草稿写回当前投影，立即复位 dirty 基线而不重新读取目标或重置页面视口。
export function applyPathConfigDraft(configuration: PathConfiguration, draft: PathConfigDraft, revision: number): PathConfiguration {
  const next = structuredClone(configuration) as PathConfiguration
  next.revision = revision
  next.status = 'configured'
  for (const group of next.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (Object.prototype.hasOwnProperty.call(draft.fields, field.key)) {
          field.value = draft.fields[field.key]
          // 保存接口已经用当前目标快照验证成功，原先可编辑项的受影响标记可以安全复位。
          if (field.editable) {
            field.affected = false
            field.note = ''
          }
        }
      }
      for (const action of node.actions) {
        if (Object.prototype.hasOwnProperty.call(draft.actions, action.key)) action.current = draft.actions[action.key]
      }
      for (const person of node.persons) {
        if (Object.prototype.hasOwnProperty.call(draft.persons, person.key)) {
          person.selected = [...draft.persons[person.key]]
          if (person.editable) {
            person.affected = false
            person.note = ''
          }
        }
      }
      const hasAffectedItem = node.fields.some((field) => field.affected) || node.persons.some((person) => person.affected)
      if ((node.status === 'pending' || node.status === 'partial' || node.status === 'affected')
        && node.gaps.length === 0
        && !hasAffectedItem) {
        node.status = 'configured'
        node.statusName = '已配置'
      }
    }
  }
  next.progress.completed = next.groups.flatMap((group) => group.nodes).filter((node) => node.status === 'configured').length
  next.progress.pending = Math.max(0, next.progress.total - next.progress.completed)
  next.nextNodeKey = next.groups.flatMap((group) => group.nodes).find((node) => node.status === 'pending' || node.status === 'partial' || node.status === 'affected')?.key ?? ''
  return next
}

// buildPathConfigSavePayload 把草稿收敛为后端最小回写体，只包含当前配置中的可编辑项。
export function buildPathConfigSavePayload(configuration: PathConfiguration, draft: PathConfigDraft): { fields: PathConfigFieldValue[], actions: PathConfigActionValue[] } {
  const fields: PathConfigFieldValue[] = []
  const actions: PathConfigActionValue[] = []
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (field.editable && Object.prototype.hasOwnProperty.call(draft.fields, field.key)) {
          fields.push({ key: field.key, value: draft.fields[field.key] })
        }
      }
      for (const action of node.actions) {
        if (Object.prototype.hasOwnProperty.call(draft.actions, action.key)) {
          actions.push({ key: action.key, action: draft.actions[action.key] })
        }
      }
      for (const person of node.persons) {
        if (person.editable && Object.prototype.hasOwnProperty.call(draft.persons, person.key)) {
          // 人员选择继续复用既有 actions 最小回写数组，后端按不透明键区分真实动作与人员命名空间。
          actions.push({ key: person.key, action: JSON.stringify(draft.persons[person.key]) })
        }
      }
    }
  }
  return { fields, actions }
}

// copyPathConfigPersonStrategy 把可能来自 Vue Proxy 的人员策略逐字段复制成普通对象。
function copyPathConfigPersonStrategy(input: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput {
  return { key: input.key, strategy: input.strategy, seed: input.seed, selected: [...input.selected] }
}

// copyPathConfigActionStep 把处理结果复制成可直接序列化的普通对象，处理意见不属于 F-007 新配置。
function copyPathConfigActionStep(input: PathConfigActionStepInput): PathConfigActionStepInput {
  return {
    kind: input.kind,
    opinion: '',
    target: input.target || '',
    person: input.person ? copyPathConfigPersonStrategy(input.person) : undefined,
  }
}

// copyPathConfigConfiguredAction 复制动作组合项，次数和组合循环次数各自独立归一。
function copyPathConfigConfiguredAction(input: PathConfigConfiguredActionInput): PathConfigConfiguredActionInput {
  return { key: input.key, kind: input.kind, count: normalizedAddSignCount(input.count), target: input.target || '', person: input.person ? copyPathConfigPersonStrategy(input.person) : undefined }
}

// copyPathConfigActionPlan 复制独立加签节点与唯一处理结果，避免 Vue Proxy 在请求前触发 DataCloneError。
export function copyPathConfigActionPlan(input: PathConfigActionPlanInput): PathConfigActionPlanInput {
  return {
    actions: (input.actions ?? []).map(copyPathConfigConfiguredAction),
    combinationCount: normalizedAddSignCount(input.combinationCount),
    // 加签节点可能来自后端投影或本地草稿；统一按空数组兜底，避免 null 触发 .map 异常。
    addSignNodes: (input.addSignNodes ?? []).map(item => ({ person: copyPathConfigPersonStrategy(item.person), count: normalizedAddSignCount(item.count) })),
    result: copyPathConfigActionStep(input.result ?? { kind: 'approve_pass', opinion: '', target: '' }),
  }
}

// normalizedAddSignCount 把加签次数收敛到 1..MAX_ADD_SIGN_NODES，缺失或非法按 1 处理。
export function normalizedAddSignCount(count: unknown): number {
  return typeof count === 'number' && Number.isInteger(count) && count >= 1 && count <= MAX_ADD_SIGN_NODES ? count : 1
}

// pathConfigActionPlanInput 把服务端公开投影转换成语义化草稿，不暴露内部兼容存储结构。
export function pathConfigActionPlanInput(node: PathConfigNode): PathConfigActionPlanInput {
  const resultKind: PathConfigActionKind = node.actionPlan.result.kind || (node.kind === 'start' ? 'submit' : 'approve_pass')
  const legacyActions: PathConfigConfiguredActionInput[] = (node.actionPlan.addSignNodes ?? []).map((item, index) => ({ key: `action-${index + 1}`, kind: 'add_sign', count: normalizedAddSignCount(item.count), target: '', person: item.person }))
  if (resultKind) legacyActions.push({ key: `action-${legacyActions.length + 1}`, kind: resultKind, count: 1, target: node.actionPlan.result.target || '', person: node.actionPlan.result.person })
  return copyPathConfigActionPlan({
    actions: node.actionPlan.actions?.length ? node.actionPlan.actions.map(action => ({ key: action.key, kind: action.kind, count: action.count, target: action.target, person: action.person })) : legacyActions,
    combinationCount: node.actionPlan.combinationCount,
    // 后端对无加签节点的 start/common/synergy 返回 null，必须兜底为空数组，否则整页投影抛 TypeError。
    addSignNodes: (node.actionPlan.addSignNodes ?? []).map(item => ({ person: item.person, count: item.count })),
    result: {
      kind: resultKind,
      opinion: '',
      target: node.actionPlan.result.target || '',
      person: node.actionPlan.result.person,
    },
  })
}

// buildPathConfigNodeSavePayload 只收敛当前节点的动作与人员，不允许一次保存覆盖其他节点。
export function buildPathConfigNodeSavePayload(node: PathConfigNode, draft: PathConfigDraft, actionCycles?: PathConfigActionCycleInput[], included?: boolean): PathConfigNodeSavePayload {
  const persons = node.persons
    .filter(person => person.editable)
    .map(person => normalizedPersonStrategy(person, draft.personStrategies[person.key]))
  // 保存前必须脱离 Vue 深响应式对象，否则浏览器会在 fetch 之前抛 DataCloneError，导致节点 PUT 根本没有发出。
  return { persons, actionPlan: copyPathConfigActionPlan(draft.actionPlans[node.key] ?? pathConfigActionPlanInput(node)), actionCycles, included }
}

// currentNodeConfigurationComplete 判断当前节点人数约束是否满足；表单字段不再属于节点侧栏。
export function currentNodeConfigurationComplete(node: PathConfigNode | null, draft: PathConfigDraft): { missing: string[], complete: boolean } {
  if (!node || node.lineBlocked) return { missing: [], complete: false }
  const missing: string[] = []
  for (const person of node.persons) {
    if (person.mode === 'review' || person.affected) {
      missing.push(person.title)
      continue
    }
    if (!person.editable) continue
    const selected = resolvedPersonStrategySelection(person, draft.personStrategies[person.key])
    const requiredEmpty = person.required && selected.length === 0
    const belowMinimum = selected.length > 0 && selected.length < person.minCount
    if (requiredEmpty || belowMinimum) missing.push(person.title)
  }
  const actionPlan = draft.actionPlans[node.key] ?? pathConfigActionPlanInput(node)
  // 受影响存量会先按当前目录规范化到草稿；只要规范化结果合法就允许用户重新确认保存，不能因旧状态永久禁用保存。
  if (!validPathConfigActionPlan(node, actionPlan)) missing.push('动作计划')
  const actionable = node.actionPlan.catalog.some(item => item.enabled) || node.persons.some((person) => person.editable)
  // 历史 fields/gaps 即使仍出现在兼容响应中也不能阻断节点保存；表单兼容性只由独立运行时负责。
  return { missing, complete: actionable && missing.length === 0 }
}

// hasCurrentNodeDraftChanges 只比较当前节点动作与人员，其他节点草稿不影响保存按钮。
export function hasCurrentNodeDraftChanges(node: PathConfigNode | null, draft: PathConfigDraft): boolean {
  if (!node) return false
  for (const person of node.persons) {
    if (!person.editable) continue
    if (JSON.stringify(normalizedPersonStrategy(person, draft.personStrategies[person.key])) !== JSON.stringify({
      key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: person.selected,
    })) return true
  }
  return JSON.stringify(draft.actionPlans[node.key] ?? pathConfigActionPlanInput(node)) !== JSON.stringify(pathConfigActionPlanInput(node))
}

// resolvedPersonStrategySelection 在浏览器内按公开候选顺序投影最终名单，随机结果与后端轮转算法一致。
export function resolvedPersonStrategySelection(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): string[] {
  const strategy = input?.strategy ?? person.strategy ?? 'manual'
  if (strategy === 'target_default') return [...person.defaultSelected]
  if (strategy === 'all') return person.options.map(option => option.value)
  if (strategy === 'random') {
    if (!person.options.length) return []
    const count = Math.min(person.options.length, Math.max(1, person.minCount || 1))
    const seed = normalizedPathConfigSeed(input?.seed ?? person.strategySeed)
    const start = seed % person.options.length
    return Array.from({ length: count }, (_, index) => person.options[(start + index) % person.options.length].value)
  }
  return [...(input?.selected ?? person.selected)]
}

// normalizedPersonStrategy 生成可保存人员策略，并把前端计算的最终名单一并提交供后端核对。
export function normalizedPersonStrategy(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput {
  const current = input ?? { key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: person.selected }
  const seed = normalizedPathConfigSeed(current.seed)
  return { ...current, key: person.key, seed, selected: resolvedPersonStrategySelection(person, { ...current, seed }) }
}

// normalizedPathConfigSeed 与服务端统一把非正数、非整数和超过 JavaScript 安全整数的值规范为 1。
export function normalizedPathConfigSeed(seed: unknown): number {
  return typeof seed === 'number' && Number.isSafeInteger(seed) && seed >= 1 ? seed : 1
}

const RESULT_ACTIONS = new Set<PathConfigActionKind>(['submit', 'approve_pass', 'reject_no_pass', 'draft_save', 'rollback_previous', 'transfer_approver', 'transpond'])
const MAX_ADD_SIGN_NODES = 10

// validPathConfigActionPerson 统一校验加签节点和移交的独立人员策略，页面与后端使用相同候选和人数边界。
function validPathConfigActionPerson(person: PathConfigPerson | undefined, input: PathConfigPersonStrategyInput | undefined): boolean {
  if (!person || !input || input.key !== person.key || !person.strategies.some(item => item.value === input.strategy)) return false
  const allowed = new Set(person.options.map(option => option.value))
  const selected = resolvedPersonStrategySelection(person, input)
  if (selected.some(value => !allowed.has(value))) return false
  const requiredEmpty = person.required && selected.length === 0
  const belowMinimum = selected.length > 0 && selected.length < person.minCount
  const aboveMaximum = person.maxCount > 0 && selected.length > person.maxCount
  return !requiredEmpty && !belowMinimum && !aboveMaximum
}

// validPathConfigActionPlan 即时校验零到多个独立加签节点（含次数）和唯一处理结果，服务端仍按最新目标快照复验。
export function validPathConfigActionPlan(node: PathConfigNode, input: PathConfigActionPlanInput): boolean {
  if (!input) return false
  const combinationCount = input.combinationCount ?? 1
  if (normalizedAddSignCount(combinationCount) !== combinationCount) return false
  const totalAddSigns = input.addSignNodes.reduce((sum, item) => sum + normalizedAddSignCount(item.count), 0)
  if (totalAddSigns > MAX_ADD_SIGN_NODES) return false
  const catalog = new Map(node.actionPlan.catalog.map(item => [item.kind, item]))
  const resultDefinition = catalog.get(input.result.kind)
  if (!resultDefinition?.enabled || !RESULT_ACTIONS.has(input.result.kind)) return false
  if (input.result.kind === 'submit') return totalAddSigns === 0 && node.kind === 'start' && (input.actions ?? []).every(action => action.kind === 'submit' && normalizedAddSignCount(action.count) === action.count)
  if (node.kind !== 'common' && node.kind !== 'synergy') return false
  const addSignDefinition = catalog.get('add_sign')
  if (totalAddSigns > 0 && !addSignDefinition?.enabled) return false
  for (const addSign of input.addSignNodes) {
    if (!validPathConfigActionPerson(addSignDefinition?.person, addSign.person)) return false
  }
  if (resultDefinition.requiresTarget && !node.actionPlan.rollbackTargets.some(option => option.value === input.result.target)) return false
  if (resultDefinition.requiresPerson && !validPathConfigActionPerson(resultDefinition.person, input.result.person)) return false
  for (const action of input.actions ?? []) {
    const definition = catalog.get(action.kind)
    if (!definition?.enabled || normalizedAddSignCount(action.count) !== action.count) return false
    if (definition.requiresTarget && !node.actionPlan.rollbackTargets.some(option => option.value === action.target)) return false
    if (definition.requiresPerson && !validPathConfigActionPerson(definition.person, action.person)) return false
  }
  return true
}

// nextFormGenerationSeed 对换一组稳定推进种子，同一输入种子重复调用仍可复现。
export function nextFormGenerationSeed(seed: number): number {
  if (!Number.isSafeInteger(seed) || seed < 1) return 1
  return seed >= Number.MAX_SAFE_INTEGER - 104729 ? 1 : seed + 104729
}

// allEditableFieldsFilled 判断必填字段和人员人数约束是否满足，用于保存前即时提示。
export function allEditableFieldsFilled(configuration: PathConfiguration, draft: PathConfigDraft): { missing: string[], complete: boolean } {
  const missing: string[] = []
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (!field.editable || !field.required) continue
        const value = parsePathConfigValue(field, draft.fields[field.key] ?? '')
        const empty = Array.isArray(value) ? value.length === 0 : value === '' || value === undefined || value === null
        if (empty) missing.push(field.name)
      }
      for (const person of node.persons) {
        if (!person.editable) continue
        const selected = resolvedPersonStrategySelection(person, draft.personStrategies[person.key])
        // 可跳过节点允许保持零选择；主动选择后仍须一次满足模板最低人数，避免前端放行不完整会签组。
        const requiredEmpty = person.required && selected.length === 0
        const partialSelection = selected.length > 0 && selected.length < person.minCount
        if (requiredEmpty || partialSelection) missing.push(person.title)
      }
    }
  }
  return { missing, complete: missing.length === 0 }
}

// disagreesInDraft 统计草稿中不同意动作数量，用于线路影响提示。
export function disagreesInDraft(draft: PathConfigDraft): number {
  let count = 0
  for (const key of Object.keys(draft.actions)) {
    if (draft.actions[key] === 'disagree') count++
  }
  return count
}
