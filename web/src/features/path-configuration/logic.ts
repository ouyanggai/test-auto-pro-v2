import type {
  PathActionContainer,
  PathConfigActionKind,
  PathConfigConfiguredActionInput,
  PathConfiguration,
  PathConfigDraft,
  PathConfigNode,
  PathConfigPerson,
  PathConfigPersonDisplayItem,
  PathConfigPersonStrategyInput,
  PathActionConfigurationInput,
  PathActionKey,
  PathActionScope,
  PathConfiguredActionInput,
  PathFormStatus,
} from './types.ts'
import type { FlowConfigurationNodeState, FlowGraph } from '../flow-graph/types.ts'
import type { ExecutionPathAnalysis } from '../execution-paths/types.ts'

// pathConfigurationStatusName 将内部状态收敛为配置页面可见名称。
export function pathConfigurationStatusName(status: string): '待配置' | '部分配置' | '已配置' {
  if (status === 'configured' || status === 'valid' || status === 'not_required') return '已配置'
  if (status === 'pending' || status === 'empty' || status === 'unsupported') return '待配置'
  return '部分配置'
}

// pathConfigurationMessage 不显示内部状态术语，只提示用户补齐当前配置。
export function pathConfigurationMessage(message: string): string { return String(message || '').replace(/配置失效|需要重新确认|受影响需确认|需要重新核对/g, '请补充配置') }

// pathConfigNodeKey 使用与后端相同的稳定哈希绑定图节点与配置节点。
export async function pathConfigNodeKey(nodeID: string): Promise<string> {
  const bytes = new TextEncoder().encode(`node:${nodeID.trim()}:configuration`)
  const digest = await globalThis.crypto.subtle.digest('SHA-256', bytes)
  return [...new Uint8Array(digest).slice(0, 16)].map(value => value.toString(16).padStart(2, '0')).join('')
}

// bindPathConfigurationNodes 建立已核实的图节点和配置节点关系。
export async function bindPathConfigurationNodes(graph: FlowGraph, configuration: PathConfiguration) {
  const configByKey = new Map(configuration.groups.flatMap(group => group.nodes).map(node => [node.key, node]))
  const graphNodeIDByKey = new Map<string, string>()
  await Promise.all(graph.nodes.map(async node => graphNodeIDByKey.set(await pathConfigNodeKey(node.id), node.id)))
  for (const key of configByKey.keys()) if (!graphNodeIDByKey.has(key)) throw new Error('路径节点配置与当前流程结构不一致')
  return { byGraphNodeID: pathConfigurationNodesByGraphID(configuration, graphNodeIDByKey), graphNodeIDByKey }
}

// pathConfigurationNodesByGraphID 为画布建立当前配置节点索引。
export function pathConfigurationNodesByGraphID(configuration: PathConfiguration, graphNodeIDByKey: Map<string, string>) {
  const result = new Map<string, PathConfigNode>()
  for (const group of configuration.groups) for (const node of group.nodes) { const graphNodeID = graphNodeIDByKey.get(node.key); if (graphNodeID) result.set(graphNodeID, node) }
  return result
}

// initialPathConfigurationNodeID 优先定位服务端给出的下一待配置节点。
export function initialPathConfigurationNodeID(configuration: PathConfiguration, graphNodeIDByKey: Map<string, string>): string {
  const next = graphNodeIDByKey.get(configuration.nextNodeKey)
  if (next) return next
  for (const group of configuration.groups) for (const node of group.nodes) { const graphNodeID = graphNodeIDByKey.get(node.key); if (graphNodeID) return graphNodeID }
  return ''
}

export type ConfirmedNodeSaveDestination = { kind: 'next-node', nodeID: string } | { kind: 'form' } | { kind: 'complete' } | { kind: 'unmapped' }

