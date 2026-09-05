<script setup lang="ts">
import { Controls } from '@vue-flow/controls'
import { NButton, NEmpty, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { VueFlow as VueFlowCanvas, useVueFlow } from '@vue-flow/core'
import type { NodeChange, NodeMouseEvent } from '@vue-flow/core'

import FlowGraphNode from './FlowGraphNode.vue'
import FlowRoutingHub from './FlowRoutingHub.vue'
import FlowTreeEdge from './FlowTreeEdge.vue'
import {
  compensateViewportForContainerWidth,
  flowNodeWidth,
  flowRoutingHubSize,
  initialViewportForGraph,
  safeLayoutFlowGraph,
  shouldSetInitialViewport,
} from './layout'
import type { FlowConfigurationNodeState, FlowGraph } from './types'
import {
  analyzeExecutionPath,
  classifyExecutionPathEdges,
  nextExecutionPathRouteID,
  projectExecutionPathGuide,
  viewportForCandidateGroupCentered,
  viewportForPointCentered,
} from '../execution-paths/logic'
import type { ExecutionPathChoice } from '../execution-paths/types'
import type { ExecutionPathGuideCandidate } from '../execution-paths/logic'

const props = withDefaults(defineProps<{
  graph: FlowGraph
  choices?: ExecutionPathChoice[]
  workspaceOpen?: boolean
  branchEditing?: boolean
  workspaceExitDisabled?: boolean
  saveGuideVisible?: boolean
  savedPathsOpen?: boolean
  configurationMode?: boolean
  configurationNodeStates?: Record<string, FlowConfigurationNodeState>
  configurationFormStatus?: string
  configurationFormStatusName?: string
  // runMode 是 F-016 的运行画布变体：真实节点承载九个中文运行态与当前步指示器。
  runMode?: boolean
  runNodeStates?: Record<string, { status: string; statusName: string }>
  currentRunNodeKey?: string
  // runTakenEdgeIds 是运行中真实走过的连线（按步骤顺序相邻节点连接）；
  // runDeviationEdgeIds 是其中偏离已配置路径的连线（标红显示）。
  runTakenEdgeIds?: string[]
  runDeviationEdgeIds?: string[]
}>(), {
  choices: () => [], workspaceOpen: false, branchEditing: false, workspaceExitDisabled: false, saveGuideVisible: false, savedPathsOpen: false,
  configurationMode: false, configurationNodeStates: () => ({}), configurationFormStatus: '', configurationFormStatusName: '',
  runMode: false, runNodeStates: () => ({}), currentRunNodeKey: '', runTakenEdgeIds: () => [], runDeviationEdgeIds: () => [],
})
const emit = defineEmits<{
  retry: []
  selectBranch: [choice: ExecutionPathChoice]
  closeSavedPaths: []
  requestWorkspaceExit: []
  selectConfigurationNode: [nodeID: string]
  openConfigurationForm: []
  selectRunNode: [nodeID: string]
  runViewportChange: [viewport: { x: number, y: number, zoom: number }]
}>()
const themeVars = useThemeVars()
const canvasRoot = ref<HTMLElement | null>(null)
const isPageFullscreen = ref(false)
const isSelectionPanelCollapsed = ref(false)
const canvasSize = ref({ width: 0, height: 0 })
const viewportState = ref({ x: 0, y: 0, zoom: 1 })
const guideBubble = ref<{
  key: string
  message: string
  candidates: ExecutionPathGuideCandidate[]
  complete: boolean
} | null>(null)
const dismissedGuideKey = ref('')
const layoutResult = computed(() => safeLayoutFlowGraph(props.graph))
const laidOut = computed(() => layoutResult.value.layout)
const pathAnalysis = computed(() => analyzeExecutionPath(props.graph, props.choices))
const displayedLayout = computed(() => {
  if (!laidOut.value || (!props.workspaceOpen && !props.configurationMode && !props.runMode)) return laidOut.value
  const analysis = pathAnalysis.value
  const edgeStates = classifyExecutionPathEdges(props.graph, analysis, props.choices)
  // 运行态叠加：真实走过的连线加粗带流向，偏离已配置路径的连线标红（T08 要求）。
  const takenIds = new Set(props.runTakenEdgeIds || [])
  const deviatedIds = new Set(props.runDeviationEdgeIds || [])
  const showRunEdges = props.runMode && (takenIds.size > 0 || deviatedIds.size > 0)
  return {
    nodes: laidOut.value.nodes.map((node) => {
      const configurationState = props.configurationNodeStates[node.id]
      const configurationInteractive = Boolean(props.configurationMode && configurationState?.interactive)
      // 运行模式：路径内节点可点开侧栏，路径外节点弱化只读。
      const runState = props.runNodeStates[node.id]
      const runInteractive = Boolean(props.runMode && analysis.reachableNodeIds.has(node.id))
      return {
        ...node,
        // Vue Flow 会用节点自身的 selectable/focusable 覆盖全局只读值；仅当前路径节点进入官方事件链。
        selectable: props.runMode ? runInteractive : configurationInteractive,
        focusable: props.runMode ? runInteractive : configurationInteractive,
        draggable: false,
        connectable: false,
        deletable: false,
        ariaLabel: props.runMode
          ? `${node.data?.name || '流程节点'}，${runState?.statusName || '未开始'}${node.id === props.currentRunNodeKey ? '，当前步' : ''}`
          : configurationInteractive
            ? `${node.data?.name || '流程节点'}，${configurationState?.statusName || '待配置'}，按回车或空格选择节点`
            : `${node.data?.name || '流程节点'}，不可配置`,
        class: analysis.reachableNodeIds.has(node.id) ? 'flow-node--path-active' : 'flow-node--path-muted',
        data: {
          ...node.data,
          configurationMode: props.configurationMode && !props.runMode,
          configurationStatus: configurationState?.status,
          configurationStatusName: configurationState?.statusName,
          configurationInteractive,
          configurationSelected: configurationState?.selected ?? false,
          configurationFormStatus: props.configurationFormStatus,
          configurationFormStatusName: props.configurationFormStatusName,
          runMode: props.runMode,
          runStatus: runState?.status,
          runStatusName: runState?.statusName,
          runCurrent: props.runMode && node.id === props.currentRunNodeKey,
        },
      }
    }),
    edges: laidOut.value.edges.map((edge) => {
      const kind = edge.data?.kind
      let state: { selected: boolean; candidate: boolean; dimmed: boolean; active: boolean; taken?: boolean; deviated?: boolean }
        = edgeStates.get(edge.id) ?? { selected: false, candidate: false, dimmed: true, active: false }
      if (showRunEdges) {
        const taken = takenIds.has(edge.id)
        const deviated = deviatedIds.has(edge.id)
        state = {
          ...state,
          taken,
          deviated,
          // 被走过的边不再弱化；偏离边以独立红色表达。
          dimmed: state.dimmed && !taken && !deviated,
        }
      }
      return {
        ...edge,
        data: edge.data
          ? {
              ...edge.data,
              workspaceOpen: true,
              branchEditing: props.configurationMode ? false : props.branchEditing,
              ...state,
              parallelRequired: kind === 'parallel' && state.selected,
            }
          : edge.data,
      }
    }),
  }
})
const canvasStyle = computed(() => ({
  '--flow-edge-color': themeVars.value.borderColor,
  '--flow-label-color': themeVars.value.textColor2,
  '--flow-surface-color': themeVars.value.bodyColor,
  '--flow-direction-color': themeVars.value.primaryColor,
  '--success-color': themeVars.value.successColor,
  '--warning-color': themeVars.value.warningColor,
  '--error-color': themeVars.value.errorColor,
  '--info-color': themeVars.value.infoColor,
}))
const { getViewport, onInit, setViewport } = useVueFlow()
let ready = false
let positionedPlanId = ''
let viewportVersion = 0
let requestedPageFullscreen = false
let pageFullscreenTask: Promise<void> | null = null
let pageFullscreenDisposed = false
let previousDocumentOverflow: string | null = null
let guideVersion = 0
let pendingGuideAnchor = ''
let canvasResizeObserver: ResizeObserver | null = null

const reservedRight = computed(() => props.workspaceOpen || props.configurationMode ? 336 : 0)
const guideProjection = computed(() => guideBubble.value
  ? projectExecutionPathGuide(
      guideBubble.value.candidates,
      viewportState.value,
      canvasSize.value,
      reservedRight.value,
    )
  : null)

function reducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

async function setInitialViewport() {
  const version = ++viewportVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== viewportVersion || !shouldSetInitialViewport(ready, positionedPlanId, props.graph.planId)) return
  if (!laidOut.value) return
  const viewport = initialViewportForGraph(laidOut.value.nodes, canvasRoot.value?.clientWidth ?? 0)
  if (!viewport) return
  positionedPlanId = props.graph.planId
  await setViewport(viewport, { duration: reducedMotion() ? 0 : 220 })
  viewportState.value = viewport
}

