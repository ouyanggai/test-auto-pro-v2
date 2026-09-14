<script setup lang="ts">
import { NAlert, NButton, NInputNumber, NPopconfirm, NPopover, NResult, NSelect, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import type { FlowGraph } from '../features/flow-graph/types'
import {
  approveRun,
  fetchRunDetail,
  formatElapsed,
  formatTime,
  removeBreakpoint,
  requestPause,
  RunApiError,
  retryFailedAction,
  setBreakpoint,
  stopRun,
  switchRunMode,
} from '../features/runs/api'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { BreakpointInput, PathRunDetail } from '../features/runs/api'
import { fetchRunEvents, type RunEventItem } from '../features/runs/api'
import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { pathConfigNodeKey } from '../features/path-configuration/logic'
import RunNodePanel from '../features/runs/RunNodePanel.vue'
import { actionLabel } from '../features/runs/presentation'

// RunDetailView 是路径运行详情：运行画布为主体，顶部固定条控制放行与停止。
// 放行会发出真实写请求：只接受明确点击，不绑定单键快捷键。
const route = useRoute()
const router = useRouter()
const runId = String(route.params.runId || '')
const runEvents = ref<RunEventItem[]>([])
let lastEventID = 0
let eventPathRunID = 0
const activeTab = ref<'canvas' | 'events'>('canvas')
const selectedPathRunID = ref<number>(Number(route.params.pathRunId || 0) || 0)

let requestVersion = 0
let disposed = false
const inflightControllers = new Set<AbortController>()
const lastValidCurrentNodeByVersion = new Map<number, string>()

function beginRequest(): { version: number; controller: AbortController } {
  requestVersion += 1
  for (const controller of inflightControllers) {
    controller.abort()
  }
  inflightControllers.clear()
  const controller = new AbortController()
  inflightControllers.add(controller)
  return { version: requestVersion, controller }
}

function trackSignal(controller: AbortController): AbortSignal {
  inflightControllers.add(controller)
  return controller.signal
}

function isCurrent(version: number, pathRunId: number): boolean {
  if (disposed || version !== requestVersion) return false
  if (pathRunId === selectedPathRunID.value) return true
  // 首次进入还没有路径参数时，本次响应会把缺省路径写回路由；同代次仍算当前请求。
  return pathRunId === 0
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

function switchPathRun(pathRunID: number) {
  if (pathRunID === selectedPathRunID.value) return
  closeNodePanel()
  void router.push(`/runs/${runId}/paths/${pathRunID}`).catch((error) => {
    if (!isAbortError(error)) {
      errorText.value = error instanceof RunApiError ? error.message : '切换路径失败'
    }
  })
}

watch(() => route.params.pathRunId, (next) => {
  const nextID = Number(next || 0) || 0
  if (!nextID || nextID === selectedPathRunID.value) return
  closeNodePanel()
  selectedPathRunID.value = nextID
  lastEventID = 0
  eventPathRunID = nextID
  runEvents.value = []
  void loadDetail().catch(() => {})
})
const themeVars = useThemeVars()

const detail = ref<PathRunDetail | null>(null)
const graph = ref<FlowGraph | null>(null)
const loading = ref(true)
const errorText = ref('')
const loadErrorText = ref('')
const loadFailure = ref<RunApiError | null>(null)
const detailNotFound = computed(() => loadFailure.value?.status === 404)
const actionText = ref('')
const acting = ref(false)

const lastUpdateAt = ref<number>(Date.now())

// 轮询：状态只在放行后变化，间隔来自后端配置。
let pollTimer: number | null = null

// 自动跟随：默认把当前步平移到操作区中央；用户手动平移后暂停并显示「回到当前步」。
const followPaused = ref(false)
const programmaticMove = ref(false)
const selectedNodeKey = ref('')

const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)

// pathChoices 是这条路径已保存的分支选择：画布据此区分路径内/路径外节点（评审缺陷 8 的修复点）。
const pathChoices = computed(() => detail.value?.pathChoices ?? [])

// runPathAnalysis 按已保存分支选择遍历真实结构，得到已配置路线经过的节点与连线。
const runPathAnalysis = computed(() => (graph.value ? analyzeExecutionPath(graph.value, pathChoices.value) : null))

// runTakenEdgeIds 是实际走过的连线：按已落账步骤顺序连接相邻节点（连线表达实际走向，T08）。
const runTakenEdgeIds = computed<string[]>(() => {
  if (!graph.value || !detail.value) return []
  const settledKeys = detail.value.steps.map((step) => step.nodeId || step.nodeKey)
  if (settledKeys.length < 2) return []
  const ids: string[] = []
  for (let index = 0; index + 1 < settledKeys.length; index += 1) {
    for (const edge of graph.value.edges) {
      if (edge.source === settledKeys[index] && edge.target === settledKeys[index + 1]) {
        ids.push(edge.id)
      }
    }
  }
  return ids
})

// runDeviationEdgeIds 是走过但不在已配置路线里的连线（偏离标红）。
const runDeviationEdgeIds = computed<string[]>(() => {
  if (!runPathAnalysis.value) return []
  return runTakenEdgeIds.value.filter((edgeID) => !runPathAnalysis.value!.reachableEdgeIds.has(edgeID))
})

// 运行现场已丢失（服务重启或执行结果无法确认）：只展示一句大白话说明与只读记录，
// 不提供任何对账、重试或登记入口；用户继续执行的唯一方式是从计划重新发起一次运行。
const sceneLostNote = computed(() => detail.value?.sceneLostNote || '')

// 放行命令与条件写参数：命令集合由后端给出，游标与版本取自详情（重复点击只产生一次效果）。
const approveCommand = ref('step')
const approveCursor = ref(0)
const approveVersion = ref(0)

// syncControl 从最新详情同步命令与条件写参数。
function syncControl(next: PathRunDetail): void {
  approveCommand.value = (next.commands?.length ?? 0) > 0 ? next.commands[0].command : 'step'
  approveCursor.value = next.currentStepNo
  approveVersion.value = next.controlVersion
}

// runCommand 执行一个非 step 的连续命令（执行到下一节点/继续运行），随后交给轮询刷新。
const looping = ref(false)
async function runCommand(command: string): Promise<void> {
  if (looping.value || !detail.value) return
  looping.value = true
  errorText.value = ''
  try {
    detail.value = await approveRun(runId, command, detail.value.currentStepNo, detail.value.controlVersion, detail.value?.pathRunId)
    syncControl(detail.value)
    lastUpdateAt.value = Date.now()
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '命令执行失败，请查看日志'
  } finally {
    looping.value = false
    schedulePoll()
  }
}

// pauseNow 提交暂停请求（本步完成结果确认与记录后生效）。
const pausing = ref(false)

async function pauseNow(): Promise<void> {
  if (pausing.value) return
  pausing.value = true
  try {
    await requestPause(runId, detail.value?.pathRunId)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '暂停请求失败，请重试'
  } finally {
    pausing.value = false
  }
}

// addNodeBreakpoint 在当前选中节点上就地挂节点断点。
// 画布给的是图节点 ID，命中判定用的是配置令牌键：挂载前必须换键，否则断点永不命中（评审 P1）。
async function addNodeBreakpoint(): Promise<void> {
  if (!selectedNodeKey.value) return
  try {
    const tokenKey = await pathConfigNodeKey(selectedNodeKey.value)
    const list = await setBreakpoint(runId, { type: 'node', nodeKey: tokenKey }, detail.value?.pathRunId)
    applyBreakpoints(list)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '设置断点失败'
  }
}

// deleteBreakpoint 删除一个断点（路径偏离断点由后端拒绝并给中文原因）。
async function deleteBreakpoint(bp: BreakpointInput): Promise<void> {
  try {
    const list = await removeBreakpoint(runId, bp, detail.value?.pathRunId)
    applyBreakpoints(list)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '删除断点失败'
  }
}

// 断点分区：强制生效（不可删）与手工挂载（可删）分开，避免把"删不掉的"和"能删的"混在一张列表里。
const forcedBreakpoints = computed(() => (detail.value?.breakpoints ?? []).filter(bp => bp.type === 'path_deviation' || bp.type === 'first_write'))
const userBreakpoints = computed(() => (detail.value?.breakpoints ?? []).filter(bp => bp.type !== 'path_deviation' && bp.type !== 'first_write'))

// forcedBreakpointNote 说明强制断点为什么在这里、能不能关。
function forcedBreakpointNote(type: string): string {
  if (type === 'path_deviation') return '始终生效且不可关闭：实际走向与已配置路径不一致时强制停下'
  if (type === 'first_write') return '默认开启：拦住本次运行的第一个写请求，可删除'
  return ''
}

// breakpointTargetText 把断点挂载对象写成中文，不显示内部键。
function breakpointTargetText(bp: { type: string, nodeName?: string, stepNo?: number, action?: string }): string {
  if (bp.type === 'node') return bp.nodeName ? `节点：${bp.nodeName}` : '节点：未指定'
  if (bp.type === 'step') return bp.stepNo ? `第 ${bp.stepNo} 步` : '步骤：未指定'
  if (bp.type === 'action') return bp.action ? `动作：${breakpointActionLabel(bp.action)}` : '动作：未指定'
  return ''
}

// 新增断点的三种挂载方式：节点取画布选中项，步骤取序号，动作取动作类型。
const newBreakpointType = ref<'node' | 'step' | 'action'>('node')
const newBreakpointStep = ref<number | null>(null)
const newBreakpointAction = ref<string | null>(null)
const breakpointTypeOptions = [
  { label: '节点断点', value: 'node' },
  { label: '步骤断点', value: 'step' },
  { label: '动作断点', value: 'action' },
]
const actionBreakpointOptions = [
  { label: '发起', value: 'submit' },
  { label: '同意', value: 'approve' },
  { label: '不同意', value: 'reject' },
  { label: '暂存表单', value: 'storage_form_data' },
  { label: '重新提交', value: 'resubmit' },
  { label: '回退上一级', value: 'rollback_previous' },
  { label: '取回', value: 'retrieve' },
  { label: '撤回', value: 'withdraw' },
  { label: '转发', value: 'forward' },
]
const breakpointTypeHint = computed(() => {
  if (newBreakpointType.value === 'node') return '挂在尚未执行的节点上，运行到该节点前停下'
  if (newBreakpointType.value === 'step') return '挂在尚未执行的步骤序号上；已执行过的步骤会被后端拒绝并说明原因'
  return '挂在动作类型上：本次运行里每次要执行该动作前都停下'
})

// breakpointActionLabel 把断点下拉里的动作键转成中文；未登记的键原样显示，不猜。
function breakpointActionLabel(action: string): string {
  return actionBreakpointOptions.find(option => option.value === action)?.label ?? action
}

// addStepBreakpoint / addActionBreakpoint 与节点断点走同一条即时生效链路。
async function addStepBreakpoint(): Promise<void> {
  if (!newBreakpointStep.value) return
  try {
    applyBreakpoints(await setBreakpoint(runId, { type: 'step', stepNo: newBreakpointStep.value }, detail.value?.pathRunId))
    newBreakpointStep.value = null
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '设置步骤断点失败'
  }
}

async function addActionBreakpoint(): Promise<void> {
  if (!newBreakpointAction.value) return
  try {
    applyBreakpoints(await setBreakpoint(runId, { type: 'action', action: newBreakpointAction.value }, detail.value?.pathRunId))
    newBreakpointAction.value = null
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '设置动作断点失败'
  }
}

// breakpointTypeName 把断点类型转成中文名，五类都要覆盖，不允许把内部键漏到界面上。
function breakpointTypeName(type: string): string {
  switch (type) {
    case 'node': return '节点断点'
    case 'step': return '步骤断点'
    case 'action': return '动作断点'
    case 'first_write': return '首次写断点'
    case 'path_deviation': return '路径偏离断点'
    default: return type
  }
}

// applyBreakpoints 把后端返回的断点列表同步进详情（即时可见，不需要刷新页面）。
function applyBreakpoints(list: BreakpointInput[]): void {
  if (!detail.value) return
  detail.value.breakpoints = list.map((bp) => ({
    type: bp.type,
    typeName: breakpointTypeName(bp.type),
    // nodeKey 是挂载键（删除断点要原样带回）；nodeName 是服务端翻译好的业务名称（不显示内部键）。
    nodeKey: bp.nodeKey,
    nodeName: bp.nodeName,
    stepNo: bp.stepNo,
    action: bp.action,
  }))
}

const currentNodeKey = computed(() => {
  const previewKey = detail.value?.currentPreview?.nodeId || detail.value?.currentPreview?.nodeKey || ''
  if (previewKey) {
    lastValidCurrentNodeByVersion.set(requestVersion, previewKey)
    return previewKey
  }
  const kept = lastValidCurrentNodeByVersion.get(requestVersion) || ''
  if (kept && detail.value && !['已完成', '失败', '结果待确认', '已停止', '已取消'].includes(detail.value.pathRunStatusName)) {
    return kept
  }
  if (detail.value?.steps && detail.value.steps.length > 0) {
    const lastStep = detail.value.steps[detail.value.steps.length - 1]
    return lastStep.nodeId || lastStep.nodeKey || ''
  }
  return ''
})

// runNodeStates 把九个中文运行态与当前步标记交给画布。
const runNodeStates = computed(() => {
  const states: Record<string, { status: string; statusName: string }> = {}
  for (const [nodeKey, state] of Object.entries(detail.value?.nodeStates || {})) {
    states[nodeKey] = { status: state.status, statusName: state.statusName }
  }
  return states
})

// runErrorNotes 是失败/结果待确认节点上的一句话错误摘要：
// 取最后一个已落账步骤里第一个非成功尝试的原因原文，只呈现、不猜测。
const runErrorNotes = computed<Record<string, string>>(() => {
  const notes: Record<string, string> = {}
  if (!detail.value || detail.value.steps.length === 0) return notes
  const closing = ['失败', '结果待确认']
  if (!closing.includes(detail.value.pathRunStatusName)) return notes
  const last = detail.value.steps[detail.value.steps.length - 1]
  const bad = last.attempts.find((attempt) => attempt.verdictName !== '执行成功' && attempt.verdictName !== '确定成功' && attempt.verdictName !== '目标跳过')
  if (bad?.reason) {
    const nodeID = last.nodeId || last.nodeKey
    if (nodeID) notes[nodeID] = bad.reason
  }
  return notes
})

// isActing 表示一次放行或停止请求在途：此时放行/停止按钮进入忙碌态。
const isActing = computed(() => acting.value)

// runBusy 表示当前步此刻真的在执行：放行请求在途、连续执行中，或路径运行处于核验中。
// 画布据此在当前步节点上播执行动画；只是停在这里等放行时保持静态高亮。
const runBusy = computed(() => acting.value
  || looping.value
  || Boolean(detail.value?.loopRunning)
  || Boolean(detail.value?.stepInFlight)
  || detail.value?.pathRunStatusName === '确认结果中')

// runStepNotes 按图节点 ID 汇总每个节点最近一次已落账步骤的紧凑事实：
// 步序·动作·处理人·耗时都取自运行详情 DTO 的同源事实，卡片直接呈现，不点开右栏也能读。
// 多步骤节点（如动作次数循环）显示最后一步的耗时，步序显示该节点最后走过的序号。
const runStepNotes = computed<Record<string, string>>(() => {
  const notes: Record<string, string> = {}
  if (!detail.value) return notes
  for (const step of detail.value.steps) {
    const nodeID = step.nodeId || step.nodeKey
    if (!nodeID) continue
    const seconds = step.durationMs >= 1000 ? `${Math.round(step.durationMs / 1000)}秒` : `${step.durationMs}毫秒`
    const actor = step.actorName ? `· ${step.actorName}` : ''
    notes[nodeID] = `第 ${step.stepNo} 步 ${actionLabel(step.actionName, step.action)} ${actor} · ${seconds}`.replace(/\s+/g, ' ').trim()
  }
  return notes
})

// headerVars 给传送到顶栏的身份区单独带上主题色：传送出去的节点不在本页根节点下，
// 拿不到根上声明的自定义属性。
const headerVars = computed(() => ({ '--run-secondary-text-color': themeVars.value.textColor3 }))

// graphNodeByID 是图节点索引：面板标题与断点提示都用真实业务名称，不显示内部标识。
const graphNodeByID = computed(() => new Map((graph.value?.nodes ?? []).map((node) => [node.id, node])))
const selectedNodeName = computed(() => graphNodeByID.value.get(selectedNodeKey.value)?.name || '')
const selectedNodeTypeName = computed(() => graphNodeByID.value.get(selectedNodeKey.value)?.typeName || '')

async function loadDetail(): Promise<void> {
  if (!runId) {
    loadErrorText.value = '运行标识缺失，无法打开详情。'
    loading.value = false
    return
  }
  const firstLoad = !detail.value
  loading.value = firstLoad
  loadErrorText.value = ''
  loadFailure.value = null
  const { version, controller } = beginRequest()
  const pathRunId = selectedPathRunID.value
  try {
    const next = await fetchRunDetail(runId, controller.signal, pathRunId)
    if (!isCurrent(version, pathRunId)) return
    if (!next) {
      loadErrorText.value = '任务详情返回空数据，请重试'
      return
    }
    applyDetail(next, version)
    if (!selectedPathRunID.value && next.pathRunId) {
      selectedPathRunID.value = next.pathRunId
      void router.replace(`/runs/${runId}/paths/${next.pathRunId}`).catch((error) => {
        if (!isAbortError(error) && isCurrent(version, next.pathRunId)) {
          errorText.value = error instanceof RunApiError ? error.message : '更新路径地址失败'
        }
      })
    }
    await pollEvents(version, controller.signal)
    if (!graph.value) {
      const graphSignal = new AbortController()
      try {
        const nextGraph = await fetchFlowGraph(String(next.planId), trackSignal(graphSignal))
        if (isCurrent(version, pathRunId || next.pathRunId)) {
          graph.value = nextGraph
        }
      } catch (error) {
        if (!isAbortError(error) && isCurrent(version, pathRunId || next.pathRunId)) {
          loadErrorText.value = error instanceof Error ? error.message : '暂时无法读取流程结构'
        }
      }
    }
    schedulePoll(version)
  } catch (error) {
    if (isAbortError(error) || !isCurrent(version, pathRunId)) return
    loadFailure.value = error instanceof RunApiError ? error : null
    loadErrorText.value = error instanceof RunApiError ? error.message : '暂时无法读取任务详情，请重试'
  } finally {
    inflightControllers.delete(controller)
    if (isCurrent(version, pathRunId)) {
      loading.value = false
      if (detail.value) schedulePoll(version)
    }
  }
}

function applyDetail(next: PathRunDetail, version: number): void {
  const previewKey = next.currentPreview?.nodeId || next.currentPreview?.nodeKey || ''
  if (previewKey) {
    lastValidCurrentNodeByVersion.set(version, previewKey)
  } else if (detail.value && selectedPathRunID.value === next.pathRunId) {
    const kept = lastValidCurrentNodeByVersion.get(version)
    if (kept) lastValidCurrentNodeByVersion.set(version, kept)
  }
  detail.value = next
  syncControl(next)
  lastUpdateAt.value = Date.now()
}

function schedulePoll(version = requestVersion): void {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }
  if (!detail.value || disposed) return
  const pollVersion = version
  const terminalStatuses = ['已完成', '失败', '结果待确认', '已停止', '已取消']
  const siblingActive = (detail.value.paths ?? []).some((path) => !terminalStatuses.includes(path.statusName) && path.statusName !== '暂停')
  if (terminalStatuses.includes(detail.value.pathRunStatusName) && !siblingActive) return
  const interval = Math.max(500, detail.value.pollIntervalMs || 2000)
  pollTimer = window.setTimeout(async () => {
    if (!isCurrent(pollVersion, selectedPathRunID.value)) return
    const controller = new AbortController()
    inflightControllers.add(controller)
    try {
      const next = await fetchRunDetail(runId, controller.signal, selectedPathRunID.value)
      if (!isCurrent(pollVersion, selectedPathRunID.value)) return
      applyDetail(next, pollVersion)
      await pollEvents(pollVersion, controller.signal)
    } catch (error) {
      if (!isAbortError(error) && isCurrent(pollVersion, selectedPathRunID.value)) {
        errorText.value = error instanceof RunApiError ? error.message : '刷新任务详情失败'
      }
    } finally {
      inflightControllers.delete(controller)
    }
    if (isCurrent(pollVersion, selectedPathRunID.value)) schedulePoll(pollVersion)
  }, interval)
}

