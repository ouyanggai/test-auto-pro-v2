<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDescriptions,
  NDescriptionsItem,
  NEmpty,
  NPopconfirm,
  NSpin,
  NTag,
  useMessage,
} from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import {
  createExecutionPath,
  deleteExecutionPath,
  ExecutionPathApiError,
  fetchExecutionPaths,
  updateExecutionPath,
} from '../features/execution-paths/api'
import {
  analyzeExecutionPath,
  applyExecutionPathChoice,
  canCreateAdditionalPath,
  canEnterExecutionPathSelection,
  projectExecutionPathSummary,
  reconcileExecutionPathChoices,
  refreshExecutionPathDraft,
} from '../features/execution-paths/logic'
import type { ExecutionPath, ExecutionPathChoice } from '../features/execution-paths/types'
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
const draftMode = ref<'new' | 'copy' | 'edit' | null>(null)
const draftChoices = ref<ExecutionPathChoice[]>([])
const draftChangedByGraph = ref(false)
const createKey = ref('')
const saving = ref(false)
const deleting = ref(false)
const selectionMode = ref(false)
const selectionStarted = ref(false)
const draftRecoveryLoading = ref(false)
const draftRecoveryError = ref('')
let loadController: AbortController | null = null
let loadVersion = 0
let draftRecoveryController: AbortController | null = null
let draftRecoveryVersion = 0

const planID = computed(() => String(route.params.id || ''))
const activePath = computed(() => paths.value.find((path) => path.id === activePathID.value) ?? null)
const pathAnalysis = computed(() => graph.value
  ? analyzeExecutionPath(graph.value, draftChoices.value)
  : null)
const remainingChoices = computed(() => pathAnalysis.value?.missingRouteNodeIds.length ?? 0)
const saveDisabled = computed(() => !draftMode.value
  || !pathAnalysis.value?.complete
  || saving.value
  || draftRecoveryLoading.value
  || Boolean(draftRecoveryError.value))
const allowNewPath = computed(() => Boolean(
  graph.value
  && plan.value
  && pathsLoaded.value
  && !pathsError.value
  && canCreateAdditionalPath(plan.value.flowSource, paths.value.length),
))
const allowCopy = computed(() => pathsLoaded.value && plan.value?.flowSource === 'new' && Boolean(activePath.value))
const selectionAvailable = computed(() => canEnterExecutionPathSelection({
  graphReady: Boolean(graph.value),
  pathsLoaded: pathsLoaded.value,
  pathsFailed: Boolean(pathsError.value),
  hasDraft: Boolean(draftMode.value),
  canCreate: allowNewPath.value,
}))
const selectionResumable = computed(() => selectionStarted.value && Boolean(draftMode.value))
const pathSummary = computed(() => graph.value && pathAnalysis.value
  ? projectExecutionPathSummary(graph.value, pathAnalysis.value, draftChoices.value)
  : [])

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
  selectionMode.value = false
  selectionStarted.value = false
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
    }
    else {
      const caught = pathsResult.reason
      pathsError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取执行路径'
      pathsLoaded.value = false
    }
    if (graph.value && paths.value[0]) selectSavedPath(paths.value[0])
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
    if (graph.value && items[0]) selectSavedPath(items[0])
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
  draftMode.value = 'edit'
  draftChoices.value = reconciled.choices
  draftChangedByGraph.value = reconciled.changed
  draftRecoveryError.value = ''
  createKey.value = ''
}

function startNewPath() {
  if (!allowNewPath.value) return
  activePathID.value = null
  draftMode.value = 'new'
  draftChoices.value = []
  draftChangedByGraph.value = false
  draftRecoveryError.value = ''
  createKey.value = crypto.randomUUID()
}

function copyActivePath() {
  if (!allowCopy.value || !activePath.value || !graph.value) return
  const reconciled = reconcileExecutionPathChoices(graph.value, activePath.value.choices)
  activePathID.value = null
  draftMode.value = 'copy'
  draftChoices.value = reconciled.choices
  draftChangedByGraph.value = reconciled.changed
  draftRecoveryError.value = ''
  createKey.value = crypto.randomUUID()
}

function clearDraft() {
  activePathID.value = null
  draftMode.value = null
  draftChoices.value = []
  draftChangedByGraph.value = false
  draftRecoveryError.value = ''
  createKey.value = ''
}

function selectBranch(choice: ExecutionPathChoice) {
  if (!graph.value || !draftMode.value || !selectionMode.value || saving.value) return
  draftChoices.value = applyExecutionPathChoice(graph.value, draftChoices.value, choice.routeNodeId, choice.branchId)
  draftChangedByGraph.value = false
}