function setDocumentScrollLocked(locked: boolean) {
  if (locked && previousDocumentOverflow === null) {
    previousDocumentOverflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
  }
  else if (!locked && previousDocumentOverflow !== null) {
    document.documentElement.style.overflow = previousDocumentOverflow
    previousDocumentOverflow = null
  }
}

function clearGuideBubble() {
  guideBubble.value = null
}

function dismissGuideBubble() {
  if (guideBubble.value) dismissedGuideKey.value = guideBubble.value.key
  clearGuideBubble()
}

function guideCandidatesForRoute(routeNodeID: string): ExecutionPathGuideCandidate[] {
  if (!laidOut.value) return []
  return laidOut.value.edges.flatMap((edge): ExecutionPathGuideCandidate[] => {
    if (edge.data?.routeNodeId !== routeNodeID
      || (edge.data.kind !== 'condition' && edge.data.kind !== 'manual')
      || edge.data.labelX === undefined
      || edge.data.labelY === undefined) return []
    return [{ id: edge.id, x: edge.data.labelX, y: edge.data.labelY }]
  })
}

async function guideSelectionNext(anchorNodeID = '') {
  const version = ++guideVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== guideVersion || !props.workspaceOpen || !props.branchEditing || !laidOut.value || !canvasRoot.value) return

  const nextRouteID = nextExecutionPathRouteID(pathAnalysis.value)
  if (!nextRouteID && !props.saveGuideVisible) {
    clearGuideBubble()
    return
  }
  const targetID = nextRouteID || anchorNodeID || props.graph.entryNodeIds[0]
  const targetNode = laidOut.value.nodes.find((node) => node.id === targetID)
  if (!targetNode) return
  const nodeWidth = targetNode.type === 'routingHub' ? flowRoutingHubSize : flowNodeWidth
  const point = { x: targetNode.position.x + nodeWidth / 2, y: targetNode.position.y + 20 }
  const viewport = getViewport()
  const candidates = nextRouteID ? guideCandidatesForRoute(nextRouteID) : [{ id: targetID, ...point }]
  const container = { width: canvasRoot.value.clientWidth, height: canvasRoot.value.clientHeight }
  // 下一步以整组候选标签为定位基准，候选再宽也只平移而不改变用户缩放；完成态继续定位最后操作点。
  const nextViewport = nextRouteID
    ? viewportForCandidateGroupCentered(viewport, candidates, container, reservedRight.value)
    : viewportForPointCentered(viewport, point, container, reservedRight.value)
  await setViewport(nextViewport, { duration: reducedMotion() ? 0 : 250 })
  if (version !== guideVersion || !props.workspaceOpen || !props.branchEditing || !canvasRoot.value) return
  viewportState.value = nextViewport
  const guideKey = nextRouteID || `complete:${anchorNodeID || targetID}`
  clearGuideBubble()
  if (dismissedGuideKey.value === guideKey) return
  guideBubble.value = {
    key: guideKey,
    message: nextRouteID
      ? `请在以下 ${candidates.length} 个分支中选择 1 个（还剩 ${pathAnalysis.value.missingRouteNodeIds.length} 处）`
      : '线路已完整，请保存',
    candidates,
    complete: !nextRouteID,
  }
}