async function pollEvents(version = requestVersion, signal?: AbortSignal): Promise<void> {
  const pathRunId = selectedPathRunID.value
  if (eventPathRunID !== pathRunId) {
    lastEventID = 0
    eventPathRunID = pathRunId
    runEvents.value = []
  }
  try {
    const fresh = await fetchRunEvents(runId, lastEventID, pathRunId || undefined, signal)
    if (!isCurrent(version, pathRunId)) return
    for (const event of fresh) {
      if (event.id > lastEventID) {
        runEvents.value.push(event)
        lastEventID = event.id
      }
    }
  } catch (error) {
    if (isAbortError(error) || !isCurrent(version, pathRunId)) return
    errorText.value = error instanceof RunApiError ? error.message : '读取事件流失败'
  }
}

// focusNodeQuietly 把某个节点平移到画布中央，并标记这是程序化移动（不算用户接管跟随）。
function focusNodeQuietly(nodeID: string): void {
  if (!nodeID) return
  programmaticMove.value = true
  canvasRef.value?.focusNode(nodeID)
  window.setTimeout(() => { programmaticMove.value = false }, 320)
}

// centerCurrentNode 把当前步节点平移到画布中央。
function centerCurrentNode(): void {
  focusNodeQuietly(currentNodeKey.value)
}

