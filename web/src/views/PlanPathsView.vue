<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCheckbox,
  NDescriptions,
  NDescriptionsItem,
  NDropdown,
  NEmpty,
  NInput,
  NModal,
  NPopconfirm,
  NSpin,
  NTag,
  NVirtualList,
  useMessage,
  useThemeVars,
} from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import {
  createExecutionPath,
  deleteExecutionPath,
  ExecutionPathApiError,
  fetchExecutionPaths,
  generateAllExecutionPaths,
  updateExecutionPath,
} from '../features/execution-paths/api'
import {
  analyzeExecutionPath,
  applyExecutionPathChoice,
  canCreateAdditionalPath,
  deriveExecutionPathWorkspacePresentation,
  deriveExecutionPathDecisionProgress,
  deriveExecutionPathWorkspaceDisposition,
  hasExecutionPathDraftChanges,
  projectExecutionPathSummary,
  previewAllExecutionPaths,
  reconcileExecutionPathChoices,
  refreshExecutionPathDraft,
  transitionExecutionPathWorkspace,
} from '../features/execution-paths/logic'
import type { ExecutionPath, ExecutionPathChoice, ExecutionPathWorkspaceMode } from '../features/execution-paths/types'
import { applyPathConfigurationPreset, savePathConfigurationSelection } from '../features/path-configuration/api'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph, FlowGraphApiError } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import { planStatusLabels } from '../features/plans/logic'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import { flowSourceLabels } from '../features/plans/selection'
import type { PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const themeVars = useThemeVars()
const plan = ref<PersistedPlan | null>(null)
const graph = ref<FlowGraph | null>(null)
const paths = ref<ExecutionPath[]>([])
const planLoading = ref(false)
const graphLoading = ref(false)
const pathsLoading = ref(false)
const pathsLoaded = ref(false)
const planError = ref('')
const graphError = ref<FlowGraphApiError | null>(null)
const pathsError = ref('')
const planNotFound = ref(false)
const activePathID = ref<string | null>(null)
const workspaceMode = ref<ExecutionPathWorkspaceMode>(null)
const draftChoices = ref<ExecutionPathChoice[]>([])
const draftName = ref('')
const draftChangedByGraph = ref(false)
const createKey = ref('')
const saving = ref(false)
const deleting = ref(false)
const generatingAll = ref(false)
const generateAllKey = ref('')
const pathWorkspaceOpen = ref(false)
const savedPathsOpen = ref(false)
const discardConfirmOpen = ref(false)
const deleteConfirmOpen = ref(false)
const draftRecoveryLoading = ref(false)
const draftRecoveryError = ref('')
const selectedRunPathIDs = ref(new Set<string>())
const pathSelectionRevisions = ref<Record<string, number>>({})
const pathSelectionLoading = ref(false)
const pathSelectionSaving = ref(false)
const pathSelectionError = ref('')
const presettingSelected = ref(false)
const presetProgress = ref({ completed: 0, total: 0 })
const SAVED_PATH_ITEM_SIZE = 44
const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)
const graphScreenRef = ref<HTMLElement | null>(null)
let loadController: AbortController | null = null
let loadVersion = 0
let draftRecoveryController: AbortController | null = null
let draftRecoveryVersion = 0
let pageScrollContainer: HTMLElement | null = null

const planID = computed(() => String(route.params.id || ''))
const activePath = computed(() => paths.value.find((path) => path.id === activePathID.value) ?? null)
const configuredPaths = computed(() => paths.value.filter((path) => path.configurationStatus === 'configured'))
const partialPaths = computed(() => paths.value.filter((path) => path.configurationStatus === 'partial'))
const pendingPaths = computed(() => paths.value.filter((path) => path.configurationStatus !== 'configured'))
const pageThemeStyle = computed(() => ({
  '--plan-card-color': themeVars.value.cardColor,
  '--plan-page-color': themeVars.value.bodyColor,
  '--plan-border-color': themeVars.value.borderColor,
  '--plan-divider-color': themeVars.value.dividerColor,
  '--plan-text-color': themeVars.value.textColor1,
  '--plan-text-secondary-color': themeVars.value.textColor2,
}))
const pathAnalysis = computed(() => graph.value
  ? analyzeExecutionPath(graph.value, draftChoices.value)
  : null)
const remainingChoices = computed(() => pathAnalysis.value?.missingRouteNodeIds.length ?? 0)
const decisionProgress = computed(() => graph.value && pathAnalysis.value
  ? deriveExecutionPathDecisionProgress(graph.value, pathAnalysis.value, draftChoices.value)
  : { selected: 0, pending: 0, total: 0 })
const saveDisabled = computed(() => !workspacePresentation.value.branchEditing
  || !pathAnalysis.value?.complete
  || (workspaceMode.value === 'edit' && !draftHasUnsavedChanges.value && !draftChangedByGraph.value)
  || saving.value
  || generatingAll.value
  || draftRecoveryLoading.value
  || Boolean(draftRecoveryError.value))
const allowNewPath = computed(() => Boolean(
  graph.value
  && plan.value
  && pathsLoaded.value
  && !pathsError.value
  && canCreateAdditionalPath(plan.value.flowSource, paths.value.length),
))
const allowCopy = computed(() => pathsLoaded.value
  && plan.value?.flowSource === 'new'
  && workspaceMode.value === 'view'
  && Boolean(activePath.value))
const draftHasUnsavedChanges = computed(() => hasExecutionPathDraftChanges(
  workspaceMode.value,
  draftName.value,
  draftChoices.value,
  activePath.value,
))
const workspacePresentation = computed(() => deriveExecutionPathWorkspacePresentation({
  mode: workspaceMode.value,
  dirty: workspaceMode.value !== 'view' && (draftHasUnsavedChanges.value || draftChangedByGraph.value),
  remainingChoices: remainingChoices.value,
  invalid: Boolean(pathAnalysis.value?.invalid),
  changedByGraph: draftChangedByGraph.value,
}))
const hasDiscardableChanges = computed(() => workspacePresentation.value.branchEditing
  && (draftHasUnsavedChanges.value || draftChangedByGraph.value))
const workspaceActionBusy = computed(() => saving.value
  || deleting.value
  || generatingAll.value
  || draftRecoveryLoading.value)
const pathSummary = computed(() => graph.value && pathAnalysis.value
  ? projectExecutionPathSummary(graph.value, pathAnalysis.value, draftChoices.value)
  : [])
const batchPreview = computed(() => graph.value && plan.value?.flowSource === 'new'
  ? previewAllExecutionPaths(graph.value, paths.value)
  : { totalCount: 0, existingCount: 0, pendingCount: 0, exceeded: false })