function toggleSelectionPanel() {
  isSelectionPanelCollapsed.value = !isSelectionPanelCollapsed.value
  if (isSelectionPanelCollapsed.value) emit('closeSavedPaths')
}

function handleViewportChange(viewport: { x: number, y: number, zoom: number }) {
  // 引导端点必须跟随 Vue Flow 当前变换，拖动或缩放后不能继续指向旧屏幕坐标。
  viewportState.value = { x: viewport.x, y: viewport.y, zoom: viewport.zoom }
  if (props.runMode) {
    // 运行画布把视口变化交给详情页判断"自动跟随是否被用户接管"。
    emit('runViewportChange', viewport)
  }
}

function guideArrowPath(candidate: ExecutionPathGuideCandidate): string {
  if (!guideProjection.value) return ''
  const startX = guideProjection.value.bubble.x
  const startY = guideProjection.value.bubble.y + 23
  const endY = candidate.y - 19
  const railY = startY + Math.max(14, (endY - startY) * 0.48)
  return `M ${startX} ${startY} L ${startX} ${railY} L ${candidate.x} ${railY} L ${candidate.x} ${endY}`
}

function requestPageFullscreen(next: boolean) {
  requestedPageFullscreen = next
  if (!pageFullscreenTask) {
    pageFullscreenTask = runPageFullscreenTransitions().finally(() => {
      pageFullscreenTask = null
      if (!pageFullscreenDisposed && isPageFullscreen.value !== requestedPageFullscreen) {
        void requestPageFullscreen(requestedPageFullscreen)
      }
    })
  }
  return pageFullscreenTask
}