// shouldFollowCurrent 决定这次刷新要不要把画布拉回当前步。
// 用户手动平移过、正在检视别的节点、或正在看事件流时都不跟随（纲领 12.2「不抢用户注意力」）。
function shouldFollowCurrent(): boolean {
  if (followPaused.value || !currentNodeKey.value || activeTab.value !== 'canvas') return false
  return selectedNodeKey.value === '' || selectedNodeKey.value === currentNodeKey.value
}

// handleRunViewportChange 在用户手动平移/缩放时暂停自动跟随。
function handleRunViewportChange(): void {
  if (programmaticMove.value) return
  if (isActing.value || currentNodeKey.value) {
    followPaused.value = true
  }
}

// resumeFollow 恢复自动跟随并立即回到当前步。
function resumeFollow(): void {
  followPaused.value = false
  centerCurrentNode()
}

// approve 放行当前步：发送当前可用命令集合的第一条（自动模式是连续执行，手动控制是单步），
// 不再硬编码 step——自动模式下硬编码 step会把连续运行拆成一步一停（实测缺陷）。
// 等待响应期间指示器进入执行中，写请求不可中断。
async function approve(): Promise<void> {
  if (acting.value || !detail.value?.currentPreview) return
  const command = approveCommand.value || 'step'
  const pathRunId = detail.value.pathRunId
  const version = requestVersion
  acting.value = true
  actionText.value = ''
  errorText.value = ''
  try {
    const result = await approveRun(runId, command, detail.value?.currentStepNo ?? 0, detail.value?.controlVersion ?? 0, pathRunId)
    if (!isCurrent(version, pathRunId)) return
    if (!result) {
      errorText.value = '放行后未收到有效的运行状态，请刷新页面查看'
      return
    }
    applyDetail(result, version)
  } catch (error) {
    if (isAbortError(error) || !isCurrent(version, pathRunId)) return
    errorText.value = error instanceof RunApiError ? error.message : '放行执行失败，请查看日志'
  } finally {
    if (isCurrent(version, pathRunId)) {
      acting.value = false
      schedulePoll(version)
    }
  }
}