const savedPathListHeight = computed(() => Math.min(paths.value.length, 5) * SAVED_PATH_ITEM_SIZE)
const allPathsSelectedForRun = computed(() => paths.value.length > 0 && paths.value.every(path => selectedRunPathIDs.value.has(path.id)))
const pathMoreOptions = computed(() => [
  ...(allowCopy.value ? [{ label: '复制路径', key: 'copy' }] : []),
  { label: '删除路径', key: 'delete' },
])

async function loadPage() {
  loadController?.abort()
  draftRecoveryController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  planLoading.value = true
  graphLoading.value = false
  pathsLoading.value = false
  pathsLoaded.value = false
  planError.value = ''
  graphError.value = null
  pathsError.value = ''
  planNotFound.value = false
  plan.value = null
  graph.value = null
  paths.value = []
  selectedRunPathIDs.value = new Set()
  pathSelectionRevisions.value = {}
  pathSelectionError.value = ''
  pathWorkspaceOpen.value = false
  savedPathsOpen.value = false
  draftRecoveryLoading.value = false
  draftRecoveryError.value = ''
  clearDraft()
  try {
    const storedPlan = await fetchPlan(planID.value, controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = storedPlan
    planLoading.value = false
    graphLoading.value = true
    pathsLoading.value = true
    const [graphResult, pathsResult] = await Promise.allSettled([
      fetchFlowGraph(planID.value, controller.signal),
      fetchExecutionPaths(planID.value, controller.signal),
    ])
    if (controller.signal.aborted || version !== loadVersion) return
    if (graphResult.status === 'fulfilled') graph.value = graphResult.value
    else {
      const caught = graphResult.reason
      graphError.value = caught instanceof FlowGraphApiError
        ? caught
        : new FlowGraphApiError('暂时无法读取流程，请重试', { retryable: true })
    }
    if (pathsResult.status === 'fulfilled') {
      paths.value = pathsResult.value
      pathsLoaded.value = true
		selectedRunPathIDs.value = new Set(pathsResult.value.filter(path => path.included).map(path => path.id))
		pathSelectionRevisions.value = Object.fromEntries(pathsResult.value.map(path => [path.id, path.configurationRevision]))
    }
    else {
      const caught = pathsResult.reason
      pathsError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取执行路径'
      pathsLoaded.value = false
    }
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    const apiError = caught instanceof PlanApiError ? caught : new PlanApiError('暂时无法读取计划，请重试')
    planNotFound.value = apiError.code === 'PLAN_NOT_FOUND'
    planError.value = apiError.message
  }
  finally {
    if (version === loadVersion) {
      planLoading.value = false
      graphLoading.value = false
      pathsLoading.value = false
    }
  }
}

async function retryGraph() {
  if (!plan.value) {
    await loadPage()
    return
  }
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  graphLoading.value = true
  graphError.value = null
  graph.value = null
  try {
    graph.value = await fetchFlowGraph(planID.value, controller.signal)
    if (activePath.value) selectSavedPath(activePath.value)
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    graphError.value = caught instanceof FlowGraphApiError
      ? caught
      : new FlowGraphApiError('暂时无法读取流程，请重试', { retryable: true })
  }
  finally {
    if (version === loadVersion) graphLoading.value = false
  }
}

async function retryPaths() {
  if (!plan.value) return
  const controller = new AbortController()
  const version = loadVersion
  pathsLoading.value = true
  pathsLoaded.value = false
  pathsError.value = ''
  try {
    const items = await fetchExecutionPaths(planID.value, controller.signal)
    if (version !== loadVersion) return
    paths.value = items
    pathsLoaded.value = true
		selectedRunPathIDs.value = new Set(items.filter(path => path.included).map(path => path.id))
		pathSelectionRevisions.value = Object.fromEntries(items.map(path => [path.id, path.configurationRevision]))
  }
  catch (caught) {
    pathsLoaded.value = false
    pathsError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取执行路径'
  }
  finally {
    pathsLoading.value = false
  }
}

function selectSavedPath(path: ExecutionPath) {
  if (!graph.value) return
  const reconciled = reconcileExecutionPathChoices(graph.value, path.choices)
  activePathID.value = path.id
  workspaceMode.value = transitionExecutionPathWorkspace(workspaceMode.value, 'select-saved')
  draftChoices.value = reconciled.choices
  draftName.value = path.name || `路径 ${path.sequenceNo}`
  draftChangedByGraph.value = reconciled.changed
  draftRecoveryError.value = ''
  createKey.value = ''
  pathWorkspaceOpen.value = true
}

// persistRunPathSelection 复用现有路径选择接口，只改变当前路径的运行纳入标记。
async function persistRunPathSelection(path: ExecutionPath, included: boolean) {
  const revision = pathSelectionRevisions.value[path.id]
  if (!Number.isInteger(revision)) throw new Error('当前路径选择尚未读取完成')
  const saved = await savePathConfigurationSelection(planID.value, path.id, revision, included, crypto.randomUUID())
  pathSelectionRevisions.value = { ...pathSelectionRevisions.value, [path.id]: saved.nodeRevision }
  const next = new Set(selectedRunPathIDs.value)
  if (included) next.add(path.id)
  else next.delete(path.id)
  selectedRunPathIDs.value = next
}

// applySelectedPreset 仅对用户勾选的路径应用安全预设，并在本地展示已完成数量。
async function applySelectedPreset() {
  const selected = paths.value.filter(path => selectedRunPathIDs.value.has(path.id))
  if (presettingSelected.value || selected.length === 0) return
  presettingSelected.value = true
  presetProgress.value = { completed: 0, total: selected.length }
  pathSelectionError.value = ''
  try {
    // 后端按 selected 范围重新读取当前勾选事实；只发起一次请求，避免浏览器逐条加载配置。
    await applyPathConfigurationPreset(planID.value, selected[0].id, 'selected')
    presetProgress.value = { completed: selected.length, total: selected.length }
    await retryPaths()
    message.success(`已完成 ${selected.length} 条路径的一键预设`)
  }
  catch (caught) {
    pathSelectionError.value = caught instanceof Error ? caught.message : '一键预设失败，请重试'
  }
  finally {
    presettingSelected.value = false
  }
}

// updateRunPathSelection 保存用户手动勾选的一条运行路径。
async function updateRunPathSelection(path: ExecutionPath, included: boolean) {
  if (pathSelectionSaving.value || pathSelectionLoading.value) return
  pathSelectionSaving.value = true
  pathSelectionError.value = ''
  try {
    await persistRunPathSelection(path, included)
  }
  catch (caught) {
    pathSelectionError.value = caught instanceof Error ? caught.message : '运行路径选择保存失败，请重试'
  }
  finally {
    pathSelectionSaving.value = false
  }
}

// setAllRunPathSelections 批量全选或取消全选当前列表中的所有路径。
async function setAllRunPathSelections(included: boolean) {
  if (pathSelectionSaving.value || pathSelectionLoading.value || (included && allPathsSelectedForRun.value) || (!included && selectedRunPathIDs.value.size === 0)) return
  pathSelectionSaving.value = true
  pathSelectionError.value = ''
  const targets = paths.value.filter(path => selectedRunPathIDs.value.has(path.id) !== included)
  const results = await Promise.allSettled(targets.map(path => persistRunPathSelection(path, included)))
  const failed = results.find(result => result.status === 'rejected')
  if (failed?.status === 'rejected') pathSelectionError.value = failed.reason instanceof Error ? failed.reason.message : '部分运行路径选择保存失败，请重试'
  pathSelectionSaving.value = false
}

function closeSavedPaths() {
  savedPathsOpen.value = false
}

function toggleSavedPaths() {
  savedPathsOpen.value = !savedPathsOpen.value
}

function requestSavedPathSwitch(path: ExecutionPath) {
  if (activePathID.value === path.id) {
    closeSavedPaths()
    return
  }
  // 保存列表只在普通或只读详情态开放，因此切换不会跨过未保存编辑草稿。
  selectSavedPath(path)
  closeSavedPaths()
}

async function startNewPath() {
  if (!allowNewPath.value) return
  closeSavedPaths()
  // 新增入口永远从空状态开始，避免已查看路径被误带入新记录。
  clearDraft()
  activePathID.value = null
  workspaceMode.value = transitionExecutionPathWorkspace(workspaceMode.value, 'new')
  draftChoices.value = []
  draftName.value = ''
  draftChangedByGraph.value = false
  draftRecoveryError.value = ''
  createKey.value = crypto.randomUUID()
  pathWorkspaceOpen.value = true
  await canvasRef.value?.setPageFullscreen(true)
}

async function copyActivePath() {
  if (!allowCopy.value || !activePath.value || !graph.value) return
  closeSavedPaths()
  const reconciled = reconcileExecutionPathChoices(graph.value, activePath.value.choices)
  activePathID.value = null
  workspaceMode.value = transitionExecutionPathWorkspace(workspaceMode.value, 'copy')
  draftChoices.value = reconciled.choices
  draftName.value = ''
  draftChangedByGraph.value = reconciled.changed
  draftRecoveryError.value = ''
  createKey.value = crypto.randomUUID()
  pathWorkspaceOpen.value = true
  await canvasRef.value?.setPageFullscreen(true)
}

function clearDraft() {
  activePathID.value = null
  workspaceMode.value = null
  draftChoices.value = []
  draftName.value = ''
  draftChangedByGraph.value = false
  draftRecoveryError.value = ''
  createKey.value = ''
}

function selectBranch(choice: ExecutionPathChoice) {
  if (!graph.value || !workspacePresentation.value.branchEditing || !pathWorkspaceOpen.value || saving.value || generatingAll.value) return
  if (draftChoices.value.some((item) => item.routeNodeId === choice.routeNodeId && item.branchId === choice.branchId)) return
  draftChoices.value = applyExecutionPathChoice(graph.value, draftChoices.value, choice.routeNodeId, choice.branchId)
  draftChangedByGraph.value = false
}

// enterPathEditing 让“编辑路径”明确承担线路管理入口；页面全屏按钮本身只负责查看放大。
async function enterPathEditing() {
  if (!graph.value || !pathsLoaded.value || pathsError.value || saving.value || deleting.value || draftRecoveryLoading.value) return
  if (paths.value.length === 0) {
    await startNewPath()
    return
  }
  clearDraft()
  pathWorkspaceOpen.value = false
  closeSavedPaths()
  await canvasRef.value?.setPageFullscreen(true)
  savedPathsOpen.value = true
}

async function editActivePath() {
  if (!activePath.value || workspaceMode.value !== 'view' || workspaceActionBusy.value) return
  workspaceMode.value = transitionExecutionPathWorkspace(workspaceMode.value, 'edit')
  closeSavedPaths()
  await canvasRef.value?.setPageFullscreen(true)
}

// openPathConfiguration 从只读路径详情进入 F-007 单条路径节点配置画布。
function openPathConfiguration(path: ExecutionPath | null = activePath.value) {
  if (!path) return
  router.push('/plans/' + planID.value + '/paths/' + path.id + '/configure')
}

function resetWorkspaceState() {
  clearDraft()
  pathWorkspaceOpen.value = false
  closeSavedPaths()
}

function closePathDetails() {
  if (workspaceMode.value !== 'view') return
  // 关闭只读详情只清理当前投影，不改变用户主动开启的页面全屏浏览状态。
  resetWorkspaceState()
}

async function completeWorkspaceReset() {
  // 保存成功和用户确认放弃共用同一复位边界，保证不会遗留名称、选择或创建幂等键。
  resetWorkspaceState()
  await canvasRef.value?.setPageFullscreen(false)
}

function requestWorkspaceExit() {
  // 保存请求可能已在服务端提交成功，响应返回前禁止丢弃工作区，避免出现“已放弃但迟到响应仍保存”的竞态。
  if (saving.value) return
  const disposition = deriveExecutionPathWorkspaceDisposition('fullscreen-exit', hasDiscardableChanges.value)
  if (disposition === 'confirm') {
    discardConfirmOpen.value = true
    return
  }
  void completeWorkspaceReset()
}

function cancelPathEditing() {
  const disposition = deriveExecutionPathWorkspaceDisposition('cancel', hasDiscardableChanges.value)
  if (disposition === 'confirm') {
    discardConfirmOpen.value = true
    return
  }
  void completeWorkspaceReset()
}

function confirmDiscardWorkspace() {
  discardConfirmOpen.value = false
  void completeWorkspaceReset()
}

function handlePathMoreAction(key: string) {
  if (workspaceActionBusy.value) return
  if (key === 'copy') {
    void copyActivePath()
    return
  }
  if (key === 'delete') deleteConfirmOpen.value = true
}

async function refreshDraftAfterInvalidSave() {
  if (!workspacePresentation.value.branchEditing) return
  draftRecoveryController?.abort()
  const controller = new AbortController()
  draftRecoveryController = controller
  const version = ++draftRecoveryVersion
  draftRecoveryLoading.value = true
  draftRecoveryError.value = ''
  const result = await refreshExecutionPathDraft(
    draftChoices.value,
    () => fetchFlowGraph(planID.value, controller.signal),
  )
  if (controller.signal.aborted || version !== draftRecoveryVersion) return
  draftChangedByGraph.value = true
  if (result.graph) {
    // 只替换同一计划的当前图和仍能精确对应的选择，不触发 fitView，也不改变草稿模式或幂等键。
    graph.value = result.graph
    draftChoices.value = result.choices
    message.warning('流程已变化，需要重新选择')
  }
  else {
    draftChoices.value = result.choices
    draftRecoveryError.value = '流程已变化，但暂时无法读取最新流程，请重试'
    message.error(draftRecoveryError.value)
  }
  draftRecoveryLoading.value = false
}

async function savePath() {
  if (!plan.value || !graph.value || !workspacePresentation.value.branchEditing || saveDisabled.value) return
  saving.value = true
  try {
    const saved = workspaceMode.value === 'edit' && activePathID.value
      ? await updateExecutionPath(planID.value, activePathID.value, draftName.value, draftChoices.value)
      : await createExecutionPath(planID.value, draftName.value, draftChoices.value, createKey.value)
    const existingIndex = paths.value.findIndex((path) => path.id === saved.id)
    if (existingIndex >= 0) paths.value.splice(existingIndex, 1, saved)
    else paths.value.push(saved)
    paths.value.sort((left, right) => left.sequenceNo - right.sequenceNo)
    plan.value.pathCount = paths.value.length
    try {
      // 路径线路保存接口不负责配置状态；保存后重新读取本地列表，避免已配置路径被响应中的空状态覆盖。
      const refreshed = await fetchExecutionPaths(planID.value, new AbortController().signal)
      paths.value = refreshed
      plan.value.pathCount = refreshed.length
		selectedRunPathIDs.value = new Set(refreshed.filter(path => path.included).map(path => path.id))
		pathSelectionRevisions.value = Object.fromEntries(refreshed.map(path => [path.id, path.configurationRevision]))
    }
    catch {
      // 列表刷新失败不影响已成功保存的线路，也不清空当前列表或草稿状态。
    }
    if (deriveExecutionPathWorkspaceDisposition('save-success', true) === 'reset') {
      await completeWorkspaceReset()
    }
    message.success('执行路径已保存')
  }
  catch (caught) {
    const apiError = caught instanceof ExecutionPathApiError
      ? caught
      : new ExecutionPathApiError('保存失败，请重试')
    if (apiError.code === 'EXECUTION_PATH_INVALID') {
      // 后端已经证明旧图不可继续使用；保留草稿后读取最新图协调，不能让用户从头重选。
      await refreshDraftAfterInvalidSave()
    }
    else {
      // 失败不触碰工作区状态，用户可用原草稿和同一创建幂等键再次提交。
      message.error(apiError.message)
    }
  }
  finally {
    saving.value = false
  }
}

function pathDisplayName(path: ExecutionPath): string {
  return path.name?.trim() || `路径 ${path.sequenceNo}`
}

function explainGenerateAllUnavailable() {
  if (batchPreview.value.exceeded) {
    message.warning('当前流程超过 128 条路径，请手动建立重点路径')
    return
  }
  message.info('当前真实流程的全部路径都已存在')
}

async function generateAllPaths() {
  if (!plan.value || plan.value.flowSource !== 'new' || generatingAll.value || batchPreview.value.exceeded || batchPreview.value.pendingCount === 0) return
  if (!generateAllKey.value) generateAllKey.value = crypto.randomUUID()
  generatingAll.value = true
  try {
    const result = await generateAllExecutionPaths(planID.value, generateAllKey.value)
    const controller = new AbortController()
    const refreshed = await fetchExecutionPaths(planID.value, controller.signal)
    paths.value = refreshed
    pathsLoaded.value = true
    pathsError.value = ''
    plan.value.pathCount = refreshed.length
		selectedRunPathIDs.value = new Set(refreshed.filter(path => path.included).map(path => path.id))
		pathSelectionRevisions.value = Object.fromEntries(refreshed.map(path => [path.id, path.configurationRevision]))
    // 批量生成是旁路管理动作，只刷新持久化列表；当前未保存草稿、名称和单条创建键都不能被新路径覆盖。
    generateAllKey.value = ''
    closeSavedPaths()
    if (result.createdCount === 0) message.info(`全部 ${result.totalCount} 条路径均已存在，无需新增`)
    else message.success(`已新增 ${result.createdCount} 条路径，跳过 ${result.existingCount} 条已存在路径`)
  }
  catch (caught) {
    const apiError = caught instanceof ExecutionPathApiError
      ? caught
      : new ExecutionPathApiError('生成全部路径失败，请重试')
    // 失败保留同一个批量幂等键，网络响应丢失时再次确认不会生成第二批记录。
    message.error(apiError.message)
  }
  finally {
    generatingAll.value = false
  }
}

async function removeActivePath() {
  if (!activePath.value || !plan.value || deleting.value) return
  const deletingID = activePath.value.id
  deleting.value = true
  try {
    await deleteExecutionPath(planID.value, deletingID)
    paths.value = paths.value.filter((path) => path.id !== deletingID)
    const selected = new Set(selectedRunPathIDs.value)
    selected.delete(deletingID)
    selectedRunPathIDs.value = selected
    const revisions = { ...pathSelectionRevisions.value }
    delete revisions[deletingID]
    pathSelectionRevisions.value = revisions
    plan.value.pathCount = paths.value.length
    deleteConfirmOpen.value = false
    await completeWorkspaceReset()
    message.success('执行路径已删除')
  }
  catch (caught) {
    const apiError = caught instanceof ExecutionPathApiError
      ? caught
      : new ExecutionPathApiError('删除失败，请重试')
    message.error(apiError.message)
  }
  finally {
    deleting.value = false
  }
}

function runModeText(value: PersistedPlan): string {
  if (value.runMode === 'parallel') return `并行（最大 ${value.maxConcurrency ?? '-'}）`
  return '串行'
}

function scheduledAtText(value: string | null): string {
  if (!value) return '未设置'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? '时间异常' : parsed.toLocaleString('zh-CN', { hour12: false })
}

// scrollToGraphStructure 只响应用户点击导航；滚轮浏览完全交给 CSS 吸附，减少动态效果时不播放平滑动画。
function scrollToGraphStructure() {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  graphScreenRef.value?.scrollIntoView({ behavior: reducedMotion ? 'auto' : 'smooth', block: 'start' })
}

watch(planID, () => { void loadPage() }, { immediate: true })
onMounted(() => {
  // 主内容区是页面唯一滚动容器；组件只挂载吸附样式类，不接管滚轮事件或强制跳屏。
  pageScrollContainer = document.querySelector<HTMLElement>('.app-main > .n-layout-scroll-container')
  pageScrollContainer?.classList.add('plan-paths-scroll-container')
})
onBeforeUnmount(() => {
  loadController?.abort()
  draftRecoveryController?.abort()
  pageScrollContainer?.classList.remove('plan-paths-scroll-container')
  pageScrollContainer = null
})
</script>

<template>
  <section class="plan-paths-page" :style="pageThemeStyle">
    <n-spin :show="planLoading">
      <div v-if="plan" class="paths-content">
        <section class="plan-paths-screen plan-paths-screen--overview" aria-labelledby="plan-paths-heading">
          <div class="paths-back-bar">
            <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
          </div>
          <header class="page-heading">
            <div>
              <h1 id="plan-paths-heading">{{ plan.name }}</h1>
              <p>从当前入口选择执行线路，并保存为计划路径。</p>
            </div>
            <div class="page-heading__actions">
              <n-tag size="small" type="warning" :bordered="false">
                {{ planStatusLabels[plan.status] }}
              </n-tag>
            </div>
          </header>

          <n-descriptions label-placement="left" :column="3" bordered size="small">
            <n-descriptions-item label="目标流程">{{ plan.targetObjectName }}</n-descriptions-item>
            <n-descriptions-item label="发起账号">
              {{ plan.accountDisplayName ? `${plan.accountDisplayName}（${plan.account}）` : plan.account }}
            </n-descriptions-item>
            <n-descriptions-item label="流程来源">{{ flowSourceLabels[plan.flowSource] }}</n-descriptions-item>
            <n-descriptions-item label="运行方式">{{ runModeText(plan) }}</n-descriptions-item>
            <n-descriptions-item label="定时启动">{{ scheduledAtText(plan.scheduledAt) }}</n-descriptions-item>
            <n-descriptions-item label="路径数量">{{ plan.pathCount }}</n-descriptions-item>
          </n-descriptions>

          <section class="path-preparation" aria-labelledby="path-preparation-heading">
            <div class="path-preparation__header">
              <div>
                <h2 id="path-preparation-heading">路径准备与运行选择</h2>
                <p v-if="pathsLoading">正在读取本地路径配置状态</p>
                <p v-else-if="pathsError">暂时无法读取路径状态，请先重试</p>
						<p v-else>已保存 {{ paths.length }} 条，已配置 {{ configuredPaths.length }} 条，部分配置 {{ partialPaths.length }} 条，待配置 {{ pendingPaths.length - partialPaths.length }} 条，已选 {{ selectedRunPathIDs.size }} 条</p>
              </div>
              <div class="path-preparation__header-actions">
						<n-button v-if="paths.length" size="small" secondary :disabled="pathSelectionLoading || pathSelectionSaving || allPathsSelectedForRun" @click="setAllRunPathSelections(true)">全选</n-button>
                <n-button v-if="paths.length" size="small" secondary :disabled="pathSelectionLoading || pathSelectionSaving || selectedRunPathIDs.size === 0" @click="setAllRunPathSelections(false)">取消全选</n-button>
						<n-popconfirm v-if="selectedRunPathIDs.size" :show-icon="false" positive-text="确认预设" negative-text="取消" @positive-click="applySelectedPreset">
							<template #trigger><n-button size="small" type="primary" secondary :loading="presettingSelected" :disabled="pathSelectionLoading || pathSelectionSaving">一键预设</n-button></template>
							仅对已勾选的 {{ selectedRunPathIDs.size }} 条路径应用安全默认动作，不覆盖已保存配置。
						</n-popconfirm>
              </div>
            </div>
            <n-alert v-if="pathSelectionError" type="error" :show-icon="false">
              {{ pathSelectionError }}
						<n-button text type="primary" @click="retryPaths">重新读取</n-button>
            </n-alert>
					<p v-if="presettingSelected" class="path-preparation__progress">一键预设：{{ presetProgress.completed }}/{{ presetProgress.total }}</p>

            <div v-if="!pathsLoading && !pathsError && !paths.length" class="path-preparation__empty">
              <span>请先配置并保存执行路径</span>
              <n-button type="primary" :disabled="!graph || graphLoading || !allowNewPath" @click="enterPathEditing">配置路径</n-button>
            </div>
            <div v-else-if="!pathsLoading && !pathsError && paths.length" class="path-preparation__list">
              <div v-for="path in paths" :key="path.id" class="path-preparation__item">
                <n-checkbox
                  :checked="selectedRunPathIDs.has(path.id)"
                  :disabled="pathSelectionLoading || pathSelectionSaving"
                  @update:checked="value => updateRunPathSelection(path, value)"
                >
                  运行
                </n-checkbox>
                <div class="path-preparation__identity">
                  <span class="path-preparation__sequence">#{{ path.sequenceNo }}</span>
                  <span class="path-preparation__name" :title="pathDisplayName(path)">{{ pathDisplayName(path) }}</span>
						<n-tag size="small" :bordered="false" :type="path.configurationStatus === 'configured' ? 'success' : path.configurationStatus === 'partial' ? 'warning' : 'default'" :title="path.configurationDetail">
                    {{ path.configurationStatus === 'configured' ? '已配置' : path.configurationStatus === 'partial' ? '部分配置' : '待配置' }}
                  </n-tag>
                </div>
                <n-button size="small" type="primary" secondary @click="openPathConfiguration(path)">配置节点</n-button>
              </div>
            </div>
          </section>

          <div class="flow-structure-jump">
            <n-button size="small" secondary @click="scrollToGraphStructure">查看流程结构 ↓</n-button>
          </div>
        </section>

        <section ref="graphScreenRef" class="plan-paths-screen plan-paths-screen--graph graph-section" aria-labelledby="flow-graph-heading">
          <div class="graph-heading">
            <div>
              <h2 id="flow-graph-heading">流程结构</h2>
              <p>在条件或手动分支上选择线路；并行分支会自动全部纳入。</p>
            </div>
            <n-spin v-if="pathsLoading" size="small" description="正在读取路径" />
          </div>

          <n-alert v-if="pathsError" class="paths-load-error" type="error" :show-icon="false">
            {{ pathsError }}
            <n-button text type="primary" @click="retryPaths">重试</n-button>
          </n-alert>

          <div class="graph-region">
            <div v-if="graphLoading" class="graph-state">
              <n-spin size="large" description="正在读取流程结构" />
            </div>
            <template v-else-if="graph">
              <n-alert v-if="graph.warnings.length" class="graph-warning" type="warning" :show-icon="false">
                {{ graph.warnings.join('；') }}
              </n-alert>
              <flow-graph-canvas
                ref="canvasRef"
                :graph="graph"
                :choices="draftChoices"
                :workspace-open="pathWorkspaceOpen"
                :branch-editing="workspacePresentation.branchEditing"
                :workspace-exit-disabled="saving"
                :save-guide-visible="workspacePresentation.showSave"
                :saved-paths-open="savedPathsOpen"
                @select-branch="selectBranch"
                @request-workspace-exit="requestWorkspaceExit"
                @close-saved-paths="closeSavedPaths"
                @retry="retryGraph"
              >
                <template #canvas-actions-normal>
                  <n-button
                    size="small"
                    type="primary"
                    :disabled="!pathsLoaded || Boolean(pathsError) || saving || deleting || generatingAll || draftRecoveryLoading"
                    @click="enterPathEditing"
                  >
                    编辑路径
                  </n-button>
                </template>
                <template #canvas-actions>
                  <n-button
                    v-if="!workspacePresentation.branchEditing"
                    size="small"
                    secondary
                    :aria-expanded="savedPathsOpen"
                    :disabled="!pathsLoaded || Boolean(pathsError)"
                    @click="toggleSavedPaths"
                  >
                    已保存的路径
                  </n-button>
                  <n-button
                    v-if="!workspacePresentation.branchEditing && allowNewPath"
                    size="small"
                    type="primary"
                    :disabled="saving || deleting || generatingAll || draftRecoveryLoading"
                    @click="startNewPath"
                  >
                    新增路径
                  </n-button>
                </template>
                <template #workspace-panel>
                  <section class="path-selection-panel">
                    <header class="path-selection-panel__header">
                      <div class="path-selection-panel__title-row">
                        <h3>{{ workspacePresentation.title }}</h3>
                        <n-button
                          v-if="workspaceMode === 'view'"
                          size="small"
                          text
                          circle
                          aria-label="关闭路径详情"
                          title="关闭路径详情"
                          @click="closePathDetails"
                        >
                          <span aria-hidden="true">×</span>
                        </n-button>
                      </div>
                      <div v-if="workspacePresentation.branchEditing" class="path-selection-panel__progress" aria-label="决策进度">
                        <span>已选 {{ decisionProgress.selected }}</span>
                        <span>待选 {{ decisionProgress.pending }}</span>
                        <span>共 {{ decisionProgress.total }}</span>
                      </div>
                    </header>

                    <section v-if="workspaceMode === 'view'" class="path-selection-panel__details" aria-label="当前路径">
                      <dl>
                        <div>
                          <dt>序号</dt>
                          <dd>{{ activePath ? `#${activePath.sequenceNo}` : '待分配' }}</dd>
                        </div>
                        <div>
                          <dt>名称</dt>
                          <dd :title="activePath ? pathDisplayName(activePath) : undefined">
                            {{ activePath ? pathDisplayName(activePath) : '路径' }}
                          </dd>
                        </div>
                      </dl>
                    </section>

                    <div v-if="workspacePresentation.showNameInput" class="path-selection-panel__name">
                      <div class="path-selection-panel__name-label">
                        <span>路径名称</span>
                        <n-tag size="small" :bordered="false">
                          {{ activePath ? `稳定序号 #${activePath.sequenceNo}` : '保存后分配稳定序号' }}
                        </n-tag>
                      </div>
                      <n-input
                        v-model:value="draftName"
                        maxlength="50"
                        show-count
                        clearable
                        placeholder="留空后按实际序号命名，例如：路径 3"
                        :disabled="saving || generatingAll || draftRecoveryLoading"
                      />
                    </div>

                    <n-alert v-if="draftRecoveryError" class="path-selection-panel__error" type="error" :show-icon="false">
                      {{ draftRecoveryError }}
                      <n-button text type="primary" :loading="draftRecoveryLoading" @click="refreshDraftAfterInvalidSave">重新读取</n-button>
                    </n-alert>

                    <div class="path-selection-panel__summary" aria-label="实时线路摘要">
                      <h4>实时流向</h4>
                      <ol v-if="pathSummary.length" class="path-summary">
                        <li
                          v-for="item in pathSummary"
                          :key="`${item.kind}-${item.id}`"
                          class="path-summary__item"
                          :class="`path-summary__item--${item.kind}`"
                        >
                          <span class="path-summary__marker" aria-hidden="true" />
                          <div>
                            <strong>{{ item.label }}</strong>
                            <span>{{ item.detail }}</span>
                          </div>
                        </li>
                      </ol>
                      <n-empty v-else size="small" description="当前没有可投影的线路" />
                    </div>
                    <footer class="path-selection-panel__footer">
                      <template v-if="workspaceMode === 'view'">
                        <n-button :disabled="!activePath || workspaceActionBusy" @click="() => openPathConfiguration()">配置节点</n-button>
                        <n-button type="primary" :disabled="!activePath || workspaceActionBusy" @click="editActivePath">编辑路径</n-button>
                        <n-dropdown trigger="click" :options="pathMoreOptions" :disabled="workspaceActionBusy" @select="handlePathMoreAction">
                          <n-button secondary :disabled="!activePath || workspaceActionBusy">更多</n-button>
                        </n-dropdown>
                      </template>
                      <template v-else>
                        <n-button :disabled="saving" @click="cancelPathEditing">取消</n-button>
                        <n-button type="primary" :loading="saving" :disabled="saveDisabled" @click="savePath">保存路径</n-button>
                      </template>
                    </footer>
                  </section>
                </template>
                <template #workspace-collapsed>
                  <div class="path-selection-panel__collapsed-summary">
                    <strong>{{ workspacePresentation.title }}</strong>
                    <div class="path-selection-panel__progress" aria-label="折叠决策进度">
                      <span>已选 {{ decisionProgress.selected }}</span>
                      <span>待选 {{ decisionProgress.pending }}</span>
                      <span>共 {{ decisionProgress.total }}</span>
                    </div>
                  </div>
                </template>
                <template #saved-paths>
                  <section class="saved-paths-popover">
                    <header class="saved-paths-popover__header">
                      <h3>已保存的路径</h3>
                      <n-button text size="small" aria-label="关闭已保存路径" @click="closeSavedPaths">关闭</n-button>
                    </header>
                    <div v-if="plan.flowSource === 'new'" class="saved-paths-popover__generate">
                      <n-popconfirm
                        v-if="!batchPreview.exceeded && batchPreview.pendingCount > 0"
                        :show-icon="false"
                        positive-text="确认生成"
                        negative-text="取消"
                        @positive-click="generateAllPaths"
                      >
                        <template #trigger>
                          <n-button block size="small" secondary :loading="generatingAll" :disabled="saving || deleting || draftRecoveryLoading">
                            一键生成全部路径
                          </n-button>
                        </template>
                        当前真实流程共 {{ batchPreview.totalCount }} 条完整路径，已存在 {{ batchPreview.existingCount }} 条，本次将新增 {{ batchPreview.pendingCount }} 条。确认继续？
                      </n-popconfirm>
                      <n-button v-else block size="small" secondary :disabled="generatingAll" @click="explainGenerateAllUnavailable">
                        一键生成全部路径
                      </n-button>
                    </div>
                    <n-virtual-list
                      v-if="paths.length"
                      class="saved-paths-popover__list"
                      :items="paths"
                      :item-size="SAVED_PATH_ITEM_SIZE"
                      :style="{ height: `${savedPathListHeight}px` }"
                      key-field="id"
                    >
                      <template #default="{ item }">
                      <n-button
                        class="saved-paths-popover__item"
                        :class="{ 'saved-paths-popover__item--active': activePathID === item.id }"
                        text
                        :title="pathDisplayName(item)"
                        :disabled="saving || deleting || generatingAll || draftRecoveryLoading"
                        :aria-current="activePathID === item.id ? 'true' : undefined"
                        @click="requestSavedPathSwitch(item)"
                      >
                        <span class="saved-paths-popover__sequence">#{{ item.sequenceNo }}</span>
                        <span class="saved-paths-popover__name">{{ pathDisplayName(item) }}</span>
                      </n-button>
                      </template>
                    </n-virtual-list>
                    <n-empty v-else class="saved-paths-popover__empty" size="small" description="暂无已保存路径" />
                  </section>
                </template>
              </flow-graph-canvas>
            </template>
            <div v-else class="graph-state">
              <n-empty :description="graphError?.message || '暂时无法读取流程'">
                <template #extra>
                  <n-button type="primary" secondary @click="retryGraph">重试</n-button>
                </template>
              </n-empty>
            </div>
          </div>
        </section>
      </div>

      <section v-else-if="!planLoading" class="plan-paths-screen plan-paths-screen--error paths-error-region">
        <div class="paths-back-bar">
          <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
        </div>
        <n-empty :description="planNotFound ? '计划不存在或已不可用' : planError || '暂时无法读取计划'">
          <template #extra>
            <div class="error-actions">
              <n-button v-if="!planNotFound" type="primary" secondary @click="loadPage">重试</n-button>
              <n-button @click="router.push('/plans')">返回测试计划</n-button>
            </div>
          </template>
        </n-empty>
      </section>
    </n-spin>
    <n-modal
      v-model:show="discardConfirmOpen"
      preset="dialog"
      title="放弃本次修改"
      positive-text="放弃修改"
      negative-text="继续编辑"
      @positive-click="confirmDiscardWorkspace"
    >
      当前名称或线路尚未保存，放弃后本次修改无法恢复。
    </n-modal>
    <n-modal
      v-model:show="deleteConfirmOpen"
      preset="dialog"
      title="删除路径"
      positive-text="确认删除"
      negative-text="取消"
      :loading="deleting"
      @positive-click="removeActivePath"
    >
      只删除当前工具中的路径记录，确认继续？
    </n-modal>
  </section>
</template>

<style scoped>
.plan-paths-page {
  --plan-screen-height: calc(100dvh - 144px);

  width: 100%;
  min-width: 0;
  color: var(--plan-text-color);
  background: var(--plan-page-color);
}

:global(.app-main > .n-layout-scroll-container.plan-paths-scroll-container) {
  scroll-padding-block: 40px;
  scroll-snap-type: y mandatory;
  overscroll-behavior-y: contain;
}

.paths-content {
  display: grid;
  width: min(1180px, 100%);
  margin: 0 auto;
  gap: 40px;
}

.plan-paths-screen {
  width: 100%;
  height: var(--plan-screen-height);
  min-height: 0;
  scroll-snap-align: start;
  scroll-snap-stop: always;
}

.plan-paths-screen--overview,
.plan-paths-screen--graph,
.plan-paths-screen--error {
  display: flex;
  flex-direction: column;
}

.paths-back-bar {
  flex: 0 0 auto;
  margin-bottom: 12px;
}

.page-heading,
.graph-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.page-heading {
  flex: 0 0 auto;
  margin-bottom: 18px;
}

.page-heading__actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-heading h1,
.graph-heading h2 {
  margin: 0;
  font-weight: 600;
}

.page-heading h1 {
  margin-bottom: 8px;
  color: var(--plan-text-color);
  font-size: 28px;
}

.path-preparation {
  display: grid;
  flex: 0 1 auto;
  gap: 12px;
  margin-top: 20px;
  padding: 14px 16px;
  overflow: hidden;
  color: var(--plan-text-color);
  background: var(--plan-card-color);
  border: 1px solid var(--plan-border-color);
  border-radius: 4px;
}

.path-preparation__header,
.path-preparation__item,
.path-preparation__empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.path-preparation__header-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.path-preparation__header h2 {
  margin: 0 0 4px;
  font-size: 15px;
}

.path-preparation__header p {
  margin: 0;
  color: var(--plan-text-secondary-color);
  font-size: 13px;
}

.path-preparation__empty {
  padding-top: 4px;
  color: var(--plan-text-color);
  font-size: 13px;
}

.path-preparation__list {
  display: grid;
  min-height: 0;
  gap: 6px;
  max-height: clamp(96px, calc(var(--plan-screen-height) - 380px), 280px);
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.path-preparation__item {
  min-height: 40px;
  padding: 6px 8px;
  border-top: 1px solid var(--plan-divider-color);
}

.path-preparation__item:first-child {
  border-top: 0;
}

.path-preparation__identity {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  min-width: 0;
  gap: 8px;
}

.path-preparation__sequence {
  flex: 0 0 auto;
  color: var(--plan-text-secondary-color);
  font-variant-numeric: tabular-nums;
}

.path-preparation__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.graph-heading h2 {
  margin-bottom: 5px;
  font-size: 18px;
}

.page-heading p,
.graph-heading p {
  margin: 0;
  color: var(--plan-text-secondary-color);
}

.graph-section {
  margin-top: 0;
}

.graph-heading {
  flex: 0 0 auto;
  margin-bottom: 14px;
}

.flow-structure-jump {
  display: flex;
  flex: 0 0 auto;
  justify-content: center;
  margin-top: auto;
  padding-top: 14px;
}

.graph-region {
  position: relative;
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.graph-state {
  display: grid;
  width: 100%;
  height: 100%;
  min-height: 0;
  place-items: center;
  border-top: 1px solid var(--plan-border-color);
}

.graph-region :deep(.flow-graph-canvas:not(.flow-graph-canvas--page-fullscreen)) {
  height: 100%;
  min-height: 0;
}

.plan-paths-screen--error {
  justify-content: flex-start;
}

.paths-error-region > .n-empty {
  flex: 1 1 auto;
  align-self: center;
}

.graph-warning {
  position: absolute;
  top: 12px;
  right: 16px;
  left: 16px;
  z-index: 4;
}

.paths-load-error {
  flex: 0 0 auto;
  margin-bottom: 12px;
}

.path-selection-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.path-selection-panel__header {
  padding: 4px 14px 10px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.path-selection-panel__header h3,
.path-selection-panel__summary h4 {
  margin: 0;
}

.path-selection-panel__header h3 {
  margin-bottom: 4px;
  font-size: 16px;
}

.path-selection-panel__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 28px;
  gap: 8px;
}

.path-selection-panel__title-row h3 {
  margin-bottom: 0;
}

.path-summary__item span {
  color: var(--flow-label-color);
  font-size: 12px;
  opacity: 0.72;
}

.path-selection-panel__details {
  padding: 10px 14px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.path-selection-panel__details dl {
  display: grid;
  gap: 6px;
  margin: 0;
}

.path-selection-panel__details dl > div {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 8px;
}

.path-selection-panel__details dt {
  color: var(--flow-label-color);
  font-size: 12px;
  opacity: 0.68;
}

.path-selection-panel__details dd {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--flow-label-color);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-selection-panel__progress {
  display: flex;
  gap: 12px;
  color: var(--flow-label-color);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  opacity: 0.72;
}

.path-selection-panel__collapsed-summary {
  display: grid;
  gap: 8px;
  color: var(--flow-label-color);
}

.saved-paths-popover {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-height: 320px;
}

.saved-paths-popover__header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 48px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.saved-paths-popover__header h3 {
  margin: 0;
  font-size: 15px;
}

.saved-paths-popover__list {
  width: 100%;
  max-height: 220px;
  padding: 0 8px;
}

.saved-paths-popover__generate {
  flex: 0 0 auto;
  padding: 8px 12px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.saved-paths-popover__item {
  width: 100%;
  min-width: 0;
  height: 44px;
  padding: 0 8px;
  border-radius: 3px;
}

.saved-paths-popover__item :deep(.n-button__content) {
  display: flex;
  width: 100%;
  min-width: 0;
  justify-content: flex-start;
}

.saved-paths-popover__item--active {
  color: var(--flow-direction-color);
  background: color-mix(in srgb, var(--flow-direction-color) 10%, var(--flow-surface-color));
}

.saved-paths-popover__sequence {
  flex: 0 0 auto;
  font-variant-numeric: tabular-nums;
}

.saved-paths-popover__sequence {
  width: 42px;
  text-align: left;
  opacity: 0.68;
}

.saved-paths-popover__name {
  min-width: 0;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.saved-paths-popover__empty {
  padding: 18px 12px;
}

.path-selection-panel__name {
  display: grid;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.path-selection-panel__name-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: var(--flow-label-color);
  font-size: 13px;
}

.path-selection-panel__name-label span:last-child {
  font-size: 12px;
  opacity: 0.68;
}

.path-selection-panel__error {
  margin: 10px 14px 0;
}

.path-selection-panel__summary {
  flex: 1 1 auto;
  min-height: 0;
  padding: 14px;
  overflow-y: auto;
}

.path-selection-panel__summary h4 {
  margin-bottom: 12px;
  font-size: 13px;
}

.path-selection-panel__footer {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 14px;
  border-top: 1px solid var(--flow-edge-color);
}

.path-summary {
  margin: 0;
  padding: 0 0 0 9px;
  list-style: none;
}

.path-summary__item {
  position: relative;
  min-height: 48px;
  padding: 0 0 14px 16px;
}

.path-summary__item:not(:last-child)::before,
.path-summary__item:not(:last-child)::after {
  position: absolute;
  left: -10px;
  width: 2px;
  content: '';
  pointer-events: none;
}

.path-summary__item:not(:last-child)::before {
  top: 14px;
  bottom: 0;
  background: var(--flow-edge-color);
}

.path-summary__item--node:not(:last-child)::after,
.path-summary__item--choice:not(:last-child)::after,
.path-summary__item--parallel:not(:last-child)::after {
  top: 14px;
  height: 14px;
  background: linear-gradient(to bottom, transparent, var(--flow-direction-color), transparent);
  animation: path-summary-flow 1.25s linear infinite;
}

.path-summary__item:last-child {
  padding-bottom: 0;
}

.path-summary__marker {
  position: absolute;
  top: 5px;
  left: -14px;
  width: 9px;
  height: 9px;
  background: var(--flow-surface-color);
  border: 2px solid var(--flow-edge-color);
  border-radius: 50%;
}

.path-summary__item div {
  display: grid;
  gap: 3px;
}

.path-summary__item strong {
  overflow: hidden;
  color: var(--flow-label-color);
  font-size: 13px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-summary__item--choice .path-summary__marker,
.path-summary__item--parallel .path-summary__marker {
  background: var(--flow-direction-color);
  border-color: var(--flow-direction-color);
}

.path-summary__item--next strong {
  color: var(--flow-direction-color);
}

.path-summary__item--next .path-summary__marker {
  border-color: var(--flow-direction-color);
}

.path-summary__item--pending {
  opacity: 0.62;
}

.path-summary__item--pending .path-summary__marker {
  border-style: dashed;
}

.error-actions {
  display: flex;
  gap: 12px;
}

@keyframes path-summary-flow {
  from { transform: translateY(-14px); }
  to { transform: translateY(28px); }
}

@media (prefers-reduced-motion: reduce) {
  :global(.app-main > .n-layout-scroll-container.plan-paths-scroll-container) {
    scroll-behavior: auto;
  }

  .path-summary__item::after {
    animation: none !important;
  }
}
</style>
