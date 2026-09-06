<script setup lang="ts">
import { NButton, NEmpty, NForm, NFormItem, NInput, NInputNumber, NPopconfirm, NSelect, NSpin, NTag, useThemeVars } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import type { FlowGraph } from '../features/flow-graph/types'
import {
  approveRun,
  fetchRunDetail,
  formatElapsed,
  recoveryAction,
  reconcileNow,
  removeBreakpoint,
  requestPause,
  RunApiError,
  setBreakpoint,
  stopRun,
} from '../features/runs/api'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { BreakpointInput, PathRunDetail, ReconcileView } from '../features/runs/api'
import { fetchRunEvents, type RunEventItem } from '../features/runs/api'
import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { pathConfigNodeKey } from '../features/path-configuration/logic'
import RunNodePanel from '../features/runs/RunNodePanel.vue'

// RunDetailView 是路径运行详情：运行画布为主体，顶部固定条控制放行与停止。
// 放行会发出真实写请求：只接受明确点击，不绑定单键快捷键。
const route = useRoute()
const router = useRouter()
const runId = String(route.params.runId || '')
// 事件流（F-021）：按数据库自增键增量追加，轮询只取新事件，历史不重排。
const runEvents = ref<RunEventItem[]>([])
let lastEventID = 0
// 主区页签：流程图（默认）/ 事件流。
const activeTab = ref<'canvas' | 'events'>('canvas')
// selectedPathRunID 是多路径运行里当前查看的路径运行（路由查询 ?path=）；缺省由后端取第一条。
const selectedPathRunID = ref<number>(Number(route.query.path || 0) || 0)

// switchPathRun 切换查看的路径运行：写回路由让刷新与分享保留选择，随后重读详情。
function switchPathRun(pathRunID: number) {
  if (pathRunID === selectedPathRunID.value) return
  selectedPathRunID.value = pathRunID
  reconcileView.value = null
  // 事件流按路径过滤：切换后清空并重置游标，重新只追加当前路径的事件。
  runEvents.value = []
  lastEventID = 0
  activeTab.value = 'canvas'
  void router.replace({ query: { ...route.query, path: pathRunID ? String(pathRunID) : undefined } })
  void loadDetail()
}
const themeVars = useThemeVars()

const detail = ref<PathRunDetail | null>(null)
const graph = ref<FlowGraph | null>(null)
const loading = ref(false)
const errorText = ref('')
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

// 待对账工作区（F-018）：对账结论与唯一合法动作。
const reconciling = ref(false)
const reconcileView = ref<ReconcileView | null>(null)
const manualForm = ref({ instanceStatus: '', currentNode: '', note: '', reporter: '' })

// partialEffectWarned 只在对账依据里真的出现「部分生效」时才提示"表单数据可能已经写进去了"。
// 这句话是语义清单第 2.4 节那个特定形状的结论，对普通的证据不完整并不成立——
// 不加区分地一直显示会把没有依据的判断说成事实。
const partialEffectWarned = computed(
  () => (reconcileView.value?.reasons ?? []).some((reason) => reason.includes('部分生效')),
)

async function doReconcile(): Promise<void> {
  if (reconciling.value) return
  reconciling.value = true
  errorText.value = ''
  try {
    reconcileView.value = await reconcileNow(runId, detail.value?.pathRunId)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '对账失败，请重试'
  } finally {
    reconciling.value = false
  }
}

// runRecovery 执行对账给出的唯一动作；完成后以服务端重读结果刷新。
async function runRecovery(action: string): Promise<void> {
  if (reconciling.value) return
  reconciling.value = true
  try {
    const manual = action === 'manual_end' ? manualForm.value : undefined
    detail.value = await recoveryAction(runId, action, manual, detail.value?.pathRunId)
    reconcileView.value = null
    lastUpdateAt.value = Date.now()
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '恢复动作失败，请重试'
  } finally {
    reconciling.value = false
  }
}

