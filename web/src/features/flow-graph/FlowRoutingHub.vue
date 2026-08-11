<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { NTag } from 'naive-ui'

import type { FlowNodeData } from './types'

defineProps<{ data?: FlowNodeData }>()
</script>

<template>
  <div
    class="flow-routing-hub"
    :class="{ 'flow-routing-hub--configuration': data?.configurationMode }"
    :aria-hidden="data?.configurationMode ? undefined : 'true'"
  >
    <handle type="target" :position="Position.Top" :connectable="false" />
    <button
      v-if="data?.configurationMode"
      type="button"
      class="flow-routing-hub__configuration"
      :class="{
        'flow-routing-hub__configuration--selected': data.configurationSelected,
        'flow-routing-hub__configuration--disabled': !data.configurationInteractive,
      }"
      :disabled="!data.configurationInteractive"
      :aria-pressed="data.configurationSelected"
      :title="data.configurationInteractive ? `${data.name}，${data.configurationStatusName}` : `${data.name}，不在当前路径`"
    >
      <span>{{ data.name }}</span>
      <n-tag size="tiny" :bordered="false">{{ data.configurationInteractive ? data.configurationStatusName : '路径外上下文' }}</n-tag>
    </button>
    <handle type="source" :position="Position.Bottom" :connectable="false" />
  </div>
</template>

<style scoped>
.flow-routing-hub {
  position: relative;
  width: 8px;
  height: 8px;
  pointer-events: none;
  opacity: 0;
}

.flow-routing-hub--configuration {
  opacity: 1;
}

.flow-routing-hub__configuration {
  position: absolute;
  top: 50%;
  left: 50%;
  z-index: 2;
  display: grid;
  width: 136px;
  min-height: 42px;
  padding: 4px 8px;
  color: var(--flow-label-color);
  cursor: pointer;
  background: var(--flow-surface-color);
  border: 1px dashed var(--flow-edge-color);
  border-radius: 4px;
  transform: translate(-50%, -50%);
  pointer-events: all;
}

.flow-routing-hub__configuration:hover:not(:disabled),
.flow-routing-hub__configuration:focus-visible,
.flow-routing-hub__configuration--selected {
  border-color: var(--flow-direction-color);
  outline: none;
}

.flow-routing-hub__configuration--selected {
  background: color-mix(in srgb, var(--flow-direction-color) 9%, var(--flow-surface-color));
  border-style: solid;
}

.flow-routing-hub__configuration--disabled {
  cursor: default;
  opacity: 0.5;
}

.flow-routing-hub :deep(.vue-flow__handle) {
  width: 1px;
  height: 1px;
  border: 0;
  background: transparent;
  pointer-events: none;
}
</style>
