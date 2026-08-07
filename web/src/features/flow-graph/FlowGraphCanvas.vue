<script setup lang="ts">
import { Controls } from '@vue-flow/controls'
import { NButton, NEmpty, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { VueFlow as VueFlowCanvas, useVueFlow } from '@vue-flow/core'

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
import type { FlowGraph } from './types'
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
}>(), {
  choices: () => [], workspaceOpen: false, branchEditing: false, workspaceExitDisabled: false, saveGuideVisible: false, savedPathsOpen: false,
})
const emit = defineEmits<{
  retry: []
  selectBranch: [choice: ExecutionPathChoice]
  closeSavedPaths: []
  requestWorkspaceExit: []
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
  if (!laidOut.value || !props.workspaceOpen) return laidOut.value
  const analysis = pathAnalysis.value
  const edgeStates = classifyExecutionPathEdges(props.graph, analysis, props.choices)
  return {
    nodes: laidOut.value.nodes.map((node) => ({
      ...node,
      class: analysis.reachableNodeIds.has(node.id) ? 'flow-node--path-active' : 'flow-node--path-muted',
    })),
    edges: laidOut.value.edges.map((edge) => {
      const kind = edge.data?.kind
      const state = edgeStates.get(edge.id) ?? { selected: false, candidate: false, dimmed: true, active: false }
      return {
        ...edge,
        data: edge.data
          ? {
              ...edge.data,
              workspaceOpen: true,
              branchEditing: props.branchEditing,
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

const reservedRight = computed(() => props.workspaceOpen ? 336 : 0)
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

defineExpose({ setPageFullscreen })

function handleSelectBranch(choice: ExecutionPathChoice) {
  if (!props.workspaceOpen || !props.branchEditing) return
  pendingGuideAnchor = choice.routeNodeId
  emit('selectBranch', choice)
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
      'flow-graph-canvas--panel-collapsed': isSelectionPanelCollapsed,
    }"
    :style="canvasStyle"
    :aria-label="workspaceOpen ? (branchEditing ? '线路编辑流程图' : '路径查看流程图') : '只读流程图'"
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
      @viewport-change="handleViewportChange"
    >
      <template #node-flowNode="{ data }">
        <flow-graph-node :data="data" />
      </template>
      <template #node-routingHub>
        <flow-routing-hub />
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

.flow-graph-canvas--workspace :deep(.vue-flow__controls) {
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
