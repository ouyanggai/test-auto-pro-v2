<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
</script>

<template>
  <div class="flow-routing-hub" aria-hidden="true">
    <handle type="target" :position="Position.Top" :connectable="false" />
    <handle type="source" :position="Position.Bottom" :connectable="false" />
  </div>
</template>

<style scoped>
.flow-routing-hub {
  width: 8px;
  height: 8px;
  pointer-events: none;
  opacity: 0;
}

.flow-routing-hub :deep(.vue-flow__handle) {
  width: 1px;
  height: 1px;
  border: 0;
  background: transparent;
  pointer-events: none;
}
</style>