function enterSelectionMode() {
  if (!selectionAvailable.value || !graph.value) return
  // 首次进入时才创建本地草稿；退出页面全屏不会清理它，确保再次进入可继续选择。
  if (!draftMode.value) {
    if (allowNewPath.value) startNewPath()
    else if (paths.value[0]) selectSavedPath(paths.value[0])
  }
  if (!draftMode.value) return
  selectionMode.value = true
  selectionStarted.value = true
}

function exitSelectionMode() {
  // 这里只关闭交互展示，草稿、活动路径和创建键都必须原样保留。
  selectionMode.value = false
}

async function refreshDraftAfterInvalidSave() {
  if (!draftMode.value) return
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
  if (!plan.value || !graph.value || !draftMode.value || saveDisabled.value) return
  saving.value = true
  try {
    const saved = draftMode.value === 'edit' && activePathID.value
      ? await updateExecutionPath(planID.value, activePathID.value, draftChoices.value)
      : await createExecutionPath(planID.value, draftChoices.value, createKey.value)
    const existingIndex = paths.value.findIndex((path) => path.id === saved.id)
    if (existingIndex >= 0) paths.value.splice(existingIndex, 1, saved)
    else paths.value.push(saved)
    paths.value.sort((left, right) => left.sequenceNo - right.sequenceNo)
    plan.value.pathCount = paths.value.length
    selectSavedPath(saved)
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
      message.error(apiError.message)
    }
  }
  finally {
    saving.value = false
  }
}

