<script setup lang="ts">
import { NCard } from 'naive-ui'
import { computed } from 'vue'

import type { LatencyStats, ReportMetrics } from './api'
import { formatMs, formatPercent } from './api'
import { VChart } from './echarts'

// ReportMetricsPanel 渲染一个范围（全局/计划/路径）的专业测试指标：KPI 卡片、时延分位图与分类占比图。
// L0/L1/L2 复用同一组件，保证同口径同呈现；每张卡片附中文释义，说明「这个数怎么算、分母是什么」。
const props = defineProps<{ metrics: ReportMetrics }>()

// 图表配色：与产品浅/深主题解耦，用固定语义色（成功=绿、失败=红、其余=中性）保证可读。
const resultColors: Record<string, string> = {
  成功: '#18a058',
  失败: '#d03050',
  已停止: '#f0a020',
  已取消: '#909399',
  结果待确认: '#2080f0',
}
const failurePalette = ['#d03050', '#f0a020', '#7c3aed', '#2080f0', '#909399']

// hasResults / hasFailures 决定是否渲染饼图；空数据改用文字状态，不渲染空图。
const hasResults = computed(() => Object.keys(props.metrics.resultDistribution || {}).length > 0)
const hasFailures = computed(() => Object.keys(props.metrics.failureClasses || {}).length > 0)

// buildLatencyOption 生成单口径时延分位柱状图配置（P50/P90/P95/均值）。
function buildLatencyOption(stats: LatencyStats, color: string): Record<string, unknown> {
  return {
    tooltip: { trigger: 'axis', valueFormatter: (value: number) => formatMs(value) },
    grid: { left: 48, right: 16, top: 24, bottom: 28 },
    xAxis: { type: 'category', data: ['P50', 'P90', 'P95', '均值'] },
    yAxis: { type: 'value', axisLabel: { formatter: (value: number) => formatMs(value) } },
    series: [{
      type: 'bar',
      barWidth: '52%',
      itemStyle: { color, borderRadius: [4, 4, 0, 0] },
      data: [stats.p50, stats.p90, stats.p95, stats.mean],
    }],
  }
}

// buildPieOption 生成结果/分类占比环形图配置；颜色按语义映射，未知分类回落到调色板。
function buildPieOption(distribution: Record<string, number>, colorFor: (label: string, index: number) => string): Record<string, unknown> {
  const data = Object.entries(distribution).map(([name, value], index) => ({
    name,
    value,
    itemStyle: { color: colorFor(name, index) },
  }))
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, type: 'scroll' },
    series: [{
      type: 'pie',
      radius: ['40%', '68%'],
      center: ['50%', '44%'],
      label: { formatter: '{b} {c}' },
      data,
    }],
  }
}

const stepLatencyOption = computed(() => buildLatencyOption(props.metrics.stepLatencyMs, '#2080f0'))
const interfaceLatencyOption = computed(() => buildLatencyOption(props.metrics.interfaceLatencyMs, '#18a058'))
const resultPieOption = computed(() => buildPieOption(props.metrics.resultDistribution || {}, (label, index) => resultColors[label] || failurePalette[index % failurePalette.length]))
const failurePieOption = computed(() => buildPieOption(props.metrics.failureClasses || {}, (_, index) => failurePalette[index % failurePalette.length]))
</script>

