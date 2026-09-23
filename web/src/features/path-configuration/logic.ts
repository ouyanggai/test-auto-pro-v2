import type {
  PathActionContainer,
  PathConfigActionKind,
  PathCompiledActionStep,
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

// pathConfigurationMessage 保留服务端的具体中文原因，不能把可处理的定位信息替换成泛化提示。
export function pathConfigurationMessage(message: string): string { return String(message || '') }

// copyPersonSelection 把服务端空人员集合统一收敛为空数组，避免历史配置中的 null 中断整页初始化。
function copyPersonSelection(values: readonly string[] | null | undefined): string[] {
  return Array.isArray(values) ? [...values] : []
}

// sha256Fallback 在非安全上下文中补齐 Web Crypto 不可用时的 SHA-256 计算。
// 这里必须保持标准填充、字节序和 32 位溢出规则，否则生成的节点 key 会与后端脱节。
function sha256Fallback(input: Uint8Array): Uint8Array {
  const roundConstants = new Uint32Array([
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ])
  const paddedLength = Math.ceil((input.length + 9) / 64) * 64
  const padded = new Uint8Array(paddedLength)
  padded.set(input)
  padded[input.length] = 0x80
  const bitLength = input.length * 8
  const highLength = Math.floor(bitLength / 0x100000000)
  const lowLength = bitLength >>> 0
  padded[paddedLength - 8] = highLength >>> 24
  padded[paddedLength - 7] = highLength >>> 16
  padded[paddedLength - 6] = highLength >>> 8
  padded[paddedLength - 5] = highLength
  padded[paddedLength - 4] = lowLength >>> 24
  padded[paddedLength - 3] = lowLength >>> 16
  padded[paddedLength - 2] = lowLength >>> 8
  padded[paddedLength - 1] = lowLength

  let h0 = 0x6a09e667; let h1 = 0xbb67ae85; let h2 = 0x3c6ef372; let h3 = 0xa54ff53a
  let h4 = 0x510e527f; let h5 = 0x9b05688c; let h6 = 0x1f83d9ab; let h7 = 0x5be0cd19
  const rotateRight = (value: number, count: number) => (value >>> count) | (value << (32 - count))
  const words = new Uint32Array(64)

  for (let offset = 0; offset < padded.length; offset += 64) {
    for (let index = 0; index < 16; index++) {
      const position = offset + index * 4
      words[index] = (padded[position] << 24) | (padded[position + 1] << 16) | (padded[position + 2] << 8) | padded[position + 3]
    }
    for (let index = 16; index < 64; index++) {
      const value15 = words[index - 15]
      const value2 = words[index - 2]
      const small0 = rotateRight(value15, 7) ^ rotateRight(value15, 18) ^ (value15 >>> 3)
      const small1 = rotateRight(value2, 17) ^ rotateRight(value2, 19) ^ (value2 >>> 10)
      words[index] = (words[index - 16] + small0 + words[index - 7] + small1) >>> 0
    }

    let a = h0; let b = h1; let c = h2; let d = h3; let e = h4; let f = h5; let g = h6; let h = h7
    for (let index = 0; index < 64; index++) {
      const big1 = rotateRight(e, 6) ^ rotateRight(e, 11) ^ rotateRight(e, 25)
      const choose = (e & f) ^ (~e & g)
      const first = (h + big1 + choose + roundConstants[index] + words[index]) >>> 0
      const big0 = rotateRight(a, 2) ^ rotateRight(a, 13) ^ rotateRight(a, 22)
      const majority = (a & b) ^ (a & c) ^ (b & c)
      const second = (big0 + majority) >>> 0
      h = g; g = f; f = e; e = (d + first) >>> 0; d = c; c = b; b = a; a = (first + second) >>> 0
    }
    h0 = (h0 + a) >>> 0; h1 = (h1 + b) >>> 0; h2 = (h2 + c) >>> 0; h3 = (h3 + d) >>> 0
    h4 = (h4 + e) >>> 0; h5 = (h5 + f) >>> 0; h6 = (h6 + g) >>> 0; h7 = (h7 + h) >>> 0
  }

  const result = new Uint8Array(32)
  const digest = [h0, h1, h2, h3, h4, h5, h6, h7]
  digest.forEach((value, index) => {
    result[index * 4] = value >>> 24; result[index * 4 + 1] = value >>> 16
    result[index * 4 + 2] = value >>> 8; result[index * 4 + 3] = value
  })
  return result
}

// sha256Digest 优先使用浏览器原生实现，非安全上下文或原生调用失败时回退到本地实现。
async function sha256Digest(input: Uint8Array): Promise<Uint8Array> {
  const subtle = globalThis.crypto?.subtle
  if (subtle) {
    try { return new Uint8Array(await subtle.digest('SHA-256', input as unknown as BufferSource)) } catch { /* 非安全上下文由本地实现兜底。 */ }
  }
  return sha256Fallback(input)
}

// pathConfigNodeKey 使用与后端相同的稳定哈希绑定图节点与配置节点。
export async function pathConfigNodeKey(nodeID: string): Promise<string> {
  const bytes = new TextEncoder().encode(`node:${nodeID.trim()}:configuration`)
  const digest = await sha256Digest(bytes)
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
    // 路由与并行拆分节点在当前路径上，但它们不执行任何动作，服务端不会给出配置投影。
    // 这类节点既不能点开配置，也不该显示"路径外上下文"——那会让用户以为它不在本路径上。
    const routeNode = node.type === 'condition' || node.type === 'manual' || node.type === 'parallel'
    const statusName = configured
      ? pathConfigurationStatusName(configured.status)
      : (onCurrentPath && routeNode ? '分支节点，无需配置' : '路径外上下文')
    states[node.id] = { status: configured?.status ?? 'not_required', statusName, interactive: Boolean(onCurrentPath && configured), selected: Boolean(onCurrentPath && configured && node.id === selectedNodeID) }
  }
  return states
}

