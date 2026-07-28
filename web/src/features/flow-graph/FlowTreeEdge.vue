<script setup lang="ts">
import { BaseEdge, type EdgeProps } from '@vue-flow/core'

import type { FlowTreeEdgeData } from './types'

defineProps<EdgeProps<FlowTreeEdgeData>>()
</script>

<template>
  <base-edge
    :id="id"
    :path="data.path"
    :marker-start="markerStart"
    :marker-end="markerEnd"
    :interaction-width="interactionWidth"
    :label="label"
    :label-x="data.labelX"
    :label-y="data.labelY"
    :label-style="labelStyle"
    :label-show-bg="labelShowBg"
    :label-bg-style="labelBgStyle"
    :label-bg-padding="labelBgPadding"
    :label-bg-border-radius="labelBgBorderRadius"
  />
  <path class="flow-tree-edge__direction" :d="data.path" aria-hidden="true" />
</template>

<style scoped>
.flow-tree-edge__direction {
  fill: none;
  stroke: var(--flow-direction-color);
  stroke-width: 1.6;
  stroke-dasharray: 3 19;
  stroke-linecap: round;
  opacity: 0.42;
  pointer-events: none;
  animation: flow-tree-direction 1.6s linear infinite;
}

@keyframes flow-tree-direction {
  to {
    stroke-dashoffset: -22;
  }
}

@media (prefers-reduced-motion: reduce) {
  .flow-tree-edge__direction {
    animation: none;
  }
}
</style>
