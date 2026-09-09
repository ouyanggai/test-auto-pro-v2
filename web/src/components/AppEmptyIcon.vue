<script setup lang="ts">
import { FileTrayFullOutline } from '@vicons/ionicons5'
import { NEl, NIcon } from 'naive-ui'
</script>

<template>
  <n-el tag="span" class="app-empty-icon" aria-hidden="true">
    <n-icon :size="64" class="app-empty-icon__glyph">
      <FileTrayFullOutline />
    </n-icon>
  </n-el>
</template>

<style scoped>
.app-empty-icon {
  display: block;
  width: 72px;
  height: 72px;
  color: var(--primary-color);
}

.app-empty-icon__glyph {
  opacity: 0.78;
}
</style>
