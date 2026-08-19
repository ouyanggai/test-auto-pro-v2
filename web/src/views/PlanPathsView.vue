<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
  NProgress,
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
  fetchExecutionPath,
  fetchExecutionPaths,
  fetchPathGeneration,
  startPathGeneration,
  cancelPathGeneration,
  resumePathGeneration,
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
  reconcileExecutionPathChoices,
  refreshExecutionPathDraft,
  selectedUnconfiguredExecutionPaths,
  transitionExecutionPathWorkspace,
} from '../features/execution-paths/logic'
import type { ExecutionPath, ExecutionPathChoice, ExecutionPathWorkspaceMode, PathGenerationJob } from '../features/execution-paths/types'
import { savePathConfigurationSelection } from '../features/path-configuration/api'
import {
  cancelPathPreparation,
  createPathPreparation,
  fetchActivePathPreparation,
  fetchPathPreparation,
  resumePathPreparation,
} from '../features/path-preparation/api'
import type { PathPreparationJob } from '../features/path-preparation/types'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph, FlowGraphApiError } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
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
const generationJob = ref<PathGenerationJob | null>(null)
const generationError = ref('')
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
const preparationJob = ref<PathPreparationJob | null>(null)
const preparationLoading = ref(false)
const preparationFilter = ref<'all' | 'needs_attention'>('all')
const unconfiguredHighlightPathID = ref<string | null>(null)
const SAVED_PATH_ITEM_SIZE = 44
const PREPARATION_PATH_ITEM_SIZE = 84
const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)
const preparationListRef = ref<{ scrollTo: (options: { index: number }) => void } | null>(null)
const graphScreenRef = ref<HTMLElement | null>(null)
let loadController: AbortController | null = null
let loadVersion = 0
let draftRecoveryController: AbortController | null = null
let draftRecoveryVersion = 0
let pageScrollContainer: HTMLElement | null = null
let generationTimer: ReturnType<typeof setTimeout> | null = null
let preparationTimer: ReturnType<typeof setTimeout> | null = null
let unconfiguredHighlightTimer: ReturnType<typeof setTimeout> | null = null
let lastUnconfiguredSelectionSignature = ''

const planID = computed(() => String(route.params.id || ''))
const planMutable = computed(() => plan.value?.status === 'not_started')
const activePath = computed(() => paths.value.find((path) => path.id === activePathID.value) ?? null)
const configuredPaths = computed(() => paths.value.filter((path) => path.configurationStatus === 'configured'))
const partialPaths = computed(() => paths.value.filter((path) => path.configurationStatus === 'partial'))
const generatedDataPaths = computed(() => paths.value.filter((path) => path.dataStatus === 'generated' || path.dataStatus === 'confirmed' || path.dataStatus === 'not_required'))
const attentionDataPaths = computed(() => paths.value.filter((path) => path.dataStatus === 'needs_attention'))
const visiblePaths = computed(() => preparationFilter.value === 'needs_attention'
  ? paths.value.filter(path => path.dataStatus === 'needs_attention')
  : paths.value)
const preparationPathListHeight = computed(() => Math.min(visiblePaths.value.length, 5) * PREPARATION_PATH_ITEM_SIZE)
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
  || !planMutable.value
  || !pathAnalysis.value?.complete
  || (workspaceMode.value === 'edit' && !draftHasUnsavedChanges.value && !draftChangedByGraph.value)
  || saving.value
  || generationBusy.value
  || draftRecoveryLoading.value
  || Boolean(draftRecoveryError.value))
const allowNewPath = computed(() => Boolean(
  graph.value
  && plan.value
  && pathsLoaded.value
  && !pathsError.value
  && planMutable.value
  && canCreateAdditionalPath(plan.value.flowSource, paths.value.length),
))
const allowCopy = computed(() => pathsLoaded.value
  && planMutable.value
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
  || generationBusy.value
  || draftRecoveryLoading.value)
const pathSummary = computed(() => graph.value && pathAnalysis.value
  ? projectExecutionPathSummary(graph.value, pathAnalysis.value, draftChoices.value)
  : [])
