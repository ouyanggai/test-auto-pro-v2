<script setup lang="ts">
import { Controls } from '@vue-flow/controls'
import { useThemeVars } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, watch } from 'vue'
import { VueFlow as VueFlowCanvas, useVueFlow } from '@vue-flow/core'

import FlowGraphNode from './FlowGraphNode.vue'
import { layoutFlowGraph, shouldFitInitialGraph } from './layout'
import type { FlowGraph } from './types'

const props = defineProps<{ graph: FlowGraph }>()
const themeVars = useThemeVars()
const laidOut = computed(() => layoutFlowGraph(props.graph))
const canvasStyle = computed(() => ({
  '--flow-edge-color': themeVars.value.borderColor,
  '--flow-label-color': themeVars.value.textColor2,
  '--flow-surface-color': themeVars.value.bodyColor,
}))
const { onInit, fitView } = useVueFlow()
let ready = false
let fittedPlanId = ''
let fitVersion = 0

function reducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

async function fitInitialGraph() {
  const version = ++fitVersion
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== fitVersion || !shouldFitInitialGraph(ready, fittedPlanId, props.graph.planId)) return
  fittedPlanId = props.graph.planId
  await fitView({ padding: 0.18, duration: reducedMotion() ? 0 : 280, maxZoom: 1.15 })
}

onInit(() => {
  ready = true
  void fitInitialGraph()
})

watch(() => props.graph.planId, () => {
  void fitInitialGraph()
})

onBeforeUnmount(() => {
  fitVersion++
})
</script>

<template>
  <div class="flow-graph-canvas" :style="canvasStyle" aria-label="只读流程图">
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
      <controls position="bottom-right" :show-interactive="false" />
    </vue-flow-canvas>
  </div>
</template>

<style scoped>
.flow-graph-canvas {
  width: 100%;
  height: 560px;
  min-height: 560px;
  overflow: hidden;
  background: var(--flow-surface-color);
  border-top: 1px solid var(--flow-edge-color);
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
