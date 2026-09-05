<script setup lang="ts">
import { NAlert, NButton, NCard, NEmpty, NModal, NSpace, NSpin, NTag, useNotification, useThemeVars } from 'naive-ui'
import type { NotificationReactive } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { fetchExecutionPath, fetchExecutionPaths } from '../features/execution-paths/api'
import type { HistoryDataIssue } from '../features/history-replay/types'
import type { ExecutionPath } from '../features/execution-paths/types'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import BaseFormDataPicker from '../features/history-replay/BaseFormDataPicker.vue'
import ActionOrchestrationEditor from '../features/path-configuration/ActionOrchestrationEditor.vue'
import FormDataHintsPanel from '../features/path-configuration/FormDataHintsPanel.vue'
import FormRuntimeFrame from '../features/path-configuration/FormRuntimeFrame.vue'
import NodeConfigurationPanel from '../features/path-configuration/NodeConfigurationPanel.vue'
import {
  bindPathConfigurationNodes,
  buildPathActionConfigurationInput,
  containerActionsDraft,
	copyPathConfigActions,
  currentNodeConfigurationComplete,
  hasContainerDraftChanges,
  hasCurrentNodeDraftChanges,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  instanceActionContainer,
  instanceActionsComplete,
  nodeActionContainer,
  pathActionFlowLabels,
  pathConfigurationMessage,
  pathConfigurationStatusName,
  pathConfigurationNodesByGraphID,
  projectPathConfigurationNodeStates,
  resolveConfirmedNodeSaveDestination,
} from '../features/path-configuration/logic'
import {
  fetchPathCompiledScenario,
  fetchPathConfiguration,
  fetchPathConfigurationData,
  fetchPathFormRuntimeSession,
  PathConfigApiError,
  savePathActionConfiguration,
  savePathConfigurationData,
} from '../features/path-configuration/api'
import { retryPathLoad } from '../features/path-configuration/retry'
import type {
  PathActionConfigurationIssue,
  PathCompiledActionStep,
	PathConfigConfiguredActionInput,
  PathConfigDraft,
  PathConfigNode,
  PathConfiguration,
  PathConfigPerson,
  PathConfigPersonStrategyInput,
  PathConfigurationDataWorkspace,
  PathConfigurationRouteChange,
  PathConfigurationRuntimeValidation,
  PathFormRuntimeSession,
} from '../features/path-configuration/types'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import type { PersistedPlan } from '../features/plans/types'

type FormRuntimeExpose = InstanceType<typeof FormRuntimeFrame> & {
  setValues: (values: Record<string, unknown>, signal?: AbortSignal) => Promise<Record<string, unknown>>
  restoreSaved: () => Promise<Record<string, unknown>>
  getValues: (signal?: AbortSignal) => Promise<Record<string, unknown>>
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
const configurationByGraphNodeID = ref(new Map<string, PathConfigNode>())
const graphNodeIDByConfigurationKey = ref(new Map<string, string>())
const selectedNodeID = ref('')
const workspace = ref<'nodes' | 'form'>('nodes')
const runtimeSession = ref<PathFormRuntimeSession | null>(null)
const dataWorkspace = ref<PathConfigurationDataWorkspace | null>(null)
const runtimeStats = ref<{ filledEditable: number, manualPending: number }>({ filledEditable: 0, manualPending: 0 })
const runtimeUnsupported = ref<string[]>([])
const pageLoading = ref(false)
const pathDetailLoading = ref(false)
const savingNode = ref(false)
const savingInstanceActions = ref(false)
const compiledScenario = ref<PathCompiledActionStep[]>([])
const compiledIssues = ref<PathActionConfigurationIssue[]>([])
const compiledLoading = ref(false)
const compiledError = ref('')
const instanceSaveError = ref('')
const instanceSaveDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const instanceSavedSuccessfully = ref(false)
const formRuntimeLoading = ref(false)
const formRestoring = ref(false)
const dataPickerOpen = ref(false)
const formSaving = ref(false)
const routeConfirmationOpen = ref(false)
const routeConfirmationToken = ref('')
const routeConfirmationChange = ref<PathConfigurationRouteChange | null>(null)
const pageError = ref('')
const nodeSaveError = ref('')
const nodeSaveDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const formError = ref('')
const formErrorDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const nodeSavedSuccessfully = ref(false)
const formSavedSuccessfully = ref(false)
const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)
const formFrame = ref<FormRuntimeExpose | null>(null)
let loadVersion = 0
let loadController: AbortController | null = null
let runtimeEpoch = 0
let runtimeSessionController: AbortController | null = null
let formOperationController: AbortController | null = null
let nodeSaveKey = ''
let instanceSaveKey = ''
let formSaveKey = ''
let compiledVersion = 0
let compiledController: AbortController | null = null
const notification = useNotification()
let formNotice: NotificationReactive | null = null