const generationBusy = computed(() => generationJob.value?.status === 'queued' || generationJob.value?.status === 'running')
const preparationBusy = computed(() => preparationJob.value?.status === 'queued' || preparationJob.value?.status === 'running')
const savedPathListHeight = computed(() => Math.min(paths.value.length, 5) * SAVED_PATH_ITEM_SIZE)
const allPathsSelectedForRun = computed(() => paths.value.length > 0 && paths.value.every(path => selectedRunPathIDs.value.has(path.id)))
const pathMoreOptions = computed(() => planMutable.value ? [
  ...(allowCopy.value ? [{ label: '复制路径', key: 'copy' }] : []),
  { label: '删除路径', key: 'delete' },
] : [])

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
  preparationJob.value = null
  unconfiguredHighlightPathID.value = null
  lastUnconfiguredSelectionSignature = ''
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
			lastUnconfiguredSelectionSignature = unconfiguredSelectionSignature()
			if (planMutable.value && plan.value.flowSource === 'new' && pathsResult.value.length === 0) void startAutomaticGeneration()
		void restoreActivePreparation(controller.signal)
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
    if (activePath.value) void selectSavedPath(activePath.value)
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

async function retryPaths(): Promise<boolean> {
  if (!plan.value) return false
  const controller = new AbortController()
  const version = loadVersion
  pathsLoading.value = true
  pathsLoaded.value = false
  pathsError.value = ''
  try {
    const items = await fetchExecutionPaths(planID.value, controller.signal)
    if (version !== loadVersion) return false
    paths.value = items
    pathsLoaded.value = true
		selectedRunPathIDs.value = new Set(items.filter(path => path.included).map(path => path.id))
		pathSelectionRevisions.value = Object.fromEntries(items.map(path => [path.id, path.configurationRevision]))
		lastUnconfiguredSelectionSignature = unconfiguredSelectionSignature()
		return true
  }
  catch (caught) {
    pathsLoaded.value = false
		pathsError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取执行路径'
		return false
  }
  finally {
    pathsLoading.value = false
  }
}

async function selectSavedPath(path: ExecutionPath) {
  if (!graph.value) return
  try {
    const detail = path.choices.length > 0 ? path : await fetchExecutionPath(planID.value, path.id)
    const reconciled = reconcileExecutionPathChoices(graph.value, detail.choices)
    activePathID.value = path.id
    workspaceMode.value = transitionExecutionPathWorkspace(workspaceMode.value, 'select-saved')
    draftChoices.value = reconciled.choices
    draftName.value = detail.name || `路径 ${detail.sequenceNo}`
    draftChangedByGraph.value = reconciled.changed
    draftRecoveryError.value = ''
    createKey.value = ''
    pathWorkspaceOpen.value = true
  }
  catch (caught) {
    pathSelectionError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取路径详情'
  }
}

// persistRunPathSelection 复用现有路径选择接口，只改变当前路径的运行纳入标记。
async function persistRunPathSelection(path: ExecutionPath, included: boolean) {
	if (!planMutable.value) throw new Error('当前计划只能查看')
  const revision = pathSelectionRevisions.value[path.id]
  if (!Number.isInteger(revision)) throw new Error('当前路径选择尚未读取完成')
  const saved = await savePathConfigurationSelection(planID.value, path.id, revision, included, crypto.randomUUID())
  pathSelectionRevisions.value = { ...pathSelectionRevisions.value, [path.id]: saved.nodeRevision }
  const next = new Set(selectedRunPathIDs.value)
  if (included) next.add(path.id)
  else next.delete(path.id)
  selectedRunPathIDs.value = next
}

// applySelectedConfiguration 只创建当前勾选路径的持久后台任务。
async function applySelectedConfiguration() {
  if (!planMutable.value || preparationBusy.value || selectedRunPathIDs.value.size === 0) return
  preparationLoading.value = true
  pathSelectionError.value = ''
  try {
    preparationJob.value = await createPathPreparation(planID.value, crypto.randomUUID())
    schedulePreparationPoll()
  }
  catch (caught) {
    pathSelectionError.value = caught instanceof Error ? caught.message : '一键配置失败，请重试'
  }
  finally {
    preparationLoading.value = false
  }
}