async function runPageFullscreenTransitions() {
  while (!pageFullscreenDisposed && isPageFullscreen.value !== requestedPageFullscreen) {
    const next = requestedPageFullscreen
    const beforeWidth = canvasRoot.value?.clientWidth ?? 0
    const viewport = getViewport()
    isPageFullscreen.value = next
    setDocumentScrollLocked(next)
    await nextTick()
    await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
    if (pageFullscreenDisposed) return
    if (!laidOut.value) {
      if (!next) setDocumentScrollLocked(false)
      continue
    }
    const afterWidth = canvasRoot.value?.clientWidth ?? 0
    const compensated = compensateViewportForContainerWidth(viewport, beforeWidth, afterWidth)
    await setViewport(compensated, { duration: 0 })
    viewportState.value = compensated
  }
  if (!isPageFullscreen.value) setDocumentScrollLocked(false)
}

function handlePageFullscreenKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !isPageFullscreen.value) return
  if (props.branchEditing && props.workspaceExitDisabled) return
  if (props.savedPathsOpen) {
    emit('closeSavedPaths')
    return
  }
  if (props.branchEditing) {
    emit('requestWorkspaceExit')
    return
  }
  void requestPageFullscreen(false)
}

function togglePageFullscreen() {
  if (isPageFullscreen.value && props.branchEditing) {
    if (props.workspaceExitDisabled) return
    emit('requestWorkspaceExit')
    return
  }
  void requestPageFullscreen(!isPageFullscreen.value)
}

async function setPageFullscreen(next: boolean) {
  await requestPageFullscreen(next)
}

async function focusNode(nodeID: string) {
  await nextTick()
  if (!laidOut.value || !canvasRoot.value) return
  const targetNode = laidOut.value.nodes.find((node) => node.id === nodeID)
  if (!targetNode) return
  const nodeWidth = targetNode.type === 'routingHub' ? flowRoutingHubSize : flowNodeWidth
  const point = {
    x: targetNode.position.x + nodeWidth / 2,
    y: targetNode.position.y + (targetNode.type === 'routingHub' ? flowRoutingHubSize : 72) / 2,
  }
  // 节点配置定位只在当前缩放下平移到扣除右侧面板的安全区域，不 fitView，也不改变用户对大图的阅读尺度。
  const current = getViewport()
  const nextViewport = viewportForPointCentered(
    current,
    point,
    { width: canvasRoot.value.clientWidth, height: canvasRoot.value.clientHeight },
    reservedRight.value,
  )
  await setViewport(nextViewport, { duration: reducedMotion() ? 0 : 220 })
  viewportState.value = nextViewport
}

defineExpose({ setPageFullscreen, focusNode })