// dismissFormNotice 销毁表单工作区悬浮反馈，避免离开路径后旧通知残留到下一条路径。
function dismissFormNotice() {
  formNotice?.destroy()
  formNotice = null
}

// showFormNotice 统一把生成、保存和规则阻断反馈放到 Naive UI 通知层，不改变 iframe 的布局高度。
function showFormNotice() {
  dismissFormNotice()
  if (workspace.value !== 'form') return
  const details = formErrorDetails.value.map(item => pathConfigurationMessage(item.reason || item.name)).filter(Boolean)
  if (formError.value) {
    formNotice = notification.error({
      title: '表单处理提示',
      content: [formError.value, ...details].filter(Boolean).join('；'),
      duration: 0,
      closable: true,
    })
    return
  }
  if (runtimeBlocked.value) {
    formNotice = notification.warning({
      title: '当前配置需要处理',
      content: runtimeBlockingReasons.value.map(pathConfigurationMessage).join('；'),
      duration: 0,
      closable: true,
    })
    return
  }
  if (formSavedSuccessfully.value) {
    formNotice = notification.success({
      title: '表单数据已保存',
      content: '已完成服务端复验，节点仍需逐个完成。',
      duration: 4500,
      closable: true,
    })
  }
}

const pageThemeStyle = computed(() => ({
  '--path-config-page-color': themeVars.value.bodyColor,
  '--path-config-card-color': themeVars.value.cardColor,
  '--path-config-border-color': themeVars.value.borderColor,
  '--path-config-text-color': themeVars.value.textColor1,
  '--path-config-text-secondary-color': themeVars.value.textColor2,
}))
const planMutable = computed(() => plan.value?.status === 'not_started')
const formReadOnly = computed(() => !planMutable.value)
// 表单权限视图：目标按节点声明字段权限，真实用户在一个节点上只能改该节点声明可编辑的字段。
// 默认发起人视图；切到审批节点视图后只放开那个节点能改的字段，数据仍是同一份表单数据。
const selectedFormViewName = ref('')
const formNodeViews = computed(() => dataWorkspace.value?.nodeViews ?? [])
const selectedFormView = computed(() => formNodeViews.value.find(view => view.nodeName === selectedFormViewName.value) ?? null)
const runtimeForm = computed(() => {
  if (!dataWorkspace.value) return null
  const base = { ...dataWorkspace.value, readOnly: formReadOnly.value, viewName: selectedFormViewName.value }
  const view = selectedFormView.value
  if (!view) return base
  // 按视图权限渲染：该节点声明可编辑的字段放开，只有后续节点才能填的字段隐藏，
  // 其余保持只读。发起人视图同样走这条路径，否则会出现"这一步填不了却带着历史值显示"的矛盾。
  return { ...base, permissions: view.permissions }
})

// 节点视图默认选中发起人（配置阶段的表单永远处于发起态）；换路径重载后重新归位。
watch(formNodeViews, views => {
  if (views.length === 0) {
    selectedFormViewName.value = ''
    return
  }
  if (views.some(view => view.nodeName === selectedFormViewName.value)) return
  selectedFormViewName.value = (views.find(view => view.isInitiator) ?? views[0]).nodeName
}, { immediate: true })
const pathAnalysis = computed(() => graph.value && currentPath.value ? analyzeExecutionPath(graph.value, currentPath.value.choices) : null)
const selectedNode = computed(() => configurationByGraphNodeID.value.get(selectedNodeID.value) ?? null)
const configurationNodeStates = computed(() => graph.value && pathAnalysis.value
  ? projectPathConfigurationNodeStates(graph.value, pathAnalysis.value, configurationByGraphNodeID.value, selectedNodeID.value)
  : {})