// stopRunAction 停止路径运行。
async function stopRunAction(): Promise<void> {
  if (acting.value) return
  const pathRunId = detail.value?.pathRunId || selectedPathRunID.value
  const version = requestVersion
  acting.value = true
  actionText.value = ''
  errorText.value = ''
  try {
    const result = await stopRun(runId, pathRunId)
    if (!isCurrent(version, pathRunId)) return
    applyDetail(result, version)
  } catch (error) {
    if (isAbortError(error) || !isCurrent(version, pathRunId)) return
    errorText.value = error instanceof RunApiError ? error.message : '停止失败，请重试'
  } finally {
    if (isCurrent(version, pathRunId)) {
      acting.value = false
      schedulePoll(version)
    }
  }
}

// retryFailed 失败动作重试（F-028）：点击后同一运行从失败步骤重新装填，按原运行模式继续。
// 重试本身不发任何写请求；人工控制模式下装填后仍需按「放行」才会重新执行失败步骤。
// 装填成功后立即恢复轮询：路径运行已从失败终态回到运行中。
async function retryFailed(): Promise<void> {
  if (!detail.value || acting.value) return
  const pathRunId = detail.value.pathRunId
  const version = requestVersion
  acting.value = true
  errorText.value = ''
  actionText.value = ''
  try {
    const next = await retryFailedAction(runId, pathRunId)
    if (!isCurrent(version, pathRunId)) return
    applyDetail(next, version)
    actionText.value = next.modeName === '人工控制'
      ? '已重新装填失败步骤；确认无误后按「放行」重新执行这一步。'
      : '已重新装填失败步骤；运行按原模式继续。'
    await pollEvents(version)
    schedulePoll(version)
  } catch (error) {
    if (isAbortError(error) || !isCurrent(version, pathRunId)) return
    errorText.value = error instanceof RunApiError ? error.message : '重试执行失败，请查看日志'
  } finally {
    if (isCurrent(version, pathRunId)) {
      acting.value = false
    }
  }
}

// handleSelectRunNode 打开右侧检视面板。面板会挤掉画布宽度，首次打开时把这个节点重新
// 平移到画布中央，否则用户刚点的节点会被面板推出视野。
function handleSelectRunNode(nodeID: string): void {
  const firstOpen = selectedNodeKey.value === ''
  selectedNodeKey.value = nodeID
  if (firstOpen) {
    void nextTick().then(() => focusNodeQuietly(nodeID)).catch(() => {})
  }
}

// closeNodePanel 关闭检视面板；画布恢复整宽后如果仍在自动跟随，就把当前步重新居中。
function closeNodePanel(): void {
  selectedNodeKey.value = ''
}

// modeHint 用中文解释当前模式意味着什么，避免只给一个模式名。
const modeHint = computed(() => {
  switch (detail.value?.modeName) {
    case '单步': return '每一步执行前都停下等放行（旧运行遗留，新运行已不提供）'
    case '自动': return '放行后连续执行，命中断点或异常时才停下'
    case '人工控制': return '每个动作组执行完停下，由我确认后再继续'
    default: return ''
  }
})

// 模式切换（2026-09-06）：自动/人工控制双向互切，只影响当前路径；
// 请求带控制版本（重复点击只产生一次控制事实），生效边界由后端判定并如实反馈。
const switchingMode = ref(false)
const modeFeedback = ref('')
// canSwitchMode 只在自动与人工控制之间提供切换；终态不显示切换入口。
const canSwitchMode = computed(() => {
  if (!detail.value || overviewDone.value) return false
  return ['自动', '人工控制'].includes(detail.value.modeName)
})
// targetModeName 是切换按钮的目标模式：当前自动就切人工控制，反之亦然。
const targetModeName = computed(() => (detail.value?.modeName === '自动' ? '人工控制' : '自动'))

// applyModeFeedback 严格区分后端返回：已收到并将在本步后生效 / 已经生效 / 失败。
function applyModeFeedback(next: PathRunDetail): void {
  if (next.modeSwitchPending && next.pendingModeName) {
    modeFeedback.value = `已收到切换请求：将在本步完成后切换为${next.pendingModeName}模式`
  } else {
    modeFeedback.value = `已切换为${next.modeName}模式`
  }
}