function handleSelectBranch(choice: ExecutionPathChoice) {
  if (!props.workspaceOpen || !props.branchEditing) return
  pendingGuideAnchor = choice.routeNodeId
  emit('selectBranch', choice)
}

function handleSelectConfigurationNode(nodeID: string) {
  if (!props.configurationMode || !props.configurationNodeStates[nodeID]?.interactive) return
  emit('selectConfigurationNode', nodeID)
}

// handleConfigurationNodeClick 使用 Vue Flow 官方节点点击事件，确保包装层而非内部样式承担 pointer 边界。
function handleConfigurationNodeClick({ node }: NodeMouseEvent) {
  // 运行模式下节点点击交给运行侧栏；配置模式维持原行为。
  if (props.runMode) {
    emit('selectRunNode', node.id)
    return
  }
  handleSelectConfigurationNode(node.id)
}

// handleConfigurationNodeChanges 接住包装层 Enter/Space 产生的选择变更；不可配置节点不会产生有效切换。
function handleConfigurationNodeChanges(changes: NodeChange[]) {
  if (!props.configurationMode) return
  for (const change of changes) {
    if (change.type !== 'select' || !change.selected) continue
    handleSelectConfigurationNode(change.id)
    return
  }
}

onInit(() => {
  ready = true
  void setInitialViewport()
})

watch(() => props.graph.planId, () => {
  void setInitialViewport()
})
watch(() => props.graph, () => {
  // 保存冲突换入同计划最新图时保持当前缩放，只重新定位仍待选择的下一处。
  if (props.workspaceOpen && props.branchEditing) void guideSelectionNext()
})
watch(() => props.choices, () => {
  const anchor = pendingGuideAnchor
  pendingGuideAnchor = ''
  if (props.workspaceOpen && props.branchEditing) void guideSelectionNext(anchor)
})
watch(laidOut, (value) => {
  if (value || !isPageFullscreen.value) return
  void requestPageFullscreen(false)
})
watch(isPageFullscreen, (value) => {
  if (!value) emit('closeSavedPaths')
})
watch(() => props.workspaceOpen, (enabled) => {
  if (!enabled) {
    isSelectionPanelCollapsed.value = false
    guideVersion++
    clearGuideBubble()
    emit('closeSavedPaths')
    return
  }
  if (props.branchEditing) void guideSelectionNext()
})
watch(() => props.branchEditing, (enabled) => {
  // 保存成功和加载已保存路径都会切回查看态；此时必须立即清理保存引导，不能等待下一次路径变化。
  if (!enabled) {
    guideVersion++
    clearGuideBubble()
    return
  }
  if (props.workspaceOpen) void guideSelectionNext()
})
watch(() => props.saveGuideVisible, () => {
  if (props.workspaceOpen && props.branchEditing) void guideSelectionNext()
})
watch(isSelectionPanelCollapsed, () => {
  if (props.workspaceOpen && props.branchEditing) void guideSelectionNext()
})
watch(() => props.savedPathsOpen, (open) => {
  if (!open && props.workspaceOpen && props.branchEditing && !guideBubble.value) void guideSelectionNext()
})

onMounted(() => {
  document.addEventListener('keydown', handlePageFullscreenKeydown)
  if (canvasRoot.value) {
    canvasSize.value = { width: canvasRoot.value.clientWidth, height: canvasRoot.value.clientHeight }
    canvasResizeObserver = new ResizeObserver(([entry]) => {
      if (!entry) return
      canvasSize.value = { width: entry.contentRect.width, height: entry.contentRect.height }
    })
    canvasResizeObserver.observe(canvasRoot.value)
  }
})
onBeforeUnmount(() => {
  viewportVersion++
  guideVersion++
  pageFullscreenDisposed = true
  requestedPageFullscreen = false
  clearGuideBubble()
  canvasResizeObserver?.disconnect()
  canvasResizeObserver = null
  setDocumentScrollLocked(false)
  document.removeEventListener('keydown', handlePageFullscreenKeydown)
})
</script>