const selectedNodeRequirement = computed(() => currentNodeConfigurationComplete(selectedNode.value, draft.value))
const nodeSaveDisabled = computed(() => !planMutable.value || pageLoading.value || savingNode.value || !selectedNode.value || selectedNode.value.lineBlocked || selectedNode.value.status === 'not_required' || selectedNode.value.status === 'runtime' || !selectedNodeRequirement.value.complete)
const saveAllNodesDisabled = computed(() => !planMutable.value || pageLoading.value || savingNode.value || !configuration.value)
const nodeDraftHasUnsavedChanges = computed(() => hasCurrentNodeDraftChanges(selectedNode.value, draft.value))
// actionFlowLabels 让只读流程图显示节点与动作的中文名，编译步骤里带的都是内部键。
const actionFlowLabels = computed(() => pathActionFlowLabels(configuration.value, graph.value, graphNodeIDByConfigurationKey.value))
const instanceContainer = computed(() => configuration.value ? instanceActionContainer(configuration.value) : null)
const instanceActionsSaved = computed(() => instanceContainer.value ? containerActionsDraft(instanceContainer.value, draft.value) : [])
const instanceDraftHasUnsavedChanges = computed(() => hasContainerDraftChanges(instanceContainer.value, draft.value))
const instanceSaveDisabled = computed(() => !planMutable.value
  || pageLoading.value
  || savingInstanceActions.value
  || !instanceContainer.value
  || !instanceActionsComplete(instanceContainer.value, draft.value))
const draftHasUnsavedChanges = computed(() => nodeDraftHasUnsavedChanges.value || instanceDraftHasUnsavedChanges.value)
const runtimeBlockingReasons = computed(() => [...new Set(runtimeUnsupported.value)])
const runtimeBlocked = computed(() => runtimeBlockingReasons.value.length > 0)
// runtimeCoordinationIssues 是复制运行时实时回报的选项协调阻断问题，与脚本问题同面板展示。
const runtimeCoordinationIssues = ref<HistoryDataIssue[]>([])
function applyRuntimeIssues(payload: Record<string, unknown>) {
  if (!Array.isArray(payload.issues)) return
  runtimeCoordinationIssues.value = payload.issues.map(issue => ({
    code: String(issue?.code ?? 'runtime_issue'),
    path: issue?.fieldPath ? String(issue.fieldPath) : undefined,
    message: String(issue?.message ?? ''),
    blocking: issue?.status === 'blocked' || issue?.blocking === true,
  }))
}
// applyRuntimeFormState 只接受 iframe 已核验会话回传的原始值统计，宿主不持有字段映射或生成所有权。
function applyRuntimeFormState(payload: Record<string, unknown>) {
  if (workspace.value !== 'form') return
  applyRuntimeIssues(payload)
  const stats = payload.stats
  if (stats && typeof stats === 'object') {
    const current = stats as { filledEditable?: unknown, manualPending?: unknown }
    runtimeStats.value = {
      filledEditable: typeof current.filledEditable === 'number' && Number.isInteger(current.filledEditable) && current.filledEditable >= 0 ? current.filledEditable : 0,
      manualPending: typeof current.manualPending === 'number' && Number.isInteger(current.manualPending) && current.manualPending >= 0 ? current.manualPending : 0,
    }
  }
}

