<script setup lang="ts">
import { NCard, NCollapse, NCollapseItem, NAlert, NButton, NEmpty, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { analyzeExecutionPath } from '../features/execution-paths/logic'
import { fetchExecutionPaths } from '../features/execution-paths/api'
import type { ExecutionPath } from '../features/execution-paths/types'
import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import FormRuntimeFrame from '../features/path-configuration/FormRuntimeFrame.vue'
import NodeConfigurationPanel from '../features/path-configuration/NodeConfigurationPanel.vue'
import {
  bindPathConfigurationNodes,
  buildPathConfigNodeSavePayload,
  copyPathConfigActionPlan,
  currentNodeConfigurationComplete,
  hasCurrentNodeDraftChanges,
  initialPathConfigurationNodeID,
  initPathConfigDraft,
  nextFormGenerationSeed,
  pathConfigurationNodesByGraphID,
  projectPathConfigurationNodeStates,
  resolveConfirmedNodeSaveDestination,
} from '../features/path-configuration/logic'
import {
  fetchPathConfiguration,
  fetchPathFormRuntimeSession,
  generatePathFormData,
  PathConfigApiError,
  savePathConfigurationNode,
  savePathFormData,
} from '../features/path-configuration/api'
import type {
  PathConfigActionPlanInput,
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
  restoreSaved: () => Promise<Record<string, unknown>>
  getValues: () => Promise<Record<string, unknown>>
  validateAndGetValues: () => Promise<Record<string, unknown>>
  destroyRuntime: () => void
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
const draft = ref<PathConfigDraft>({ fields: {}, actions: {}, persons: {}, personStrategies: {}, actionPlans: {} })
const configurationByGraphNodeID = ref(new Map<string, PathConfigNode>())
const graphNodeIDByConfigurationKey = ref(new Map<string, string>())
const selectedNodeID = ref('')
const workspace = ref<'nodes' | 'form'>('nodes')
const runtimeSession = ref<PathFormRuntimeSession | null>(null)
const runtimeUnsupported = ref<string[]>([])
const loading = ref(false)
const savingNode = ref(false)
const formBusy = ref(false)
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
let nodeSaveKey = ''
let formSaveKey = ''

const pageThemeStyle = computed(() => ({
  '--path-config-page-color': themeVars.value.bodyColor,
  '--path-config-card-color': themeVars.value.cardColor,
  '--path-config-border-color': themeVars.value.borderColor,
  '--path-config-text-color': themeVars.value.textColor1,
  '--path-config-text-secondary-color': themeVars.value.textColor2,
}))
const pathAnalysis = computed(() => graph.value && currentPath.value ? analyzeExecutionPath(graph.value, currentPath.value.choices) : null)
const selectedNode = computed(() => configurationByGraphNodeID.value.get(selectedNodeID.value) ?? null)
const configurationNodeStates = computed(() => graph.value && pathAnalysis.value
  ? projectPathConfigurationNodeStates(graph.value, pathAnalysis.value, configurationByGraphNodeID.value, selectedNodeID.value)
  : {})
const selectedNodeRequirement = computed(() => currentNodeConfigurationComplete(selectedNode.value, draft.value))
const selectedNodeDirty = computed(() => hasCurrentNodeDraftChanges(selectedNode.value, draft.value))
const nodeSaveDisabled = computed(() => loading.value || savingNode.value || !selectedNodeRequirement.value.complete
  || (selectedNode.value?.status === 'configured' && !selectedNodeDirty.value))
const runtimeBlockingReasons = computed(() => [...new Set([
  ...(configuration.value?.form.unsupported ?? []),
  ...runtimeUnsupported.value,
])])
const runtimeBlocked = computed(() => runtimeBlockingReasons.value.length > 0)

// publicPageError 把读取链路异常收敛为不含内部标识的稳定页面错误。
function publicPageError(caught: unknown): string {
  if (caught instanceof PlanApiError || caught instanceof PathConfigApiError) return caught.message
  if (caught instanceof Error && ['已保存路径不存在或已删除', '当前已保存路径与真实流程不一致，请先编辑路径', '路径节点配置与当前流程结构不一致'].includes(caught.message)) return caught.message
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

// loadPage 读取计划、路径、真实图和权威节点/表单配置，切换路径时销毁旧 SID 会话。
async function loadPage() {
  loadController?.abort()
  formFrame.value?.destroyRuntime()
  runtimeSession.value = null
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  pageError.value = ''
  nodeSaveError.value = ''
  formError.value = ''
  nodeSavedSuccessfully.value = false
  formSavedSuccessfully.value = false
  workspace.value = 'nodes'
  try {
    const [storedPlan, storedGraph, storedPaths] = await Promise.all([
      fetchPlan(planID.value, controller.signal), fetchFlowGraph(planID.value, controller.signal), fetchExecutionPaths(planID.value, controller.signal),
    ])
    if (controller.signal.aborted || version !== loadVersion) return
    const storedPath = storedPaths.find(path => path.id === pathID.value)
    if (!storedPath) throw new Error('已保存路径不存在或已删除')
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
    if (version === loadVersion) loading.value = false
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
  const allowed = new Set(person.options.map(option => option.value))
  draft.value.personStrategies[person.key] = { ...value, selected: value.selected.filter(candidate => allowed.has(candidate)) }
  draft.value.persons[person.key] = [...draft.value.personStrategies[person.key].selected]
  nodeSavedSuccessfully.value = false
}

// updateNodeActionPlan 替换当前节点的加签节点与处理结果草稿，不允许面板越过节点边界写其他节点。
function updateNodeActionPlan(nodeKey: string, value: PathConfigActionPlanInput) {
  if (selectedNode.value?.key !== nodeKey) return
  // 子组件事件值仍可能携带 Vue Proxy；父页面只持有逐字段复制后的普通草稿，避免保存前再次触发克隆异常。
  draft.value.actionPlans[nodeKey] = copyPathConfigActionPlan(value)
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
    await savePathConfigurationNode(planID.value, pathID.value, node.key, previousRevision, buildPathConfigNodeSavePayload(node, draft.value), nodeSaveKey)
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
      nodeSaveError.value = caught.message
      nodeSaveDetails.value = caught.details
    }
    else nodeSaveError.value = '保存失败，当前节点和草稿已保留，请重试'
  }
  finally {
    savingNode.value = false
  }
}

// openFormWorkspace 获取短期 SID 后切换到全宽真实表单；SID 不进入配置对象或持久状态。
async function openFormWorkspace() {
  if (!configuration.value || formBusy.value) return
  workspace.value = 'form'
  formError.value = ''
  if (runtimeSession.value) return
  formBusy.value = true
  try {
    runtimeSession.value = await fetchPathFormRuntimeSession(planID.value, pathID.value)
  }
  catch (caught) {
    formError.value = publicPageError(caught)
  }
  finally {
    formBusy.value = false
  }
}

// returnToNodes 销毁 iframe 会话并清除 SID，节点画布保持原有图与选择。
function returnToNodes() {
  formFrame.value?.destroyRuntime()
  runtimeSession.value = null
  runtimeUnsupported.value = []
  workspace.value = 'nodes'
}

// generateFormData 首次生成或换一组；换组仅替换生成器拥有字段，人工覆盖由 runtime 返回。
async function generateFormData(nextGroup: boolean) {
  const current = configuration.value
  if (!current || current.form.readOnly || formBusy.value || !formFrame.value) return
  formBusy.value = true
  formError.value = ''
  formErrorDetails.value = []
  formSavedSuccessfully.value = false
  try {
    const captured = await formFrame.value.getValues()
    const values = (captured.values || current.form.values) as Record<string, unknown>
    const manual = Array.isArray(captured.manualOverridePaths) ? captured.manualOverridePaths.map(String) : current.form.manualOverridePaths
    const seed = nextGroup ? nextFormGenerationSeed(current.form.seed) : current.form.seed
    const generated = await generatePathFormData(planID.value, pathID.value, seed, values, manual)
    current.form.values = generated.values
    current.form.seed = generated.seed
    current.form.status = 'draft'
    current.form.statusName = '草稿待校验'
    current.form.generatedFieldPaths = generated.generatedFieldPaths
    current.form.manualOverridePaths = generated.manualOverridePaths
    current.form.sampleSummary = generated.sampleSummary
    current.form.autoFilled = generated.autoFilled
    current.form.manualPending = generated.manualPending
    current.form.unsupported = generated.unsupported
    await formFrame.value.setGeneratedData(generated.values, generated.generatedFieldPaths, generated.manualOverridePaths)
  }
  catch (caught) {
    formError.value = publicPageError(caught)
  }
  finally {
    formBusy.value = false
  }
}

// restoreSavedForm 恢复本次 GET 装载的服务端 values，不重读或重新生成。
async function restoreSavedForm() {
  if (!formFrame.value || formBusy.value) return
  formBusy.value = true
  formError.value = ''
  try { await formFrame.value.restoreSaved() }
  catch (caught) { formError.value = publicPageError(caught) }
  finally { formBusy.value = false }
}

// saveFormData 先经真实 getData(true)/getValues，再由服务端按最新模板与路径复验并独立保存。
async function saveFormData() {
  const current = configuration.value
  if (!current || current.form.readOnly || runtimeBlocked.value || formBusy.value || !formFrame.value) return
  formBusy.value = true
  formError.value = ''
  formErrorDetails.value = []
  formSavedSuccessfully.value = false
  const previousRevision = current.form.revision
  try {
    const captured = await formFrame.value.validateAndGetValues()
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
    })
    await reloadConfiguration()
    formSaveKey = crypto.randomUUID()
    formSavedSuccessfully.value = true
    // 保存成功后自动回到节点画布；发起人节点上会显示“表单已配置”提示。
    returnToNodes()
  }
  catch (caught) {
    try {
      const reconciled = await reloadConfiguration()
      if (reconciled.form.revision > previousRevision && reconciled.form.status === 'valid') {
        formSaveKey = crypto.randomUUID()
        formSavedSuccessfully.value = true
        returnToNodes()
        return
      }
    }
    catch { /* 对账失败保留当前 iframe、values 和幂等键。 */ }
    formError.value = caught instanceof Error ? caught.message : '保存失败，当前表单数据已保留，请重试'
    formErrorDetails.value = caught instanceof PathConfigApiError ? caught.details : []
  }
  finally {
    formBusy.value = false
  }
}