<template>
  <div
    ref="canvasRoot"
    class="flow-graph-canvas"
    :class="{
      'flow-graph-canvas--page-fullscreen': isPageFullscreen,
      'flow-graph-canvas--workspace': workspaceOpen,
      'flow-graph-canvas--configuration': configurationMode,
      'flow-graph-canvas--panel-collapsed': isSelectionPanelCollapsed,
    }"
    :style="canvasStyle"
    :aria-label="configurationMode ? '路径节点配置流程图' : workspaceOpen ? (branchEditing ? '线路编辑流程图' : '路径查看流程图') : '只读流程图'"
  >
    <div v-if="laidOut" class="flow-graph-canvas__actions">
      <slot v-if="isPageFullscreen" name="canvas-actions" />
      <slot v-else name="canvas-actions-normal" />
      <n-button
        class="flow-graph-canvas__fullscreen-button"
        size="small"
        secondary
        :aria-pressed="isPageFullscreen"
        :aria-label="isPageFullscreen ? '退出全屏' : '页面全屏'"
        :title="isPageFullscreen ? '退出全屏' : '页面全屏'"
        :disabled="branchEditing && workspaceExitDisabled"
        @click="togglePageFullscreen"
      >
        {{ isPageFullscreen ? '退出全屏' : '页面全屏' }}
      </n-button>
    </div>
    <vue-flow-canvas
      v-if="laidOut"
      :nodes="displayedLayout?.nodes"
      :edges="displayedLayout?.edges"
      :nodes-draggable="false"
      :nodes-connectable="false"
      :elements-selectable="false"
      :nodes-focusable="configurationMode"
      :select-nodes-on-drag="false"
      :delete-key-code="null"
      :multi-selection-key-code="null"
      :zoom-on-double-click="false"
      :pan-on-drag="true"
      :zoom-on-scroll="true"
      :zoom-on-pinch="true"
      :prevent-scrolling="true"
      :min-zoom="0.15"
      :max-zoom="2"
      :fit-view-on-init="false"
      @node-click="handleConfigurationNodeClick"
      @nodes-change="handleConfigurationNodeChanges"
      @viewport-change="handleViewportChange"
    >
      <template #node-flowNode="{ data }">
        <flow-graph-node :data="data" @open-form="emit('openConfigurationForm')" />
      </template>
      <template #node-routingHub="{ data }">
        <flow-routing-hub :data="data" />
      </template>
      <template #edge-treeEdge="edgeProps">
        <flow-tree-edge v-bind="edgeProps" @select-branch="handleSelectBranch" />
      </template>
      <controls position="bottom-right" :show-interactive="false" />
    </vue-flow-canvas>
    <aside
      v-if="laidOut && workspaceOpen"
      class="flow-graph-canvas__selection-panel"
      :class="{ 'flow-graph-canvas__selection-panel--collapsed': isSelectionPanelCollapsed }"
      aria-label="线路选择面板"
    >
      <n-button
        class="flow-graph-canvas__panel-toggle"
        size="small"
        text
        :aria-expanded="!isSelectionPanelCollapsed"
        @click="toggleSelectionPanel"
      >
        {{ isSelectionPanelCollapsed ? '展开' : '收起' }}
      </n-button>
      <div v-if="!isSelectionPanelCollapsed" class="flow-graph-canvas__selection-panel-content">
        <slot name="workspace-panel" />
      </div>
      <div v-else class="flow-graph-canvas__selection-panel-collapsed-content">
        <slot name="workspace-collapsed" />
      </div>
    </aside>
    <aside
      v-if="laidOut && configurationMode"
      class="flow-graph-canvas__configuration-panel"
      aria-label="节点配置面板"
    >
      <slot name="configuration-panel" />
    </aside>
    <aside
      v-if="laidOut && isPageFullscreen && savedPathsOpen && (!workspaceOpen || !isSelectionPanelCollapsed)"
      class="flow-graph-canvas__saved-paths"
      aria-label="已保存路径"
    >
      <slot name="saved-paths" />
    </aside>
    <svg
      v-if="guideBubble && guideProjection && workspaceOpen && branchEditing && !savedPathsOpen && !guideBubble.complete"
      class="flow-graph-canvas__guide-lines"
      aria-hidden="true"
    >
      <defs>
        <marker id="flow-guide-arrow" viewBox="0 0 10 10" ref-x="8" ref-y="5" marker-width="5" marker-height="5" orient="auto-start-reverse">
          <path d="M 0 0 L 10 5 L 0 10 z" />
        </marker>
      </defs>
      <path
        v-for="candidate in guideProjection.visibleCandidates"
        :key="candidate.id"
        class="flow-graph-canvas__guide-line"
        :d="guideArrowPath(candidate)"
        marker-end="url(#flow-guide-arrow)"
      />
    </svg>
    <div
      v-if="guideBubble && guideProjection && workspaceOpen && branchEditing && !savedPathsOpen"
      class="flow-graph-canvas__guide"
      :style="{ left: `${guideProjection.bubble.x}px`, top: `${guideProjection.bubble.y}px` }"
      role="status"
      aria-live="polite"
    >
      <span>{{ guideBubble.message }}</span>
      <button type="button" aria-label="关闭提示" @click="dismissGuideBubble">×</button>
    </div>
    <div
      v-if="guideBubble && guideProjection?.hiddenLeftCount && workspaceOpen && branchEditing && !savedPathsOpen"
      class="flow-graph-canvas__guide-overflow flow-graph-canvas__guide-overflow--left"
    >
      ← 还有 {{ guideProjection.hiddenLeftCount }} 个候选
    </div>
    <div
      v-if="guideBubble && guideProjection?.hiddenRightCount && workspaceOpen && branchEditing && !savedPathsOpen"
      class="flow-graph-canvas__guide-overflow flow-graph-canvas__guide-overflow--right"
      :style="{ right: `${reservedRight + 8}px` }"
    >
      还有 {{ guideProjection.hiddenRightCount }} 个候选 →
    </div>
    <div v-if="!laidOut" class="flow-graph-canvas__error">
      <n-empty :description="layoutResult.error">
        <template #extra>
          <n-button type="primary" secondary @click="emit('retry')">重试</n-button>
        </template>
      </n-empty>
    </div>
  </div>
