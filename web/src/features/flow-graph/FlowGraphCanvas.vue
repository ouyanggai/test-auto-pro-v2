<script setup lang="ts">
import { Controls } from '@vue-flow/controls'
import { NButton, useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { VueFlow as VueFlowCanvas, useVueFlow } from '@vue-flow/core'

import FlowGraphNode from './FlowGraphNode.vue'
import FlowRoutingHub from './FlowRoutingHub.vue'
import { initialViewportForGraph, layoutFlowGraph, shouldSetInitialViewport } from './layout'
import type { FlowGraph } from './types'

const props = defineProps<{ graph: FlowGraph }>()
const themeVars = useThemeVars()
const canvasRoot = ref<HTMLElement | null>(null)
const isPageFullscreen = ref(false)
const laidOut = computed(() => layoutFlowGraph(props.graph))
const canvasStyle = computed(() => ({
  '--flow-edge-color': themeVars.value.borderColor,
  '--flow-label-color': themeVars.value.textColor2,
  '--flow-surface-color': themeVars.value.bodyColor,
}))
const { onInit, setViewport } = useVueFlow()
let ready = false
let positionedPlanId = ''
let viewportVersion = 0
let previousDocumentOverflow: string | null = null

function reducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

async function setInitialViewport() {
  const version = ++viewportVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== viewportVersion || !shouldSetInitialViewport(ready, positionedPlanId, props.graph.planId)) return
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

function togglePageFullscreen() {
  isPageFullscreen.value = !isPageFullscreen.value
  setDocumentScrollLocked(isPageFullscreen.value)
}

function handlePageFullscreenKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !isPageFullscreen.value) return
  isPageFullscreen.value = false
  setDocumentScrollLocked(false)
}

onInit(() => {
  ready = true
  void setInitialViewport()
})

watch(() => props.graph.planId, () => {
  void setInitialViewport()
})

onMounted(() => document.addEventListener('keydown', handlePageFullscreenKeydown))
onBeforeUnmount(() => {
  viewportVersion++
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
    <n-button
      class="flow-graph-canvas__fullscreen"
      size="small"
      secondary
      :aria-pressed="isPageFullscreen"
      @click="togglePageFullscreen"
    >
      {{ isPageFullscreen ? '退出全屏' : '页面全屏' }}
    </n-button>
    <vue-flow-canvas
      :nodes="laidOut.nodes"
      :edges="laidOut.edges"
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
      <controls position="bottom-right" :show-interactive="false" />
    </vue-flow-canvas>
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