// initPathConfigDraft 从权威投影初始化当前页面的人员和动作草稿。
export function initPathConfigDraft(configuration: PathConfiguration): PathConfigDraft {
  const persons: Record<string, string[]> = {}; const personStrategies: Record<string, PathConfigPersonStrategyInput> = {}; const actionConfigurations: Record<string, PathConfigConfiguredActionInput[]> = {}
  for (const group of configuration.groups) for (const node of group.nodes) {
    for (const person of node.persons) {
      if (!person.editable) continue
      const value = { key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: copyPersonSelection(person.selected) }
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
export function buildPathActionConfigurationInput(container: PathActionContainer, draft: PathConfigDraft): PathActionConfigurationInput {
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
    if (item.person) personsByKey.set(item.person.key, { key: item.person.key, strategy: item.person.strategy, seed: item.person.seed, selected: copyPersonSelection(item.person.selected) })
    actions.push({
      key: item.key, action, scope, nodeKey: container.key, order,
      ...(Object.keys(parameters).length ? { parameters } : {}),
      ...((item.actorPolicy || item.person?.strategy) ? { actorPolicy: item.actorPolicy || item.person?.strategy } : {}),
      ...(item.note ? { note: item.note } : {}),
    })
  })
  return { persons: [...personsByKey.values()], actions }
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
  for (const person of node.persons) if (person.editable && JSON.stringify(normalizedPersonStrategy(person, draft.personStrategies[person.key])) !== JSON.stringify({ key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: resolvedPersonStrategySelection(person) })) return true
  return hasContainerDraftChanges(nodeActionContainer(node), draft)
}

// resolvedPersonStrategySelection 在浏览器内按公开候选顺序投影最终名单。
export function resolvedPersonStrategySelection(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): string[] {
  const strategy = input?.strategy ?? person.strategy ?? 'manual'
  if (strategy === 'target_default') return copyPersonSelection(person.defaultSelected)
  if (strategy === 'all') return person.options.map(option => option.value)
  if (strategy === 'random') { if (!person.options.length) return []; const count = Math.min(person.options.length, Math.max(1, person.minCount || 1)); const start = normalizedPathConfigSeed(input?.seed ?? person.strategySeed) % person.options.length; return Array.from({ length: count }, (_, index) => person.options[(start + index) % person.options.length].value) }
  return copyPersonSelection(input?.selected ?? person.selected)
}

// normalizedPersonStrategy 生成可保存人员策略，并把前端计算的最终名单一并提交供服务端复验。
export function normalizedPersonStrategy(person: PathConfigPerson, input?: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput {
  const current = input ?? { key: person.key, strategy: person.strategy || 'manual', seed: person.strategySeed || 1, selected: copyPersonSelection(person.selected) }
  const seed = normalizedPathConfigSeed(current.seed)
  return { ...current, key: person.key, seed, selected: resolvedPersonStrategySelection(person, { ...current, seed }) }
}

// normalizedPathConfigSeed 保持浏览器 seed 在安全整数范围内。
export function normalizedPathConfigSeed(seed: unknown): number { return typeof seed === 'number' && Number.isSafeInteger(seed) && seed >= 1 ? seed : 1 }

// buildPersonSelectOptions 将人员候选按公司分组为 Naive UI 的 SelectGroupOption 或平铺列表。
export function buildPersonSelectOptions(person: PathConfigPerson): Array<Record<string, unknown>> {
  const groups = new Map<string, Array<{ label: string; value: string }>>()
  const ungrouped: Array<{ label: string; value: string }> = []

  for (const option of person.options) {
    const item = { label: option.label, value: option.value }
    const groupName = option.group ? option.group.trim() : ''
    if (groupName) {
      let list = groups.get(groupName)
      if (!list) {
        list = []
        groups.set(groupName, list)
      }
      list.push(item)
    } else {
      ungrouped.push(item)
    }
  }

  if (groups.size === 0) {
    return ungrouped
  }

  const result: Array<Record<string, unknown>> = []
  for (const [groupName, children] of groups.entries()) {
    result.push({
      type: 'group',
      label: groupName,
      key: groupName,
      children,
    })
  }
  if (ungrouped.length > 0) {
    result.push({
      type: 'group',
      label: '其他',
      key: '__other__',
      children: ungrouped,
    })
  }
  return result
}

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
function copyPathConfigPersonStrategy(input: PathConfigPersonStrategyInput): PathConfigPersonStrategyInput { return { key: input.key, strategy: input.strategy, seed: input.seed, selected: copyPersonSelection(input.selected) } }

// PathActionFlowLabels 保存流程图需要的中文名：动作键与语义节点键都是内部标识，不能直接显示。
export interface PathActionFlowLabels { actions: Record<string, string>; nodes: Record<string, string> }

// PathActionFlowSegment 是流程图里的一段连续步骤，同一段内的步骤属于同一个语义节点。
export interface PathActionFlowSegment { nodeKey: string; title: string; current: boolean; steps: PathCompiledActionStep[] }

// pathActionFlowLabels 汇总当前路径的中文名供只读流程图显示：分支与并行拆分节点不在可配置节点里，
// 它们的系统导航步骤只能从画布节点取名字，否则流程图上只剩一个没有出处的兜底文案。
export function pathActionFlowLabels(configuration: PathConfiguration | null, graph: FlowGraph | null, graphNodeIDByConfigurationKey: Map<string, string>): PathActionFlowLabels {
  const actions: Record<string, string> = {}; const nodes: Record<string, string> = {}
  if (!configuration) return { actions, nodes }
  const graphNameByID = new Map((graph?.nodes ?? []).map(node => [node.id, node.name]))
  for (const [nodeKey, graphNodeID] of graphNodeIDByConfigurationKey) { const name = graphNameByID.get(graphNodeID); if (name) nodes[nodeKey] = name }
  const collect = (items: Array<{ kind: PathConfigActionKind; label: string }>) => { for (const item of items) if (item.label && !actions[item.kind]) actions[item.kind] = item.label }
  collect(configuration.instanceActions?.catalog ?? []); collect(configuration.instanceActions?.actions ?? [])
  if (configuration.instanceActionKey) nodes[configuration.instanceActionKey] = '实例级动作'
  for (const group of configuration.groups) for (const node of group.nodes) {
    if (node.name) nodes[node.key] = node.name
    collect(node.actionConfiguration?.catalog ?? []); collect(node.actionConfiguration?.actions ?? [])
  }
  return { actions, nodes }
}

// pathActionFlowSegments 把只读编译步骤按 sequence 排序后切成连续节点段，让流程图能显示步骤落在哪个节点。
export function pathActionFlowSegments(steps: PathCompiledActionStep[], labels: PathActionFlowLabels, currentNodeKey: string): PathActionFlowSegment[] {
  const ordered = [...(steps ?? [])].sort((left, right) => left.sequence - right.sequence)
  return ordered.reduce<PathActionFlowSegment[]>((segments, step) => {
    const nodeKey = String(step.nodeKey ?? '')
    const last = segments[segments.length - 1]
    if (last && last.nodeKey === nodeKey) return [...segments.slice(0, -1), { ...last, steps: [...last.steps, step] }]
    return [...segments, { nodeKey, title: pathActionFlowSegmentTitle(nodeKey, labels), current: Boolean(nodeKey) && nodeKey === currentNodeKey, steps: [step] }]
  }, [])
}

// pathActionFlowSegmentTitle 只显示节点中文名；未在当前路径投影里的键退回通用说明，不暴露内部键。
function pathActionFlowSegmentTitle(nodeKey: string, labels: PathActionFlowLabels): string {
  if (!nodeKey) return '整个流程实例'
  return labels.nodes[nodeKey] || '流程中的其他节点'
}

// pathActionFlowStepName 优先用目录里的中文动作名，系统自动步骤没有用户动作语义。
export function pathActionFlowStepName(step: PathCompiledActionStep, labels: PathActionFlowLabels): string {
  if (step.action === 'system_automatic') return '目标引擎自动处理'
  return labels.actions[step.action] || step.action
}

// pathActionFlowParameters 把服务端编译出的参数展平成可读文本，浏览器不提交任何步骤正文。
export function pathActionFlowParameters(step: PathCompiledActionStep): string[] {
  return Object.entries(step.parameters ?? {}).map(([name, value]) => `${name}=${value !== null && typeof value === 'object' ? JSON.stringify(value) : String(value)}`)
}