async function removeActivePath() {
  if (!activePath.value || !plan.value || deleting.value) return
  const deletingID = activePath.value.id
  deleting.value = true
  try {
    await deleteExecutionPath(planID.value, deletingID)
    paths.value = paths.value.filter((path) => path.id !== deletingID)
    plan.value.pathCount = paths.value.length
    if (paths.value[0] && graph.value) selectSavedPath(paths.value[0])
    else {
      clearDraft()
      selectionMode.value = false
    }
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

watch(planID, () => { void loadPage() }, { immediate: true })
onBeforeUnmount(() => {
  loadController?.abort()
  draftRecoveryController?.abort()
})
</script>

<template>
  <section class="plan-paths-page">
    <div class="paths-back-bar">
      <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
    </div>

    <n-spin :show="planLoading">
      <div v-if="plan" class="paths-content">
        <header class="page-heading">
          <div>
            <h1>{{ plan.name }}</h1>
            <p>从当前入口选择执行线路，并保存为计划路径。</p>
          </div>
          <n-tag size="small" type="warning" :bordered="false">
            {{ planStatusLabels[plan.status] }}
          </n-tag>
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

        <section class="graph-section" aria-labelledby="flow-graph-heading">
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
                :graph="graph"
                :choices="draftChoices"
                :selection-mode="selectionMode"
                :selection-available="selectionAvailable"
                :selection-resumable="selectionResumable"
                @select-branch="selectBranch"
                @enter-selection="enterSelectionMode"
                @exit-selection="exitSelectionMode"
                @retry="retryGraph"
              >
                <template #selection-panel>
                  <section class="path-selection-panel">
                    <header class="path-selection-panel__header">
                      <div>
                        <h3>线路选择</h3>
                        <p v-if="draftChangedByGraph">流程已变化，需要重新选择</p>
                        <p v-else-if="pathAnalysis?.invalid">当前选择已失效</p>
                        <p v-else-if="remainingChoices > 0">还需选择 {{ remainingChoices }} 处</p>
                        <p v-else>线路已完整，请保存</p>
                      </div>
                      <n-tag size="small" :type="pathAnalysis?.invalid ? 'error' : remainingChoices > 0 ? 'warning' : 'success'" :bordered="false">
                        {{ pathAnalysis?.invalid ? '选择异常' : `${remainingChoices} 处待选` }}
                      </n-tag>
                    </header>

                    <div class="path-selection-panel__paths" aria-label="已保存路径">
                      <n-button
                        v-for="item in paths"
                        :key="item.id"
                        size="small"
                        :type="activePathID === item.id ? 'primary' : 'default'"
                        :secondary="activePathID === item.id"
                        :disabled="saving || deleting || draftRecoveryLoading"
                        @click="selectSavedPath(item)"
                      >
                        路径 {{ item.sequenceNo }}
                      </n-button>
                      <n-tag v-if="draftMode === 'new'" size="small" type="info" :bordered="false">新路径</n-tag>
                      <n-tag v-if="draftMode === 'copy'" size="small" type="info" :bordered="false">路径副本</n-tag>
                    </div>

                    <div class="path-selection-panel__create-actions">
                      <n-button v-if="allowNewPath" size="small" :disabled="saving || deleting || draftRecoveryLoading" @click="startNewPath">
                        {{ plan.flowSource === 'new' ? '新增路径' : '选择当前实例后续路径' }}
                      </n-button>
                      <n-button v-if="allowCopy" size="small" :disabled="saving || deleting || draftRecoveryLoading" @click="copyActivePath">复制此路径</n-button>
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
                      <n-button
                        v-if="draftMode"
                        type="primary"
                        :loading="saving"
                        :disabled="saveDisabled"
                        @click="savePath"
                      >
                        保存路径
                      </n-button>
                      <n-popconfirm v-if="activePath" @positive-click="removeActivePath">
                        <template #trigger>
                          <n-button size="small" type="error" secondary :loading="deleting">删除路径</n-button>
                        </template>
                        只删除当前工具中的路径记录，确认继续？
                      </n-popconfirm>
                    </footer>
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

      <div v-else-if="!planLoading" class="paths-error-region">
        <n-empty :description="planNotFound ? '计划不存在或已不可用' : planError || '暂时无法读取计划'">
          <template #extra>
            <div class="error-actions">
              <n-button v-if="!planNotFound" type="primary" secondary @click="loadPage">重试</n-button>
              <n-button @click="router.push('/plans')">返回测试计划</n-button>
            </div>
          </template>
        </n-empty>
      </div>
    </n-spin>
  </section>
</template>

<style scoped>
.plan-paths-page {
  width: 100%;
  min-width: 0;
}

.paths-back-bar {
  position: sticky;
  top: -32px;
  z-index: 5;
  margin: -32px -48px 24px;
  padding: 16px 48px 12px;
  background: var(--n-color);
  border-bottom: 1px solid var(--n-border-color);
}

.paths-content {
  width: min(1180px, 100%);
  margin: 0 auto;
}

.page-heading,
.graph-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.page-heading {
  margin-bottom: 24px;
}

.page-heading h1,
.graph-heading h2 {
  margin: 0;
  font-weight: 600;
}

.page-heading h1 {
  margin-bottom: 8px;
  font-size: 28px;
}

.graph-heading h2 {
  margin-bottom: 5px;
  font-size: 18px;
}

.page-heading p,
.graph-heading p {
  margin: 0;
  color: var(--n-text-color-2);
}

.graph-section {
  margin-top: 28px;
}

.graph-heading {
  margin-bottom: 14px;
}

.graph-region,
.graph-state,
.paths-error-region {
  min-height: 560px;
}

.graph-region {
  position: relative;
  min-width: 0;
}

.graph-state,
.paths-error-region {
  display: grid;
  place-items: center;
  border-top: 1px solid var(--n-border-color);
}

.graph-warning {
  position: absolute;
  top: 12px;
  right: 16px;
  left: 16px;
  z-index: 4;
}

.paths-load-error {
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
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 4px 14px 12px;
  border-bottom: 1px solid var(--flow-edge-color);
}

.path-selection-panel__header h3,
.path-selection-panel__header p,
.path-selection-panel__summary h4 {
  margin: 0;
}

.path-selection-panel__header h3 {
  margin-bottom: 4px;
  font-size: 16px;
}

.path-selection-panel__header p,
.path-summary__item span {
  color: var(--flow-label-color);
  font-size: 12px;
  opacity: 0.72;
}

.path-selection-panel__paths,
.path-selection-panel__create-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px 14px 0;
}

.path-selection-panel__paths {
  max-height: 96px;
  overflow-y: auto;
}

.path-selection-panel__create-actions {
  padding-bottom: 10px;
  border-bottom: 1px solid var(--flow-edge-color);
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

.path-summary {
  margin: 0;
  padding: 0 0 0 9px;
  list-style: none;
  border-left: 1px solid var(--flow-edge-color);
}

.path-summary__item {
  position: relative;
  min-height: 48px;
  padding: 0 0 14px 16px;
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

.path-selection-panel__footer {
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
  padding: 12px 14px;
  background: var(--flow-surface-color);
  border-top: 1px solid var(--flow-edge-color);
}

.path-selection-panel__footer > :first-child {
  flex: 1 1 auto;
}

.error-actions {
  display: flex;
  gap: 12px;
}
</style>