</template>

<style scoped>
.flow-graph-canvas {
  position: relative;
  width: 100%;
  height: 560px;
  min-height: 560px;
  overflow: hidden;
  background: var(--flow-surface-color);
  border-top: 1px solid var(--flow-edge-color);
}

.flow-graph-canvas--page-fullscreen {
  position: fixed;
  inset: 0;
  z-index: 1000;
  width: auto;
  height: auto;
  min-height: 0;
  border: 0;
}

.flow-graph-canvas__actions {
  position: absolute;
  top: 12px;
  right: 16px;
  z-index: 6;
  display: flex;
  gap: 8px;
}

.flow-graph-canvas__selection-panel {
  position: absolute;
  right: 12px;
  bottom: 12px;
  z-index: 7;
  width: 320px;
  overflow: hidden;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
  height: calc(100% - 68px);
  transition: height 160ms ease;
}

.flow-graph-canvas__configuration-panel {
  position: absolute;
  top: 56px;
  right: 12px;
  bottom: 12px;
  z-index: 7;
  width: 320px;
  min-height: 0;
  overflow: hidden;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
}

.flow-graph-canvas__selection-panel--collapsed {
  height: 104px;
}

.flow-graph-canvas__panel-toggle {
  position: absolute;
  top: 6px;
  right: 6px;
  z-index: 2;
  min-width: 32px;
  min-height: 32px;
}

.flow-graph-canvas__selection-panel-content {
  width: 100%;
  height: 100%;
  padding-top: 40px;
}

.flow-graph-canvas__selection-panel-collapsed-content {
  width: 100%;
  height: 100%;
  padding: 10px 52px 10px 14px;
}

.flow-graph-canvas__saved-paths {
  position: absolute;
  top: 104px;
  right: 16px;
  z-index: 7;
  width: 280px;
  max-height: 320px;
  overflow: hidden;
  color: var(--flow-label-color);
  background: color-mix(in srgb, var(--flow-surface-color) 90%, transparent);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
  -webkit-backdrop-filter: blur(8px);
  backdrop-filter: blur(8px);
  animation: flow-saved-paths-in 140ms ease-out;
}