// toggleMode 提交模式切换；以服务端返回的最新详情为准刷新界面。
async function toggleMode(): Promise<void> {
  if (switchingMode.value || !detail.value) return
  switchingMode.value = true
  modeFeedback.value = ''
  errorText.value = ''
  try {
    const target = detail.value.modeName === '自动' ? 'manual_control' : 'auto'
    const next = await switchRunMode(runId, target, detail.value.controlVersion, detail.value?.pathRunId)
    detail.value = next
    syncControl(next)
    applyModeFeedback(next)
    lastUpdateAt.value = Date.now()
  } catch (error) {
    modeFeedback.value = ''
    errorText.value = error instanceof RunApiError ? error.message : '模式切换失败，请重试'
  } finally {
    switchingMode.value = false
  }
}

// statusTagType 让状态标签的颜色与语义一致；颜色之外始终有中文文字，不靠颜色单独表意。
const statusTagType = computed<'default' | 'info' | 'success' | 'warning' | 'error'>(() => {
  switch (detail.value?.pathRunStatusName) {
    case '已完成': return 'success'
    case '失败': return 'error'
    case '结果待确认': return 'warning'
    case '运行中':
    case '确认结果中': return 'info'
    default: return 'default'
  }
})

// commandButtonText 把命令写成"点下去会发生什么"，写请求类命令明确标出。
function commandButtonText(command: { command: string, label: string }): string {
  if (command.command === 'step') return '执行这一步（会向目标平台发一次真实写请求）'
  if (command.command === 'next_node') return '执行到下一节点'
  if (command.command === 'continue') return '继续执行后续步骤（到断点或路径结束）'
  return command.label.split('（')[0]
}

// noCommandReason 解释为什么现在没有任何可用命令：状态本身就是结论，不留空白。
const noCommandReason = computed(() => {
  if (!detail.value) return ''
  if (detail.value.loopRunning) return '正在连续执行，命令在停下后可用'
  if (detail.value.sceneLost) return '本次运行已停止；请查看步骤错误后从计划重新发起运行'
  return '当前状态下没有可用命令，请查看上方停止原因'
})

// overviewDone 表示整图进入结果总览（路径运行终态）。
const overviewDone = computed(() => {
  const status = detail.value?.pathRunStatusName || ''
  return ['已完成', '失败', '结果待确认', '已停止', '已取消'].includes(status)
})

// instanceMetaText 用一句话说清目标实例身份与日志目录：名称不可用时如实说明，
// 目录按页面身份（runId/pathRunId）定位，用户不需要遍历全盘或按时间猜目录。
const instanceMetaText = computed(() => {
  if (!detail.value) return ''
  const parts: string[] = []
  if (detail.value.instanceNameAvailable && detail.value.instanceName) {
    parts.push(`实例名称 ${detail.value.instanceName}`)
  } else {
    parts.push('实例名称不可用')
  }
  if (detail.value.instanceId) parts.push(`实例编号 ${detail.value.instanceId}`)
  if (detail.value.logDir) parts.push(`日志 ${detail.value.logDir}`)
  return parts.join(' · ')
})

// instanceMetaTitle 悬浮说明名称不可用的真实原因与完整日志目录。
const instanceMetaTitle = computed(() => {
  if (!detail.value) return ''
  const parts: string[] = []
  if (detail.value.instanceNameNote) parts.push(detail.value.instanceNameNote)
  if (detail.value.instanceId) parts.push(`实例编号：${detail.value.instanceId}`)
  if (detail.value.logDir) parts.push(`日志目录：${detail.value.logDir}`)
  return parts.join('；')
})

// topConclusion 用用户视角表述终态：先说结果，再说走了多少步、走到哪；
// 不再暴露目标内部的节点 ID 与待办计数（对用户是噪音）。
const topConclusion = computed(() => {
  if (!detail.value) return ''
  if (!overviewDone.value) return ''
  const parts: string[] = []
  if (detail.value.resultName) {
    parts.push(`路径结果：${detail.value.resultName}`)
  }
  // 进度一句话：成功时强调全部走完；非成功时说清停在哪一步，方便用户定位问题。
  // 详情接口没有冻结总步骤数，只用已落账步骤数表述，不虚构分母。
  const done = detail.value.steps?.length ?? 0
  if (detail.value.resultName === '成功') {
    if (done > 0) parts.push(`全部 ${done} 个步骤执行完成`)
  } else if (done > 0) {
    parts.push(`执行到第 ${done} 步后停止`)
  }
  // 最终目标事实只在「有实际意义」时说人话：实例没结束、还有待办时才提醒去向；
  // 实例已结束且无待办时什么都不用补——「成功」已经说明了一切。
  const finalTarget = detail.value.finalTarget as { statusName?: string; currentNodeNames?: string[]; dueNodeNames?: string[] } | undefined
  const due = finalTarget?.dueNodeNames ?? []
  if (finalTarget?.statusName && (due.length > 0 || (finalTarget.statusName !== '已结束' && finalTarget.statusName !== '草稿'))) {
    if (due.length > 0) {
      parts.push(`目标侧还有 ${due.length} 个待办${finalTarget.currentNodeNames?.length ? `（在「${finalTarget.currentNodeNames.join('、')}」）` : ''}`)
    } else {
      parts.push(`目标实例${finalTarget.statusName}`)
    }
  }
  // 没有任何可说的结论时不占位：空结论行对用户没有信息量。
  return parts.join('；')
})

// nowTick 是本地秒级时钟：新鲜度读数按它推进，不跟轮询节奏跳（纲领 12.2「动画由本地时钟驱动」）。
const nowTick = ref(Date.now())
let tickTimer: number | null = null

// freshnessStale 表示超过后端给的预算仍没有新事实：这时必须明说疑似无响应，不让用户自己猜。
const freshnessStale = computed(() => {
  const budget = detail.value?.staleAfterMs ?? 0
  return budget > 0 && nowTick.value - lastUpdateAt.value > budget
})

// freshnessText 用中文说明这屏事实有多新。
const freshnessText = computed(() => {
  const elapsed = nowTick.value - lastUpdateAt.value
  if (freshnessStale.value) return `已 ${formatElapsed(elapsed)}没有新的状态更新，疑似目标没有响应`
  if (elapsed < 3000) return '刚刚更新'
  return `${formatElapsed(elapsed)}前更新`
})

// 从事件流切回流程图时补一次定位：画布隐藏期间不跟随，切回来必须对得上当前步。
watch(activeTab, (tab) => {
  if (tab === 'canvas' && currentNodeKey.value) {
    void nextTick().then(() => focusNodeQuietly(currentNodeKey.value)).catch(() => {})
  }
})

onMounted(() => {
  disposed = false
  void loadDetail().catch(() => {})
  tickTimer = window.setInterval(() => { nowTick.value = Date.now() }, 1000)
})

onBeforeUnmount(() => {
  disposed = true
  closeNodePanel()
  if (pollTimer !== null) window.clearTimeout(pollTimer)
  if (tickTimer !== null) window.clearInterval(tickTimer)
  for (const controller of inflightControllers) {
    controller.abort()
  }
  inflightControllers.clear()
})
</script>