// backToPlan 离开页面前销毁短期运行时会话；节点或表单未保存时由各自保存状态保持现场。
function backToPlan() {
  formFrame.value?.destroyRuntime()
  runtimeSession.value = null
  router.push('/plans/' + planID.value + '/paths')
}

watch([planID, pathID], () => { void loadPage() })
onBeforeUnmount(() => {
  loadVersion++
  loadController?.abort()
  formFrame.value?.destroyRuntime()
  runtimeSession.value = null
})

void loadPage()
</script>

<template>
  <main
    class="path-configuration-page"
    :class="{ 'path-configuration-page--form': workspace === 'form' }"
    :style="pageThemeStyle"
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
        <span>节点 {{ configuration.progress.completed }} / {{ configuration.progress.total }}</span>
        <span>表单：{{ configuration.form.statusName }}</span>
        <n-tag size="small" :bordered="false" :type="configuration.status === 'configured' ? 'success' : configuration.status === 'affected' ? 'error' : 'warning'">
          {{ configuration.status === 'configured' ? '路径已配置' : configuration.status === 'affected' ? '需要重新确认' : '待配置' }}
        </n-tag>
        <n-button v-if="workspace === 'nodes' && configuration.nextNodeKey" size="small" secondary @click="selectNextConfigurationNode">下一待配置节点</n-button>
      </div>
    </header>

    <nav v-if="configuration" class="path-configuration-page__switch" aria-label="配置工作区">
      <n-button :type="workspace === 'nodes' ? 'primary' : 'default'" :secondary="workspace !== 'nodes'" @click="returnToNodes">节点配置</n-button>
      <n-button :type="workspace === 'form' ? 'primary' : 'default'" :secondary="workspace !== 'form'" @click="openFormWorkspace">表单数据</n-button>
    </nav>

    <n-spin :show="loading || formBusy" class="path-configuration-page__stage">
      <n-alert v-if="pageError" type="error" :show-icon="false" class="path-configuration-page__error">
        <span>{{ pageError }}</span>
        <n-button size="small" @click="loadPage">重新读取</n-button>
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
        :configuration-form-status-name="configuration.form.statusName"
        @select-configuration-node="selectConfigurationNode"
        @open-configuration-form="openFormWorkspace"
        @retry="loadPage"
      >
        <template #configuration-panel>
          <node-configuration-panel
            :node="selectedNode"
            :draft="draft"
            :saving="savingNode"
            :save-disabled="nodeSaveDisabled"
            :missing-count="selectedNodeRequirement.missing.length"
            :save-error="nodeSaveError"
            :save-details="nodeSaveDetails"
            :saved-successfully="nodeSavedSuccessfully"
            :form-complete="configuration.form.status === 'valid'"
            @update-person-strategy="updatePersonStrategy"
            @update-action-plan="updateNodeActionPlan"
            @save="saveCurrentNode"
            @back-to-plan="backToPlan"
            @open-form="openFormWorkspace"
          />
        </template>
      </flow-graph-canvas>

      <section v-else-if="workspace === 'form' && configuration" class="path-configuration-page__form-workspace">
        <header class="path-configuration-page__form-toolbar">
          <div>
            <h2>表单数据</h2>
            <p v-if="configuration.form.readOnly">已发/待发路径使用实例当前值，只读且不会写回目标平台。</p>
            <p v-else>自动填充 {{ configuration.form.autoFilled }} 项 · 仍需手工 {{ configuration.form.manualPending }} 项</p>
          </div>
          <div class="path-configuration-page__form-actions">
            <n-button size="small" @click="returnToNodes">返回节点画布</n-button>
            <template v-if="!configuration.form.readOnly">
              <n-button size="small" :disabled="formBusy" @click="generateFormData(false)">智能生成</n-button>
              <n-button size="small" :disabled="formBusy" @click="generateFormData(true)">换一组</n-button>
              <n-button size="small" :disabled="formBusy" @click="restoreSavedForm">恢复已保存</n-button>
              <n-button size="small" type="primary" :loading="formBusy" :disabled="runtimeBlocked" @click="saveFormData">保存表单数据</n-button>
            </template>
          </div>
        </header>
        <n-card v-if="configuration.form.conditionHints.length" size="small" class="path-configuration-page__form-hints">
          <n-collapse :default-expanded-names="['condition-hints']" arrow-placement="right">
            <n-collapse-item title="分支关键数据提示" name="condition-hints">
              <div class="path-configuration-page__form-hints-body">
                <p>以下关键数据被当前路径的条件分支使用，仅作参考提示；修改这些字段可能影响实际分支走向。</p>
                <ul>
                  <li v-for="(hint, index) in configuration.form.conditionHints" :key="`${hint.field}-${index}`">{{ hint.text }}</li>
                </ul>
              </div>
            </n-collapse-item>
          </n-collapse>
        </n-card>
        <div v-if="formError || formSavedSuccessfully || runtimeBlocked" class="path-configuration-page__form-feedback">
          <n-alert v-if="formError" type="error" :show-icon="false" size="small">
            <div>{{ formError }}</div>
            <ul v-if="formErrorDetails.length" class="path-configuration-page__form-error-details">
              <li v-for="(item, index) in formErrorDetails" :key="`${item.kind}-${index}`">{{ item.reason || item.name }}</li>
            </ul>
          </n-alert>
          <n-alert v-else-if="formSavedSuccessfully" type="success" :show-icon="false" size="small">
            表单数据已保存并完成服务端复验。节点仍需逐个完成，整条路径不会被静默标记。
          </n-alert>
          <n-alert v-if="runtimeBlocked" type="warning" :show-icon="false" size="small">
            {{ runtimeBlockingReasons.join('；') }}
          </n-alert>
        </div>
        <form-runtime-frame
          v-if="runtimeSession"
          ref="formFrame"
          class="path-configuration-page__form-frame"
          :form="configuration.form"
          :runtime-session="runtimeSession"
          @ready="(items) => runtimeUnsupported = items"
          @error="(message) => formError = message"
        />
        <n-empty v-else-if="!formBusy" description="表单运行时会话暂不可用，请返回节点画布后重试" />
      </section>

      <n-empty v-else-if="!loading && !pageError" description="暂时没有可配置内容" />
    </n-spin>
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

