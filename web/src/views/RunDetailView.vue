<script setup lang="ts">
import { NButton, NEmpty, NSpin, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

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
import { analyzeExecutionPath } from '../features/execution-paths/logic'
import RunNodePanel from '../features/runs/RunNodePanel.vue'
import RunStatusIndicator from '../features/runs/RunStatusIndicator.vue'

// RunDetailView 是路径运行详情：运行画布为主体，顶部固定条控制放行与停止。
// 放行会发出真实写请求：只接受明确点击，不绑定单键快捷键。
const route = useRoute()
const runId = String(route.params.runId || '')
const themeVars = useThemeVars()

const detail = ref<PathRunDetail | null>(null)
const graph = ref<FlowGraph | null>(null)
const loading = ref(false)
const errorText = ref('')
const actionText = ref('')
const acting = ref(false)

// 本地时钟：执行中的已耗时以本地时钟连续插值，不被轮询节奏带跳，读数单调不减。
const clockTick = ref(0)
let clockTimer: number | null = null
const approveStartedAt = ref<number | null>(null)
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
  const settledKeys = detail.value.steps.map((step) => step.nodeKey)
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

async function doReconcile(): Promise<void> {
  if (reconciling.value) return
  reconciling.value = true
  errorText.value = ''
  try {
    reconcileView.value = await reconcileNow(runId)
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
    detail.value = await recoveryAction(runId, action, manual)
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
  await runRecovery('manual_end')
}

// 放行命令与条件写参数：命令集合由后端给出，游标与版本取自详情（重复点击只产生一次效果）。
const approveCommand = ref('step')
const approveCursor = ref(0)
const approveVersion = ref(0)
const breakpointInput = ref('')

// syncControl 从最新详情同步命令与条件写参数。
function syncControl(next: PathRunDetail): void {
  approveCommand.value = next.commands.length > 0 ? next.commands[0].command : 'step'
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
    detail.value = await approveRun(runId, command, detail.value.currentStepNo, detail.value.controlVersion)
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
    await requestPause(runId)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '暂停请求失败，请重试'
  } finally {
    pausing.value = false
  }
}

// addNodeBreakpoint 在当前选中节点上就地挂节点断点。
async function addNodeBreakpoint(): Promise<void> {
  if (!selectedNodeKey.value) return
  try {
    const list = await setBreakpoint(runId, { type: 'node', nodeKey: selectedNodeKey.value })
    applyBreakpoints(list)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '设置断点失败'
  }
}

// deleteBreakpoint 删除一个断点（路径偏离断点由后端拒绝并给中文原因）。
async function deleteBreakpoint(bp: BreakpointInput): Promise<void> {
  try {
    const list = await removeBreakpoint(runId, bp)
    applyBreakpoints(list)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '删除断点失败'
  }
}

// applyBreakpoints 把后端返回的断点列表同步进详情（即时可见，不需要刷新页面）。
function applyBreakpoints(list: BreakpointInput[]): void {
  if (!detail.value) return
  detail.value.breakpoints = list.map((bp) => ({
    type: bp.type,
    typeName: bp.type === 'node' ? '节点断点' : bp.type === 'step' ? '步骤断点' : bp.type === 'action' ? '动作断点' : bp.type,
    nodeName: bp.nodeKey,
    stepNo: bp.stepNo,
    action: bp.action,
  }))
}

// currentNodeKey 是当前步所在节点（预览给出），画布据此高亮与居中。
const currentNodeKey = computed(() => detail.value?.currentPreview?.nodeKey || '')

// runNodeStates 把九个中文运行态与当前步标记交给画布。
const runNodeStates = computed(() => {
  const states: Record<string, { status: string; statusName: string }> = {}
  for (const [nodeKey, state] of Object.entries(detail.value?.nodeStates || {})) {
    states[nodeKey] = { status: state.status, statusName: state.statusName }
  }
  return states
})

// isActing 表示一次放行或停止请求在途：指示器进入执行中状态。
const isActing = computed(() => acting.value)

// indicatorRunning 让连续执行循环里的实时阶段也能驱动指示器：
// 循环存活且后端上报了当前阶段时，即使没有放行请求在途也应显示阶段推进而不是等待放行。
const indicatorRunning = computed(() => Boolean(detail.value?.loopRunning && detail.value?.currentPhase))

// staleBudgetElapsed 判断是否超过疑似无响应预算。
const staleBudgetElapsed = computed(() => {
  if (!isActing.value || !detail.value) return false
  void clockTick.value
  return Date.now() - lastUpdateAt.value > detail.value.staleAfterMs
})

// elapsedText 是三处同源的耗时读数：执行中以放行点击时刻为起点本地插值，读数单调不减。
const elapsedText = computed(() => {
  void clockTick.value
  if (isActing.value && approveStartedAt.value !== null) {
    return formatElapsed(Math.max(0, Date.now() - approveStartedAt.value))
  }
  const steps = detail.value?.steps || []
  if (steps.length > 0) {
    const last = steps[steps.length - 1]
    return formatElapsed(last.durationMs)
  }
  return formatElapsed(0)
})

// phaseDurations 取最后一步最近一次尝试的七阶段耗时（已落账时展示）。
const phaseDurations = computed(() => {
  const steps = detail.value?.steps || []
  if (steps.length === 0) return undefined
  const attempts = steps[steps.length - 1].attempts
  return attempts.length > 0 ? attempts[attempts.length - 1].phaseDurations : undefined
})

// loadDetail 拉取详情并刷新结构（结构只按计划取一次）。
async function loadDetail(): Promise<void> {
  if (!runId) {
    errorText.value = '运行标识缺失，无法打开详情。'
    return
  }
  loading.value = !detail.value
  try {
    const next = await fetchRunDetail(runId)
    detail.value = next
    syncControl(next)
    lastUpdateAt.value = Date.now()
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
  if (terminalStatuses.includes(detail.value.pathRunStatusName)) return
  pollTimer = window.setTimeout(async () => {
    try {
      const next = await fetchRunDetail(runId)
      detail.value = next
      syncControl(next)
      lastUpdateAt.value = Date.now()
      if (currentNodeKey.value && !followPaused.value) {
        centerCurrentNode()
      }
    } catch {
      // 单次轮询失败不打断页面：下一次轮询会继续。
    }
    schedulePoll()
  }, Math.max(500, detail.value.pollIntervalMs || 2000))
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
  approveStartedAt.value = Date.now()
  actionText.value = ''
  errorText.value = ''
  try {
    detail.value = await approveRun(runId)
    lastUpdateAt.value = Date.now()
    await nextTick()
    if (currentNodeKey.value && !followPaused.value) {
      centerCurrentNode()
    }
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '放行执行失败，请查看日志'
  } finally {
    acting.value = false
    approveStartedAt.value = null
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
    detail.value = await stopRun(runId)
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
  clockTimer = window.setInterval(() => { clockTick.value++ }, 250)
})

onBeforeUnmount(() => {
  if (pollTimer !== null) window.clearTimeout(pollTimer)
  if (clockTimer !== null) window.clearInterval(clockTimer)
})
</script>

<template>
  <section class="run-detail" :style="{ '--run-surface-color': themeVars.cardColor, '--run-border-color': themeVars.dividerColor }">
    <header v-if="detail" class="run-detail__topbar">
      <div class="run-detail__meta">
        <strong>运行 #{{ detail.runNo }}</strong>
        <span>{{ detail.planName }} / {{ detail.pathName }}</span>
        <span class="run-detail__mode">模式：{{ detail.modeName }}（固定）</span>
        <span>路径运行：{{ detail.pathRunStatusName }}</span>
        <span v-if="detail.failureClassName" class="run-detail__failure">{{ detail.failureClassName }}</span>
      </div>
      <div class="run-detail__actions">
        <NButton
          v-for="command in detail.commands"
          :key="command.command"
          :type="command.command === 'step' ? 'primary' : 'info'"
          :disabled="acting || looping || overviewDone"
          :title="command.label"
          @click="command.command === 'step' ? approve() : runCommand(command.command)"
        >
          {{ command.command === 'step' ? '放行（执行一步）' : command.label.split('（')[0] }}
        </NButton>
        <NButton
          v-if="detail.loopRunning"
          :disabled="pausing"
          title="暂停请求只在本步走完核验与落账后生效"
          @click="pauseNow"
        >暂停（本步结束后生效）</NButton>
        <NButton :disabled="acting || overviewDone" title="停止将在当前步骤结束后生效" @click="stopRunAction">停止</NButton>
      </div>
      <RunStatusIndicator
        class="run-detail__indicator"
        :running="isActing || indicatorRunning"
        :stale="staleBudgetElapsed"
        :elapsed-text="elapsedText"
        :phase-durations="phaseDurations"
        :note="acting && staleBudgetElapsed ? '超过预算未收到状态更新' : ''"
        :current-phase="detail?.currentPhase"
        :current-phase-note="detail?.currentPhaseNote"
      />
    </header>
    <p v-if="detail && detail.stopReason" class="run-detail__stop-reason" role="status">为什么停在这里：{{ detail.stopReason }}</p>
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
        <p v-if="reconcileView.verdict === 'not_effective'" class="run-detail__reconcile-note">
          唯一动作是重放这一步：它是一次新的尝试，会重新走门禁与七阶段；一次尝试仍然只发一次写请求。
        </p>
        <p v-if="reconcileView.verdict === 'indeterminate'" class="run-detail__reconcile-note">
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
          <input v-model="manualForm.instanceStatus" placeholder="实例状态（目标平台上看到）" />
          <input v-model="manualForm.currentNode" placeholder="当前节点" />
          <input v-model="manualForm.note" placeholder="说明（可选）" />
          <input v-model="manualForm.reporter" placeholder="登记人" />
          <NButton type="warning" size="small" :disabled="!manualForm.instanceStatus || reconciling" @click="registerManual">
            登记人工核对结论并结束
          </NButton>
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
      <div class="run-detail__canvas">
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
        <div class="run-detail__breakpoints">
          <h4>生效断点（{{ detail.breakpoints.length }}）</h4>
          <p v-if="detail.breakpoints.length === 0" class="run-detail__empty">暂无断点；路径偏离断点始终生效。</p>
          <ul>
            <li v-for="(bp, index) in detail.breakpoints" :key="index">
              {{ bp.typeName }}{{ bp.nodeName ? `（${bp.nodeName}）` : '' }}{{ bp.stepNo ? `（第 ${bp.stepNo} 步）` : '' }}
              <button v-if="bp.type !== 'path_deviation'" class="run-detail__bp-remove" @click="deleteBreakpoint(bp)">删除</button>
            </li>
          </ul>
          <button
            v-if="selectedNodeKey"
            class="run-detail__bp-add"
            @click="addNodeBreakpoint"
          >在当前选中节点设断点</button>
        </div>
        <RunNodePanel v-if="selectedNodeKey" :detail="detail" :node-key="selectedNodeKey" @close="selectedNodeKey = ''" />
        <div v-else class="run-detail__panel-placeholder">
          <p>点击画布上的节点，查看该节点的运行信息与错误。</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
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

.run-detail__mode {
  padding: 1px 8px;
  background: color-mix(in srgb, var(--info-color, #2080f0) 12%, transparent);
  border-radius: 8px;
}

.run-detail__failure { color: var(--error-color, #d03050); }
.run-detail__actions { display: flex; gap: 10px; }
.run-detail__indicator { min-width: 380px; }

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
.run-detail__bp-remove, .run-detail__bp-add {
  padding: 1px 8px;
  cursor: pointer;
  background: none;
  border: 1px solid var(--run-border-color, rgba(128,128,128,0.35));
  border-radius: 4px;
}
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
</style>
