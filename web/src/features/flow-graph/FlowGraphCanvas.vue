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
  initialViewportForGraph,
  safeLayoutFlowGraph,
  shouldSetInitialViewport,
} from './layout'
import type { FlowGraph } from './types'
import { analyzeExecutionPath, viewportForPointNearest } from '../execution-paths/logic'
import type { ExecutionPathChoice } from '../execution-paths/types'

const props = withDefaults(defineProps<{
  graph: FlowGraph
  choices?: ExecutionPathChoice[]
  selectionEnabled?: boolean
}>(), { choices: () => [], selectionEnabled: false })
const emit = defineEmits<{
  retry: []
  selectBranch: [choice: ExecutionPathChoice]
}>()
const themeVars = useThemeVars()
const canvasRoot = ref<HTMLElement | null>(null)
const isPageFullscreen = ref(false)
const layoutResult = computed(() => safeLayoutFlowGraph(props.graph))
const laidOut = computed(() => layoutResult.value.layout)
const pathAnalysis = computed(() => analyzeExecutionPath(props.graph, props.choices))
const displayedLayout = computed(() => {
  if (!laidOut.value || !props.selectionEnabled) return laidOut.value
  const selectedByRoute = new Map(props.choices.map((choice) => [choice.routeNodeId, choice.branchId]))
  const analysis = pathAnalysis.value
  return {
    nodes: laidOut.value.nodes.map((node) => ({
      ...node,
      class: analysis.reachableNodeIds.has(node.id) ? 'flow-node--path-active' : 'flow-node--path-muted',
    })),
    edges: laidOut.value.edges.map((edge) => {
      const kind = edge.data?.kind
      const selectedBranch = edge.data ? selectedByRoute.get(edge.data.routeNodeId) : undefined
      const routeReachable = edge.data ? analysis.reachableNodeIds.has(edge.data.routeNodeId) : false
      const selected = edge.data
        ? analysis.reachableEdgeIds.has(edge.id) || selectedBranch === edge.data.branchId
        : false
      const candidate = routeReachable && (kind === 'condition' || kind === 'manual') && !selectedBranch
      const active = selected || candidate || (routeReachable && (kind === 'condition' || kind === 'manual'))
      return {
        ...edge,
        data: edge.data
          ? {
              ...edge.data,
              selectionEnabled: true,
              selected,
              candidate,
              active,
              parallelRequired: kind === 'parallel' && selected,
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
    await setViewport(compensateViewportForContainerWidth(viewport, beforeWidth, afterWidth), { duration: 0 })
  }
  if (!isPageFullscreen.value) setDocumentScrollLocked(false)
}

function handlePageFullscreenKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !isPageFullscreen.value) return
  void requestPageFullscreen(false)
}

async function handleSelectBranch(choice: ExecutionPathChoice) {
  emit('selectBranch', choice)
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  const nextRouteID = pathAnalysis.value.missingRouteNodeIds[0]
  if (!nextRouteID || !laidOut.value || !canvasRoot.value) return
  const nextNode = laidOut.value.nodes.find((node) => node.id === nextRouteID)
  if (!nextNode) return
  const viewport = getViewport()
  const nextViewport = viewportForPointNearest(
    viewport,
    { x: nextNode.position.x, y: nextNode.position.y },
    { width: canvasRoot.value.clientWidth, height: canvasRoot.value.clientHeight },
  )
  if (nextViewport.x === viewport.x && nextViewport.y === viewport.y) return
  await setViewport(nextViewport, { duration: reducedMotion() ? 0 : 180 })
}

onInit(() => {
  ready = true
  void setInitialViewport()
})

watch(() => props.graph.planId, () => {
  void setInitialViewport()
})
watch(laidOut, (value) => {
  if (value || !isPageFullscreen.value) return
  void requestPageFullscreen(false)
})

onMounted(() => document.addEventListener('keydown', handlePageFullscreenKeydown))
onBeforeUnmount(() => {
  viewportVersion++
  pageFullscreenDisposed = true
  requestedPageFullscreen = false
  setDocumentScrollLocked(false)
  document.removeEventListener('keydown', handlePageFullscreenKeydown)
})
</script>

<template>
  <div
    ref="canvasRoot"
    class="flow-graph-canvas"
    :class="{ 'flow-graph-canvas--page-fullscreen': isPageFullscreen }"
    :style="canvasStyle"
    aria-label="只读流程图"
  >
	<div v-if="laidOut && $slots.toolbar" class="flow-graph-canvas__toolbar">
	  <slot name="toolbar" />
	</div>
    <n-button
      v-if="laidOut"
      class="flow-graph-canvas__fullscreen"
      size="small"
      secondary
      :aria-pressed="isPageFullscreen"
      @click="requestPageFullscreen(!isPageFullscreen)"
    >
      {{ isPageFullscreen ? '退出全屏' : '页面全屏' }}
    </n-button>
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
    <div v-else class="flow-graph-canvas__error">
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

.flow-graph-canvas__fullscreen {
  position: absolute;
  top: 12px;
  right: 16px;
  z-index: 6;
}

.flow-graph-canvas__toolbar {
  position: absolute;
  top: 12px;
  right: 126px;
  left: 16px;
  z-index: 6;
  pointer-events: none;
}

.flow-graph-canvas__toolbar :deep(*) {
  pointer-events: auto;
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

@media (prefers-reduced-motion: reduce) {
  .flow-graph-canvas *,
  .flow-graph-canvas *::before,
  .flow-graph-canvas *::after {
    scroll-behavior: auto !important;
    transition-duration: 0s !important;
  }
}
</style>
