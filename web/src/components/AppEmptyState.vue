<script setup lang="ts">
import { NEl } from 'naive-ui'

withDefaults(defineProps<{
  title: string
  description: string
  size?: 'medium' | 'large'
}>(), {
  size: 'medium',
})
</script>

<template>
  <n-el class="app-empty-state" :class="`app-empty-state--${size}`">
    <div class="app-empty-state__message" role="status" aria-live="polite" aria-atomic="true">
      <p class="app-empty-state__title">{{ title }}</p>
      <p class="app-empty-state__description">{{ description }}</p>
    </div>
    <div class="app-empty-state__actions">
      <slot />
    </div>
  </n-el>
</template>

<style scoped>
.app-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 232px;
  padding: 36px 24px;
  text-align: center;
  background: color-mix(in srgb, var(--card-color) 96%, var(--primary-color));
  border: 1px solid var(--divider-color);
  border-radius: var(--border-radius);
}

.app-empty-state--large {
  min-height: 320px;
  padding-block: 48px;
}

.app-empty-state__message {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.app-empty-state__title {
  margin: 0;
  color: var(--text-color-1);
  font-size: 16px;
  font-weight: 600;
  line-height: 1.5;
}

.app-empty-state__description {
  max-width: 440px;
  margin: 8px 0 20px;
  color: var(--text-color-3);
  line-height: 1.7;
}

.app-empty-state__actions:empty {
  display: none;
}
</style>
