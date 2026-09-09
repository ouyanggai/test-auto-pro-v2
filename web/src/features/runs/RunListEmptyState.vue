<script setup lang="ts">
import { NButton, NEl } from 'naive-ui'

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
      <svg viewBox="0 0 120 88" focusable="false">
        <circle class="run-list-empty__halo" cx="60" cy="44" r="38" />
        <path class="run-list-empty__paper run-list-empty__paper--back" d="M39 22h42a5 5 0 0 1 5 5v34H34V27a5 5 0 0 1 5-5Z" />
        <path class="run-list-empty__paper" d="M34 29h52a5 5 0 0 1 5 5v31H29V34a5 5 0 0 1 5-5Z" />
        <path class="run-list-empty__line" d="M42 42h36M42 50h25" />
        <path class="run-list-empty__tray" d="m24 58 8 14h56l8-14H73l-4 6H51l-4-6H24Z" />
        <circle class="run-list-empty__dot" cx="94" cy="27" r="3" />
        <circle class="run-list-empty__dot run-list-empty__dot--small" cx="25" cy="35" r="2" />
      </svg>
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

.run-list-empty__illustration svg {
  display: block;
  width: 100%;
  height: 100%;
}

.run-list-empty__halo {
  fill: color-mix(in srgb, var(--primary-color) 8%, transparent);
}

.run-list-empty__paper {
  fill: var(--card-color);
  stroke: color-mix(in srgb, var(--primary-color) 44%, var(--divider-color));
  stroke-width: 1.5;
}

.run-list-empty__paper--back {
  fill: color-mix(in srgb, var(--primary-color) 7%, var(--card-color));
  opacity: 0.72;
}

.run-list-empty__line {
  fill: none;
  stroke: color-mix(in srgb, var(--primary-color) 34%, var(--text-color-3));
  stroke-linecap: round;
  stroke-width: 2;
}

.run-list-empty__tray {
  fill: color-mix(in srgb, var(--primary-color) 12%, var(--card-color));
  stroke: var(--primary-color);
  stroke-linejoin: round;
  stroke-width: 1.8;
}

.run-list-empty__dot {
  fill: color-mix(in srgb, var(--primary-color) 48%, transparent);
}

.run-list-empty__dot--small {
  opacity: 0.56;
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