<template>
  <section
    class="run-detail"
    :style="{
      '--run-surface-color': themeVars.cardColor,
      '--run-border-color': themeVars.dividerColor,
      '--run-secondary-text-color': themeVars.textColor3,
      '--run-primary-color': themeVars.primaryColor,
      '--success-color': themeVars.successColor,
      '--warning-color': themeVars.warningColor,
      '--error-color': themeVars.errorColor,
      '--info-color': themeVars.infoColor,
    }"
  >
    <!-- 返回入口与本次运行的身份挂到应用顶栏：页面内不再重复一条页头，横向空间全部留给操作区。 -->
    <Teleport v-if="detail" to="#app-header-context">
      <div class="run-detail__identity" :style="headerVars">
        <n-button quaternary circle size="small" aria-label="返回本次运行的执行路径" title="返回本次运行的执行路径" @click="router.push(`/runs/${runId}`)">←</n-button>
        <h2 class="run-detail__title">运行 #{{ detail.runNo }}</h2>
        <span class="run-detail__meta-path" :title="`${detail.planName} / ${detail.pathName} · ${formatTime(detail.startedAt)}`">{{ detail.planName }} / {{ detail.pathName }} · {{ formatTime(detail.startedAt) }}</span>
        <!-- 目标实例身份与日志位置：实例名称只作业务信息；名称不可用时如实说明，不用其他名字冒充。 -->
        <span
          v-if="detail.instanceId || detail.logDir"
          class="run-detail__meta-path"
          :title="instanceMetaTitle"
        >{{ instanceMetaText }}</span>
        <n-tag size="small" :bordered="false" type="info" :title="modeHint">{{ detail.modeName }}模式</n-tag>
        <n-tag size="small" :bordered="false" :type="statusTagType">{{ detail.pathRunStatusName }}</n-tag>
        <n-tag v-if="detail.failureClassName" size="small" :bordered="false" type="error">{{ detail.failureClassName }}</n-tag>
      </div>
    </Teleport>

    <div v-if="loading" class="run-detail__loading"><n-spin size="small" /><span>正在读取任务详情……</span></div>
    <div v-else-if="!detail" class="run-detail__result">
      <n-result
        :status="detailNotFound ? '404' : 'error'"
        size="small"
        :title="detailNotFound ? '未找到任务' : '任务详情读取失败'"
        :description="loadErrorText || '未找到该任务。'"
        role="alert"
      />
      <div class="run-detail__result-actions">
        <n-button v-if="runId && (!loadFailure || loadFailure.retryable)" type="primary" secondary @click="loadDetail">重新读取</n-button>
      </div>
    </div>

    <template v-else>
      <!-- 主区第一行：左上角是内容页签，右上角是操作区，主操作「放行」固定在最右。 -->
      <div class="run-detail__bar">
        <div class="run-detail__tabs" role="tablist" aria-label="运行内容">
          <button
            type="button"
            class="run-detail__tab"
            :class="{ 'run-detail__tab--active': activeTab === 'canvas' }"
            role="tab"
            :aria-selected="activeTab === 'canvas'"
            @click="activeTab = 'canvas'"
          >流程图</button>
          <button
            type="button"
            class="run-detail__tab"
            :class="{ 'run-detail__tab--active': activeTab === 'events' }"
            role="tab"
            :aria-selected="activeTab === 'events'"
            @click="activeTab = 'events'"
          >事件流（{{ runEvents.length }}）</button>
        </div>

        <div class="run-detail__actions">
          <span
            v-if="!overviewDone"
            class="run-detail__freshness"
            :class="{ 'run-detail__freshness--stale': freshnessStale }"
            role="status"
          >{{ freshnessText }}</span>

          <!-- 模式切换：只在自动/单步之间互切；反馈严格区分「已收到」「将在本步后生效」「已生效」。 -->
          <template v-if="canSwitchMode">
            <n-button
              size="small"
              :loading="switchingMode"
              :title="`切换只影响当前路径，并在安全边界（本步完成结果确认与记录）后生效`"
              @click="toggleMode"
            >切换为{{ targetModeName }}</n-button>
            <span v-if="modeFeedback" class="run-detail__mode-feedback" role="status">{{ modeFeedback }}</span>
            <span
              v-else-if="detail.modeSwitchPending && detail.pendingModeName"
              class="run-detail__mode-feedback"
              role="status"
            >将在本步完成后切换为{{ detail.pendingModeName }}模式</span>
          </template>

          <!-- 断点是调试能力而不是常看的信息：收进弹出层，画布不再被一整块说明占掉。 -->
          <n-popover trigger="click" placement="bottom-end" :width="342" :keep-alive-on-hover="false">
            <template #trigger>
              <n-button size="small" quaternary title="查看与管理本次运行的断点">断点（{{ detail.breakpoints?.length ?? 0 }}）</n-button>
            </template>
            <div class="run-detail__breakpoints">
              <p class="run-detail__bp-hint">断点只对本次运行生效，命中都在放行之前判定。</p>

              <div class="run-detail__bp-group">
                <span class="run-detail__bp-group-title">强制生效</span>
                <ul>
                  <li v-for="bp in forcedBreakpoints" :key="bp.type">
                    <n-tag size="tiny" :bordered="false" type="warning">{{ bp.typeName }}</n-tag>
                    <span class="run-detail__bp-note">{{ forcedBreakpointNote(bp.type) }}</span>
                  </li>
                  <li v-if="forcedBreakpoints.length === 0" class="run-detail__empty">当前没有强制断点</li>
                </ul>
              </div>

              <div class="run-detail__bp-group">
                <span class="run-detail__bp-group-title">已挂载（{{ userBreakpoints.length }}）</span>
                <ul>
                  <li v-for="(bp, index) in userBreakpoints" :key="`${bp.type}-${index}`">
                    <n-tag size="tiny" :bordered="false">{{ bp.typeName }}</n-tag>
                    <span>{{ breakpointTargetText(bp) }}</span>
                    <n-button text size="tiny" type="error" :title="`删除${bp.typeName}`" @click="deleteBreakpoint(bp)">删除</n-button>
                  </li>
                  <li v-if="userBreakpoints.length === 0" class="run-detail__empty">还没有手工挂载的断点</li>
                </ul>
              </div>

              <div class="run-detail__bp-add-form">
                <span class="run-detail__bp-group-title">新增断点</span>
                <n-select
                  v-model:value="newBreakpointType"
                  size="small"
                  :options="breakpointTypeOptions"
                  aria-label="选择断点类型"
                />
                <n-button
                  v-if="newBreakpointType === 'node'"
                  size="small"
                  :disabled="!selectedNodeKey || overviewDone"
                  :title="selectedNodeKey ? '在画布上选中的节点挂节点断点' : '先在画布上点选一个节点'"
                  @click="addNodeBreakpoint"
                >{{ selectedNodeName ? `挂到「${selectedNodeName}」` : '挂到选中节点' }}</n-button>
                <template v-else-if="newBreakpointType === 'step'">
                  <n-input-number v-model:value="newBreakpointStep" size="small" :min="1" placeholder="步骤序号" aria-label="步骤序号" />
                  <n-button size="small" :disabled="!newBreakpointStep || overviewDone" @click="addStepBreakpoint">挂到该步骤</n-button>
                </template>
                <template v-else>
                  <n-select
                    v-model:value="newBreakpointAction"
                    size="small"
                    :options="actionBreakpointOptions"
                    placeholder="选择动作"
                    aria-label="动作类型"
                  />
                  <n-button size="small" :disabled="!newBreakpointAction || overviewDone" @click="addActionBreakpoint">挂到该动作</n-button>
                </template>
                <small class="run-detail__bp-note">{{ breakpointTypeHint }}</small>
              </div>
            </div>
          </n-popover>

          <!-- 次级命令与不可逆操作在前，主操作「放行」最右；下一步会发真实写请求，按钮上写清这一点。 -->
          <n-button
            v-for="command in (detail.commands || []).filter(c => c.command !== 'step')"
            :key="command.command"
            size="small"
            :disabled="acting || looping || detail.stepInFlight || detail.loopRunning || overviewDone"
            :title="command.label"
            @click="runCommand(command.command)"
          >{{ commandButtonText(command) }}</n-button>
          <n-button
            v-if="detail.loopRunning"
            size="small"
            :disabled="pausing || detail.pauseRequested"
            :title="detail.pauseRequested ? '暂停请求已提交，本步完成结果确认与记录后生效' : '暂停请求只在本步完成结果确认与记录后生效，不会打断已发出的写请求'"
            @click="pauseNow"
          >{{ detail.pauseRequested ? '暂停已请求' : '暂停' }}</n-button>
          <n-popconfirm :disabled="acting || overviewDone" @positive-click="stopRunAction">
            <template #trigger>
              <n-button size="small" type="error" ghost :disabled="acting || overviewDone">停止运行</n-button>
            </template>
            停止是终态，之后这条路径不能再前进；已发出的写请求不会被打断，已发生的事实全部保留。确定停止？
          </n-popconfirm>
          <!-- F-028 失败动作重试：只在服务端判定可重试（确定失败且失败步骤已落账）时出现。
               重试本身不发任何写请求，只把运行从失败步骤重新装填；人工控制模式装填后仍需按「放行」。 -->
          <n-button
            v-if="detail.retryable"
            size="small"
            type="primary"
            ghost
            :loading="acting"
            title="从失败的这一步重新装填本次运行；重试本身不发任何写请求，装填后按原运行模式继续"
            @click="retryFailed"
          >重试失败动作</n-button>
          <n-button
            size="small"
            type="primary"
            :loading="acting"
            :disabled="overviewDone || detail.stepInFlight || detail.loopRunning || !(detail.commands || []).length"
            :title="noCommandReason || (approveCommand === 'continue' ? '放行后连续执行到断点或结束' : '放行后执行下一步；下一步会发真实写请求')"
            @click="approve()"
          >{{ approveCommand === 'continue' ? '继续执行后续步骤' : '执行这一步' }}</n-button>
        </div>
      </div>

      <!-- F-030：当前步实时阶段说明——回答"现在在做什么、为什么要做、下一步是什么"。
           说明由后端统一口径提供（currentPhaseNote），前端缺失时显示不可用，不回退泛化词。 -->
      <div v-if="!overviewDone && (detail.currentPhaseNote || detail.currentPhase)" class="run-detail__phase-bar" data-testid="current-phase-note">
        <span class="run-detail__phase-dot"></span>
        <span class="run-detail__phase-note">{{ detail.currentPhaseNote || '当前阶段说明暂时不可用，可从日志入口查看执行过程' }}</span>
      </div>

      <!-- F-020 多路径运行：调度说明与路径切换 chips 合并为一行。 -->
      <div v-if="(detail.paths?.length ?? 0) > 1" class="run-detail__subbar">
        <span class="run-detail__schedule">{{ detail.runScheduleName }} · {{ detail.runConcurrencyLabel }}</span>
        <div class="run-detail__paths" role="tablist" aria-label="路径运行列表">
          <button
            v-for="path in detail.paths"
            :key="path.pathRunId"
            type="button"
            class="run-detail__path"
            :class="{ 'run-detail__path--active': path.pathRunId === (detail.pathRunId || selectedPathRunID) }"
            role="tab"
            :aria-selected="path.pathRunId === (detail.pathRunId || selectedPathRunID)"
            @click="switchPathRun(path.pathRunId)"
          >
            <span class="run-detail__path-name">{{ path.pathName }}</span>
            <span class="run-detail__path-status">{{ path.statusName }}<template v-if="path.resultName"> · {{ path.resultName }}</template></span>
          </button>
        </div>
      </div>

      <!-- 结论与提示区：只在真的有内容时占位，高度有界，不把画布挤没。 -->
      <div
        v-if="detail.stopReason || topConclusion || loadErrorText || errorText || actionText || sceneLostNote || detail.graphError"
        class="run-detail__notices"
      >
        <!-- 现场已丢失：一句大白话说明发生了什么、为什么、用户现在能做什么；不给任何输入或重试入口。 -->
        <n-alert v-if="sceneLostNote" type="warning" :show-icon="false" class="run-detail__notice-bar">
          {{ sceneLostNote }}
        </n-alert>
        <n-alert v-if="detail.stopReason" type="warning" :show-icon="false" class="run-detail__notice-bar">
          {{ detail.stopReason }}
        </n-alert>
        <p v-if="topConclusion" class="run-detail__conclusion" role="status">{{ topConclusion }}</p>
        <p v-if="loadErrorText" class="run-detail__error" role="alert">{{ loadErrorText }}</p>
        <p v-if="errorText" class="run-detail__error" role="alert">{{ errorText }}</p>
        <!-- 结构读取失败：原样展示后端透传的底层错误，不包装不改写，让用户看到真实原因。 -->
        <p v-if="detail.graphError" class="run-detail__error" role="alert">{{ detail.graphError }}</p>
        <p v-if="actionText" class="run-detail__notice" role="status">{{ actionText }}</p>
      </div>

      <!-- 主体：页签下方整块给流程图；右侧检视面板只在点开节点后出现。 -->
      <div class="run-detail__body">
        <div v-show="activeTab === 'canvas'" class="run-detail__canvas">
          <flow-graph-canvas
            v-if="graph"
            ref="canvasRef"
            class="run-detail__canvas-view"
            :graph="graph"
            :choices="pathChoices"
            run-mode
            :run-node-states="runNodeStates"
            :current-run-node-key="currentNodeKey"
            :run-busy="runBusy"
            :run-taken-edge-ids="runTakenEdgeIds"
            :run-deviation-edge-ids="runDeviationEdgeIds"
            :run-selected-node-key="selectedNodeKey"
            :run-error-notes="runErrorNotes"
            :run-step-notes="runStepNotes"
            @select-run-node="handleSelectRunNode"
            @run-viewport-change="handleRunViewportChange"
          >
            <!-- 常驻定位入口，浮在画布底部居中：自动跟随被用户接管后变成「回到当前步」（纲领 12.2）。 -->
            <template #canvas-floating>
              <n-button
                v-if="currentNodeKey"
                class="run-detail__follow"
                size="small"
                :type="followPaused ? 'info' : 'default'"
                :secondary="!followPaused"
                :title="followPaused ? '点这里把当前执行的节点移到画布中央' : '把当前执行的节点移到画布中央'"
                @click="resumeFollow"
              >{{ followPaused ? '回到当前步' : '定位当前节点' }}</n-button>
            </template>
          </flow-graph-canvas>
          <div v-else class="run-detail__result">
            <n-result status="warning" size="small" :show-icon="false" title="流程图暂不可用" description="真实流程结构尚未加载，无法渲染运行画布。" role="status" aria-live="polite" />
            <div class="run-detail__result-actions">
              <n-button secondary @click="loadDetail">重新读取</n-button>
            </div>
          </div>
        </div>

        <!-- F-021 事件流时间线：按数据库顺序只追加，供事后回放「状态怎么变的」。 -->
        <section v-show="activeTab === 'events'" class="run-detail__events" aria-label="事件流">
          <ol class="run-detail__event-list">
            <li v-for="event in runEvents" :key="event.id">
              <span class="run-detail__event-time">{{ event.createdAt }}</span>
              <span>{{ event.label }}</span>
            </li>
            <li v-if="runEvents.length === 0" class="run-detail__event-empty">还没有事件；放行后这里会按顺序追加。</li>
          </ol>
        </section>

        <run-node-panel
          v-if="selectedNodeKey && detail"
          :key="`${detail.pathRunId}-${selectedNodeKey}`"
          class="run-detail__side"
          :detail="detail"
          :node-key="selectedNodeKey"
          :node-name="selectedNodeName"
          :node-type-name="selectedNodeTypeName"
          @close="closeNodePanel"
          @retry="retryFailed"
        />
      </div>
    </template>
  </section>
