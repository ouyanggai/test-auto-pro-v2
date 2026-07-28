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
  reconcileExecutionPathChoices,
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
let loadController: AbortController | null = null
let loadVersion = 0

const planID = computed(() => String(route.params.id || ''))
const activePath = computed(() => paths.value.find((path) => path.id === activePathID.value) ?? null)
const pathAnalysis = computed(() => graph.value
  ? analyzeExecutionPath(graph.value, draftChoices.value)
  : null)
const remainingChoices = computed(() => pathAnalysis.value?.missingRouteNodeIds.length ?? 0)
const saveDisabled = computed(() => !draftMode.value || !pathAnalysis.value?.complete || saving.value)
const allowNewPath = computed(() => Boolean(
  graph.value && plan.value && canCreateAdditionalPath(plan.value.flowSource, paths.value.length),
))
const allowCopy = computed(() => plan.value?.flowSource === 'new' && Boolean(activePath.value))

async function loadPage() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  planLoading.value = true
  graphLoading.value = false
  pathsLoading.value = false
  planError.value = ''
  graphError.value = null
  pathsError.value = ''
  planNotFound.value = false
  plan.value = null
  graph.value = null
  paths.value = []
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
    if (pathsResult.status === 'fulfilled') paths.value = pathsResult.value
    else {
      const caught = pathsResult.reason
      pathsError.value = caught instanceof ExecutionPathApiError ? caught.message : '暂时无法读取执行路径'
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
  pathsError.value = ''
  try {
    const items = await fetchExecutionPaths(planID.value, controller.signal)
    if (version !== loadVersion) return
    paths.value = items
    if (graph.value && items[0]) selectSavedPath(items[0])
  }
  catch (caught) {
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
  createKey.value = ''
}

function startNewPath() {
  if (!allowNewPath.value) return
  activePathID.value = null
  draftMode.value = 'new'
  draftChoices.value = []
  draftChangedByGraph.value = false
  createKey.value = crypto.randomUUID()
}

function copyActivePath() {
  if (!allowCopy.value || !activePath.value || !graph.value) return
  const reconciled = reconcileExecutionPathChoices(graph.value, activePath.value.choices)
  activePathID.value = null
  draftMode.value = 'copy'
  draftChoices.value = reconciled.choices
  draftChangedByGraph.value = reconciled.changed
  createKey.value = crypto.randomUUID()
}

function clearDraft() {
  activePathID.value = null
  draftMode.value = null
  draftChoices.value = []
  draftChangedByGraph.value = false
  createKey.value = ''
}

function selectBranch(choice: ExecutionPathChoice) {
  if (!graph.value || !draftMode.value || saving.value) return
  draftChoices.value = applyExecutionPathChoice(graph.value, draftChoices.value, choice.routeNodeId, choice.branchId)
  draftChangedByGraph.value = false
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
    message.error(apiError.message)
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
    else clearDraft()
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
onBeforeUnmount(() => loadController?.abort())
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
                :graph="graph"
                :choices="draftChoices"
                :selection-enabled="draftMode !== null"
                @select-branch="selectBranch"
                @retry="retryGraph"
              >
                <template #toolbar>
                  <div class="path-toolbar">
                    <div class="path-toolbar__paths">
                      <n-spin v-if="pathsLoading" size="small" />
                      <n-button
                        v-for="item in paths"
                        :key="item.id"
                        size="small"
                        :type="activePathID === item.id ? 'primary' : 'default'"
                        :secondary="activePathID === item.id"
						:disabled="saving || deleting"
                        @click="selectSavedPath(item)"
                      >
                        路径 {{ item.sequenceNo }}
                      </n-button>
                      <n-tag v-if="draftMode === 'new'" size="small" type="info" :bordered="false">新路径</n-tag>
                      <n-tag v-if="draftMode === 'copy'" size="small" type="info" :bordered="false">路径副本</n-tag>
                    </div>
                    <div class="path-toolbar__actions">
                      <span v-if="draftMode" class="path-toolbar__remaining">
                        <template v-if="draftChangedByGraph">流程已变化，需要重新选择</template>
                        <template v-else-if="pathAnalysis?.invalid">当前选择已失效</template>
                        <template v-else-if="remainingChoices > 0">还需选择 {{ remainingChoices }} 处</template>
                        <template v-else>路径已完整</template>
                      </span>
                      <n-button v-if="allowNewPath" size="small" :disabled="saving || deleting" @click="startNewPath">
                        {{ plan.flowSource === 'new' ? '新增路径' : '选择当前实例后续路径' }}
                      </n-button>
                      <n-button v-if="allowCopy" size="small" :disabled="saving || deleting" @click="copyActivePath">复制此路径</n-button>
                      <n-button
                        v-if="draftMode"
                        size="small"
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
                    </div>
                  </div>
                  <n-alert v-if="pathsError" class="path-toolbar__error" type="error" :show-icon="false">
                    {{ pathsError }}
                    <n-button text type="primary" @click="retryPaths">重试</n-button>
                  </n-alert>
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

.path-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 40px;
  gap: 16px;
  padding: 5px 8px;
  background: color-mix(in srgb, var(--flow-surface-color) 94%, transparent);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
}

.path-toolbar__paths,
.path-toolbar__actions {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}

.path-toolbar__paths {
  overflow-x: auto;
}

.path-toolbar__actions {
  flex: 0 0 auto;
}

.path-toolbar__remaining {
  color: var(--flow-label-color);
  font-size: 13px;
  white-space: nowrap;
}

.path-toolbar__error {
  width: max-content;
  max-width: 100%;
  margin-top: 6px;
}

.error-actions {
  display: flex;
  gap: 12px;
}
</style>
