<script setup lang="ts">
import { NButton, NEl } from 'naive-ui'

import AppEmptyIcon from '../../components/AppEmptyIcon.vue'

defineProps<{
  filtered: boolean
}>()

const emit = defineEmits<{
  clearFilter: []
  openPlans: []
}>()
</script>

<template>
  <n-el class="run-list-empty" role="status" aria-live="polite">
    <div class="run-list-empty__illustration" aria-hidden="true">
      <AppEmptyIcon />
    </div>

    <p class="run-list-empty__title">
      {{ filtered ? '没有符合条件的运行记录' : '还没有运行记录' }}
    </p>
    <p class="run-list-empty__description">
      {{ filtered ? '当前筛选条件下没有结果，清除筛选后可以查看全部记录。' : '运行测试计划后，每次运行的结果都会按时间出现在这里。' }}
    </p>

    <n-button v-if="filtered" secondary type="primary" @click="emit('clearFilter')">
      查看全部记录
    </n-button>
    <n-button v-else type="primary" @click="emit('openPlans')">
      前往测试计划
    </n-button>
  </n-el>
</template>

<style scoped>
.run-list-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 320px;
  padding: 48px 24px;
  text-align: center;
  background: color-mix(in srgb, var(--card-color) 96%, var(--primary-color));
  border: 1px solid var(--divider-color);
  border-radius: var(--border-radius);
}

.run-list-empty__illustration {
  width: 120px;
  height: 88px;
  margin-bottom: 20px;
  color: var(--primary-color);
}

.run-list-empty__title {
  margin: 0;
  color: var(--text-color-1);
  font-size: 16px;
  font-weight: 600;
  line-height: 1.5;
}

.run-list-empty__description {
  max-width: 440px;
  margin: 8px 0 20px;
  color: var(--text-color-3);
  line-height: 1.7;
}
</style>