<template>
  <div class="metrics-panel">
    <!-- 专业指标 KPI 卡片：每张附中文释义写清算法与分母 -->
    <section class="metrics-panel__kpis">
      <NCard size="small" class="kpi">
        <p class="kpi__label">路径运行成功率</p>
        <p class="kpi__value">
          <template v-if="metrics.finishedPathRuns > 0">{{ formatPercent(metrics.successRate) }}</template>
          <template v-else>暂无已结束运行</template>
        </p>
        <p class="kpi__hint">成功的路径运行数 {{ metrics.succeededPathRuns }} / 已结束路径运行数 {{ metrics.finishedPathRuns }}。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">一次通过率（FPY）</p>
        <p class="kpi__value">
          <template v-if="metrics.finishedPathRuns > 0">{{ formatPercent(metrics.firstPassRate) }}</template>
          <template v-else>暂无已结束运行</template>
        </p>
        <p class="kpi__hint">无重试即成功的路径运行数 {{ metrics.firstPassRuns }} / 已结束路径运行数 {{ metrics.finishedPathRuns }}。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">路径覆盖度</p>
        <p class="kpi__value">
          <template v-if="metrics.totalPaths > 0">{{ formatPercent(metrics.pathCoverage) }}</template>
          <template v-else>暂无已保存路径</template>
        </p>
        <p class="kpi__hint">至少运行过一次的路径 {{ metrics.coveredPaths }} / 已保存路径 {{ metrics.totalPaths }}。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">目标接口调用</p>
        <p class="kpi__value">{{ metrics.interfaceCalls }} 次</p>
        <p class="kpi__hint">读 {{ metrics.interfaceReadCalls }} 次 · 写 {{ metrics.interfaceWriteCalls }} 次（来自 network.log 真实传输记录）。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">重试统计</p>
        <p class="kpi__value">{{ metrics.retryAttempts }} 次</p>
        <p class="kpi__hint">发生重试的路径运行 {{ metrics.retriedPathRuns }} 条；写连接未建立的重试逐条计数，不合并。</p>
      </NCard>
    </section>

    <!-- 时延分位：步骤耗时与目标接口耗时口径不同，分开展示，绝不合并 -->
    <section class="metrics-panel__latency">
      <NCard size="small" title="步骤耗时分位" class="chart-card">
        <template #header-extra><span class="chart-card__note">来自步骤尝试墙钟 duration_ms</span></template>
        <VChart v-if="metrics.stepLatencyMs.sampleCount > 0" class="chart" :option="stepLatencyOption" autoresize />
        <p v-else class="chart__empty">暂无足够样本</p>
        <p class="chart-card__foot">样本数 {{ metrics.stepLatencyMs.sampleCount }}</p>
      </NCard>
      <NCard size="small" title="目标接口耗时分位" class="chart-card">
        <template #header-extra><span class="chart-card__note">来自 network.log 传输层 duration_s</span></template>
        <VChart v-if="metrics.interfaceKnown && metrics.interfaceLatencyMs.sampleCount > 0" class="chart" :option="interfaceLatencyOption" autoresize />
        <p v-else-if="!metrics.interfaceKnown" class="chart__empty">暂无真实请求耗时</p>
        <p v-else class="chart__empty">暂无足够样本</p>
        <p class="chart-card__foot">
          样本数 {{ metrics.interfaceLatencyMs.sampleCount }}
          <template v-if="metrics.interfaceWriteLatencyMs.sampleCount > 0"> · 写请求均值 {{ formatMs(metrics.interfaceWriteLatencyMs.mean) }}</template>
        </p>
      </NCard>
    </section>

    <!-- 结果与停机分类占比 -->
    <section class="metrics-panel__dist">
      <NCard size="small" title="结果与停机分类占比" class="chart-card">
        <VChart v-if="hasResults" class="chart" :option="resultPieOption" autoresize />
        <p v-else class="chart__empty">暂无已结束运行</p>
      </NCard>
      <NCard size="small" title="失败分类分布" class="chart-card">
        <VChart v-if="hasFailures" class="chart" :option="failurePieOption" autoresize />
        <p v-else class="chart__empty">范围内暂无失败运行</p>
      </NCard>
    </section>

    <p v-if="metrics.sampleCapped" class="metrics-panel__capped">
      接口耗时样本已达采样上限，仅统计最近的运行样本；计数类指标仍为全量。
    </p>
  </div>
</template>

<style scoped>
.metrics-panel { display: grid; gap: 16px; }
.metrics-panel__kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}
.kpi__label { margin: 0; color: var(--text-color-3); font-size: 13px; }
.kpi__value { margin: 6px 0; font-size: 24px; font-weight: 600; color: var(--text-color-1); }
.kpi__hint { margin: 0; color: var(--text-color-3); font-size: 12px; line-height: 1.6; }
.metrics-panel__latency,
.metrics-panel__dist {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
}
.chart { height: 260px; }
.chart__empty {
  display: flex; align-items: center; justify-content: center;
  height: 260px; margin: 0; color: var(--text-color-3);
}
.chart-card__note { color: var(--text-color-3); font-size: 12px; }
.chart-card__foot { margin: 8px 0 0; color: var(--text-color-3); font-size: 12px; }
.metrics-panel__capped { margin: 0; color: var(--warning-color, #f0a020); font-size: 12px; }
</style>
