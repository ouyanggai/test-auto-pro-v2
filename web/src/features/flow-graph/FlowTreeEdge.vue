<script setup lang="ts">
import { BaseEdge, EdgeLabelRenderer, type EdgeProps } from '@vue-flow/core'

import type { FlowTreeEdgeData } from './types'

defineProps<EdgeProps<FlowTreeEdgeData>>()
const emit = defineEmits<{
  selectBranch: [choice: { routeNodeId: string, branchId: string }]
}>()
</script>

<template>
  <base-edge
    :id="id"
	:class="{
	  'flow-tree-edge__base--selected': data.selected,
	  'flow-tree-edge__base--dimmed': data.selectionEnabled && data.active === false,
	}"
    :path="data.path"
    :marker-start="markerStart"
    :marker-end="markerEnd"
    :interaction-width="interactionWidth"
	:label="data.selectionEnabled && data.kind !== 'sequence' ? undefined : label"
    :label-x="data.labelX"
    :label-y="data.labelY"
    :label-style="labelStyle"
    :label-show-bg="labelShowBg"
    :label-bg-style="labelBgStyle"
    :label-bg-padding="labelBgPadding"
    :label-bg-border-radius="labelBgBorderRadius"
  />
  <path
	class="flow-tree-edge__direction"
	:class="{
	  'flow-tree-edge__direction--selected': data.selected,
	  'flow-tree-edge__direction--dimmed': data.active === false,
	}"
	:d="data.path"
	aria-hidden="true"
  />
	<edge-label-renderer
	  v-if="data.selectionEnabled
		&& data.kind !== 'sequence'
		&& data.active !== false
		&& (data.kind === 'condition' || data.kind === 'manual' || data.parallelRequired)
		&& data.labelX !== undefined
		&& data.labelY !== undefined"
	>
	<button
	  v-if="data.kind === 'condition' || data.kind === 'manual'"
	  type="button"
	  class="flow-tree-edge__choice"
	  :class="{
		'flow-tree-edge__choice--selected': data.selected,
		'flow-tree-edge__choice--candidate': data.candidate,
	  }"
	  :style="{ transform: `translate(-50%, -50%) translate(${data.labelX}px, ${data.labelY}px)` }"
	  :aria-pressed="data.selected"
	  :aria-label="`${label || '此分支'}，${data.selected ? '已选择' : '选择此分支'}`"
	  @click="emit('selectBranch', { routeNodeId: data.routeNodeId, branchId: data.branchId })"
	>
	  {{ label || '选择此分支' }}<span v-if="data.selected"> · 已选择</span>
	</button>
	<span
	  v-else
	  class="flow-tree-edge__required"
	  :style="{ transform: `translate(-50%, -50%) translate(${data.labelX}px, ${data.labelY}px)` }"
	>
	  {{ label ? `${label} · 并行必经` : '并行必经' }}
	</span>
  </edge-label-renderer>
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

:deep(.flow-tree-edge__base--selected) {
  stroke: var(--flow-direction-color);
  stroke-width: 2;
}

:deep(.flow-tree-edge__base--dimmed) {
  opacity: 0.34;
}

.flow-tree-edge__direction--selected {
  stroke-width: 2.4;
  opacity: 0.86;
}

.flow-tree-edge__direction--dimmed {
  opacity: 0.16;
}

.flow-tree-edge__choice,
.flow-tree-edge__required {
  position: absolute;
  z-index: 4;
  min-height: 32px;
  padding: 5px 10px;
  color: var(--flow-label-color);
  white-space: nowrap;
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  border-radius: 4px;
  pointer-events: all;
}

.flow-tree-edge__choice {
  cursor: pointer;
  transition: color 120ms ease, border-color 120ms ease, background-color 120ms ease, opacity 120ms ease;
}

.flow-tree-edge__choice:hover,
.flow-tree-edge__choice:focus-visible,
.flow-tree-edge__choice--candidate {
  color: var(--flow-direction-color);
  border-color: var(--flow-direction-color);
  outline: none;
}

.flow-tree-edge__choice--selected {
  color: var(--flow-direction-color);
  background: color-mix(in srgb, var(--flow-direction-color) 10%, var(--flow-surface-color));
  border-color: var(--flow-direction-color);
}

.flow-tree-edge__choice--dimmed {
  opacity: 0.5;
}

.flow-tree-edge__required {
  display: inline-flex;
  align-items: center;
  color: var(--flow-direction-color);
  pointer-events: none;
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

  .flow-tree-edge__choice {
    transition: none;
  }
}
</style>
