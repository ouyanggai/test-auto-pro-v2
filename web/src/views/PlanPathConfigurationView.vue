<script setup lang="ts">
import { NAlert, NButton, NCard, NCollapse, NCollapseItem, NEmpty, NModal, NSelect, NSpace, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { fetchExecutionPath, fetchExecutionPaths } from '../features/execution-paths/api'
import type { ExecutionPath } from '../features/execution-paths/types'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import FormRuntimeFrame from '../features/path-configuration/FormRuntimeFrame.vue'
import NodeConfigurationPanel from '../features/path-configuration/NodeConfigurationPanel.vue'
import {
  bindPathConfigurationNodes,
  buildPathConfigNodeSavePayload,
	copyPathConfigActions,
  currentNodeConfigurationComplete,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  nextFormGenerationSeed,
  pathConfigurationMessage,
  pathConfigurationStatusName,
  pathConfigurationNodesByGraphID,
  projectPathConfigurationNodeStates,
  resolveConfirmedNodeSaveDestination,
} from '../features/path-configuration/logic'
import {
  copyPathConfigurationCycles,
  fetchPathConfiguration,
  fetchPathFormRuntimeSession,
  generatePathFormData,
  PathConfigApiError,
  savePathConfigurationNode,
  savePathFormData,
} from '../features/path-configuration/api'
import { retryPathLoad } from '../features/path-configuration/retry'
import type {
  PathConfigActionCycleInput,
	PathConfigConfiguredActionInput,
  PathConfigDraft,
  PathConfigNode,
  PathConfiguration,
  PathConfigPerson,
  PathConfigPersonStrategyInput,
  PathFormRuntimeSession,
} from '../features/path-configuration/types'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import type { PersistedPlan } from '../features/plans/types'

type FormRuntimeExpose = InstanceType<typeof FormRuntimeFrame> & {
  setGeneratedData: (values: Record<string, unknown>, paths: string[], manualPaths: string[]) => Promise<Record<string, unknown>>
  reloadRuntime: () => Promise<Record<string, unknown>>
  restoreSaved: () => Promise<Record<string, unknown>>
  getValues: () => Promise<Record<string, unknown>>
  validateAndGetValues: () => Promise<Record<string, unknown>>
}

const route = useRoute()
const router = useRouter()
const themeVars = useThemeVars()
const planID = computed(() => String(route.params.planId || ''))
const pathID = computed(() => String(route.params.pathId || ''))
const plan = ref<PersistedPlan | null>(null)
const graph = ref<FlowGraph | null>(null)
const currentPath = ref<ExecutionPath | null>(null)
const executionPaths = ref<ExecutionPath[]>([])
const configuration = ref<PathConfiguration | null>(null)
const draft = ref<PathConfigDraft>({ fields: {}, persons: {}, personStrategies: {}, actionConfigurations: {} })
const actionCycles = ref<PathConfigActionCycleInput[]>([])
const configurationByGraphNodeID = ref(new Map<string, PathConfigNode>())
const graphNodeIDByConfigurationKey = ref(new Map<string, string>())
const selectedNodeID = ref('')
const workspace = ref<'nodes' | 'form'>('nodes')
const runtimeSession = ref<PathFormRuntimeSession | null>(null)
const runtimeUnsupported = ref<string[]>([])
const pageLoading = ref(false)
const pathDetailLoading = ref(false)
const savingNode = ref(false)
const formRuntimeLoading = ref(false)
const formGenerating = ref(false)
const formGenerationKind = ref<'smart' | 'next' | null>(null)
const formRestoring = ref(false)
const formSaving = ref(false)
const pageError = ref('')
const nodeSaveError = ref('')
const nodeSaveDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const formError = ref('')
const formErrorDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const nodeSavedSuccessfully = ref(false)
const formSavedSuccessfully = ref(false)
const cycleCopyModalOpen = ref(false)
const cycleCopyTargetID = ref('')
const cycleCopyBusy = ref(false)
const cycleCopyError = ref('')
const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)
const formFrame = ref<FormRuntimeExpose | null>(null)
let loadVersion = 0
let loadController: AbortController | null = null
let runtimeEpoch = 0
let runtimeSessionController: AbortController | null = null
let formOperationController: AbortController | null = null
let nodeSaveKey = ''
let formSaveKey = ''

const pageThemeStyle = computed(() => ({
  '--path-config-page-color': themeVars.value.bodyColor,
  '--path-config-card-color': themeVars.value.cardColor,
  '--path-config-border-color': themeVars.value.borderColor,
  '--path-config-text-color': themeVars.value.textColor1,
  '--path-config-text-secondary-color': themeVars.value.textColor2,
}))
const planMutable = computed(() => plan.value?.status === 'not_started')
const formReadOnly = computed(() => !planMutable.value || Boolean(configuration.value?.form.readOnly))
const runtimeForm = computed(() => configuration.value ? { ...configuration.value.form, readOnly: formReadOnly.value } : null)
const pathAnalysis = computed(() => graph.value && currentPath.value ? analyzeExecutionPath(graph.value, currentPath.value.choices) : null)
const selectedNode = computed(() => configurationByGraphNodeID.value.get(selectedNodeID.value) ?? null)
const configurationNodeStates = computed(() => graph.value && pathAnalysis.value
  ? projectPathConfigurationNodeStates(graph.value, pathAnalysis.value, configurationByGraphNodeID.value, selectedNodeID.value)
  : {})