// restoreActivePreparation 恢复刷新前仍在执行的同计划任务。
async function restoreActivePreparation(signal?: AbortSignal) {
  try {
    const active = await fetchActivePathPreparation(planID.value, signal)
    if (!active) return
    preparationJob.value = active
    schedulePreparationPoll()
  }
  catch (caught) {
    if (signal?.aborted) return
    pathSelectionError.value = caught instanceof Error ? caught.message : '暂时无法读取批量准备进度'
  }
}

// schedulePreparationPoll 按任务真实状态刷新聚合进度，完成后重新读取路径双状态。
function schedulePreparationPoll() {
  if (preparationTimer) clearTimeout(preparationTimer)
  if (!preparationBusy.value || !preparationJob.value) return
  preparationTimer = setTimeout(async () => {
    const current = preparationJob.value
    if (!current) return
    try {
      const refreshedJob = await fetchPathPreparation(planID.value, current.id)
      preparationJob.value = refreshedJob
      if (refreshedJob.status === 'completed') {
        if (await retryPaths()) {
          preparationJob.value = null
          message.success(`批量配置完成，已处理 ${refreshedJob.processed} 条路径`)
        }
        return
      }
      schedulePreparationPoll()
    }
    catch (caught) {
      pathSelectionError.value = caught instanceof Error ? caught.message : '暂时无法刷新批量准备进度'
    }
  }, 700)
}

// cancelCurrentPreparation 取消任务并保留当前检查点。
async function cancelCurrentPreparation() {
  if (!planMutable.value || !preparationJob.value || !preparationBusy.value) return
  preparationLoading.value = true
	try {
		preparationJob.value = await cancelPathPreparation(planID.value, preparationJob.value.id)
		if (preparationTimer) clearTimeout(preparationTimer)
	}
	catch (caught) {
		pathSelectionError.value = caught instanceof Error ? caught.message : '取消批量准备失败，请重试'
	}
	finally {
    preparationLoading.value = false
  }
}

// resumeCurrentPreparation 恢复取消或失败任务。
async function resumeCurrentPreparation() {
  if (!planMutable.value || !preparationJob.value || !['cancelled', 'failed'].includes(preparationJob.value.status)) return
  preparationLoading.value = true
	try {
		preparationJob.value = await resumePathPreparation(planID.value, preparationJob.value.id)
		schedulePreparationPoll()
	}
	catch (caught) {
		pathSelectionError.value = caught instanceof Error ? caught.message : '恢复批量准备失败，请重试'
	}
	finally {
    preparationLoading.value = false
  }
}

// unconfiguredSelectionSignature 为当前已勾选且未完成节点配置的集合生成稳定签名。
function unconfiguredSelectionSignature(): string {
	return selectedUnconfiguredExecutionPaths(paths.value, selectedRunPathIDs.value).map(path => path.id).join('|')
}

// revealSelectedUnconfiguredPath 在集合首次变化时定位第一条未配置路径并显示三秒提示。
async function revealSelectedUnconfiguredPath() {
	const signature = unconfiguredSelectionSignature()
	if (!signature) {
		lastUnconfiguredSelectionSignature = ''
		return
	}
	if (signature === lastUnconfiguredSelectionSignature) return
	lastUnconfiguredSelectionSignature = signature
	const target = selectedUnconfiguredExecutionPaths(paths.value, selectedRunPathIDs.value)[0]
	if (!target) return
	preparationFilter.value = 'all'
	await nextTick()
	const index = visiblePaths.value.findIndex(path => path.id === target.id)
	if (index < 0) return
	preparationListRef.value?.scrollTo({ index })
	unconfiguredHighlightPathID.value = target.id
	if (unconfiguredHighlightTimer) clearTimeout(unconfiguredHighlightTimer)
	unconfiguredHighlightTimer = setTimeout(() => {
		if (unconfiguredHighlightPathID.value === target.id) unconfiguredHighlightPathID.value = null
	}, 3000)
}