// registerManual 登记人工核对结论（仍无法判定的唯一出路）。
async function registerManual(): Promise<void> {
  // 先过表单校验：三项必填缺一不可，校验不通过就地提示，不发请求。
  try {
    await manualFormRef.value?.validate()
  } catch {
    errorText.value = '请先补全人工核对结论里的必填项'
    return
  }
  await runRecovery('manual_end')
}

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

// pauseNow 提交暂停请求（本步走完核验与落账后生效）。
const pausing = ref(false)
// 待对账运行自动触发一次只读对账（安全）。

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
  if (bp.type === 'action') return bp.action ? `动作：${actionLabel(bp.action)}` : '动作：未指定'
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

// actionLabel 把动作键转成中文；未登记的键原样显示，不猜。
function actionLabel(action: string): string {
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

// 人工结论表单：三项必填（实例状态、当前节点、登记人），说明选填。
// 实例状态用目标的真实状态集合做选项，避免自由文本写出目标没有的状态。
const manualFormRef = ref<FormInst | null>(null)
const instanceStatusOptions = [
  { label: '运行中（run）', value: 'run' },
  { label: '已结束（end）', value: 'end' },
  { label: '已驳回（rejected）', value: 'rejected' },
  { label: '已撤回（withdraw）', value: 'withdraw' },
  { label: '已终止（termination）', value: 'termination' },
  { label: '已作废（abandon）', value: 'abandon' },
  { label: '草稿（draft）', value: 'draft' },
  { label: '待发（await_sent）', value: 'await_sent' },
  { label: '目标平台上看不到这条实例', value: 'not_visible' },
]
const manualRules: FormRules = {
  instanceStatus: [{ required: true, message: '请选择你在目标平台上看到的实例状态', trigger: ['change', 'blur'] }],
  currentNode: [{ required: true, message: '请填写目标平台上显示的当前节点', trigger: ['input', 'blur'] }],
  reporter: [{ required: true, message: '请填写登记人，人工结论要可追溯', trigger: ['input', 'blur'] }],
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

// currentNodeKey 是当前步所在节点（预览给出），画布据此高亮与居中。
// currentNodeKey 用图节点 ID（画布键空间）；旧后端没有 nodeId 时回退 nodeKey。
const currentNodeKey = computed(() => detail.value?.currentPreview?.nodeId || detail.value?.currentPreview?.nodeKey || '')

// runNodeStates 把九个中文运行态与当前步标记交给画布。
const runNodeStates = computed(() => {
  const states: Record<string, { status: string; statusName: string }> = {}
  for (const [nodeKey, state] of Object.entries(detail.value?.nodeStates || {})) {
    states[nodeKey] = { status: state.status, statusName: state.statusName }
  }
  return states
})

// isActing 表示一次放行或停止请求在途：此时放行/停止按钮进入忙碌态。
const isActing = computed(() => acting.value)

// loadDetail 拉取详情并刷新结构（结构只按计划取一次）。
async function loadDetail(): Promise<void> {
  if (!runId) {
    errorText.value = '运行标识缺失，无法打开详情。'
    return
  }
  loading.value = !detail.value
  try {
    const next = await fetchRunDetail(runId, undefined, selectedPathRunID.value)
    detail.value = next
    syncControl(next)
    lastUpdateAt.value = Date.now()
    void pollEvents()
    // 进入待对账后自动做一次只读对账（纲领第 4.4 节）：用户不需要先点一下才看到依据。
    // 只在还没有结论时触发一次；对账是只读的，服务重启后它会顺带按运行事实重建现场。
    if (next.pathRunStatusName === '待对账' && !reconcileView.value && !reconciling.value) {
      void doReconcile()
    }
    if (!graph.value) {
      graph.value = await fetchFlowGraph(String(next.planId), new AbortController().signal)
    }
    await nextTick()
    if (currentNodeKey.value && !followPaused.value) {
      centerCurrentNode()
    }
    schedulePoll()
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '暂时无法读取运行详情，请重试'
  } finally {
    loading.value = false
    // 轮询链不因首次加载失败而断：详情已在（或结构读失败但运行事实还在）时，
    // 后续推进与恢复仍按配置间隔刷新，用户不需要手动刷新页面（纲领 12.2）。
    if (detail.value) schedulePoll()
  }
}

// schedulePoll 按配置间隔轮询；路径运行进入终态后停止。
function schedulePoll(): void {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }
  if (!detail.value) return
  const terminalStatuses = ['已完成', '失败', '待对账', '已停止', '已取消']
  // 多路径运行：当前路径终态但还有未终态兄弟路径时继续轮询，切换区的状态不能停滞（评审 P2）。
  const siblingActive = (detail.value.paths ?? []).some((path) => !terminalStatuses.includes(path.statusName) && path.statusName !== '暂停')
  if (terminalStatuses.includes(detail.value.pathRunStatusName) && !siblingActive) return
  pollTimer = window.setTimeout(async () => {
    try {
      const next = await fetchRunDetail(runId, undefined, selectedPathRunID.value)
      detail.value = next
      syncControl(next)
      lastUpdateAt.value = Date.now()
      if (currentNodeKey.value && !followPaused.value) {
        centerCurrentNode()
      }
      void pollEvents()
    } catch {
      // 单次轮询失败不打断页面：下一次轮询会继续。
    }
    schedulePoll()
  }, Math.max(500, detail.value.pollIntervalMs || 2000))
}

// pollEvents 增量拉取事件流：游标为已取到的最大事件 ID，只追加不重排（F-021）。
async function pollEvents(): Promise<void> {
  try {
    const fresh = await fetchRunEvents(runId, lastEventID, selectedPathRunID.value || undefined)
    for (const event of fresh) {
      if (event.id > lastEventID) {
        runEvents.value.push(event)
        lastEventID = event.id
      }
    }
  } catch {
    // 事件流拉取失败不打断页面：下一次轮询会带游标重试，已有事件不丢失。
  }
}

// centerCurrentNode 把当前步节点平移到操作区中央。
function centerCurrentNode(): void {
  if (!currentNodeKey.value) return
  programmaticMove.value = true
  canvasRef.value?.focusNode(currentNodeKey.value)
  window.setTimeout(() => { programmaticMove.value = false }, 320)
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

// approve 放行当前步：等待响应期间指示器进入执行中，写请求不可中断。
async function approve(): Promise<void> {
  if (acting.value || !detail.value?.currentPreview) return
  acting.value = true
  actionText.value = ''
  errorText.value = ''
  try {
    detail.value = await approveRun(runId, 'step', detail.value?.currentStepNo ?? 0, detail.value?.controlVersion ?? 0, detail.value?.pathRunId)
    lastUpdateAt.value = Date.now()
    await nextTick()
    if (currentNodeKey.value && !followPaused.value) {
      centerCurrentNode()
    }
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '放行执行失败，请查看日志'
  } finally {
    acting.value = false
    schedulePoll()
  }
}

// stopRunAction 停止路径运行。
async function stopRunAction(): Promise<void> {
  if (acting.value) return
  acting.value = true
  actionText.value = ''
  errorText.value = ''
  try {
    detail.value = await stopRun(runId, detail.value?.pathRunId)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '停止失败，请重试'
  } finally {
    acting.value = false
    schedulePoll()
  }
}

// handleSelectRunNode 记录侧栏选中的节点。
function handleSelectRunNode(nodeID: string): void {
  selectedNodeKey.value = nodeID
}

// modeHint 用中文解释当前模式意味着什么，避免只给一个模式名。
const modeHint = computed(() => {
  switch (detail.value?.modeName) {
    case '单步': return '每一步执行前都停下等放行，只有「执行一步」一条命令'
    case '自动': return '连续执行，首个写步骤与断点命中处必停'
    case '人工控制': return '停在第一步之前，暂停时拥有全部三条命令'
    default: return '运行模式在启动时确定，运行中不可切换'
  }
})

// statusTagType 让状态标签的颜色与语义一致；颜色之外始终有中文文字，不靠颜色单独表意。
const statusTagType = computed<'default' | 'info' | 'success' | 'warning' | 'error'>(() => {
  switch (detail.value?.pathRunStatusName) {
    case '已完成': return 'success'
    case '失败': return 'error'
    case '待对账': return 'warning'
    case '运行中':
    case '核验中': return 'info'
    default: return 'default'
  }
})

// commandButtonText 把命令写成"点下去会发生什么"，写请求类命令明确标出。
function commandButtonText(command: { command: string, label: string }): string {
  if (command.command === 'step') return '放行（执行一步，会发真实写请求）'
  if (command.command === 'next_node') return '执行到下一节点'
  if (command.command === 'continue') return '继续运行（直到断点或结束）'
  return command.label.split('（')[0]
}

// noCommandReason 解释为什么现在没有任何可用命令：状态本身就是结论，不留空白。
const noCommandReason = computed(() => {
  if (!detail.value) return ''
  if (detail.value.loopRunning) return '正在连续执行，命令在停下后可用'
  if (detail.value.pathRunStatusName === '待对账') return '写结果不确定，先在下方待对账区完成对账，这里不提供重试或继续'
  return '当前状态下没有可用命令，请查看上方停止原因'
})

// overviewDone 表示整图进入结果总览（路径运行终态）。
const overviewDone = computed(() => {
  const status = detail.value?.pathRunStatusName || ''
  return ['已完成', '失败', '待对账', '已停止', '已取消'].includes(status)
})

// topConclusion 把路径结果与最终目标事实分开表述。
const topConclusion = computed(() => {
  if (!detail.value) return ''
  if (!overviewDone.value) return ''
  const parts: string[] = []
  parts.push(`路径结果：${detail.value.resultName || '—'}`)
  const finalTarget = detail.value.finalTarget as { statusName?: string; currentNodeNames?: string[]; dueNodeNames?: string[] } | undefined
  if (finalTarget) {
    const due = finalTarget.dueNodeNames || []
    parts.push(`最终目标事实：实例${finalTarget.statusName || '状态未知'}${finalTarget.currentNodeNames?.length ? `，当前节点 ${finalTarget.currentNodeNames.join('、')}` : ''}，待办 ${due.length} 个`)
  }
  return parts.join('；')
})

onMounted(() => {
  void loadDetail()
})

onBeforeUnmount(() => {
  if (pollTimer !== null) window.clearTimeout(pollTimer)
})
</script>

<template>
  <section class="run-detail" :style="{ '--run-surface-color': themeVars.cardColor, '--run-border-color': themeVars.dividerColor }">
    <header v-if="detail" class="run-detail__topbar">
      <div class="run-detail__topbar-left">
        <NButton quaternary circle size="small" aria-label="返回运行列表" @click="router.push('/runs')">←</NButton>
        <h2 class="run-detail__title">运行 #{{ detail.runNo }}</h2>
        <span class="run-detail__meta-path">{{ detail.planName }} / {{ detail.pathName }}</span>
        <NTag size="small" :bordered="false" type="info">{{ detail.modeName }}模式</NTag>
        <NTag size="small" :bordered="false" :type="statusTagType">{{ detail.pathRunStatusName }}</NTag>
        <NTag v-if="detail.failureClassName" size="small" :bordered="false" type="error">{{ detail.failureClassName }}</NTag>
      </div>
      <div class="run-detail__topbar-actions">
        <!-- 次级命令与不可逆操作在前，主操作「放行」最右；下一步会发真实写请求，按钮上写清这一点。 -->
        <NButton
          v-for="command in (detail.commands || []).filter(c => c.command !== 'step')"
          :key="command.command"
          size="small"
          :disabled="acting || looping || overviewDone"
          :title="command.label"
          @click="runCommand(command.command)"
        >{{ commandButtonText(command) }}</NButton>
        <NButton
          v-if="detail.loopRunning"
          size="small"
          :disabled="pausing || detail.pauseRequested"
          :title="detail.pauseRequested ? '暂停请求已提交，本步走完核验与落账后生效' : '暂停请求只在本步走完核验与落账后生效，不会打断已发出的写请求'"
          @click="pauseNow"
        >{{ detail.pauseRequested ? '暂停已请求' : '暂停' }}</NButton>
        <NPopconfirm :disabled="acting || overviewDone" @positive-click="stopRunAction">
          <template #trigger>
            <NButton size="small" type="error" ghost :disabled="acting || overviewDone">停止运行</NButton>
          </template>
          停止是终态，之后这条路径不能再前进；已发出的写请求不会被打断，已发生的事实全部保留。确定停止？
        </NPopconfirm>
        <NButton
          size="small"
          type="primary"
          :loading="acting"
          :disabled="overviewDone || !(detail.commands || []).some(c => c.command === 'step')"
          :title="noCommandReason || '放行后执行下一步；下一步会发真实写请求'"
          @click="approve()"
        >放行（执行下一步）</NButton>
      </div>
    </header>
    <!-- F-020 多路径运行：路径切换 chips + 调度说明合并为一行（参照参考图第二行）。 -->
    <div v-if="detail && (detail.paths?.length ?? 0) > 1" class="run-detail__subbar">
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
    <NAlert v-if="detail && (detail.stopReason || detail.structureNote)" type="warning" :show-icon="false" class="run-detail__notice-bar">
      {{ [detail.stopReason, detail.structureNote].filter(Boolean).join('；') }}
    </NAlert>
    <div
      v-if="detail && detail.pathRunStatusName === '待对账'"
      class="run-detail__reconcile"
      role="region"
      aria-label="待对账工作区"
    >
      <h4>待对账</h4>
      <p v-if="reconciling">正在只读对账……</p>
      <template v-else-if="reconcileView">
        <p class="run-detail__reconcile-verdict">对账结论：{{ reconcileView.verdictName }}</p>
        <p>{{ reconcileView.headline }}</p>
        <ul>
          <li v-for="(reason, index) in reconcileView.reasons" :key="index">{{ reason }}</li>
        </ul>
        <p v-if="reconcileView.action === 'replay'" class="run-detail__reconcile-note">
          唯一动作是重放这一步：它是一次新的尝试，会重新走门禁与七阶段；一次尝试仍然只发一次写请求。
          已用重放 {{ reconcileView.replaysUsed }} / {{ reconcileView.replaysMax }} 次。
        </p>
        <p v-else-if="reconcileView.replayExhausted" class="run-detail__reconcile-note">
          证据仍指向未生效，但重放次数已用完（{{ reconcileView.replaysMax }} 次），不再提供重放；
          只能登记你在目标平台上看到的事实并结束这条路径运行。
        </p>
        <p v-else-if="partialEffectWarned" class="run-detail__reconcile-note">
          表单数据可能已经写进去了，重放会再写一次；请登记你在目标平台上看到的事实。
        </p>
        <NButton
          v-if="reconcileView.action === 'advance'"
          type="primary" size="small" :disabled="reconciling"
          @click="runRecovery('advance')"
        >确认并前进到下一步</NButton>
        <NButton
          v-else-if="reconcileView.action === 'replay'"
          type="primary" size="small" :disabled="reconciling"
          @click="runRecovery('replay')"
        >重放这一步</NButton>
        <NButton v-else-if="reconcileView.action === 'reconcile_again'" size="small" :disabled="reconciling" @click="doReconcile">重新对账</NButton>
        <div v-else-if="reconcileView.action === 'manual_end'" class="run-detail__manual-form">
          <p class="run-detail__manual-lead">
            请登记你在目标平台上亲眼看到的事实。登记后这条路径运行进入终态、不能再前进，
            人工结论会作为运行事实永久保留。
          </p>
          <NForm
            ref="manualFormRef"
            :model="manualForm"
            :rules="manualRules"
            label-placement="left"
            :label-width="96"
            size="small"
            require-mark-placement="left"
          >
            <NFormItem label="实例状态" path="instanceStatus">
              <NSelect
                v-model:value="manualForm.instanceStatus"
                :options="instanceStatusOptions"
                placeholder="选择目标平台上这条实例的当前状态"
                aria-label="实例状态"
              />
            </NFormItem>
            <NFormItem label="当前节点" path="currentNode">
              <NInput v-model:value="manualForm.currentNode" placeholder="目标平台上显示的当前节点名称" />
            </NFormItem>
            <NFormItem label="登记人" path="reporter">
              <NInput v-model:value="manualForm.reporter" placeholder="你的姓名或账号，事后可追溯" />
            </NFormItem>
            <NFormItem label="补充说明" path="note">
              <NInput
                v-model:value="manualForm.note"
                type="textarea"
                :autosize="{ minRows: 2, maxRows: 4 }"
                placeholder="选填：你据以判断的依据，例如在目标平台看到的待办或已办"
              />
            </NFormItem>
          </NForm>
          <NPopconfirm :disabled="reconciling" @positive-click="registerManual">
            <template #trigger>
              <NButton type="warning" size="small" :disabled="reconciling">登记人工核对结论并结束</NButton>
            </template>
            登记后本路径运行进入终态，不能再放行或重放。确认你登记的是目标平台上的真实状态？
          </NPopconfirm>
        </div>
      </template>
      <template v-else>
        <NButton size="small" type="info" :disabled="reconciling" @click="doReconcile">对账</NButton>
      </template>
    </div>
    <p v-if="topConclusion" class="run-detail__conclusion" role="status">{{ topConclusion }}</p>
    <p v-if="errorText" class="run-detail__error" role="alert">{{ errorText }}</p>
    <p v-if="actionText" class="run-detail__notice" role="status">{{ actionText }}</p>

    <div v-if="loading" class="run-detail__loading"><NSpin size="small" /><span>正在读取运行详情……</span></div>
    <NEmpty v-else-if="!detail" :description="errorText || '未找到该运行记录。'" />

    <div v-else class="run-detail__body">
      <!-- 页签：流程图 / 事件流（F-021）。参照参考图的主区结构。 -->
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
      <!-- F-021 事件流时间线：按数据库顺序只追加，供事后回放“状态怎么变的”。 -->
      <section v-show="activeTab === 'events'" class="run-detail__events" aria-label="事件流">
        <ol class="run-detail__event-list">
          <li v-for="event in runEvents" :key="event.id">
            <span class="run-detail__event-time">{{ event.createdAt }}</span>
            <span>{{ event.label }}</span>
          </li>
          <li v-if="runEvents.length === 0" class="run-detail__event-empty">还没有事件。</li>
        </ol>
      </section>
      <div v-show="activeTab === 'canvas'" class="run-detail__canvas">
        <FlowGraphCanvas
          ref="canvasRef"
          v-if="graph"
          :graph="graph"
          :choices="pathChoices"
          run-mode
          :run-node-states="runNodeStates"
          :current-run-node-key="currentNodeKey"
          :run-taken-edge-ids="runTakenEdgeIds"
          :run-deviation-edge-ids="runDeviationEdgeIds"
          @select-run-node="handleSelectRunNode"
          @run-viewport-change="handleRunViewportChange"
        />
        <NEmpty v-else description="真实流程结构尚未加载，无法渲染运行画布。" />
        <NButton
          v-if="followPaused && currentNodeKey"
          class="run-detail__follow"
          size="small"
          type="info"
          @click="resumeFollow"
        >
          回到当前步
        </NButton>
      </div>
      <div class="run-detail__side">
        <section class="run-detail__breakpoints" aria-label="断点">
          <header class="run-detail__bp-head">
            <h4>断点（{{ detail.breakpoints?.length ?? 0 }}）</h4>
            <span class="run-detail__bp-hint">断点只对本次运行生效，命中都在放行之前判定</span>
          </header>

          <div class="run-detail__bp-group">
            <span class="run-detail__bp-group-title">强制生效</span>
            <ul>
              <li v-for="bp in forcedBreakpoints" :key="bp.type">
                <NTag size="tiny" :bordered="false" type="warning">{{ bp.typeName }}</NTag>
                <span class="run-detail__bp-note">{{ forcedBreakpointNote(bp.type) }}</span>
              </li>
              <li v-if="forcedBreakpoints.length === 0" class="run-detail__empty">当前没有强制断点</li>
            </ul>
          </div>

          <div class="run-detail__bp-group">
            <span class="run-detail__bp-group-title">已挂载（{{ userBreakpoints.length }}）</span>
            <ul>
              <li v-for="(bp, index) in userBreakpoints" :key="`${bp.type}-${index}`">
                <NTag size="tiny" :bordered="false">{{ bp.typeName }}</NTag>
                <span>{{ breakpointTargetText(bp) }}</span>
                <NButton text size="tiny" type="error" :title="`删除${bp.typeName}`" @click="deleteBreakpoint(bp)">删除</NButton>
              </li>
              <li v-if="userBreakpoints.length === 0" class="run-detail__empty">还没有手工挂载的断点</li>
            </ul>
          </div>

          <div class="run-detail__bp-add-form">
            <span class="run-detail__bp-group-title">新增断点</span>
            <NSelect
              v-model:value="newBreakpointType"
              size="small"
              :options="breakpointTypeOptions"
              aria-label="选择断点类型"
            />
            <NButton
              v-if="newBreakpointType === 'node'"
              size="small"
              :disabled="!selectedNodeKey || overviewDone"
              :title="selectedNodeKey ? '在画布上选中的节点挂节点断点' : '先在画布上点选一个节点'"
              @click="addNodeBreakpoint"
            >挂到选中节点</NButton>
            <template v-else-if="newBreakpointType === 'step'">
              <NInputNumber v-model:value="newBreakpointStep" size="small" :min="1" placeholder="步骤序号" aria-label="步骤序号" />
              <NButton size="small" :disabled="!newBreakpointStep || overviewDone" @click="addStepBreakpoint">挂到该步骤</NButton>
            </template>
            <template v-else>
              <NSelect
                v-model:value="newBreakpointAction"
                size="small"
                :options="actionBreakpointOptions"
                placeholder="选择动作"
                aria-label="动作类型"
              />
              <NButton size="small" :disabled="!newBreakpointAction || overviewDone" @click="addActionBreakpoint">挂到该动作</NButton>
            </template>
            <small class="run-detail__bp-note">{{ breakpointTypeHint }}</small>
          </div>
        </section>
        <RunNodePanel v-if="selectedNodeKey" :detail="detail" :node-key="selectedNodeKey" @close="selectedNodeKey = ''" />
        <div v-else class="run-detail__panel-placeholder">
          <p>点击画布上的节点，查看该节点的运行信息与错误。</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
/* 头部：左标题右操作，单行紧凑（参照参考图）。 */
.run-detail__topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 0 10px;
  border-bottom: 1px solid var(--run-border-color);
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.run-detail__topbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.run-detail__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
}

.run-detail__meta-path {
  color: var(--preflight-secondary-text-color, #909090);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.run-detail__topbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

/* 次级行：调度说明 + 多路径 chips。 */
.run-detail__subbar {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 0 0 10px;
  flex-wrap: wrap;
}

.run-detail__schedule {
  color: var(--preflight-secondary-text-color, #909090);
  font-size: 13px;
}

.run-detail__actions-empty {
  font-size: 12px;
  opacity: 0.75;
}

.run-detail__bp-head {
  display: flex;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
}

.run-detail__bp-hint,
.run-detail__bp-note {
  font-size: 12px;
  opacity: 0.75;
}

.run-detail__bp-group {
  display: grid;
  gap: 4px;
  padding: 6px 0;
  border-top: 1px solid var(--run-border-color);
}

.run-detail__bp-group-title {
  font-weight: 600;
  font-size: 12px;
}

.run-detail__bp-group ul {
  display: grid;
  gap: 4px;
  padding: 0;
  margin: 0;
  list-style: none;
}

.run-detail__bp-group li {
  display: flex;
  gap: 6px;
  align-items: center;
  flex-wrap: wrap;
}

.run-detail__bp-add-form {
  display: grid;
  gap: 6px;
  padding-top: 6px;
  border-top: 1px solid var(--run-border-color);
}

.run-detail__manual-lead {
  margin: 0 0 8px;
  font-size: 12px;
  opacity: 0.85;
}

.run-detail {
  display: grid;
  gap: 10px;
  align-content: start;
  padding: 14px 18px;
}

.run-detail__topbar {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  border: 1px solid var(--run-border-color, rgba(128,128,128,0.35));
  border-radius: 8px;
}

.run-detail__meta {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  align-items: center;
  font-size: 13px;
}


.run-detail__failure { color: var(--error-color, #d03050); }
.run-detail__actions { display: flex; gap: 10px; }

.run-detail__conclusion {
  margin: 0;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--success-color, #18a058) 10%, transparent);
  border-radius: 6px;
}

.run-detail__error { margin: 0; color: var(--error-color, #d03050); }
.run-detail__notice { margin: 0; opacity: 0.8; }
.run-detail__loading { display: flex; gap: 10px; align-items: center; }

.run-detail__body {
  display: grid;
  grid-template-columns: 1fr 336px;
  gap: 12px;
  align-items: start;
}

.run-detail__side {
  display: grid;
  gap: 10px;
  align-content: start;
}

.run-detail__breakpoints {
  padding: 10px;
  font-size: 13px;
  border: 1px solid var(--run-border-color, rgba(128,128,128,0.35));
  border-radius: 8px;
}
.run-detail__breakpoints h4 { margin: 0 0 6px; }
.run-detail__breakpoints ul { margin: 0; padding-left: 18px; }
.run-detail__empty { opacity: 0.7; }
.run-detail__reconcile {
  padding: 10px 12px;
  border: 1px solid var(--run-border-color, rgba(128,128,128,0.35));
  border-radius: 8px;
}
.run-detail__reconcile h4 { margin: 0 0 6px; }
.run-detail__reconcile p, .run-detail__reconcile ul { margin: 4px 0; font-size: 13px; }
.run-detail__reconcile-verdict { font-weight: 600; }
.run-detail__reconcile-note { color: var(--warning-color, #f0a020); }
.run-detail__manual-form { display: grid; gap: 6px; max-width: 420px; }

.run-detail__events {
  margin: 0 0 8px;
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 4px;
  padding: 8px 12px;
  max-height: 180px;
  overflow-y: auto;
}

.run-detail__events h4 {
  margin: 0 0 6px;
  font-weight: 500;
  color: v-bind('themeVars.textColor3');
}

.run-detail__event-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
}

.run-detail__event-time {
  color: v-bind('themeVars.textColor3');
  margin-right: 8px;
  font-variant-numeric: tabular-nums;
}

.run-detail__paths {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 0 0 8px;
}

.run-detail__path {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 4px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  padding: 6px 10px;
}

.run-detail__path--active {
  border-color: v-bind('themeVars.primaryColor');
  color: v-bind('themeVars.primaryColor');
}

.run-detail__path-name { font-weight: 500; }
.run-detail__path-status { font-size: 12px; opacity: 0.8; }

.run-detail__schedule {
  margin: 0 0 8px;
  color: v-bind('themeVars.textColor3');
}

.run-detail__structure-note {
  margin: 0;
  color: var(--run-detail-warning-text, #d48806);
  line-height: 1.6;
}

.run-detail__stop-reason {
  margin: 0;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--warning-color, #f0a020) 14%, transparent);
  border-radius: 6px;
}

.run-detail__canvas { position: relative; }

.run-detail__follow {
  position: absolute;
  right: 14px;
  bottom: 14px;
  z-index: 5;
}

.run-detail__panel-placeholder {
  padding: 12px;
  font-size: 13px;
  opacity: 0.7;
  border: 1px dashed var(--run-border-color, rgba(128,128,128,0.35));
  border-radius: 8px;
}

@media (max-width: 1100px) {
  .run-detail__body { grid-template-columns: 1fr; }
}

/* 页签：流程图 / 事件流（参照参考图的下划线页签）。横跨整行，不参与画布/侧栏两列网格。 */
.run-detail__tabs {
  grid-column: 1 / -1;
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--run-border-color);
  margin-bottom: 10px;
}

.run-detail__tab {
  border: none;
  background: transparent;
  font: inherit;
  color: var(--preflight-secondary-text-color, #909090);
  padding: 8px 14px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.run-detail__tab--active {
  color: var(--preflight-primary-color, #18a058);
  border-bottom-color: var(--preflight-primary-color, #18a058);
  font-weight: 500;
}

.run-detail__event-empty {
  color: var(--preflight-secondary-text-color, #909090);
}
</style>