// handleRuntimeReady 接收真实组件注册表与首次字段统计，避免初始化阶段显示未经运行时确认的估算值。
function handleRuntimeReady(payload: Record<string, unknown>) {
  if (workspace.value !== 'form' || !runtimeSession.value) return
  runtimeUnsupported.value = Array.isArray(payload.unsupported) ? payload.unsupported.map(String) : []
  applyRuntimeIssues(payload)
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
  dataWorkspace.value = null
  runtimeStats.value = { filledEditable: 0, manualPending: 0 }
  routeConfirmationOpen.value = false
  routeConfirmationToken.value = ''
  routeConfirmationChange.value = null
  runtimeUnsupported.value = []
  formRuntimeLoading.value = false
  formRestoring.value = false
  formSaving.value = false
  dismissFormNotice()
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
  instanceSaveError.value = ''
  instanceSaveDetails.value = []
  instanceSavedSuccessfully.value = false
  resetCompiledScenario()
  workspace.value = 'nodes'
  try {
    const [storedPlan, storedGraph, storedPaths] = await retryPathLoad(signal => Promise.all([
      fetchPlan(planID.value, signal),
      fetchFlowGraph(planID.value, signal),
      fetchExecutionPaths(planID.value, signal),
    ]), controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    pathDetailLoading.value = true
    const storedPath = await retryPathLoad(signal => fetchExecutionPath(planID.value, pathID.value, signal), controller.signal)
    pathDetailLoading.value = false
    if (controller.signal.aborted || version !== loadVersion) return
    if (!storedPaths.some(path => path.id === storedPath.id)) throw new Error('已保存路径不存在或已删除')
    const analysis = analyzeExecutionPath(storedGraph, storedPath.choices)
    if (!analysis.complete || analysis.invalid) throw new Error('当前已保存路径与真实流程不一致，请先编辑路径')
    // 目标流程读取偶发超时不该让整页失败：与计划、流程图同一套受控重试。
    const storedConfiguration = await retryPathLoad(signal => fetchPathConfiguration(planID.value, pathID.value, signal), controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = storedPlan
    graph.value = storedGraph
    currentPath.value = storedPath
    executionPaths.value = storedPaths
    await applyConfiguration(storedConfiguration, false)
    nodeSaveKey = crypto.randomUUID()
    instanceSaveKey = crypto.randomUUID()
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
    dataWorkspace.value?.dataStatus === 'ready' ? 'ready' : 'empty',
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

// resetCompiledScenario 失效编译预览的异步代次，避免旧路径的只读步骤留在新路径页面。
function resetCompiledScenario() {
  compiledVersion += 1
  compiledController?.abort()
  compiledController = null
  compiledScenario.value = []
  compiledIssues.value = []
  compiledLoading.value = false
  compiledError.value = ''
}

// loadCompiledScenario 只读取服务端编译结果；浏览器不推导、不补齐、不提交任何步骤。
async function loadCompiledScenario() {
  if (!configuration.value) return
  compiledController?.abort()
  const controller = new AbortController()
  compiledController = controller
  const version = ++compiledVersion
  compiledLoading.value = true
  compiledError.value = ''
  try {
    const result = await fetchPathCompiledScenario(planID.value, pathID.value, controller.signal)
    if (controller.signal.aborted || version !== compiledVersion) return
    compiledScenario.value = result.compiledScenario
    compiledIssues.value = result.issues
  }
  catch (caught) {
    if (controller.signal.aborted || version !== compiledVersion) return
    compiledScenario.value = []
    compiledIssues.value = []
    compiledError.value = publicPageError(caught)
  }
  finally {
    if (version === compiledVersion) compiledLoading.value = false
    if (compiledController === controller) compiledController = null
  }
}

// updateInstanceActionConfiguration 只接受实例动作容器草稿，禁止实例编辑器越界写语义节点。
function updateInstanceActionConfiguration(containerKey: string, value: PathConfigConfiguredActionInput[]) {
  if (!planMutable.value) return
  if (instanceContainer.value?.key !== containerKey) return
  draft.value.actionConfigurations[containerKey] = copyPathConfigActions(value)
  instanceSavedSuccessfully.value = false
}

// saveInstanceActions 保存实例作用域动作，并用服务端同一次重编译结果刷新只读预览。
async function saveInstanceActions() {
  const current = configuration.value
  const container = instanceContainer.value
  if (!current || !container || instanceSaveDisabled.value) return
  savingInstanceActions.value = true
  instanceSaveError.value = ''
  instanceSaveDetails.value = []
  instanceSavedSuccessfully.value = false
  const previousRevision = current.nodeRevision
  try {
    const result = await savePathActionConfiguration(
      planID.value,
      pathID.value,
      container.key,
      buildPathActionConfigurationInput(container, draft.value, previousRevision),
      instanceSaveKey,
    )
    instanceSaveKey = crypto.randomUUID()
    await reloadConfiguration()
    compiledVersion += 1
    compiledScenario.value = result.compiledScenario
    compiledIssues.value = result.issues
    compiledError.value = ''
    instanceSavedSuccessfully.value = true
  }
  catch (caught) {
    try {
      const reconciled = await reloadConfiguration()
      if (reconciled.nodeRevision > previousRevision) {
        instanceSaveKey = crypto.randomUUID()
        instanceSavedSuccessfully.value = true
        await loadCompiledScenario()
        return
      }
    }
    catch { /* 对账失败保留原草稿和幂等键，用户重试不会重复写入。 */ }
    if (caught instanceof PathConfigApiError) {
      instanceSaveError.value = pathConfigurationMessage(caught.message)
      instanceSaveDetails.value = caught.details
    }
    else instanceSaveError.value = '保存失败，当前实例动作草稿已保留，请重试'
  }
  finally {
    savingInstanceActions.value = false
  }
}

// updateNodeActionConfiguration 替换当前节点独立动作草稿，不允许面板越过节点边界写其他节点。
function updateNodeActionConfiguration(nodeKey: string, value: PathConfigConfiguredActionInput[]) {
	if (!planMutable.value) return
	// 节点编辑器同时编排实例级动作：实例容器键仍按实例容器保存，不越界写语义节点。
	if (instanceContainer.value && nodeKey === instanceContainer.value.key) { updateInstanceActionConfiguration(nodeKey, value); return }
	if (selectedNode.value?.key !== nodeKey) return
	// 子组件事件值仍可能携带 Vue Proxy；父页面只持有普通草稿，避免保存前再次触发克隆异常。
	draft.value.actionConfigurations[nodeKey] = copyPathConfigActions(value)
  nodeSavedSuccessfully.value = false
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
    const result = await savePathActionConfiguration(planID.value, pathID.value, node.key, buildPathActionConfigurationInput(nodeActionContainer(node), draft.value, previousRevision), nodeSaveKey)
    await reloadConfiguration()
    compiledVersion += 1
    compiledScenario.value = result.compiledScenario
    compiledIssues.value = result.issues
    compiledError.value = ''
    // 节点编辑器里同时编排的实例级动作在同一次保存里落盘，避免用户以为已保存却只写了节点。
    if (instanceDraftHasUnsavedChanges.value) await saveInstanceActions()
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
      const result = await savePathActionConfiguration(
        planID.value,
        pathID.value,
        node.key,
        buildPathActionConfigurationInput(nodeActionContainer(node), draft.value, revision),
        crypto.randomUUID(),
      )
      revision = result.nodeRevision
      compiledVersion += 1
      compiledScenario.value = result.compiledScenario
      compiledIssues.value = result.issues
      compiledError.value = ''
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

// openFormWorkspace 获取原始数据工作区和短期 SID 后切换到全宽 runtime；SID 不进入配置对象或持久状态。
async function openFormWorkspace() {
  if (!configuration.value || formRuntimeLoading.value || formRestoring.value || formSaving.value) return
  workspace.value = 'form'
  formError.value = ''
  if (runtimeSession.value && dataWorkspace.value) return
  const epoch = ++runtimeEpoch
  const controller = new AbortController()
  runtimeSessionController?.abort()
  runtimeSessionController = controller
  formRuntimeLoading.value = true
  try {
    const [data, session] = await Promise.all([
      fetchPathConfigurationData(planID.value, pathID.value, controller.signal),
      fetchPathFormRuntimeSession(planID.value, pathID.value, controller.signal),
    ])
    if (controller.signal.aborted || epoch !== runtimeEpoch || workspace.value !== 'form') return
    dataWorkspace.value = data
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

// handleBaseFormDataSaved 重新读取服务端按当前路径分支补丁处理后的 values，并通过既有 runtime setValues 透传，不在页面拼接表单状态。
async function handleBaseFormDataSaved() {
  if (workspace.value !== 'form' || !runtimeSession.value) return
  const controller = new AbortController()
  try {
    const data = await fetchPathConfigurationData(planID.value, pathID.value, controller.signal)
    if (workspace.value !== 'form' || !runtimeSession.value) return
    dataWorkspace.value = data
    runtimeStats.value = { filledEditable: 0, manualPending: 0 }
    const frame = formFrame.value
    if (frame) applyRuntimeFormState(await frame.setValues(data.effectiveFormData, controller.signal))
  }
  catch (caught) {
    if (!controller.signal.aborted) formError.value = publicPageError(caught)
  }
}

// returnToNodes 先失效宿主状态再切换工作区，iframe 只通过自身卸载钩子 teardown 一次。
function returnToNodes() {
  invalidateRuntimeSession()
  workspace.value = 'nodes'
}

// restoreSavedForm 恢复本次 GET 装载的服务端 values，不重读或重新生成。
async function restoreSavedForm() {
  const frame = formFrame.value
  const epoch = runtimeEpoch
	if (formReadOnly.value || !frame || !runtimeSession.value || formRuntimeLoading.value || formRestoring.value || formSaving.value) return
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

// saveFormData 先经复制 runtime 校验并捕获原始 values，再由服务端重算实际路径并执行换路门禁。
async function saveFormData(confirmationToken = '') {
  const data = dataWorkspace.value
  const frame = formFrame.value
  const epoch = runtimeEpoch
	if (!data || formReadOnly.value || runtimeBlocked.value || formRuntimeLoading.value || formRestoring.value || formSaving.value || !frame || !runtimeSession.value) return
  const controller = new AbortController()
  formOperationController?.abort()
  formOperationController = controller
  formSaving.value = true
  formError.value = ''
  formErrorDetails.value = []
  formSavedSuccessfully.value = false
  const previousRevision = data.revision
  try {
    const captured = await frame.validateAndGetValues()
    if (!isActiveFormOperation(epoch, frame)) return
	    const runtimeValidation: PathConfigurationRuntimeValidation = {
	      accepted: captured.validated === true,
	      issues: Array.isArray(captured.issues) ? captured.issues.map(issue => ({
	        code: String(issue?.code ?? 'runtime_validation'),
	        path: issue?.fieldPath ? String(issue.fieldPath) : undefined,
	        message: String(issue?.message ?? '表单运行时校验未通过'),
	        blocking: issue?.status === 'blocked' || issue?.blocking === true,
	      })) : [],
	    }
	    const result = await savePathConfigurationData(planID.value, pathID.value, formSaveKey, {
      revision: previousRevision,
      values: captured.values as Record<string, unknown>,
      runtimeValidation,
      ...(confirmationToken ? { confirmationToken } : {}),
    }, controller.signal)
    if (!isActiveFormOperation(epoch, frame)) return
    dataWorkspace.value = { ...data, ...result }
    runtimeStats.value = { filledEditable: 0, manualPending: 0 }
    formSaveKey = crypto.randomUUID()
    formSavedSuccessfully.value = true
    if (result.routeChanged) {
      const target = executionPaths.value.find(path => path.sequenceNo === result.path.sequenceNo && path.name === result.path.name)
      if (target && target.id !== pathID.value) {
        await router.replace(`/plans/${encodeURIComponent(planID.value)}/paths/${encodeURIComponent(target.id)}/configure`)
      } else returnToNodes()
    } else returnToNodes()
  }
  catch (caught) {
    if (!isActiveFormOperation(epoch, frame) || controller.signal.aborted) return
    if (caught instanceof PathConfigApiError && caught.code === 'PATH_ROUTE_CONFIRMATION_REQUIRED') {
      routeConfirmationToken.value = caught.confirmationToken
      routeConfirmationChange.value = caught.routeChange
      routeConfirmationOpen.value = true
      return
    }
    formError.value = caught instanceof Error ? pathConfigurationMessage(caught.message) : '保存失败，当前表单数据已保留，请重试'
    formErrorDetails.value = caught instanceof PathConfigApiError ? caught.details : []
  }
  finally {
    if (epoch === runtimeEpoch) formSaving.value = false
    if (formOperationController === controller) formOperationController = null
  }
}

// confirmRouteChange 在用户明确确认覆盖提示后复用原幂等键继续保存，取消不会写入任何路径数据。
function confirmRouteChange() {
  const token = routeConfirmationToken.value
  if (!token || formSaving.value) return
  routeConfirmationOpen.value = false
  void saveFormData(token)
}

// cancelRouteChange 清除一次性换路令牌，确保用户取消后不会隐式重试或改变目标路径。
function cancelRouteChange() {
  routeConfirmationOpen.value = false
  routeConfirmationToken.value = ''
  routeConfirmationChange.value = null
}

// confirmDiscardNodeDraft 在离开路径前拦截未保存的人员、节点动作或实例动作草稿，避免导航悄悄丢失用户编辑。
function confirmDiscardNodeDraft() {
  if (!draftHasUnsavedChanges.value || savingNode.value || savingInstanceActions.value) return true
  return window.confirm('当前人员、节点动作或实例动作配置尚未保存，确定离开？')
}

// handleBeforeUnload 浏览器关闭或刷新时复用同一未保存门禁。
function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!draftHasUnsavedChanges.value || savingNode.value || savingInstanceActions.value) return
  event.preventDefault()
  event.returnValue = ''
}

// backToPlan 只失效宿主会话并导航，iframe 卸载由 Vue 子组件生命周期负责。
function backToPlan() {
  invalidateRuntimeSession()
  router.push('/plans/' + planID.value + '/paths')
}

onBeforeRouteLeave(confirmDiscardNodeDraft)
watch([workspace, formError, formSavedSuccessfully, runtimeBlockingReasons, formErrorDetails], showFormNotice, { deep: true })
watch([planID, pathID], () => { void loadPage() })
onMounted(() => window.addEventListener('beforeunload', handleBeforeUnload))
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)
  loadVersion++
  loadController?.abort()
  resetCompiledScenario()
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
        <span>节点配置状态：{{ pathConfigurationStatusName(configuration.status) }}</span>
        <span>节点 {{ configuration.progress.completed }} / {{ configuration.progress.total }}</span>
        <n-button v-if="workspace === 'nodes' && configuration.nextNodeKey" size="small" secondary @click="selectNextConfigurationNode">下一待配置节点</n-button>
      </div>
    </header>

    <n-modal v-model:show="routeConfirmationOpen" :mask-closable="false" :closable="false">
      <n-card title="确认换路并覆盖数据" style="width: min(620px, 94vw)">
        <div class="path-configuration-page__cycle-body">
          <n-alert type="warning" :show-icon="false">{{ routeConfirmationChange?.warning || '当前表单数据命中了其他执行路径，请确认后继续保存。' }}</n-alert>
          <div v-if="routeConfirmationChange" class="path-configuration-page__route-change">
            <span>当前路径：#{{ routeConfirmationChange.from.sequenceNo }} {{ routeConfirmationChange.from.name }}</span>
            <span>实际路径：#{{ routeConfirmationChange.to.sequenceNo }} {{ routeConfirmationChange.to.name }}</span>
          </div>
          <ul v-if="routeConfirmationChange?.affected.length" class="path-configuration-page__form-summary-list">
            <li v-for="item in routeConfirmationChange.affected" :key="`${item.kind}-${item.name}`">{{ item.name }}：{{ item.reason }}</li>
          </ul>
        </div>
        <template #footer>
          <n-space justify="end"><n-button @click="cancelRouteChange">取消</n-button><n-button type="warning" :loading="formSaving" @click="confirmRouteChange">确认换路并保存</n-button></n-space>
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
        :configuration-form-status="dataWorkspace?.dataStatus === 'ready' ? 'ready' : dataWorkspace?.dataStatus === 'affected' ? 'affected' : 'empty'"
        :configuration-form-status-name="pathConfigurationStatusName(dataWorkspace?.dataStatus || 'empty')"
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
            :form-complete="dataWorkspace?.dataStatus === 'ready'"
            :instance-container="instanceContainer"
            :instance-saved-actions="instanceActionsSaved"
            :flow-labels="actionFlowLabels"
            :compiled-steps="compiledScenario"
            :compiled-issues="compiledIssues"
            :compiled-loading="compiledLoading"
            :compiled-error="compiledError"
            @request-compiled="loadCompiledScenario"
            @update-person-strategy="updatePersonStrategy"
            @update-action-configuration="updateNodeActionConfiguration"
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
            <p v-else>已填写 {{ runtimeStats.filledEditable }} 项 · 仍需手工 {{ runtimeStats.manualPending }} 项</p>
          </div>
          <div class="path-configuration-page__form-actions">
            <n-button size="small" @click="returnToNodes">返回节点画布</n-button>
			<template v-if="!formReadOnly">
              <n-button size="small" type="primary" secondary :disabled="formRuntimeLoading || formRestoring || formSaving" @click="dataPickerOpen = true">智能生成数据</n-button>
              <n-button size="small" :loading="formRestoring" :disabled="formRuntimeLoading || formRestoring || formSaving" @click="restoreSavedForm">恢复已保存数据</n-button>
              <n-button size="small" type="primary" :loading="formSaving" :disabled="runtimeBlocked || formRuntimeLoading || formRestoring" @click="saveFormData()">保存表单数据</n-button>
            </template>
          </div>
        </header>
        <base-form-data-picker
          v-model:show="dataPickerOpen"
          :plan-id="planID"
          :path-id="pathID"
          scope="path"
          confirm-text="生成数据"
          :disabled="!planMutable"
          @saved="handleBaseFormDataSaved"
        />
        <div class="path-configuration-page__form-body">
        <form-data-hints-panel
          v-if="dataWorkspace"
          v-model:selected-view="selectedFormViewName"
          :key-fields="dataWorkspace.keyFields ?? []"
          :issues="[...(dataWorkspace.issues ?? []), ...runtimeCoordinationIssues]"
          :branch-patches="dataWorkspace.branchPatches"
          :node-views="formNodeViews"
        />
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
        </div>
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
.path-configuration-page__form-summary {
  flex: 0 0 auto;
  margin: 0 16px 10px;
  padding: 8px 10px;
  border: 1px solid var(--path-config-border-color);
  border-radius: 4px;
  color: var(--path-config-text-secondary-color);
  font-size: 12px;
  line-height: 1.55;
}
.path-configuration-page__form-summary-head,
.path-configuration-page__route-change {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.path-configuration-page__form-summary-head strong { color: var(--path-config-text-color); }
.path-configuration-page__form-summary p { margin: 4px 0 0; }
.path-configuration-page__form-summary-list {
  margin: 4px 0 0;
  padding-left: 18px;
}
.path-configuration-page__route-change { flex-direction: column; align-items: flex-start; color: var(--path-config-text-color); }

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

.path-configuration-page__scenario {
  flex: 0 0 auto;
  margin: 0 16px 12px;
  padding: 8px 12px;
  border: 1px solid var(--path-config-border-color);
  border-radius: 4px;
  font-size: 13px;
}
.path-configuration-page__scenario summary { cursor: pointer; }
.path-configuration-page__scenario-workspace {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: var(--path-config-card-color);
}
.path-configuration-page__scenario-toolbar {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--path-config-border-color);
}
.path-configuration-page__scenario-toolbar h2 { margin: 0 0 4px; font-size: 18px; }
.path-configuration-page__scenario-toolbar p { margin: 0; color: var(--path-config-text-secondary-color); font-size: 12px; }
.path-configuration-page__scenario-body {
  display: flex;
  flex: 1 1 0;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
  padding: 14px;
  overflow-y: auto;
}

.path-configuration-page__form-body {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}
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
  .path-configuration-page__header, .path-configuration-page__form-toolbar, .path-configuration-page__scenario-toolbar { align-items: flex-start; flex-direction: column; }
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