// updateRunPathSelection 保存用户手动勾选的一条运行路径。
async function updateRunPathSelection(path: ExecutionPath, included: boolean) {
  if (pathSelectionSaving.value || pathSelectionLoading.value) return
  pathSelectionSaving.value = true
  pathSelectionError.value = ''
  try {
    await persistRunPathSelection(path, included)
		await revealSelectedUnconfiguredPath()
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
	await revealSelectedUnconfiguredPath()
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
  void selectSavedPath(path)
  closeSavedPaths()
}

async function startNewPath() {
  if (!planMutable.value || !allowNewPath.value) return
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
  if (!graph.value || !workspacePresentation.value.branchEditing || !pathWorkspaceOpen.value || saving.value || generationBusy.value) return
  if (draftChoices.value.some((item) => item.routeNodeId === choice.routeNodeId && item.branchId === choice.branchId)) return
  draftChoices.value = applyExecutionPathChoice(graph.value, draftChoices.value, choice.routeNodeId, choice.branchId)
  draftChangedByGraph.value = false
}

async function editActivePath() {
  if (!planMutable.value || !activePath.value || workspaceMode.value !== 'view' || workspaceActionBusy.value) return
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
  if (!planMutable.value || workspaceActionBusy.value) return
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
  if (!planMutable.value || !plan.value || !graph.value || !workspacePresentation.value.branchEditing || saveDisabled.value) return
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

// pathConfigurationLabel 把节点人员与动作状态翻译成独立业务标签。
function pathConfigurationLabel(path: ExecutionPath): string {
  if (path.configurationStatus === 'configured') return '节点已配置'
  if (path.configurationStatus === 'partial') return '节点部分配置'
  if (path.configurationStatus === 'affected') return '节点受影响'
  return '节点待配置'
}

// pathDataLabel 把表单数据准备状态翻译成独立业务标签。
function pathDataLabel(path: ExecutionPath): string {
  if (path.dataStatus === 'not_required') return '无需数据'
  if (path.dataStatus === 'generated') return '数据已生成'
  if (path.dataStatus === 'confirmed') return '数据已确认'
  if (path.dataStatus === 'needs_attention') return '数据需处理'
  return '数据未生成'
}

// startAutomaticGeneration 新发起计划首次进入详情时自动解析全部合法路径。
async function startAutomaticGeneration() {
  if (!planMutable.value || !plan.value || plan.value.flowSource !== 'new' || paths.value.length > 0 || generationBusy.value) return
  generationError.value = ''
  try {
    const key = crypto.randomUUID()
    generationJob.value = await startPathGeneration(planID.value, key)
    await pollGeneration()
  } catch (caught) {
    generationError.value = caught instanceof Error ? caught.message : '路径解析失败，请重试'
  }
}

// pollGeneration 按轻量任务快照刷新列表，不加载每条路径的完整配置。
async function pollGeneration() {
  if (!generationJob.value) return
  if (generationJob.value.status === 'completed') {
    await retryPaths()
    return
  }
  if (generationJob.value.status === 'failed' || generationJob.value.status === 'cancelled') return
  generationJob.value = await fetchPathGeneration(planID.value, generationJob.value.id)
  generationTimer = setTimeout(() => { void pollGeneration() }, 500)
}

async function cancelGeneration() {
  if (!generationJob.value || !generationBusy.value) return
  await cancelPathGeneration(planID.value, generationJob.value.id)
  generationJob.value = await fetchPathGeneration(planID.value, generationJob.value.id)
}

async function resumeGeneration() {
  if (!generationJob.value || (generationJob.value.status !== 'cancelled' && generationJob.value.status !== 'failed')) return
  generationJob.value = await resumePathGeneration(planID.value, generationJob.value.id)
  await pollGeneration()
}

async function removeActivePath() {
  if (!planMutable.value || !activePath.value || !plan.value || deleting.value) return
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

// resetPageScroll 进入计划配置页始终回到路径准备区，保留用户主动滚动时的吸附效果。
async function resetPageScroll() {
  await nextTick()
  await new Promise<void>(resolve => window.requestAnimationFrame(() => resolve()))
  pageScrollContainer?.scrollTo({ top: 0, left: 0, behavior: 'auto' })
}

watch(planID, () => {
  void loadPage()
  void resetPageScroll()
}, { immediate: true })
onMounted(() => {
  // 主内容区是页面唯一滚动容器；组件只挂载吸附样式类，不接管滚轮事件或强制跳屏。
  pageScrollContainer = document.querySelector<HTMLElement>('.app-main > .n-layout-scroll-container')
  pageScrollContainer?.classList.add('plan-paths-scroll-container')
  void resetPageScroll()
})
onBeforeUnmount(() => {
  loadController?.abort()
  draftRecoveryController?.abort()
	if (generationTimer) clearTimeout(generationTimer)
	if (preparationTimer) clearTimeout(preparationTimer)
	if (unconfiguredHighlightTimer) clearTimeout(unconfiguredHighlightTimer)
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
							<p>{{ planMutable ? '从当前入口选择执行线路，并保存为计划路径。' : '当前计划已进入只读状态。' }}</p>
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
                <p v-else>共 {{ paths.length }} 条；节点已配置 {{ configuredPaths.length }} 条，部分配置 {{ partialPaths.length }} 条；数据已准备 {{ generatedDataPaths.length }} 条，需处理 {{ attentionDataPaths.length }} 条；已选 {{ selectedRunPathIDs.size }} 条</p>
              </div>
              <div class="path-preparation__header-actions">
							<n-button v-if="planMutable && paths.length" size="small" secondary :disabled="pathSelectionLoading || pathSelectionSaving || allPathsSelectedForRun" @click="setAllRunPathSelections(true)">全选</n-button>
                <n-button v-if="planMutable && paths.length" size="small" secondary :disabled="pathSelectionLoading || pathSelectionSaving || selectedRunPathIDs.size === 0" @click="setAllRunPathSelections(false)">取消全选</n-button>
						<n-button size="small" secondary :type="preparationFilter === 'needs_attention' ? 'warning' : 'default'" @click="preparationFilter = preparationFilter === 'needs_attention' ? 'all' : 'needs_attention'">
							{{ preparationFilter === 'needs_attention' ? '显示全部' : `数据需处理 ${attentionDataPaths.length}` }}
						</n-button>
							<n-popconfirm v-if="planMutable && selectedRunPathIDs.size && !preparationBusy" :show-icon="false" positive-text="确认配置" negative-text="取消" @positive-click="applySelectedConfiguration">
							<template #trigger><n-button size="small" type="primary" secondary :loading="preparationLoading" :disabled="pathSelectionLoading || pathSelectionSaving">一键配置</n-button></template>
							仅准备已勾选的 {{ selectedRunPathIDs.size }} 条路径；已保存的节点和人工表单数据会保留。
						</n-popconfirm>
              </div>
            </div>
            <n-alert v-if="pathSelectionError" type="error" :show-icon="false">
              {{ pathSelectionError }}
						<n-button text type="primary" @click="retryPaths">重新读取</n-button>
            </n-alert>
			<n-alert v-if="generationError" type="error" :show-icon="false">{{ generationError }}</n-alert>
					<section v-if="preparationJob" class="path-preparation__job" aria-label="批量准备进度">
						<div class="path-preparation__job-summary">
							<strong>已处理 {{ preparationJob.processed }} / {{ preparationJob.total }}</strong>
							<span>节点已配置 {{ preparationJob.nodeConfigured }} · 数据已生成 {{ preparationJob.dataGenerated }} · 需处理 {{ preparationJob.needsAttention }} · 失败 {{ preparationJob.failed }}</span>
							<div class="path-preparation__job-actions">
								<n-button v-if="planMutable && preparationBusy" size="tiny" :loading="preparationLoading" @click="cancelCurrentPreparation">取消</n-button>
								<n-button v-else-if="planMutable && (preparationJob.status === 'cancelled' || preparationJob.status === 'failed')" size="tiny" type="primary" :loading="preparationLoading" @click="resumeCurrentPreparation">恢复</n-button>
							</div>
						</div>
						<div class="path-preparation__job-current">
							<span v-if="preparationJob.currentPath">{{ preparationBusy ? '当前处理' : '上次处理' }}：#{{ preparationJob.currentPath.sequenceNo }} {{ preparationJob.currentPath.pathName }}</span>
							<span v-else>{{ preparationJob.status === 'queued' ? '等待开始处理' : '尚未领取路径' }}</span>
							<small v-if="preparationJob.status === 'failed'">{{ preparationJob.error || '任务暂停，请恢复后重试' }}</small>
							<small v-else-if="preparationJob.status === 'cancelled'">任务已取消，可从未完成路径继续</small>
						</div>
						<n-progress type="line" :percentage="preparationJob.total ? Math.round(preparationJob.processed * 100 / preparationJob.total) : 0" :show-indicator="false" />
					</section>

            <div v-if="pathsLoading" class="path-preparation__state path-preparation__state--loading" role="status">
              <n-spin size="small" />
              <span>正在加载路径</span>
            </div>
            <div v-else-if="pathsError" class="path-preparation__state path-preparation__state--error" role="alert">
              <span>{{ pathsError }}</span>
              <n-button size="small" @click="retryPaths">重试</n-button>
            </div>
            <div v-else-if="!paths.length" class="path-preparation__empty">
              <span>{{ generationBusy ? '正在后台解析全部合法路径' : '请先配置并保存执行路径' }}</span>
							<n-button v-if="planMutable && !generationBusy" type="primary" :disabled="!graph || graphLoading || !allowNewPath" @click="startNewPath">
                新增路径
              </n-button>
            </div>
            <div v-else-if="!visiblePaths.length" class="path-preparation__empty">
              <span>当前没有数据需处理的路径</span>
              <n-button size="small" secondary @click="preparationFilter = 'all'">显示全部</n-button>
            </div>
            <n-virtual-list
              v-else
								ref="preparationListRef"
              class="path-preparation__list"
              :items="visiblePaths"
              :item-size="PREPARATION_PATH_ITEM_SIZE"
              :style="{ height: `${preparationPathListHeight}px` }"
              key-field="id"
            >
              <template #default="{ item: path }">
								<div class="path-preparation__item" :class="{ 'path-preparation__item--attention': unconfiguredHighlightPathID === path.id }">
                <n-checkbox
                  :checked="selectedRunPathIDs.has(path.id)"
									:disabled="!planMutable || pathSelectionLoading || pathSelectionSaving"
                  @update:checked="value => updateRunPathSelection(path, value)"
                >
                  运行
                </n-checkbox>
								<div class="path-preparation__identity">
									<div class="path-preparation__identity-main">
										<span class="path-preparation__sequence">#{{ path.sequenceNo }}</span>
										<span class="path-preparation__name" :title="pathDisplayName(path)">{{ pathDisplayName(path) }}</span>
							<n-tag size="small" :bordered="false" :type="path.configurationStatus === 'configured' ? 'success' : path.configurationStatus === 'partial' || path.configurationStatus === 'affected' ? 'warning' : 'default'" :title="path.configurationDetail">
                    {{ pathConfigurationLabel(path) }}
                  </n-tag>
							<n-tag size="small" :bordered="false" :type="path.dataStatus === 'confirmed' || path.dataStatus === 'generated' || path.dataStatus === 'not_required' ? 'success' : path.dataStatus === 'needs_attention' ? 'error' : 'default'" :title="path.dataDetail">
                    {{ pathDataLabel(path) }}
                  </n-tag>
									</div>
									<small v-if="unconfiguredHighlightPathID === path.id" class="path-preparation__attention-message">已选择未配置路径，请先配置节点</small>
                </div>
								<n-button size="small" type="primary" secondary @click="openPathConfiguration(path)">{{ planMutable ? '配置节点' : '查看配置' }}</n-button>
              </div>
              </template>
            </n-virtual-list>
          </section>

          <div class="flow-structure-jump">
            <n-button size="small" secondary @click="scrollToGraphStructure">查看流程结构 ↓</n-button>
          </div>
        </section>

        <section ref="graphScreenRef" class="plan-paths-screen plan-paths-screen--graph graph-section" aria-labelledby="flow-graph-heading">
          <div class="graph-heading">
            <div>
              <h2 id="flow-graph-heading">流程结构</h2>
            </div>
          </div>

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
										v-if="planMutable"
                    size="small"
                    type="primary"
                    :disabled="!pathsLoaded || Boolean(pathsError) || saving || deleting || generationBusy || draftRecoveryLoading"
                    @click="startNewPath"
                  >
                    新增路径
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
                    :disabled="saving || deleting || generationBusy || draftRecoveryLoading"
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
                        :disabled="saving || generationBusy || draftRecoveryLoading"
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
												<n-button :disabled="!activePath || workspaceActionBusy" @click="() => openPathConfiguration()">{{ planMutable ? '配置节点' : '查看配置' }}</n-button>
												<n-button v-if="planMutable" type="primary" :disabled="!activePath || workspaceActionBusy" @click="editActivePath">编辑路径</n-button>
												<n-dropdown v-if="planMutable" trigger="click" :options="pathMoreOptions" :disabled="workspaceActionBusy" @select="handlePathMoreAction">
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
					<div v-if="plan.flowSource === 'new' && generationJob" class="saved-paths-popover__generate">
						<span>路径解析 {{ generationJob.completed }}/{{ generationJob.total || '...' }}</span>
						<n-button v-if="generationBusy" size="small" secondary @click="cancelGeneration">取消</n-button>
						<n-button v-else-if="generationJob.status === 'cancelled' || generationJob.status === 'failed'" size="small" secondary @click="resumeGeneration">恢复</n-button>
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
                        :disabled="saving || deleting || generationBusy || draftRecoveryLoading"
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
.path-preparation__empty,
.path-preparation__state {
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

.path-preparation__job {
  display: grid;
  gap: 8px;
	padding: 10px 0;
	border-top: 1px solid var(--plan-border-color);
	border-bottom: 1px solid var(--plan-border-color);
}

.path-preparation__job-summary {
  display: flex;
  align-items: center;
	flex-wrap: wrap;
  gap: 10px;
}

.path-preparation__job-current {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	min-width: 0;
	color: var(--plan-text-secondary-color);
	font-size: 12px;
	flex-wrap: wrap;
}

.path-preparation__job-current > span {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.path-preparation__job-current > small {
	flex: 0 0 auto;
}

.path-preparation__job-summary > span {
  flex: 1 1 auto;
  color: var(--plan-text-secondary-color);
  font-size: 12px;
}

.path-preparation__job-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
}

.path-preparation__empty {
  padding-top: 4px;
  color: var(--plan-text-color);
  font-size: 13px;
}

.path-preparation__state {
  min-height: 56px;
  padding: 4px 0;
  color: var(--plan-text-secondary-color);
  font-size: 13px;
}

.path-preparation__state--loading {
  justify-content: center;
  gap: 10px;
}

.path-preparation__state--error {
  color: var(--plan-text-color);
}

.path-preparation__list {
  min-height: 0;
  max-height: clamp(96px, calc(var(--plan-screen-height) - 380px), 280px);
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.path-preparation__item {
	position: relative;
  box-sizing: border-box;
	height: 84px;
  padding: 6px 8px;
	transition: background-color 160ms ease, box-shadow 160ms ease;
  border-bottom: 1px solid var(--plan-divider-color);
}

.path-preparation__item--attention {
	background: rgba(240, 160, 32, 0.14);
	box-shadow: inset 3px 0 0 #f0a020;
	animation: path-preparation-attention 3s ease-out;
}

.path-preparation__identity {
	display: grid;
	flex: 1 1 auto;
	min-width: 0;
	gap: 3px;
}

.path-preparation__identity-main {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	min-width: 0;
	gap: 8px;
}

.path-preparation__attention-message {
	color: #b45309;
	font-size: 12px;
	line-height: 16px;
}

.path-preparation__sequence {
  flex: 0 0 auto;
  color: var(--plan-text-secondary-color);
  font-variant-numeric: tabular-nums;
}

.path-preparation__name {
	flex: 1 1 120px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@keyframes path-preparation-attention {
	0%, 35% { background: rgba(240, 160, 32, 0.22); }
	100% { background: rgba(240, 160, 32, 0.04); }
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

	.path-preparation__item--attention {
		animation: none;
	}
}
</style>
