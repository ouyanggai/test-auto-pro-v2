<script setup lang="ts">
import { defineAsyncComponent } from 'vue'

// ReportChart 是报告中心唯一的图表组件：ECharts 通过 defineAsyncComponent 懒加载，
// 首屏先把计数卡、计划表和文字指标渲染出来，图表包后到，不让用户对着白屏等 1MB 的图表库。
// 懒加载期间显示一句文字状态，不显示假进度条。
const VChart = defineAsyncComponent(() => import('./echarts').then((module) => module.VChart))

defineProps<{ option: Record<string, unknown> }>()
</script>

<template>
  <div class="report-chart">
    <Suspense>
      <VChart class="report-chart__canvas" :option="option" autoresize />
      <template #fallback>
        <p class="report-chart__loading" role="status">图表正在加载……</p>
      </template>
    </Suspense>
  </div>
</template>

<style scoped>
.report-chart {
  /* 高度属于始终存在的实体容器，不依赖 Suspense 转发父级类名，也不能由 Canvas 反向撑高。 */
  position: relative;
  width: 100%;
  min-width: 0;
  height: 220px;
  flex: none;
}
.report-chart__canvas {
  position: absolute;
  inset: 0;
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
