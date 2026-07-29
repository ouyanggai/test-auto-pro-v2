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
  viewportForPointCentered,
} from '../execution-paths/logic'
import type { ExecutionPathChoice } from '../execution-paths/types'

const props = withDefaults(defineProps<{
  graph: FlowGraph
  choices?: ExecutionPathChoice[]
  selectionMode?: boolean
  selectionAvailable?: boolean
  selectionResumable?: boolean
}>(), { choices: () => [], selectionMode: false, selectionAvailable: false, selectionResumable: false })
const emit = defineEmits<{
  retry: []
  selectBranch: [choice: ExecutionPathChoice]
  enterSelection: []
  exitSelection: []
}>()
const themeVars = useThemeVars()
const canvasRoot = ref<HTMLElement | null>(null)
const isPageFullscreen = ref(false)
const isSelectionPanelCollapsed = ref(false)
const guideBubble = ref<{ message: string, x: number, y: number } | null>(null)
const layoutResult = computed(() => safeLayoutFlowGraph(props.graph))
const laidOut = computed(() => layoutResult.value.layout)
const pathAnalysis = computed(() => analyzeExecutionPath(props.graph, props.choices))
const displayedLayout = computed(() => {
  if (!laidOut.value || !props.selectionMode) return laidOut.value
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
              selectionEnabled: true,
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
let guideBubbleTimer: number | null = null
let guideVersion = 0
let pendingGuideAnchor = ''

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
  if (guideBubbleTimer !== null) window.clearTimeout(guideBubbleTimer)
  guideBubbleTimer = null
  guideBubble.value = null
}

async function guideSelectionNext(anchorNodeID = '') {
  const version = ++guideVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== guideVersion || !props.selectionMode || !laidOut.value || !canvasRoot.value) return

  const nextRouteID = nextExecutionPathRouteID(pathAnalysis.value)
  const targetID = nextRouteID || anchorNodeID || props.graph.entryNodeIds[0]
  const targetNode = laidOut.value.nodes.find((node) => node.id === targetID)
  if (!targetNode) return
  const nodeWidth = targetNode.type === 'routingHub' ? flowRoutingHubSize : flowNodeWidth
  const point = { x: targetNode.position.x + nodeWidth / 2, y: targetNode.position.y + 20 }
  const viewport = getViewport()
  const reservedRight = isSelectionPanelCollapsed.value ? 56 : 336
  // 右侧面板不属于可操作视口；每一步都保持缩放并把明确目标放到实际操作区中央。
  const nextViewport = viewportForPointCentered(
    viewport,
    point,
    { width: canvasRoot.value.clientWidth, height: canvasRoot.value.clientHeight },
    reservedRight,
  )
  await setViewport(nextViewport, { duration: reducedMotion() ? 0 : 250 })
  if (version !== guideVersion || !props.selectionMode || !canvasRoot.value) return

  const safeWidth = Math.max(0, canvasRoot.value.clientWidth - reservedRight)
  const bubbleX = point.x * nextViewport.zoom + nextViewport.x
  const bubbleY = point.y * nextViewport.zoom + nextViewport.y
  clearGuideBubble()
  guideBubble.value = {
    message: nextRouteID
      ? `下一步：请选择一条分支（还剩 ${pathAnalysis.value.missingRouteNodeIds.length} 处）`
      : '线路已完整，请保存',
    x: Math.min(Math.max(160, bubbleX), Math.max(160, safeWidth - 160)),
    y: Math.min(Math.max(120, bubbleY), Math.max(120, canvasRoot.value.clientHeight - 80)),
  }
  guideBubbleTimer = window.setTimeout(() => { guideBubble.value = null }, 5200)
}

function toggleSelectionPanel() {
  isSelectionPanelCollapsed.value = !isSelectionPanelCollapsed.value
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

async function handleSelectionButton() {
  if (props.selectionMode) {
    clearGuideBubble()
    emit('exitSelection')
    return
  }
  if (!props.selectionAvailable) return
  // 线路编辑只允许在页面全屏中发生，先完成容器切换和视口补偿，再开放分支交互。
  if (!isPageFullscreen.value) await requestPageFullscreen(true)
  if (pageFullscreenDisposed || !isPageFullscreen.value) return
  emit('enterSelection')
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
    await setViewport(compensateViewportForContainerWidth(viewport, beforeWidth, afterWidth), { duration: 0 })
  }
  if (!isPageFullscreen.value) setDocumentScrollLocked(false)
}

function handlePageFullscreenKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !isPageFullscreen.value) return
  void requestPageFullscreen(false)
}

function handleSelectBranch(choice: ExecutionPathChoice) {
  if (!props.selectionMode) return
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
  if (props.selectionMode) void guideSelectionNext()
})
watch(() => props.choices, () => {
  const anchor = pendingGuideAnchor
  pendingGuideAnchor = ''
  if (props.selectionMode) void guideSelectionNext(anchor)
})
watch(laidOut, (value) => {
  if (value || !isPageFullscreen.value) return
  void requestPageFullscreen(false)
})
watch(isPageFullscreen, (value) => {
  // Esc 或“退出全屏”结束编辑展示，但草稿由父页面保留供下次继续选择。
  if (!value && props.selectionMode) emit('exitSelection')
})
watch(() => props.selectionMode, (enabled) => {
  if (!enabled) {
    guideVersion++
    clearGuideBubble()
    return
  }
  void guideSelectionNext()
})
watch(isSelectionPanelCollapsed, () => {
  if (props.selectionMode) void guideSelectionNext()
})

onMounted(() => document.addEventListener('keydown', handlePageFullscreenKeydown))
onBeforeUnmount(() => {
  viewportVersion++
  guideVersion++
  pageFullscreenDisposed = true
  requestedPageFullscreen = false
  clearGuideBubble()
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
      'flow-graph-canvas--selection': selectionMode,
      'flow-graph-canvas--panel-collapsed': isSelectionPanelCollapsed,
    }"
    :style="canvasStyle"
    :aria-label="selectionMode ? '线路选择流程图' : '只读流程图'"
  >
    <div v-if="laidOut" class="flow-graph-canvas__actions">
      <n-button
        size="small"
        secondary
        :disabled="!selectionAvailable && !selectionMode"
        :aria-pressed="selectionMode"
        @click="handleSelectionButton"
      >
        {{ selectionMode ? '退出选择' : selectionResumable ? '继续选择' : '线路选择' }}
      </n-button>
      <n-button
        size="small"
        secondary
        :aria-pressed="isPageFullscreen"
        @click="requestPageFullscreen(!isPageFullscreen)"
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
      v-if="laidOut && selectionMode"
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
        <slot name="selection-panel" />
      </div>
    </aside>
    <div
      v-if="guideBubble && selectionMode"
      class="flow-graph-canvas__guide"
      :style="{ left: `${guideBubble.x}px`, top: `${guideBubble.y}px` }"
      role="status"
      aria-live="polite"
    >
      <span>{{ guideBubble.message }}</span>
      <button type="button" aria-label="关闭提示" @click="clearGuideBubble">×</button>
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
  top: 56px;
  right: 12px;
  bottom: 12px;
  z-index: 7;
  width: 320px;
  overflow: hidden;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
  transition: width 160ms ease;
}

.flow-graph-canvas__selection-panel--collapsed {
  width: 40px;
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

.flow-graph-canvas__guide {
  position: absolute;
  z-index: 8;
  display: flex;
  align-items: center;
  max-width: 292px;
  min-height: 36px;
  padding: 6px 8px 6px 12px;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-direction-color);
  border-radius: 4px;
  transform: translate(-50%, calc(-100% - 18px));
  animation: flow-guide-in 140ms ease-out;
}

.flow-graph-canvas__guide::before,
.flow-graph-canvas__guide::after {
  position: absolute;
  top: 100%;
  left: 50%;
  width: 0;
  height: 0;
  content: '';
  border: solid transparent;
  transform: translateX(-50%);
}

.flow-graph-canvas__guide::before {
  border-width: 10px;
  border-top-color: var(--flow-direction-color);
}

.flow-graph-canvas__guide::after {
  margin-top: -1px;
  border-width: 8px;
  border-top-color: var(--flow-surface-color);
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

.flow-graph-canvas--selection :deep(.vue-flow__controls) {
  right: 336px;
  transition: right 160ms ease;
}

.flow-graph-canvas--selection.flow-graph-canvas--panel-collapsed :deep(.vue-flow__controls) {
  right: 56px;
}

@keyframes flow-guide-in {
  from {
    opacity: 0;
    transform: translate(-50%, calc(-100% - 12px));
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
}
</style>