const selectedNodeRequirement = computed(() => currentNodeConfigurationComplete(selectedNode.value, draft.value))
const nodeSaveDisabled = computed(() => !planMutable.value || pageLoading.value || savingNode.value || !selectedNode.value || selectedNode.value.lineBlocked || selectedNode.value.status === 'not_required' || selectedNode.value.status === 'runtime' || !selectedNodeRequirement.value.complete)
const saveAllNodesDisabled = computed(() => !planMutable.value || pageLoading.value || savingNode.value || !configuration.value)
const runtimeBlockingReasons = computed(() => [...new Set([
  ...(configuration.value?.form.unsupported ?? []),
  ...(configuration.value?.form.conditionReviews ?? []),
  ...runtimeUnsupported.value,
])])
const runtimeBlocked = computed(() => runtimeBlockingReasons.value.length > 0)
// pathSignature 与服务端使用同一选择序列，前端只展示可能成功的目标，最终仍由服务端复验。
function pathSignature(path: ExecutionPath): string { return path.choices.map(choice => `${choice.routeNodeId.trim()}:${choice.branchId.trim()}`).join('|') }
const cycleCopyTargets = computed(() => {
  const current = currentPath.value
	if (!planMutable.value || !current || !configuration.value?.actionCycles.length) return []
  const signature = pathSignature(current)
  return executionPaths.value.filter(path => path.id !== current.id && pathSignature(path) === signature)
})

// applyRuntimeFormState 只接受 iframe 已核验会话回传的统计与人工覆盖摘要，宿主不自行猜测 FormMaking 当前字段值。
function applyRuntimeFormState(payload: Record<string, unknown>) {
  if (workspace.value !== 'form') return
  const form = configuration.value?.form
  if (!form) return
  const stats = payload.stats
  if (stats && typeof stats === 'object') {
    const runtimeStats = stats as { autoFilled?: unknown, manualPending?: unknown }
    if (typeof runtimeStats.autoFilled === 'number' && Number.isInteger(runtimeStats.autoFilled) && runtimeStats.autoFilled >= 0) form.autoFilled = runtimeStats.autoFilled
    if (typeof runtimeStats.manualPending === 'number' && Number.isInteger(runtimeStats.manualPending) && runtimeStats.manualPending >= 0) form.manualPending = runtimeStats.manualPending
  }
  if (Array.isArray(payload.manualOverridePaths)) form.manualOverridePaths = payload.manualOverridePaths.map(String)
}

// handleRuntimeReady 接收真实组件注册表与首次字段统计，避免初始化阶段仍显示旧的生成器估算值。
function handleRuntimeReady(payload: Record<string, unknown>) {
  if (workspace.value !== 'form' || !runtimeSession.value) return
  runtimeUnsupported.value = Array.isArray(payload.unsupported) ? payload.unsupported.map(String) : []
  applyRuntimeFormState(payload)
}

// handleRuntimeError 忽略离开表单后的迟到错误，避免旧 iframe 覆盖节点工作区状态。
function handleRuntimeError(message: string) {
  if (workspace.value === 'form' && runtimeSession.value) formError.value = message
}

// invalidateRuntimeSession 只由宿主失效会话和异步代次，iframe 实例的销毁交给它自己的卸载钩子完成。
function invalidateRuntimeSession() {
  runtimeEpoch += 1
  runtimeSessionController?.abort()
  runtimeSessionController = null
  formOperationController?.abort()
  formOperationController = null
  runtimeSession.value = null
  runtimeUnsupported.value = []
  formRuntimeLoading.value = false
  formGenerating.value = false
  formGenerationKind.value = null
  formRestoring.value = false
  formSaving.value = false
}

// isActiveFormOperation 防止返回节点或路由切换后，旧 iframe 请求反写当前页面。
function isActiveFormOperation(epoch: number, frame: FormRuntimeExpose | null): boolean {
  return epoch === runtimeEpoch && workspace.value === 'form' && formFrame.value === frame && runtimeSession.value !== null
}

// publicPageError 把读取链路异常收敛为不含内部标识的稳定页面错误。
function publicPageError(caught: unknown): string {
  if (caught instanceof PlanApiError || caught instanceof PathConfigApiError) return pathConfigurationMessage(caught.message)
  if (caught instanceof Error && ['已保存路径不存在或已删除', '当前已保存路径与真实流程不一致，请先编辑路径', '路径节点配置与当前流程结构不一致'].includes(caught.message)) return pathConfigurationMessage(caught.message)
  return '暂时无法读取路径配置，请重试'
}

// focusSelectedNode 只在首次加载或用户明确要求时定位，普通节点切换不抢夺画布位置。
async function focusSelectedNode() {
  if (!selectedNodeID.value) return
  await nextTick()
  await new Promise<void>(resolve => window.requestAnimationFrame(() => resolve()))
  await canvasRef.value?.focusNode(selectedNodeID.value)
}

// applyConfiguration 用服务端权威响应重建草稿和图节点映射，不在前端猜测保存后的完成状态。
async function applyConfiguration(next: PathConfiguration, preserveSelected = true) {
  if (!graph.value) return
  const selected = selectedNodeID.value
  const bindings = await bindPathConfigurationNodes(graph.value, next)
  configuration.value = next
  draft.value = initPathConfigDraft(next)
  actionCycles.value = next.actionCycles.map(cycle => ({ key: cycle.key, type: cycle.type, endNodeKey: cycle.endNodeKey, count: cycle.count }))
  configurationByGraphNodeID.value = bindings.byGraphNodeID
  graphNodeIDByConfigurationKey.value = bindings.graphNodeIDByKey
  selectedNodeID.value = preserveSelected && bindings.byGraphNodeID.has(selected)
    ? selected
    : initialPathConfigurationNodeID(next, bindings.graphNodeIDByKey)
}