// resolveConfirmedNodeSaveDestination 根据服务端配置进度决定保存后的画布定位。
export function resolveConfirmedNodeSaveDestination(nextNodeKey: string, graphNodeIDByKey: Map<string, string>, formStatus: PathFormStatus): ConfirmedNodeSaveDestination {
  if (nextNodeKey) { const nodeID = graphNodeIDByKey.get(nextNodeKey); return nodeID ? { kind: 'next-node', nodeID } : { kind: 'unmapped' } }
  return formStatus === 'ready' ? { kind: 'complete' } : { kind: 'form' }
}

export interface PathConfigPersonItemSummary { preview: PathConfigPersonDisplayItem[]; total: number; hidden: number }

// summarizePathConfigPersonItems 限制侧栏人员规则预览长度。
export function summarizePathConfigPersonItems(items: PathConfigPersonDisplayItem[], limit = 3): PathConfigPersonItemSummary {
  const safeItems = Array.isArray(items) ? items.filter(item => item && item.count > 0) : []
  const total = safeItems.reduce((sum, item) => sum + item.count, 0)
  const preview: PathConfigPersonDisplayItem[] = []; let visible = 0
  for (const item of safeItems) { if (visible >= limit) break; preview.push(item); visible += item.count }
  return { preview, total, hidden: Math.max(0, total - Math.min(total, limit)) }
}

// projectPathConfigurationNodeStates 把配置状态投影给流程画布。
export function projectPathConfigurationNodeStates(graph: FlowGraph, analysis: ExecutionPathAnalysis, configurationByGraphNodeID: Map<string, PathConfigNode>, selectedNodeID: string): Record<string, FlowConfigurationNodeState> {
  const states: Record<string, FlowConfigurationNodeState> = {}
  for (const node of graph.nodes) {
    const configured = configurationByGraphNodeID.get(node.id)
    const onCurrentPath = analysis.reachableNodeIds.has(node.id)
    states[node.id] = { status: configured?.status ?? 'not_required', statusName: configured ? pathConfigurationStatusName(configured.status) : '路径外上下文', interactive: Boolean(onCurrentPath && configured), selected: Boolean(onCurrentPath && configured && node.id === selectedNodeID) }
  }
  return states
}

// initPathConfigDraft 从权威投影初始化当前页面的人员和动作草稿。
export function initPathConfigDraft(configuration: PathConfiguration): PathConfigDraft {
  const persons: Record<string, string[]> = {}; const personStrategies: Record<string, PathConfigPersonStrategyInput> = {}; const actionConfigurations: Record<string, PathConfigConfiguredActionInput[]> = {}
  for (const group of configuration.groups) for (const node of group.nodes) {
    for (const person of node.persons) {
      if (!person.editable) continue
      const value = { key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: [...person.selected] }
      persons[person.key] = [...value.selected]; personStrategies[person.key] = value
    }
    if (node.actionConfiguration.actions.length) actionConfigurations[node.key] = copyPathConfigActions(node.actionConfiguration.actions)
  }
  const instance = instanceActionContainer(configuration)
  if (instance && instance.actionConfiguration.actions.length) actionConfigurations[instance.key] = copyPathConfigActions(instance.actionConfiguration.actions)
  return { fields: {}, persons, personStrategies, actionConfigurations }
}

// instanceActionContainer 把实例作用域动作投影为独立容器，实例动作不挂在任何语义节点上。
export function instanceActionContainer(configuration: PathConfiguration): PathActionContainer | null {
  if (!configuration.instanceActionKey || !configuration.instanceActions?.catalog?.length) return null
  return { key: configuration.instanceActionKey, persons: [], actionConfiguration: configuration.instanceActions }
}

// nodeActionContainer 把语义节点投影为动作容器，与实例容器共用同一套编辑与校验逻辑。
export function nodeActionContainer(node: PathConfigNode): PathActionContainer { return { key: node.key, persons: node.persons, actionConfiguration: node.actionConfiguration } }