.path-configuration-page__header { justify-content: space-between; gap: 20px; padding-bottom: 10px; }
.path-configuration-page__identity { align-items: flex-start; gap: 16px; min-width: 0; }
.path-configuration-page__identity h1 { margin: 0 0 4px; font-size: 24px; }
.path-configuration-page__identity p { margin: 0; color: var(--path-config-text-secondary-color); }
.path-configuration-page__progress { justify-content: flex-end; gap: 10px; font-size: 13px; color: var(--path-config-text-secondary-color); }
.path-configuration-page__switch { gap: 8px; padding: 4px 0 12px; border-bottom: 1px solid var(--path-config-border-color); }

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

.path-configuration-page__stage :deep(.n-spin-content) { width: 100%; height: 100%; min-height: 0; }
.path-configuration-page__canvas { height: 100%; min-height: 0; border-top: 0; }
.path-configuration-page__error { margin: 20px; }

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
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.14);
}
.path-configuration-page__form-hints-body {
  max-height: 300px;
  overflow-y: auto;
}
.path-configuration-page__form-hints-body p {
  margin: 0 0 6px;
  color: var(--path-config-text-secondary-color);
  font-size: 12px;
}
.path-configuration-page__form-hints-body ul {
  margin: 0;
  padding-left: 18px;
  color: var(--path-config-text-secondary-color);
  font-size: 12px;
  line-height: 1.8;
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
  .path-configuration-page__progress { flex-wrap: wrap; justify-content: flex-start; }
  .path-configuration-page__stage { min-height: 640px; }
  .path-configuration-page--form > .path-configuration-page__stage,
  .path-configuration-page__form-workspace { min-height: 0; }
  .path-configuration-page__form-toolbar { flex: 0 0 auto; }
}
</style>