.flow-graph-canvas--workspace .flow-graph-canvas__saved-paths {
  right: 340px;
}

.flow-graph-canvas__guide-lines {
  position: absolute;
  inset: 0;
  z-index: 8;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}

.flow-graph-canvas__guide-line {
  fill: none;
  stroke: var(--flow-direction-color);
  stroke-width: 1.4;
  stroke-dasharray: 4 5;
  opacity: 0.82;
  animation: flow-guide-direction 900ms linear infinite;
}

.flow-graph-canvas__guide-lines marker path {
  fill: var(--flow-direction-color);
}

.flow-graph-canvas__guide {
  position: absolute;
  z-index: 9;
  display: flex;
  align-items: center;
  max-width: 292px;
  min-height: 36px;
  padding: 6px 8px 6px 12px;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-direction-color);
  border-radius: 4px;
  transform: translate(-50%, -50%);
  animation: flow-guide-in 140ms ease-out;
}

.flow-graph-canvas__guide button {
  min-width: 28px;
  min-height: 28px;
  margin-left: 4px;
  color: inherit;
  cursor: pointer;
  background: transparent;
  border: 0;
}

.flow-graph-canvas__guide-overflow {
  position: absolute;
  top: 50%;
  z-index: 9;
  padding: 5px 8px;
  color: var(--flow-direction-color);
  font-size: 12px;
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-direction-color);
  border-radius: 4px;
  transform: translateY(-50%);
  pointer-events: none;
}

.flow-graph-canvas__guide-overflow--left {
  left: 8px;
}

.flow-graph-canvas__error {
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
}

.flow-graph-canvas :deep(.vue-flow__pane) {
  cursor: grab;
}

.flow-graph-canvas :deep(.vue-flow__pane.dragging) {
  cursor: grabbing;
}

.flow-graph-canvas--configuration :deep(.vue-flow__node.selectable) {
  cursor: pointer;
}

.flow-graph-canvas :deep(.vue-flow__edge-path) {
  stroke: var(--flow-edge-color);
  stroke-width: 1.35;
}

.flow-graph-canvas :deep(.flow-node--path-muted) {
  opacity: 0.46;
}

.flow-graph-canvas :deep(.flow-node--path-active) {
  opacity: 1;
}

.flow-graph-canvas :deep(.flow-node--path-active .flow-node) {
  font-weight: 600;
  border-width: 2px;
}

.flow-graph-canvas :deep(.vue-flow__edge-textbg) {
  fill: var(--flow-surface-color);
}

.flow-graph-canvas :deep(.vue-flow__controls) {
  border: 1px solid var(--flow-edge-color);
  box-shadow: none;
}

.flow-graph-canvas :deep(.vue-flow__controls-button) {
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border-bottom-color: var(--flow-edge-color);
}

.flow-graph-canvas :deep(.vue-flow__controls-button:hover) {
  color: var(--flow-direction-color);
  background: color-mix(in srgb, var(--flow-direction-color) 10%, var(--flow-surface-color));
}

.flow-graph-canvas :deep(.vue-flow__controls-button svg) {
  fill: currentcolor;
}

.flow-graph-canvas :deep(.vue-flow__controls-button:disabled) {
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  opacity: 0.45;
}

.flow-graph-canvas--workspace :deep(.vue-flow__controls),
.flow-graph-canvas--configuration :deep(.vue-flow__controls) {
  right: 336px;
  transition: right 160ms ease;
}

@keyframes flow-guide-in {
  from {
    opacity: 0;
    transform: translate(-50%, calc(-50% + 6px));
  }
}

@keyframes flow-guide-direction {
  to { stroke-dashoffset: -9; }
}

@keyframes flow-saved-paths-in {
  from {
    opacity: 0;
    transform: translateX(8px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .flow-graph-canvas *,
  .flow-graph-canvas *::before,
  .flow-graph-canvas *::after {
    scroll-behavior: auto !important;
    transition-duration: 0s !important;
  }

  .flow-graph-canvas__guide {
    animation: none;
  }

  .flow-graph-canvas__guide-line,
  .flow-graph-canvas__saved-paths {
    animation: none;
  }
}
</style>