// copyPathConfigActions 脱离 Vue Proxy，保留每条独立动作记录的目标语义和参数。
export function copyPathConfigActions(input: Array<Pick<PathConfigConfiguredActionInput, 'key' | 'kind' | 'person' | 'parameters' | 'actorPolicy' | 'note'>>): PathConfigConfiguredActionInput[] {
  return (input ?? []).map(item => ({
    key: item.key,
    kind: item.kind,
    ...(item.person ? { person: copyPathConfigPersonStrategy(item.person) } : {}),
    ...(item.parameters ? { parameters: cloneActionParameters(item.parameters) } : {}),
    ...(item.actorPolicy ? { actorPolicy: item.actorPolicy } : {}),
    ...(item.note ? { note: item.note } : {}),
  }))
}

// cloneActionParameters 复制动作参数对象，避免编辑器保留 Vue Proxy 或共享可变引用。
function cloneActionParameters(input: Record<string, unknown>): Record<string, unknown> {
  return JSON.parse(JSON.stringify(input)) as Record<string, unknown>
}

// pathConfigActionsInput 从当前容器的独立动作配置取得可保存草稿。
export function pathConfigActionsInput(container: PathActionContainer): PathConfigConfiguredActionInput[] { return copyPathConfigActions(container.actionConfiguration.actions) }

// containerActionsDraft 优先使用页面草稿，未编辑时回落到服务端已保存动作。
export function containerActionsDraft(container: PathActionContainer, draft: PathConfigDraft): PathConfigConfiguredActionInput[] {
  return draft.actionConfigurations[container.key] ?? pathConfigActionsInput(container)
}

// buildPathActionConfigurationInput 把每条页面动作草稿按原始稳定语义发送到 F-012 服务端。
export function buildPathActionConfigurationInput(container: PathActionContainer, draft: PathConfigDraft, revision: number): PathActionConfigurationInput {
  const source = containerActionsDraft(container, draft)
  const personsByKey = new Map(container.persons
    .filter(person => person.editable)
    .map(person => {
      const value = normalizedPersonStrategy(person, draft.personStrategies[person.key])
      return [value.key, value] as const
    }))
  const actions: PathConfiguredActionInput[] = []
  let order = 1
  source.forEach(item => {
    const action = item.kind as PathActionKey
    const scope: PathActionScope = action === 'save_draft' || action === 'submit' || action === 'resubmit'
      ? 'initiator'
      : action === 'retrieve' ? 'completed_task'
        : action === 'withdraw' || action === 'urge' || action === 'forward' || action === 'follow' || action === 'unfollow' ? 'instance' : 'task'
    const parameters = {
      ...(item.parameters ? cloneActionParameters(item.parameters) : {}),
      ...(item.person?.strategy ? { actorStrategy: item.person.strategy } : {}),
    }
    if (item.person) personsByKey.set(item.person.key, { key: item.person.key, strategy: item.person.strategy, seed: item.person.seed, selected: [...item.person.selected] })
    actions.push({
      key: item.key, action, scope, nodeKey: scope === 'instance' ? undefined : container.key, order,
      ...(Object.keys(parameters).length ? { parameters } : {}),
      ...((item.actorPolicy || item.person?.strategy) ? { actorPolicy: item.actorPolicy || item.person?.strategy } : {}),
      ...(item.note ? { note: item.note } : {}),
    })
    order += 1
  })
  return { revision, persons: [...personsByKey.values()], actions }
}

// currentNodeConfigurationComplete 仅检查当前节点的人员与独立动作配置。
export function currentNodeConfigurationComplete(node: PathConfigNode | null, draft: PathConfigDraft): { missing: string[]; complete: boolean } {
  if (!node) return { missing: [], complete: false }
  if (node.lineBlocked) return { missing: [], complete: true }
  const missing: string[] = []
  for (const person of node.persons) {
    if (person.mode === 'review' || person.affected) { missing.push(person.title); continue }
    if (!person.editable) continue
    const selected = resolvedPersonStrategySelection(person, draft.personStrategies[person.key])
    if ((person.required && selected.length === 0) || (selected.length > 0 && selected.length < person.minCount)) missing.push(person.title)
  }
  if (!validPathConfigActions(nodeActionContainer(node), containerActionsDraft(nodeActionContainer(node), draft))) missing.push('动作配置')
  return { missing, complete: missing.length === 0 }
}

