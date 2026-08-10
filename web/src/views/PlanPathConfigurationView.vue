<script setup lang="ts">
import { NAlert, NButton, NEmpty, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { fetchExecutionPaths } from '../features/execution-paths/api'
import type { ExecutionPath } from '../features/execution-paths/types'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import NodeConfigurationPanel from '../features/path-configuration/NodeConfigurationPanel.vue'
import {
  applyPathConfigDraft,
  allEditableFieldsFilled,
  bindPathConfigurationNodes,
  buildPathConfigSavePayload,
  canSavePathConfiguration,
  encodePathConfigValue,
  hasPathConfigDraftChanges,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  pathConfigurationNodesByGraphID,
  projectPathConfigurationNodeStates,
  reconcilePathConfigDraft,
} from '../features/path-configuration/logic'
import {
  fetchPathConfiguration,
  PathConfigApiError,
  savePathConfiguration,
} from '../features/path-configuration/api'
import type {
  PathConfigAction,
  PathConfigDraft,
  PathConfigField,
  PathConfigNode,
  PathConfiguration,
  PathConfigPerson,
} from '../features/path-configuration/types'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import type { PersistedPlan } from '../features/plans/types'

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
const draft = ref<PathConfigDraft>({ fields: {}, actions: {}, persons: {} })
const configurationByGraphNodeID = ref(new Map<string, PathConfigNode>())
const graphNodeIDByConfigurationKey = ref(new Map<string, string>())
const selectedNodeID = ref('')
const loading = ref(false)
const saving = ref(false)
const pageError = ref('')
const saveError = ref('')
const saveDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
const savedSuccessfully = ref(false)
const canvasRef = ref<InstanceType<typeof FlowGraphCanvas> | null>(null)
let loadVersion = 0
let loadController: AbortController | null = null
let saveController: AbortController | null = null
let saveKey = ''

const pageThemeStyle = computed(() => ({
  '--path-config-page-color': themeVars.value.bodyColor,
  '--path-config-card-color': themeVars.value.cardColor,
  '--path-config-border-color': themeVars.value.borderColor,
  '--path-config-text-color': themeVars.value.textColor1,
  '--path-config-text-secondary-color': themeVars.value.textColor2,
}))
const pathAnalysis = computed(() => graph.value && currentPath.value
  ? analyzeExecutionPath(graph.value, currentPath.value.choices)
  : null)
const selectedNode = computed(() => configurationByGraphNodeID.value.get(selectedNodeID.value) ?? null)
const configurationNodeStates = computed(() => graph.value && pathAnalysis.value
  ? projectPathConfigurationNodeStates(graph.value, pathAnalysis.value, configurationByGraphNodeID.value, selectedNodeID.value)
  : {})
const requiredState = computed(() => configuration.value
  ? allEditableFieldsFilled(configuration.value, draft.value)
  : { missing: [] as string[], complete: false })
const dirty = computed(() => configuration.value ? hasPathConfigDraftChanges(configuration.value, draft.value) : false)
const saveDisabled = computed(() => loading.value
  || saving.value
  || !configuration.value
  || !canSavePathConfiguration(configuration.value, draft.value))
const nextUnconfiguredPath = computed(() => executionPaths.value.find((path) => (
  path.id !== pathID.value && path.configurationStatus !== 'configured'
)) ?? null)

// publicPageError 把读取链路异常收敛为不含内部标识的稳定页面错误。
function publicPageError(caught: unknown): string {
  if (caught instanceof PlanApiError || caught instanceof PathConfigApiError) return caught.message
  if (caught instanceof Error && (
    caught.message === '已保存路径不存在或已删除'
    || caught.message === '当前已保存路径与真实流程不一致，请先编辑路径'
    || caught.message === '路径节点配置与当前流程结构不一致'
  )) return caught.message
  return '暂时无法读取节点配置，请重试'
}

// focusSelectedNode 仅在首次加载或用户明确要求时定位，普通节点切换不抢夺画布位置。
async function focusSelectedNode() {
  if (!selectedNodeID.value) return
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  await canvasRef.value?.focusNode(selectedNodeID.value)
}

// loadPage 读取计划、已保存路径、真实图和当前节点配置，并用不透明节点键建立本次快照映射。
async function loadPage() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  pageError.value = ''
  saveError.value = ''
  saveDetails.value = []
  savedSuccessfully.value = false
  plan.value = null
  graph.value = null
  currentPath.value = null
  executionPaths.value = []
  configuration.value = null
  configurationByGraphNodeID.value = new Map()
  graphNodeIDByConfigurationKey.value = new Map()
  selectedNodeID.value = ''
  try {
    const [storedPlan, storedGraph, storedPaths] = await Promise.all([
      fetchPlan(planID.value, controller.signal),
      fetchFlowGraph(planID.value, controller.signal),
      fetchExecutionPaths(planID.value, controller.signal),
    ])
    if (controller.signal.aborted || version !== loadVersion) return
    const storedPath = storedPaths.find((path) => path.id === pathID.value)
    if (!storedPath) throw new Error('已保存路径不存在或已删除')
    const analysis = analyzeExecutionPath(storedGraph, storedPath.choices)
    if (!analysis.complete || analysis.invalid) throw new Error('当前已保存路径与真实流程不一致，请先编辑路径')

    const storedConfiguration = await fetchPathConfiguration(planID.value, pathID.value, controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    const bindings = await bindPathConfigurationNodes(storedGraph, storedConfiguration)
    if (controller.signal.aborted || version !== loadVersion) return

    plan.value = storedPlan
    graph.value = storedGraph
    currentPath.value = storedPath
    executionPaths.value = storedPaths
    configuration.value = storedConfiguration
    draft.value = initPathConfigDraft(storedConfiguration)
    configurationByGraphNodeID.value = bindings.byGraphNodeID
    graphNodeIDByConfigurationKey.value = bindings.graphNodeIDByKey
    selectedNodeID.value = initialPathConfigurationNodeID(storedConfiguration, bindings.graphNodeIDByKey)
    saveKey = crypto.randomUUID()
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    pageError.value = publicPageError(caught)
  }
  finally {
    if (version === loadVersion) loading.value = false
  }
  if (version === loadVersion && configuration.value) await focusSelectedNode()
}

// refreshAfterInvalidSave 用当前真实图重新绑定配置，只保留仍能用不透明键安全对应的草稿。
async function refreshAfterInvalidSave(controller: AbortController) {
  const preservedDraft = structuredClone(draft.value) as PathConfigDraft
  const preservedSelectedNodeID = selectedNodeID.value
  try {
    const [latestGraph, latestConfiguration] = await Promise.all([
      fetchFlowGraph(planID.value, controller.signal),
      fetchPathConfiguration(planID.value, pathID.value, controller.signal),
    ])
    if (controller.signal.aborted) return
    const bindings = await bindPathConfigurationNodes(latestGraph, latestConfiguration)
    graph.value = latestGraph
    configuration.value = latestConfiguration
    draft.value = reconcilePathConfigDraft(latestConfiguration, preservedDraft)
    configurationByGraphNodeID.value = bindings.byGraphNodeID
    graphNodeIDByConfigurationKey.value = bindings.graphNodeIDByKey
    selectedNodeID.value = bindings.byGraphNodeID.has(preservedSelectedNodeID)
      ? preservedSelectedNodeID
      : initialPathConfigurationNodeID(latestConfiguration, bindings.graphNodeIDByKey)
  }
  catch (caught) {
    if (controller.signal.aborted) return
    // 刷新失败时不替换现有图、草稿或选中节点，用户仍可在原现场重试。
    saveError.value = '目标结构已变化，但暂时无法读取最新配置；当前草稿已保留'
  }
}

// saveConfiguration 按路径保存全部节点配置；失败保留草稿和视口，成功只刷新本地配置事实。
async function saveConfiguration() {
  const current = configuration.value
  if (!current || saveDisabled.value) return
  saveController?.abort()
  const controller = new AbortController()
  saveController = controller
  saving.value = true
  savedSuccessfully.value = false
  saveError.value = ''
  saveDetails.value = []
  const payload = buildPathConfigSavePayload(current, draft.value)
  try {
    const saved = await savePathConfiguration(
      planID.value,
      pathID.value,
      current.revision,
      payload.fields,
      payload.actions,
      saveKey,
    )
    if (controller.signal.aborted) return
    const updatedConfiguration = applyPathConfigDraft(current, draft.value, saved.revision)
    configuration.value = updatedConfiguration
    // 保存后重建节点索引，画布状态与侧栏必须立即指向新基线，不能继续显示旧对象。
    configurationByGraphNodeID.value = pathConfigurationNodesByGraphID(updatedConfiguration, graphNodeIDByConfigurationKey.value)
    saveKey = crypto.randomUUID()
    savedSuccessfully.value = true
    try {
      executionPaths.value = await fetchExecutionPaths(planID.value, controller.signal)
    }
    catch (caught) {
      if (!(caught instanceof DOMException && caught.name === 'AbortError')) executionPaths.value = []
    }
  }
  catch (caught) {
    if (controller.signal.aborted) return
    if (caught instanceof PathConfigApiError) {
      saveDetails.value = caught.details
      if (caught.code === 'CONFIG_INVALID') {
        saveError.value = '目标流程配置已变化，需要重新确认受影响节点'
        await refreshAfterInvalidSave(controller)
      }
      else {
        saveError.value = caught.message
      }
    }
    else {
      saveError.value = '保存失败，当前节点和草稿已保留，请重试'
    }
  }
  finally {
    saving.value = false
  }
}

// selectConfigurationNode 切换侧栏内容但不重置或自动移动用户当前视口。
function selectConfigurationNode(nodeID: string) {
  if (!configurationByGraphNodeID.value.has(nodeID)) return
  selectedNodeID.value = nodeID
  savedSuccessfully.value = false
}

// selectNextConfigurationNode 显式定位后端给出的下一待配置节点，并保持当前缩放。
async function selectNextConfigurationNode() {
  const key = configuration.value?.nextNodeKey
  const nodeID = key ? graphNodeIDByConfigurationKey.value.get(key) : ''
  if (!nodeID) return
  selectedNodeID.value = nodeID
  await focusSelectedNode()
}

// updateFieldValue 编码受控组件值，浏览器只保存后端下发的不透明字段键。
function updateFieldValue(field: PathConfigField, value: unknown) {
  draft.value.fields[field.key] = encodePathConfigValue(field, value)
  savedSuccessfully.value = false
}

// updateActionValue 更新当前节点合法动作草稿。
function updateActionValue(action: PathConfigAction, value: string) {
  draft.value.actions[action.key] = value
  savedSuccessfully.value = false
}

// updatePersonValue 更新当前模板允许的人员候选草稿，不接收面板之外的任意人员值。
function updatePersonValue(person: PathConfigPerson, value: string[]) {
  const allowed = new Set(person.options.map((option) => option.value))
  draft.value.persons[person.key] = value.filter((candidate) => allowed.has(candidate))
  savedSuccessfully.value = false
}

// backToPlan 返回计划详情；存在真实修改时需要用户确认，首次未保存但未修改不误报。
function backToPlan() {
  if (dirty.value && !window.confirm('当前有未保存的节点配置，确定返回吗？')) return
  router.push('/plans/' + planID.value + '/paths')
}

// configureNextPath 在用户确认保存结果后切到下一条待配置路径，不自动跳转。
function configureNextPath() {
  if (!nextUnconfiguredPath.value) return
  router.push('/plans/' + planID.value + '/paths/' + nextUnconfiguredPath.value.id + '/configure')
}

watch([planID, pathID], () => {
  void loadPage()
})

onBeforeUnmount(() => {
  loadVersion++
  loadController?.abort()
  saveController?.abort()
})

void loadPage()
</script>

<template>
  <main class="path-node-configuration-page" :style="pageThemeStyle">
    <header class="path-node-configuration-page__header">
      <div class="path-node-configuration-page__identity">
        <n-button text type="primary" @click="backToPlan">返回计划详情</n-button>
        <div>
          <h1>路径节点配置</h1>
          <p v-if="plan && configuration">
            {{ plan.name }} · #{{ configuration.path.sequenceNo }} {{ configuration.path.name }}
          </p>
        </div>
      </div>
      <div v-if="configuration" class="path-node-configuration-page__progress" aria-label="节点配置进度">
        <span>已完成 {{ configuration.progress.completed }}</span>
        <span>待处理 {{ configuration.progress.pending }}</span>
        <span>共 {{ configuration.progress.total }}</span>
        <n-tag
          size="small"
          :bordered="false"
          :type="configuration.status === 'configured' ? 'success' : configuration.status === 'affected' ? 'error' : 'warning'"
        >
          {{ configuration.status === 'configured' ? '已保存配置' : configuration.status === 'affected' ? '配置需重新确认' : '待保存配置' }}
        </n-tag>
        <n-button v-if="configuration.nextNodeKey" size="small" secondary @click="selectNextConfigurationNode">
          下一待配置节点
        </n-button>
      </div>
    </header>

    <n-spin :show="loading" class="path-node-configuration-page__stage">
      <n-alert v-if="pageError" type="error" :show-icon="false" class="path-node-configuration-page__error">
        <span>{{ pageError }}</span>
        <n-button size="small" class="path-node-configuration-page__retry" @click="loadPage">重新读取</n-button>
      </n-alert>
      <flow-graph-canvas
        v-else-if="graph && currentPath && configuration"
        ref="canvasRef"
        class="path-node-configuration-page__canvas"
        :graph="graph"
        :choices="currentPath.choices"
        configuration-mode
        :configuration-node-states="configurationNodeStates"
        @select-configuration-node="selectConfigurationNode"
        @retry="loadPage"
      >
        <template #configuration-panel>
          <node-configuration-panel
            :node="selectedNode"
            :warnings="configuration.warnings"
            :draft="draft"
            :saving="saving"
            :save-disabled="saveDisabled"
            :missing-count="requiredState.missing.length"
            :save-error="saveError"
            :save-details="saveDetails"
            :saved-successfully="savedSuccessfully"
            :has-next-path="Boolean(nextUnconfiguredPath)"
            @update-field="updateFieldValue"
            @update-action="updateActionValue"
            @update-person="updatePersonValue"
            @save="saveConfiguration"
            @back-to-plan="backToPlan"
            @configure-next="configureNextPath"
          />
        </template>
      </flow-graph-canvas>
      <n-empty v-else-if="!loading && !pageError" description="暂时没有可配置的路径节点" />
    </n-spin>
  </main>
</template>

<style scoped>
.path-node-configuration-page {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  width: 100%;
  height: calc(100dvh - 144px);
  min-height: 560px;
  overflow: hidden;
  color: var(--path-config-text-color);
  background: var(--path-config-page-color);
}

.path-node-configuration-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 0 0 14px;
}

.path-node-configuration-page__identity {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  min-width: 0;
}

.path-node-configuration-page__identity h1 {
  margin: 0 0 5px;
  font-size: 24px;
  line-height: 1.3;
}

.path-node-configuration-page__identity p {
  max-width: 520px;
  margin: 0;
  overflow: hidden;
  color: var(--path-config-text-secondary-color);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-node-configuration-page__progress {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  color: var(--path-config-text-secondary-color);
  font-size: 13px;
}

.path-node-configuration-page__stage {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--path-config-card-color);
  border: 1px solid var(--path-config-border-color);
  border-radius: 4px;
}

.path-node-configuration-page__stage :deep(.n-spin-content) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.path-node-configuration-page__canvas {
  height: 100%;
  min-height: 0;
  border-top: 0;
}

.path-node-configuration-page__error {
  margin: 20px;
}

.path-node-configuration-page__retry {
  margin-left: 12px;
}

@media (max-width: 1180px) {
  .path-node-configuration-page__header {
    align-items: flex-start;
  }

  .path-node-configuration-page__progress {
    max-width: 360px;
    flex-wrap: wrap;
  }
}
</style>