// loadPage 读取计划、路径摘要、单条路径 choices、真实图和权威节点/表单配置，切换路径时销毁旧 SID 会话。
async function loadPage() {
  loadController?.abort()
  invalidateRuntimeSession()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  pageLoading.value = true
  pathDetailLoading.value = false
  pageError.value = ''
  nodeSaveError.value = ''
  formError.value = ''
  nodeSavedSuccessfully.value = false
  formSavedSuccessfully.value = false
  workspace.value = 'nodes'
  try {
    const [storedPlan, storedGraph, storedPaths] = await retryPathLoad(signal => Promise.all([
      fetchPlan(planID.value, signal),
      fetchFlowGraph(planID.value, signal),
      fetchExecutionPaths(planID.value, signal),
    ]), controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    pathDetailLoading.value = true
    const storedPath = await fetchExecutionPath(planID.value, pathID.value, controller.signal)
    pathDetailLoading.value = false
    if (controller.signal.aborted || version !== loadVersion) return
    if (!storedPaths.some(path => path.id === storedPath.id)) throw new Error('已保存路径不存在或已删除')
    const analysis = analyzeExecutionPath(storedGraph, storedPath.choices)
    if (!analysis.complete || analysis.invalid) throw new Error('当前已保存路径与真实流程不一致，请先编辑路径')
    const storedConfiguration = await fetchPathConfiguration(planID.value, pathID.value, controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = storedPlan
    graph.value = storedGraph
    currentPath.value = storedPath
    executionPaths.value = storedPaths
    await applyConfiguration(storedConfiguration, false)
    nodeSaveKey = crypto.randomUUID()
    formSaveKey = crypto.randomUUID()
  }
  catch (caught) {
    if (!controller.signal.aborted && version === loadVersion) pageError.value = publicPageError(caught)
  }
  finally {
    if (version === loadVersion) {
      pageLoading.value = false
      pathDetailLoading.value = false
    }
  }
  if (version === loadVersion && configuration.value) await focusSelectedNode()
}

// reloadConfiguration 只刷新权威配置和路径轻量状态，保持图视口与当前工作区。
async function reloadConfiguration(signal?: AbortSignal) {
  const latest = await fetchPathConfiguration(planID.value, pathID.value, signal || new AbortController().signal)
  await applyConfiguration(latest)
  try { executionPaths.value = await fetchExecutionPaths(planID.value, signal ?? new AbortController().signal) }
  catch { /* 保存事实已由配置 GET 确认，列表刷新失败不回滚当前结果。 */ }
  return latest
}

// selectConfigurationNode 切换侧栏并给节点明确选中反馈，不重置用户视口。
function selectConfigurationNode(nodeID: string) {
  if (!configurationByGraphNodeID.value.has(nodeID)) return
  selectedNodeID.value = nodeID
  nodeSavedSuccessfully.value = false
  nodeSaveError.value = ''
}

// selectNextConfigurationNode 显式定位服务端给出的下一待配置节点并保持当前缩放。
async function selectNextConfigurationNode() {
  const key = configuration.value?.nextNodeKey
  const nodeID = key ? graphNodeIDByConfigurationKey.value.get(key) : ''
  if (!nodeID) return
  selectedNodeID.value = nodeID
  await focusSelectedNode()
}

// finishConfirmedNodeSave 让正常响应与 GET 对账共用同一推进规则，绝不把“下一节点”解释为另一条路径。
async function finishConfirmedNodeSave() {
  const current = configuration.value
  if (!current) return
  nodeSaveKey = crypto.randomUUID()
  nodeSaveError.value = ''
  nodeSaveDetails.value = []
  const destination = resolveConfirmedNodeSaveDestination(
    current.nextNodeKey,
    graphNodeIDByConfigurationKey.value,
    current.form.status,
  )
  if (destination.kind === 'next-node') {
    // 推进前清除上一节点成功态，侧栏立即成为下一节点的真实草稿和要求。
    nodeSavedSuccessfully.value = false
    selectedNodeID.value = destination.nodeID
    await focusSelectedNode()
    return
  }
  if (destination.kind === 'unmapped') {
    nodeSavedSuccessfully.value = false
    nodeSaveError.value = '路径节点配置与当前流程结构不一致，请重新读取'
    return
  }
  nodeSavedSuccessfully.value = true
}

// updatePersonStrategy 只保留当前模板策略和候选中的不透明值。
function updatePersonStrategy(person: PathConfigPerson, value: PathConfigPersonStrategyInput) {
	if (!planMutable.value) return
  const allowed = new Set(person.options.map(option => option.value))
  draft.value.personStrategies[person.key] = { ...value, selected: value.selected.filter(candidate => allowed.has(candidate)) }
  draft.value.persons[person.key] = [...draft.value.personStrategies[person.key].selected]
  nodeSavedSuccessfully.value = false
}

// updateNodeActionConfiguration 替换当前节点独立动作草稿，不允许面板越过节点边界写其他节点。
function updateNodeActionConfiguration(nodeKey: string, value: PathConfigConfiguredActionInput[]) {
	if (!planMutable.value) return
	if (selectedNode.value?.key !== nodeKey) return
	// 子组件事件值仍可能携带 Vue Proxy；父页面只持有普通草稿，避免保存前再次触发克隆异常。
	draft.value.actionConfigurations[nodeKey] = copyPathConfigActions(value)
  nodeSavedSuccessfully.value = false
}

// updateActionCycles 保留循环草稿直到当前节点保存成功；成员与前驱仍由服务端重新派生。
function updateActionCycles(value: PathConfigActionCycleInput[]) {
	if (!planMutable.value) return
  actionCycles.value = value.map(cycle => ({ ...cycle }))
  nodeSavedSuccessfully.value = false
}

// openCycleCopy 打开来源路径的安全复制确认，只允许当前已保存循环复制到兼容路径。
function openCycleCopy() {
	if (!planMutable.value) return
  cycleCopyError.value = ''
  cycleCopyTargetID.value = cycleCopyTargets.value[0]?.id ?? ''
  cycleCopyModalOpen.value = true
}

// copyCycles 确认后只写目标路径的循环命名空间，不触发目标平台接口。
async function copyCycles() {
  const source = currentPath.value
	if (!planMutable.value || !source || !cycleCopyTargetID.value || cycleCopyBusy.value) return
  cycleCopyBusy.value = true
  cycleCopyError.value = ''
  try {
    await copyPathConfigurationCycles(planID.value, cycleCopyTargetID.value, source.id, crypto.randomUUID())
    cycleCopyModalOpen.value = false
  }
  catch (caught) {
    cycleCopyError.value = caught instanceof Error ? pathConfigurationMessage(caught.message) : '循环复制失败，请重试'
  }
  finally { cycleCopyBusy.value = false }
}

// saveCurrentNode 保存当前节点后立即 GET 对账；请求响应丢失时也以服务端事实为准。
async function saveCurrentNode() {
  const current = configuration.value
  const node = selectedNode.value
  if (!current || !node || nodeSaveDisabled.value) return
  savingNode.value = true
  nodeSaveError.value = ''
  nodeSaveDetails.value = []
  nodeSavedSuccessfully.value = false
  const previousRevision = current.nodeRevision
  try {
    await savePathConfigurationNode(planID.value, pathID.value, node.key, previousRevision, buildPathConfigNodeSavePayload(node, draft.value, actionCycles.value), nodeSaveKey)
    await reloadConfiguration()
    await finishConfirmedNodeSave()
  }
  catch (caught) {
    try {
      const reconciled = await reloadConfiguration()
      const reconciledNode = reconciled.groups.flatMap(group => group.nodes).find(candidate => candidate.key === node.key)
      if (reconciled.nodeRevision > previousRevision && reconciledNode?.status === 'configured') {
        await finishConfirmedNodeSave()
        return
      }
    }
    catch { /* 对账失败保留原草稿和幂等键，用户重试不会重复写入。 */ }
    if (caught instanceof PathConfigApiError) {
      nodeSaveError.value = pathConfigurationMessage(caught.message)
      nodeSaveDetails.value = caught.details
    }
    else nodeSaveError.value = '保存失败，当前节点和草稿已保留，请重试'
  }
  finally {
    savingNode.value = false
  }
}

// saveAllNodes 按当前服务端修订号顺序保存所有已满足规则的节点，不改变当前选中节点。
async function saveAllNodes() {
  const current = configuration.value
  if (!current || saveAllNodesDisabled.value) return
  savingNode.value = true
  nodeSaveError.value = ''
  nodeSaveDetails.value = []
  nodeSavedSuccessfully.value = false
  let revision = current.nodeRevision
  let savedCount = 0
  const skipped: string[] = []
  try {
    const nodes = current.groups.flatMap(group => group.nodes).filter(node => !node.lineBlocked && node.status !== 'not_required' && node.status !== 'runtime')
    for (const node of nodes) {
      const completion = currentNodeConfigurationComplete(node, draft.value)
      if (!completion.complete) {
        skipped.push(node.name)
        continue
      }
      const result = await savePathConfigurationNode(
        planID.value,
        pathID.value,
        node.key,
        revision,
        buildPathConfigNodeSavePayload(node, draft.value, actionCycles.value),
        crypto.randomUUID(),
      )
      revision = result.nodeRevision
      savedCount += 1
    }
    await reloadConfiguration()
    if (skipped.length) {
      nodeSaveError.value = `已保存 ${savedCount} 个节点，还有 ${skipped.length} 个节点需要先补充配置`
    } else {
      nodeSaveError.value = ''
      nodeSavedSuccessfully.value = savedCount > 0
    }
  }
  catch (caught) {
    try { await reloadConfiguration() } catch { /* 保留原错误，用户可以再次点击保存全部节点。 */ }
    if (caught instanceof PathConfigApiError) {
      nodeSaveError.value = pathConfigurationMessage(caught.message)
      nodeSaveDetails.value = caught.details
    } else nodeSaveError.value = '保存全部节点失败，已保存的节点不会重复提交，请重试'
  }
  finally {
    savingNode.value = false
  }
}

// openFormWorkspace 获取短期 SID 后切换到全宽真实表单；SID 不进入配置对象或持久状态。
async function openFormWorkspace() {
  if (!configuration.value || formRuntimeLoading.value || formGenerating.value || formRestoring.value || formSaving.value) return
  workspace.value = 'form'
  formError.value = ''
  if (runtimeSession.value) return
  const epoch = ++runtimeEpoch
  const controller = new AbortController()
  runtimeSessionController?.abort()
  runtimeSessionController = controller
  formRuntimeLoading.value = true
  try {
    const session = await fetchPathFormRuntimeSession(planID.value, pathID.value, controller.signal)
    if (controller.signal.aborted || epoch !== runtimeEpoch || workspace.value !== 'form') return
    runtimeSession.value = session
  }
  catch (caught) {
    if (!controller.signal.aborted && epoch === runtimeEpoch && workspace.value === 'form') formError.value = publicPageError(caught)
  }
  finally {
    if (epoch === runtimeEpoch) formRuntimeLoading.value = false
    if (runtimeSessionController === controller) runtimeSessionController = null
  }
}

// returnToNodes 先失效宿主状态再切换工作区，iframe 只通过自身卸载钩子 teardown 一次。
function returnToNodes() {
  invalidateRuntimeSession()
  workspace.value = 'nodes'
}

// generateFormData 首次生成或换一组；换组仅替换生成器拥有字段，人工覆盖由 runtime 返回。
async function generateFormData(nextGroup: boolean) {
  const current = configuration.value
  const frame = formFrame.value
  const epoch = runtimeEpoch
	if (!current || formReadOnly.value || formRuntimeLoading.value || formGenerating.value || formRestoring.value || formSaving.value || !frame || !runtimeSession.value) return
  const controller = new AbortController()
  formOperationController?.abort()
  formOperationController = controller
  formGenerating.value = true
  formGenerationKind.value = nextGroup ? 'next' : 'smart'
  formError.value = ''
  formErrorDetails.value = []
  formSavedSuccessfully.value = false
  try {
    const captured = await frame.getValues()
    if (!isActiveFormOperation(epoch, frame)) return
    const values = (captured.values || current.form.values) as Record<string, unknown>
    const manual = Array.isArray(captured.manualOverridePaths) ? captured.manualOverridePaths.map(String) : current.form.manualOverridePaths
    const seed = nextGroup ? nextFormGenerationSeed(current.form.seed) : current.form.seed
    const generated = await generatePathFormData(planID.value, pathID.value, seed, values, manual, nextGroup, controller.signal)
    if (!isActiveFormOperation(epoch, frame)) return
    current.form.values = generated.values
    current.form.seed = generated.seed
    current.form.status = 'draft'
    current.form.statusName = pathConfigurationStatusName(current.form.status)
    current.form.generatedFieldPaths = generated.generatedFieldPaths
    current.form.manualOverridePaths = generated.manualOverridePaths
    current.form.sampleSummary = generated.sampleSummary
    current.form.autoFilled = generated.autoFilled
    current.form.manualPending = generated.manualPending
    current.form.unsupported = generated.unsupported
    current.form.conditionBindings = generated.conditionBindings
    current.form.conditionReviews = generated.conditionReviews
    current.form.fieldRules = generated.fieldRules
    formErrorDetails.value = generated.issues.map(issue => ({
      kind: issue.blocking ? 'generation_blocked' : 'generation_notice',
      name: issue.field,
      reason: issue.reason,
    }))
    if (generated.generationState === 'blocked') {
      formError.value = '当前路径条件无法自动完成，请按下方原因人工处理'
    }
    else if (generated.generationState === 'partial') {
      formError.value = generated.routeVerification.matched
        ? '已生成可安全使用的数据，仍有部分内容需要人工核对'
        : '已保留可安全生成的数据，当前完整路径仍需人工核对'
    }
    // 字段规则只能在 FormMaking 创建组件前生效；重新载入后统计由真实运行时重新对账。
    await nextTick()
    if (!isActiveFormOperation(epoch, frame)) return
    applyRuntimeFormState(await frame.reloadRuntime())
  }
  catch (caught) {
    if (isActiveFormOperation(epoch, frame) && !(caught instanceof DOMException && caught.name === 'AbortError')) formError.value = publicPageError(caught)
  }
  finally {
    if (epoch === runtimeEpoch) {
      formGenerating.value = false
      formGenerationKind.value = null
    }
    if (formOperationController === controller) formOperationController = null
  }
}

// restoreSavedForm 恢复本次 GET 装载的服务端 values，不重读或重新生成。
async function restoreSavedForm() {
  const frame = formFrame.value
  const epoch = runtimeEpoch
	if (formReadOnly.value || !frame || !runtimeSession.value || formRuntimeLoading.value || formGenerating.value || formRestoring.value || formSaving.value) return
  const controller = new AbortController()
  formOperationController?.abort()
  formOperationController = controller
  formRestoring.value = true
  formError.value = ''
  try {
    const restored = await frame.restoreSaved()
    if (!isActiveFormOperation(epoch, frame)) return
    applyRuntimeFormState(restored)
  }
  catch (caught) {
    if (isActiveFormOperation(epoch, frame) && !controller.signal.aborted) formError.value = publicPageError(caught)
  }
  finally {
    if (epoch === runtimeEpoch) formRestoring.value = false
    if (formOperationController === controller) formOperationController = null
  }
}

// saveFormData 先经真实 getData(true)/getValues，再由服务端按最新模板与路径复验并独立保存。
async function saveFormData() {
  const current = configuration.value
  const frame = formFrame.value
  const epoch = runtimeEpoch
	if (!current || formReadOnly.value || runtimeBlocked.value || formRuntimeLoading.value || formGenerating.value || formRestoring.value || formSaving.value || !frame || !runtimeSession.value) return
  const controller = new AbortController()
  formOperationController?.abort()
  formOperationController = controller
  formSaving.value = true
  formError.value = ''
  formErrorDetails.value = []
  formSavedSuccessfully.value = false
  const previousRevision = current.form.revision
  try {
    const captured = await frame.validateAndGetValues()
    if (!isActiveFormOperation(epoch, frame)) return
    await savePathFormData(planID.value, pathID.value, formSaveKey, {
      revision: previousRevision,
      values: captured.values as Record<string, unknown>,
      seed: current.form.seed,
      generatedFieldPaths: Array.isArray(captured.generatedFieldPaths) ? captured.generatedFieldPaths.map(String) : current.form.generatedFieldPaths,
      manualOverridePaths: Array.isArray(captured.manualOverridePaths) ? captured.manualOverridePaths.map(String) : current.form.manualOverridePaths,
      sampleSummary: current.form.sampleSummary,
      validated: true,
      // 真实运行时注册表是组件支持性的唯一来源；服务端据此阻止未知组件被绕过保存。
      unsupported: Array.isArray(captured.unsupported) ? captured.unsupported.map(String) : runtimeUnsupported.value,
    }, controller.signal)
    if (!isActiveFormOperation(epoch, frame)) return
    await reloadConfiguration(controller.signal)
    if (!isActiveFormOperation(epoch, frame)) return
    formSaveKey = crypto.randomUUID()
    formSavedSuccessfully.value = true
    // 保存成功后自动回到节点画布；发起人节点上会显示“表单已配置”提示。
    returnToNodes()
  }
  catch (caught) {
    if (!isActiveFormOperation(epoch, frame) || controller.signal.aborted) return
    try {
      const reconciled = await reloadConfiguration(controller.signal)
      if (!isActiveFormOperation(epoch, frame)) return
      if (reconciled.form.revision > previousRevision && reconciled.form.status === 'valid') {
        formSaveKey = crypto.randomUUID()
        formSavedSuccessfully.value = true
        returnToNodes()
        return
      }
    }
    catch { /* 对账失败保留当前 iframe、values 和幂等键。 */ }
    formError.value = caught instanceof Error ? pathConfigurationMessage(caught.message) : '保存失败，当前表单数据已保留，请重试'
    formErrorDetails.value = caught instanceof PathConfigApiError ? caught.details : []
  }
  finally {
    if (epoch === runtimeEpoch) formSaving.value = false
    if (formOperationController === controller) formOperationController = null
  }
}

// backToPlan 只失效宿主会话并导航，iframe 卸载由 Vue 子组件生命周期负责。
function backToPlan() {
  invalidateRuntimeSession()
  router.push('/plans/' + planID.value + '/paths')
}

watch([planID, pathID], () => { void loadPage() })
onBeforeUnmount(() => {
  loadVersion++
  loadController?.abort()
  invalidateRuntimeSession()
})

void loadPage()
</script>

<template>
  <main
    class="path-configuration-page"
    :class="{ 'path-configuration-page--form': workspace === 'form' }"
    :style="pageThemeStyle"
    :aria-busy="pageLoading"
  >
    <header class="path-configuration-page__header">
      <div class="path-configuration-page__identity">
        <n-button text type="primary" @click="backToPlan">返回计划详情</n-button>
        <div>
          <h1>路径配置</h1>
          <p v-if="plan && configuration">{{ plan.name }} · #{{ configuration.path.sequenceNo }} {{ configuration.path.name }}</p>
        </div>
      </div>
      <div v-if="configuration" class="path-configuration-page__progress" aria-label="路径配置进度">
        <span>已准备 {{ configuration.preparation.preparedNodes }} 个节点</span>
        <span>还有 {{ configuration.preparation.pendingItems }} 项需要处理</span>
        <span>节点 {{ configuration.progress.completed }} / {{ configuration.progress.total }}</span>
        <n-button v-if="workspace === 'nodes' && configuration.nextNodeKey" size="small" secondary @click="selectNextConfigurationNode">下一待配置节点</n-button>
		<n-button v-if="planMutable && workspace === 'nodes' && configuration.actionCycles.length" size="small" :disabled="!cycleCopyTargets.length" @click="openCycleCopy">复制已保存循环</n-button>
      </div>
    </header>

    <n-modal v-model:show="cycleCopyModalOpen">
      <n-card title="复制已保存循环" style="width: min(620px, 94vw)">
        <div class="path-configuration-page__cycle-body">
          <n-alert type="info" :show-icon="false">只允许复制到流程结构完全一致的路径；复制只影响本系统配置，不会调用目标平台，也不会覆盖源路径。</n-alert>
          <n-alert v-if="cycleCopyError" type="error" :show-icon="false">{{ cycleCopyError }}</n-alert>
          <n-select
            v-model:value="cycleCopyTargetID"
            :options="cycleCopyTargets.map(path => ({ label: `#${path.sequenceNo} ${path.name}`, value: path.id }))"
            placeholder="选择结构一致的目标路径"
          />
        </div>
        <template #footer>
          <n-space justify="end"><n-button @click="cycleCopyModalOpen = false">取消</n-button><n-button type="primary" :loading="cycleCopyBusy" :disabled="!cycleCopyTargetID" @click="copyCycles">确认复制</n-button></n-space>
        </template>
      </n-card>
    </n-modal>

    <nav v-if="configuration" class="path-configuration-page__switch" aria-label="配置工作区">
      <n-button :type="workspace === 'nodes' ? 'primary' : 'default'" :secondary="workspace !== 'nodes'" @click="returnToNodes">节点配置</n-button>
      <n-button :type="workspace === 'form' ? 'primary' : 'default'" :secondary="workspace !== 'form'" @click="openFormWorkspace">表单数据</n-button>
    </nav>

    <section class="path-configuration-page__stage">
      <section v-if="pageLoading" class="path-configuration-page__initial-loading" role="status" aria-live="polite">
        <n-spin :show="true" size="large" :description="pathDetailLoading ? '正在读取路径详情' : '正在加载路径配置'" />
      </section>

      <n-alert v-else-if="pageError" type="error" :show-icon="false" class="path-configuration-page__error">
        <div class="path-configuration-page__error-content">
          <span>{{ pageError }}</span>
          <n-button size="small" @click="loadPage">重新读取</n-button>
        </div>
      </n-alert>

      <flow-graph-canvas
        v-else-if="workspace === 'nodes' && graph && currentPath && configuration"
        ref="canvasRef"
        class="path-configuration-page__canvas"
        :graph="graph"
        :choices="currentPath.choices"
        configuration-mode
        :configuration-node-states="configurationNodeStates"
        :configuration-form-status="configuration.form.status"
        :configuration-form-status-name="pathConfigurationStatusName(configuration.form.status)"
        @select-configuration-node="selectConfigurationNode"
        @open-configuration-form="openFormWorkspace"
        @retry="loadPage"
      >
        <template #configuration-panel>
          <node-configuration-panel
			:node="selectedNode"
			:draft="draft"
			:saving="savingNode"
			:read-only="!planMutable"
            :save-disabled="nodeSaveDisabled"
            :save-all-disabled="saveAllNodesDisabled"
            :missing-count="selectedNodeRequirement.missing.length"
            :save-error="nodeSaveError"
            :save-details="nodeSaveDetails"
            :saved-successfully="nodeSavedSuccessfully"
            :form-complete="configuration.form.status === 'valid'"
            :action-cycles="configuration.actionCycles"
            @update-person-strategy="updatePersonStrategy"
            @update-action-configuration="updateNodeActionConfiguration"
            @update-action-cycles="updateActionCycles"
            @save="saveCurrentNode"
            @save-all="saveAllNodes"
            @back-to-plan="backToPlan"
            @open-form="openFormWorkspace"
          />
        </template>
      </flow-graph-canvas>

      <section v-else-if="workspace === 'form' && configuration" class="path-configuration-page__form-workspace">
        <header class="path-configuration-page__form-toolbar">
          <div>
            <h2>表单数据</h2>
				<p v-if="formReadOnly">当前计划的表单数据只读。</p>
            <p v-else>自动填充 {{ configuration.form.autoFilled }} 项 · 仍需手工 {{ configuration.form.manualPending }} 项</p>
          </div>
          <div class="path-configuration-page__form-actions">
            <n-button size="small" @click="returnToNodes">返回节点画布</n-button>
			<template v-if="!formReadOnly">
              <n-button size="small" :loading="formGenerating && formGenerationKind === 'smart'" :disabled="formRuntimeLoading || formGenerating || formRestoring || formSaving" @click="generateFormData(false)">智能生成</n-button>
              <n-button size="small" :loading="formGenerating && formGenerationKind === 'next'" :disabled="formRuntimeLoading || formGenerating || formRestoring || formSaving" @click="generateFormData(true)">换一组</n-button>
              <n-button size="small" :loading="formRestoring" :disabled="formRuntimeLoading || formGenerating || formRestoring || formSaving" @click="restoreSavedForm">恢复已保存</n-button>
              <n-button size="small" type="primary" :loading="formSaving" :disabled="runtimeBlocked || formRuntimeLoading || formGenerating || formRestoring" @click="saveFormData">保存表单数据</n-button>
            </template>
          </div>
        </header>
        <section v-if="configuration.form.conditionBindings.length" class="path-configuration-page__form-hints" aria-label="当前路径分支条件">
          <n-collapse :default-expanded-names="['path-conditions']" arrow-placement="right">
            <n-collapse-item title="当前路径分支条件" name="path-conditions">
              <div class="path-configuration-page__form-hints-body">
                <ul class="path-configuration-page__form-hints-list">
                  <li v-for="binding in configuration.form.conditionBindings" :key="binding.key" :class="{ 'path-configuration-page__form-hint--selected': binding.selected, 'path-configuration-page__form-hint--review': binding.needsReview }">
                    <div class="path-configuration-page__form-hint-head">
                      <strong>{{ binding.nodeName }}</strong>
                      <span>{{ binding.branchName }}</span>
                      <n-tag v-if="binding.selected" size="small" type="success" :bordered="false">当前路径</n-tag>
                      <n-tag v-if="binding.locked" size="small" type="warning" :bordered="false">字段已锁定</n-tag>
                      <n-tag v-if="binding.needsReview" size="small" type="error" :bordered="false">需要人工核对</n-tag>
                    </div>
                    <p>{{ binding.expression }}</p>
                    <small v-if="Array.isArray(binding.fields) && binding.fields.length">{{ binding.fields.join('、') }}{{ binding.locked ? '：由当前路径条件保持' : '' }}</small>
                  </li>
                </ul>
              </div>
            </n-collapse-item>
          </n-collapse>
        </section>
        <div v-if="formError || formSavedSuccessfully || runtimeBlocked" class="path-configuration-page__form-feedback">
          <n-alert v-if="formError" type="error" :show-icon="false" size="small">
            <div>{{ formError }}</div>
            <ul v-if="formErrorDetails.length" class="path-configuration-page__form-error-details">
              <li v-for="(item, index) in formErrorDetails" :key="`${item.kind}-${index}`">{{ pathConfigurationMessage(item.reason || item.name) }}</li>
            </ul>
          </n-alert>
          <n-alert v-else-if="formSavedSuccessfully" type="success" :show-icon="false" size="small">
            表单数据已保存并完成服务端复验。节点仍需逐个完成，整条路径不会被静默标记。
          </n-alert>
          <n-alert v-if="runtimeBlocked" type="warning" :show-icon="false" size="small">
            {{ runtimeBlockingReasons.map(pathConfigurationMessage).join('；') }}
          </n-alert>
        </div>
        <section v-if="formRuntimeLoading" class="path-configuration-page__form-loading" role="status" aria-live="polite">
          <n-spin :show="true" size="large" description="正在加载表单运行时" />
        </section>
        <form-runtime-frame
		  v-else-if="runtimeSession && runtimeForm"
          ref="formFrame"
          class="path-configuration-page__form-frame"
		  :form="runtimeForm"
          :runtime-session="runtimeSession"
          @ready="handleRuntimeReady"
          @state="applyRuntimeFormState"
          @error="handleRuntimeError"
        />
        <n-empty v-else description="表单运行时会话暂不可用，请返回节点画布后重试" />
      </section>

      <n-empty v-else description="当前路径没有可配置内容" />
    </section>
  </main>
</template>

<style scoped>
.path-configuration-page {
  --path-config-app-header-height: 64px;
  --path-config-main-block-padding: 40px;
  --path-config-main-inline-padding: 48px;
  --path-config-form-toolbar-height: 54px;

  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  width: 100%;
  height: calc(100dvh - 144px);
  min-height: 600px;
  overflow: hidden;
  color: var(--path-config-text-color);
  background: var(--path-config-page-color);
}

.path-configuration-page--form {
  grid-template-rows: minmax(0, 1fr);
  width: calc(100% + var(--path-config-main-inline-padding) + var(--path-config-main-inline-padding));
  height: calc(100dvh - var(--path-config-app-header-height));
  min-height: calc(100dvh - var(--path-config-app-header-height));
  max-height: calc(100dvh - var(--path-config-app-header-height));
  margin: calc(0px - var(--path-config-main-block-padding)) calc(0px - var(--path-config-main-inline-padding));
  overflow: hidden;
}

/* 表单自身工具栏已包含返回入口；隐藏重复页头和导航才能把顶栏以下高度完整交给真实 FormMaking。 */
.path-configuration-page--form > .path-configuration-page__header,
.path-configuration-page--form > .path-configuration-page__switch {
  display: none;
}

.path-configuration-page__header,
.path-configuration-page__identity,
.path-configuration-page__progress,
.path-configuration-page__switch,
.path-configuration-page__form-toolbar,
.path-configuration-page__form-actions {
  display: flex;
  align-items: center;
}

.path-configuration-page__header { justify-content: space-between; gap: 24px; padding: 4px 0 14px; }
.path-configuration-page__identity { align-items: flex-start; gap: 12px; min-width: 0; }
.path-configuration-page__identity > :deep(.n-button) { flex: 0 0 auto; margin-top: 2px; }
.path-configuration-page__identity h1 { margin: 0 0 6px; font-size: 24px; line-height: 1.25; letter-spacing: 0; }
.path-configuration-page__identity p { margin: 0; color: var(--path-config-text-secondary-color); line-height: 1.5; }
.path-configuration-page__progress { flex-wrap: wrap; justify-content: flex-end; gap: 6px 12px; max-width: 52%; font-size: 13px; line-height: 1.5; color: var(--path-config-text-secondary-color); }
.path-configuration-page__progress span { white-space: nowrap; }
.path-configuration-page__switch { gap: 8px; padding: 4px 0 12px; border-bottom: 1px solid var(--path-config-border-color); }
.path-configuration-page__cycle-body { display: grid; gap: 12px; }

.path-configuration-page__stage {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--path-config-card-color);
  border: 1px solid var(--path-config-border-color);
  border-radius: 4px;
}

.path-configuration-page--form > .path-configuration-page__stage {
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
}

.path-configuration-page__initial-loading,
.path-configuration-page__form-loading {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  min-height: 0;
}
.path-configuration-page__initial-loading { min-height: 320px; }
.path-configuration-page__canvas { height: 100%; min-height: 0; border-top: 0; }
.path-configuration-page__error { margin: 20px; }
.path-configuration-page__error-content { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; line-height: 1.6; }
.path-configuration-page__error-content > span { flex: 1 1 280px; min-width: 0; }

.path-configuration-page__form-workspace {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--path-config-card-color);
}