// instanceActionsComplete 校验实例动作容器的独立记录是否仍然合法。
export function instanceActionsComplete(container: PathActionContainer | null, draft: PathConfigDraft): boolean {
  if (!container) return true
  return validPathConfigActions(container, containerActionsDraft(container, draft))
}

// hasContainerDraftChanges 比较任意动作容器的未保存改动。
export function hasContainerDraftChanges(container: PathActionContainer | null, draft: PathConfigDraft): boolean {
  if (!container) return false
  return JSON.stringify(containerActionsDraft(container, draft)) !== JSON.stringify(pathConfigActionsInput(container))
}

// hasCurrentNodeDraftChanges 比较当前节点的动作和人员草稿。
export function hasCurrentNodeDraftChanges(node: PathConfigNode | null, draft: PathConfigDraft): boolean {
  if (!node) return false
  for (const person of node.persons) if (person.editable && JSON.stringify(normalizedPersonStrategy(person, draft.personStrategies[person.key])) !== JSON.stringify({ key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: person.selected })) return true
  return hasContainerDraftChanges(nodeActionContainer(node), draft)
}

// resolvedPersonStrategySelection 在浏览器内按公开候选顺序投影最终名单。
export function resolvedPersonStrategySelection(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): string[] {
  const strategy = input?.strategy ?? person.strategy ?? 'manual'
  if (strategy === 'target_default') return [...person.defaultSelected]
  if (strategy === 'all') return person.options.map(option => option.value)
  if (strategy === 'random') { if (!person.options.length) return []; const count = Math.min(person.options.length, Math.max(1, person.minCount || 1)); const start = normalizedPathConfigSeed(input?.seed ?? person.strategySeed) % person.options.length; return Array.from({ length: count }, (_, index) => person.options[(start + index) % person.options.length].value) }
  return [...(input?.selected ?? person.selected)]
}

// normalizedPersonStrategy 生成可保存人员策略，并把前端计算的最终名单一并提交供服务端复验。
export function normalizedPersonStrategy(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput {
  const current = input ?? { key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: person.selected }
  const seed = normalizedPathConfigSeed(current.seed)
  return { ...current, key: person.key, seed, selected: resolvedPersonStrategySelection(person, { ...current, seed }) }
}

// normalizedPathConfigSeed 保持浏览器 seed 在安全整数范围内。
export function normalizedPathConfigSeed(seed: unknown): number { return typeof seed === 'number' && Number.isSafeInteger(seed) && seed >= 1 ? seed : 1 }

// validPathConfigActions 在提交前校验目录和动作专用人员；重复动作记录是合法的，被门禁禁用的动作不合法。
export function validPathConfigActions(container: PathActionContainer, input: PathConfigConfiguredActionInput[]): boolean {
  if (!input || input.length > 10) return false
  const catalog = new Map(container.actionConfiguration.catalog.filter(item => item.enabled && !item.systemOnly).map(item => [item.kind, item]))
  return input.every(action => {
    const definition = catalog.get(action.kind)
    if (!definition) return false
    if (!definition.requiresPerson) return !action.person
    return validPathConfigActionPerson(definition.person, action.person)
  })
}

// validPathConfigActionPerson 校验动作专用人员策略。
function validPathConfigActionPerson(person: PathConfigPerson | undefined, input: PathConfigPersonStrategyInput | undefined): boolean {
  if (!person || !input || input.key !== person.key || !person.strategies.some(item => item.value === input.strategy)) return false
  const selected = resolvedPersonStrategySelection(person, input); const allowed = new Set(person.options.map(option => option.value))
  return !selected.some(value => !allowed.has(value)) && !(person.required && selected.length === 0) && !(selected.length > 0 && selected.length < person.minCount) && !(person.maxCount > 0 && selected.length > person.maxCount)
}

// copyPathConfigPersonStrategy 把可能来自 Vue Proxy 的策略转换为普通对象。
function copyPathConfigPersonStrategy(input: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput { return { key: input.key, strategy: input.strategy, seed: input.seed, selected: [...input.selected] } }