</template>

<style scoped>
/* 运行详情是画布优先的工作台：整页不滚动，纵向分成「操作行 / 提示区 / 主体」三段，
   主体把剩下的高度全部交给流程图，右侧检视面板只在点开节点后出现。 */
.run-detail {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  min-height: 0;
  /* 水平留白与内容区统一：顶栏返回入口、标题与下方内容左边界对齐（styles.css 同一组变量）。 */
  padding: 10px var(--app-content-pad-x) 14px;
  /* 正常情况整页不滚动；只有现场丢失那种高提示区把空间挤满时才允许整页滚动，
     绝不让画布被压成零高度。 */
  overflow-y: auto;
}

/* 顶栏上下文（Teleport 到应用顶栏）：返回入口 + 运行编号 + 计划/路径 + 模式与状态。 */
.run-detail__identity {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.run-detail__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
}

.run-detail__meta-path {
  overflow: hidden;
  max-width: 34vw;
  color: var(--run-secondary-text-color, #909090);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.run-detail__loading {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 20px 0;
}

/* 操作行：页签在左上角，操作在右上角，共用一条下边线。 */
.run-detail__bar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex: 0 0 auto;
  gap: 16px;
  flex-wrap: wrap;
  border-bottom: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
}

.run-detail__tabs {
  display: flex;
  gap: 4px;
}