.path-configuration-page__form-toolbar {
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 16px;
  height: var(--path-config-form-toolbar-height);
  min-height: var(--path-config-form-toolbar-height);
  padding: 8px 12px;
  border-bottom: 1px solid var(--path-config-border-color);
}
.path-configuration-page__form-toolbar h2 { margin: 0 0 4px; font-size: 18px; }
.path-configuration-page__form-toolbar p { margin: 0; color: var(--path-config-text-secondary-color); font-size: 12px; }
.path-configuration-page__form-actions { flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.path-configuration-page__form-feedback {
  position: static;
  flex: 0 0 auto;
  display: grid;
  gap: 6px;
  max-height: 112px;
  padding: 8px 12px 0;
  overflow-y: auto;
}
.path-configuration-page__form-hints {
  position: absolute;
  top: 66px;
  right: 14px;
  z-index: 30;
  width: 300px;
  max-width: calc(100% - 28px);
  background: #fff;
  border: 1px solid var(--path-config-border-color);
  border-radius: 4px;
  color: #262626;
  color-scheme: light;
}
.path-configuration-page__form-hints :deep(.n-collapse) {
  --n-text-color: rgba(0, 0, 0, 0.82) !important;
  --n-divider-color: rgba(0, 0, 0, 0.12) !important;
  --n-title-text-color: rgba(0, 0, 0, 0.9) !important;
  --n-arrow-color: rgba(0, 0, 0, 0.7) !important;
}
.path-configuration-page__form-hints :deep(.n-collapse-item__header) { padding: 12px 14px; color: #262626 !important; }
.path-configuration-page__form-hints :deep(.n-collapse-item__header-main) { color: #262626 !important; }
.path-configuration-page__form-hints :deep(.n-collapse-item__content-wrapper) { padding: 0 14px 12px; }
.path-configuration-page__form-hints :deep(.n-base-icon) { color: #595959 !important; }
.path-configuration-page__form-hints-body {
  max-height: 300px;
  overflow-y: auto;
}
.path-configuration-page__form-hints-list {
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 12px;
}
.path-configuration-page__form-hints-list li {
  display: grid;
  align-items: start;
  gap: 6px;
  padding: 7px 8px;
  border-left: 2px solid var(--path-config-border-color);
  background: #fff;
  line-height: 1.55;
}
.path-configuration-page__form-hints-list li + li { margin-top: 5px; }
.path-configuration-page__form-hints-list strong { color: #262626; font-weight: 600; }
.path-configuration-page__form-hint-head { display: flex; align-items: center; flex-wrap: wrap; gap: 5px; }
.path-configuration-page__form-hint-head > span { color: #595959; }
.path-configuration-page__form-hints-list p { margin: 0; color: #262626; }
.path-configuration-page__form-hints-list small { color: #737373; }
.path-configuration-page__form-hint--selected {
  background: #f0f9f4;
  border-left-color: #18a058 !important;
}
.path-configuration-page__form-hint--review {
  background: #fff2f0;
  border-left-color: #d03050 !important;
}
.path-configuration-page__form-error-details {
  margin: 6px 0 0;
  padding-left: 18px;
  color: var(--path-config-text-secondary-color);
  font-size: 12px;
  line-height: 1.7;
}
.path-configuration-page__form-frame {
  flex: 1 1 0;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  overscroll-behavior: contain;
  border: 0;
}

@media (max-width: 900px) {
  .path-configuration-page { height: auto; min-height: calc(100dvh - 120px); overflow: visible; }
  .path-configuration-page--form {
    height: calc(100dvh - var(--path-config-app-header-height));
    min-height: calc(100dvh - var(--path-config-app-header-height));
    max-height: none;
    overflow: hidden;
  }
  .path-configuration-page__header, .path-configuration-page__form-toolbar { align-items: flex-start; flex-direction: column; }
  .path-configuration-page--form .path-configuration-page__form-toolbar {
    height: auto;
    min-height: var(--path-config-form-toolbar-height);
    flex-direction: row;
    flex-wrap: wrap;
  }
  .path-configuration-page__progress { width: 100%; max-width: none; justify-content: flex-start; }
  .path-configuration-page__stage { min-height: 640px; }
  .path-configuration-page--form > .path-configuration-page__stage,
  .path-configuration-page__form-workspace { min-height: 0; }
  .path-configuration-page__form-toolbar { flex: 0 0 auto; }
}
</style>
