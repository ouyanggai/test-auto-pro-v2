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
	  'flow-tree-edge__base--candidate': data.candidate,
	  'flow-tree-edge__base--dimmed': data.workspaceOpen && data.dimmed,
	  'flow-tree-edge__base--taken': Boolean(data.taken),
	  'flow-tree-edge__base--deviated': Boolean(data.deviated),
	}"
    :path="data.path"
    :marker-start="markerStart"
    :marker-end="markerEnd"
    :interaction-width="interactionWidth"
	:label="data.workspaceOpen && data.kind !== 'sequence' ? undefined : label"
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
	  'flow-tree-edge__direction--animated': !data.workspaceOpen || data.selected || Boolean(data.taken),
	  'flow-tree-edge__direction--dimmed': data.workspaceOpen && data.dimmed && !data.taken && !data.deviated,
	  'flow-tree-edge__direction--taken': Boolean(data.taken),
	  'flow-tree-edge__direction--deviated': Boolean(data.deviated),
	}"
	:d="data.path"
	aria-hidden="true"
  />
	<edge-label-renderer
	  v-if="data.workspaceOpen
		&& data.kind !== 'sequence'
		&& data.labelX !== undefined
		&& data.labelY !== undefined"
	>
	<button
	  v-if="data.branchEditing && (data.kind === 'condition' || data.kind === 'manual') && data.active !== false"
	  type="button"
	  class="flow-tree-edge__choice"
	  :class="{
		'flow-tree-edge__choice--selected': data.selected,
		'flow-tree-edge__choice--candidate': data.candidate,
		'flow-tree-edge__choice--dimmed': data.dimmed,
	  }"
	  :style="{ transform: `translate(-50%, -50%) translate(${data.labelX}px, ${data.labelY}px)` }"
	  :aria-pressed="data.selected"
	  :disabled="data.selected"
	  :aria-label="`${label || '此分支'}，${data.selected ? '已选择' : '选择此分支'}`"
	  @click="emit('selectBranch', { routeNodeId: data.routeNodeId, branchId: data.branchId })"
	>
	  {{ label || '选择此分支' }}<span v-if="data.selected"> · 已选择</span>
	</button>
	<span
	  v-else
	  class="flow-tree-edge__required"
	  :class="{ 'flow-tree-edge__required--dimmed': data.dimmed || data.active === false }"
	  :style="{ transform: `translate(-50%, -50%) translate(${data.labelX}px, ${data.labelY}px)` }"
	>
	  {{ !data.branchEditing
	    ? (data.parallelRequired
	      ? (label ? label + ' · 并行必经' : '并行必经')
	      : data.selected
	        ? (label ? label + ' · 已选择' : '已选择')
	        : (label || '未纳入'))
	    : data.parallelRequired
	      ? (label ? label + ' · 并行必经' : '并行必经')
	      : (label ? label + ' · 尚未到达' : '尚未到达') }}
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
}

.flow-tree-edge__direction--animated {
  animation: flow-tree-direction 1.6s linear infinite;
}

:deep(.flow-tree-edge__base--selected) {
  stroke: var(--flow-direction-color);
  stroke-width: 3;
}

:deep(.flow-tree-edge__base--candidate) {
  stroke: var(--flow-direction-color);
  stroke-width: 1.8;
}

:deep(.flow-tree-edge__base--dimmed) {
  opacity: 0.34;
}

/* 运行画布：真实走过的连线加粗并带流向动画；偏离已配置路径的连线标红。 */
:deep(.flow-tree-edge__base--taken) {
  stroke: var(--flow-direction-color);
  stroke-width: 3;
}

:deep(.flow-tree-edge__base--deviated) {
  stroke: #d03050;
  stroke-width: 3;
}

.flow-tree-edge__direction--taken {
  stroke-width: 3.2;
  opacity: 0.9;
}

.flow-tree-edge__direction--deviated {
  stroke: #d03050;
  stroke-width: 3.2;
  opacity: 0.95;
}

.flow-tree-edge__direction--selected {
  stroke-width: 3.2;
  opacity: 0.9;
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

.flow-tree-edge__choice--selected:disabled {
  cursor: default;
  opacity: 1;
}

.flow-tree-edge__choice--dimmed {
  opacity: 0.62;
}

.flow-tree-edge__choice--dimmed:hover,
.flow-tree-edge__choice--dimmed:focus-visible {
  opacity: 1;
}

.flow-tree-edge__required {
  display: inline-flex;
  align-items: center;
  color: var(--flow-direction-color);
  pointer-events: none;
}

.flow-tree-edge__required--dimmed {
  opacity: 0.48;
}

@keyframes flow-tree-direction {
  to {
    stroke-dashoffset: -22;
  }
}

@media (prefers-reduced-motion: reduce) {
  .flow-tree-edge__direction--animated {
    animation: none;
  }

  .flow-tree-edge__choice {
    transition: none;
  }
}
</style>
