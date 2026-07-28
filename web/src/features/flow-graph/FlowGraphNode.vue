<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { NTag } from 'naive-ui'
import { computed } from 'vue'

import type { FlowNodeData } from './types'

const props = defineProps<{ data: FlowNodeData }>()

const tagType = computed<'default' | 'success' | 'warning' | 'error' | 'info'>(() => {
  switch (props.data.type) {
    case 'start': return 'success'
    case 'end': return 'error'
    case 'condition':
    case 'manual':
    case 'parallel': return 'warning'
    case 'synergy': return 'info'
    default: return 'default'
  }
})
</script>

<template>
  <n-tag class="flow-node" :type="tagType" :bordered="true" :title="data.name">
    <handle type="target" :position="Position.Top" :connectable="false" />
    <span class="flow-node__type">{{ data.typeName }}</span>
    <span class="flow-node__name">{{ data.name }}</span>
    <handle type="source" :position="Position.Bottom" :connectable="false" />
  </n-tag>
</template>

<style scoped>
.flow-node {
  display: grid;
  width: 180px;
  height: 72px;
  padding: 9px 14px;
  align-content: center;
  justify-items: center;
  gap: 3px;
  border-radius: 4px;
  white-space: normal;
}

.flow-node__type {
  font-size: 12px;
  line-height: 1.2;
  opacity: 0.78;
}

.flow-node__name {
  display: -webkit-box;
  overflow: hidden;
  max-width: 150px;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
  text-align: center;
  text-overflow: ellipsis;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.flow-node :deep(.vue-flow__handle) {
  width: 7px;
  height: 7px;
  border: 1px solid currentcolor;
  background: currentcolor;
  pointer-events: none;
}
</style>