.run-detail__tab {
  padding: 8px 14px;
  color: var(--run-secondary-text-color, #909090);
  font: inherit;
  cursor: pointer;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.run-detail__tab--active {
  color: var(--run-primary-color, #18a058);
  font-weight: 500;
  border-bottom-color: var(--run-primary-color, #18a058);
}

.run-detail__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  padding-bottom: 6px;
}

.run-detail__freshness {
  margin-right: 2px;
  color: var(--run-secondary-text-color, #909090);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.run-detail__freshness--stale {
  color: var(--warning-color, #f0a020);
}

.run-detail__mode-feedback {
  color: var(--run-primary-color, #18a058);
  font-size: 12px;
}

/* 断点弹出层：强制生效与手工挂载分区，新增断点三种挂载方式共用一处。 */
.run-detail__breakpoints {
  display: grid;
  gap: 8px;
  font-size: 13px;
}

.run-detail__bp-hint,
.run-detail__bp-note {
  margin: 0;
  color: var(--run-secondary-text-color, #909090);
  font-size: 12px;
  line-height: 1.5;
}

.run-detail__bp-group {
  display: grid;
  gap: 4px;
  padding-top: 8px;
  border-top: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
}

.run-detail__bp-group-title {
  font-size: 12px;
  font-weight: 600;
}

.run-detail__bp-group ul {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.run-detail__bp-group li {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.run-detail__bp-add-form {
  display: grid;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
}

.run-detail__empty {
  color: var(--run-secondary-text-color, #909090);
}

/* 次级行：调度说明 + 多路径切换 chips。 */
.run-detail__subbar {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 14px;
  flex-wrap: wrap;
}

.run-detail__schedule {
  color: var(--run-secondary-text-color, #909090);
  font-size: 13px;
}

.run-detail__paths {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.run-detail__path {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 5px 10px;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
  border-radius: 4px;
}

.run-detail__path--active {
  color: var(--run-primary-color, #18a058);
  border-color: var(--run-primary-color, #18a058);
}

.run-detail__path-name { font-weight: 500; }
.run-detail__path-status { font-size: 12px; opacity: 0.8; }

/* 提示区：停止原因、结果结论、报错与现场丢失说明，高度有界，不把画布挤没。 */
.run-detail__notices {
  display: grid;
  flex: 0 0 auto;
  gap: 8px;
  max-height: 40vh;
  overflow-y: auto;
}

.run-detail__conclusion {
  margin: 0;
  padding: 7px 12px;
  background: color-mix(in srgb, var(--success-color, #18a058) 10%, transparent);
  border-radius: 6px;
}

.run-detail__error { margin: 0; color: var(--error-color, #d03050); }
.run-detail__notice { margin: 0; color: var(--run-secondary-text-color, #909090); }

/* 主体：画布占满剩余空间，右侧检视面板按需出现（不出现时不占列）。 */
.run-detail__body {
  display: grid;
  flex: 1 1 auto;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  min-height: 280px;
}

.run-detail__canvas {
  position: relative;
  min-width: 0;
  height: 100%;
}

.run-detail__canvas-view {
  height: 100%;
  min-height: 0;
  border: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
  border-radius: 8px;
}

.run-detail__follow {
  box-shadow: 0 2px 10px rgb(0 0 0 / 12%);
}

.run-detail__side {
  width: 360px;
  height: 100%;
  min-height: 0;
}

.run-detail__events {
  height: 100%;
  padding: 10px 14px;
  overflow-y: auto;
  border: 1px solid var(--run-border-color, rgba(128, 128, 128, 0.35));
  border-radius: 8px;
}

.run-detail__event-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  font-size: 13px;
  list-style: none;
}

.run-detail__event-time {
  margin-right: 8px;
  color: var(--run-secondary-text-color, #909090);
  font-variant-numeric: tabular-nums;
}

.run-detail__event-empty {
  color: var(--run-secondary-text-color, #909090);
}

@media (max-width: 1320px) {
  .run-detail__side { width: 320px; }
}
.run-detail__result {
  text-align: center;
}

.run-detail__result-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
}
</style>
<style scoped>
/* F-030 当前步实时阶段说明条：圆点呼吸 + 一句大白话，回答正在做什么。 */
.run-detail__phase-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0 2px;
  font-size: 13px;
  line-height: 1.5;
}

.run-detail__phase-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--info-color, #2080f0);
  animation: run-detail-phase-pulse 1.6s ease-in-out infinite;
}

@keyframes run-detail-phase-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

.run-detail__phase-note {
  color: inherit;
}
</style>
