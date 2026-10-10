<script setup lang="ts">
import { defineAsyncComponent } from 'vue'

// ReportChart 是报告中心唯一的图表组件：ECharts 通过 defineAsyncComponent 懒加载，
// 首屏先把计数卡、计划表和文字指标渲染出来，图表包后到，不让用户对着白屏等 1MB 的图表库。
// 懒加载期间显示一句文字状态，不显示假进度条。
const VChart = defineAsyncComponent(() => import('./echarts').then((module) => module.VChart))

defineProps<{ option: Record<string, unknown> }>()
</script>

<template>
  <Suspense>
    <VChart class="report-chart" :option="option" autoresize />
    <template #fallback>
      <p class="report-chart__loading">图表正在加载……</p>
    </template>
  </Suspense>
</template>

<style scoped>
.report-chart {
  width: 100%;
  height: 100%;
}
.report-chart__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  margin: 0;
  color: var(--text-color-3);
}
</style>
