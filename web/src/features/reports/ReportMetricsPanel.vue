<script setup lang="ts">
import { NCard, NTag } from 'naive-ui'
import { computed } from 'vue'

import ReportChart from './ReportChart.vue'
import TermHint from './TermHint.vue'
import { chartBaseOption, statusPalette } from './chartTheme'
import { skinTokens } from '../../appearance/skins'
import { useAppStore } from '../../stores/app'
import { distributionRows, formatCount, formatMs, formatPercent, latencyText, ratioText } from './api'
import type { LatencyStats, ReportMetrics } from './api'

// ReportMetricsPanel 只服务首页（L0）：完整专业指标。
// 本轮改动：每个指标带名词解释图标与时序口径，比率卡直接写分子分母，分位卡带样本数，
// 结果/失败分类在图之外补一张数值明细表（图形给观感，表格给可核对的数字）。
// L1/L2 不再复用本组件，改用 ReportMetricSummary，避免三级重复同一套面板。
const props = defineProps<{ metrics: ReportMetrics }>()

const appStore = useAppStore()
const tokens = computed(() => skinTokens[appStore.skinId][appStore.themeMode])
const resultColors = computed(() => statusPalette(tokens.value))
const failurePalette = computed(() => [tokens.value.error, tokens.value.warning, tokens.value.accent2, tokens.value.info, tokens.value.text3])

// hasResults / hasFailures 决定是否渲染饼图；空数据改用文字状态，不渲染空图。
const hasResults = computed(() => Object.keys(props.metrics.resultDistribution || {}).length > 0)
const hasFailures = computed(() => Object.keys(props.metrics.failureClasses || {}).length > 0)
const resultRows = computed(() => distributionRows(props.metrics.resultDistribution))
const failureRows = computed(() => distributionRows(props.metrics.failureClasses))

// buildLatencyOption 生成单口径时延分位柱状图配置（P50/P90/P95/均值）。
function buildLatencyOption(stats: LatencyStats, color: string): Record<string, unknown> {
  const base = chartBaseOption(tokens.value)
  return {
    ...base,
    tooltip: { ...base.tooltip, trigger: 'axis', valueFormatter: (value: number) => formatMs(value) },
    grid: { left: 48, right: 16, top: 24, bottom: 28 },
    xAxis: { ...base.xAxis, type: 'category', data: ['P50', 'P90', 'P95', '均值'] },
    yAxis: { ...base.yAxis, type: 'value', axisLabel: { ...base.yAxis.axisLabel, formatter: (value: number) => formatMs(value) } },
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
  // 环形图不添加坐标轴；只取公共文字、图例与提示样式，避免出现多余坐标轴。
  const base = chartBaseOption(tokens.value)
  const data = Object.entries(distribution).map(([name, value], index) => ({
    name,
    value,
    itemStyle: { color: colorFor(name, index) },
  }))
  return {
    color: base.color,
    textStyle: base.textStyle,
    tooltip: { ...base.tooltip, trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { ...base.legend, bottom: 0, type: 'scroll' },
    series: [{
      type: 'pie',
      radius: ['40%', '68%'],
      center: ['50%', '44%'],
      label: { formatter: '{b} {c}', color: tokens.value.text2 },
      data,
    }],
  }
}

const stepLatencyOption = computed(() => buildLatencyOption(props.metrics.stepLatencyMs, tokens.value.info))
const interfaceLatencyOption = computed(() => buildLatencyOption(props.metrics.interfaceLatencyMs, tokens.value.success))
const resultPieOption = computed(() => buildPieOption(props.metrics.resultDistribution || {}, (label, index) => resultColors.value[label] || failurePalette.value[index % failurePalette.value.length]!))
const failurePieOption = computed(() => buildPieOption(props.metrics.failureClasses || {}, (_, index) => failurePalette.value[index % failurePalette.value.length]!))
</script>

<template>
  <div class="metrics-panel">
    <!-- 专业指标 KPI 卡片：比率一律带分子分母，用户不用猜分母是什么 -->
    <section class="metrics-panel__kpis">
      <NCard size="small" class="kpi">
        <p class="kpi__label">路径运行成功率 <TermHint term-id="success-rate" /></p>
        <p class="kpi__value">
          <template v-if="metrics.finishedPathRuns > 0">{{ ratioText(metrics.successRate, metrics.succeededPathRuns, metrics.finishedPathRuns) }}</template>
          <template v-else>暂无已结束运行</template>
        </p>
        <p class="kpi__hint">分母是已结束的路径运行；还在跑和排队中的不计入。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">一次通过率（FPY） <TermHint term-id="first-pass-rate" /></p>
        <p class="kpi__value">
          <template v-if="metrics.finishedPathRuns > 0">{{ ratioText(metrics.firstPassRate, metrics.firstPassRuns, metrics.finishedPathRuns) }}</template>
          <template v-else>暂无已结束运行</template>
        </p>
        <p class="kpi__hint">无重试即成功的才算一次通过，它不高于成功率。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">路径覆盖度 <TermHint term-id="path-coverage" /></p>
        <p class="kpi__value">
          <template v-if="metrics.totalPaths > 0">{{ ratioText(metrics.pathCoverage, metrics.coveredPaths, metrics.totalPaths) }}</template>
          <template v-else>暂无已保存路径</template>
        </p>
        <p class="kpi__hint">分母是当前保存的路径总数；覆盖度低说明还有分支没验证。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">目标接口调用 <TermHint term-id="interface-calls" /></p>
        <p class="kpi__value">{{ formatCount(metrics.interfaceCalls) }} 次</p>
        <p class="kpi__hint">读 {{ formatCount(metrics.interfaceReadCalls) }} 次 · 写 {{ formatCount(metrics.interfaceWriteCalls) }} 次（来自 network.log 真实传输记录）。</p>
      </NCard>
      <NCard size="small" class="kpi">
        <p class="kpi__label">重试 <TermHint term-id="retry" /></p>
        <p class="kpi__value">{{ formatCount(metrics.retryAttempts) }} 次</p>
        <p class="kpi__hint">涉及 {{ formatCount(metrics.retriedPathRuns) }} 条路径运行；写连接未建立的重试逐条计数，不合并。</p>
      </NCard>
    </section>

    <!-- 时延分位：步骤耗时与目标接口耗时口径不同，分开展示，绝不合并 -->
    <section class="metrics-panel__latency">
      <NCard size="small" title="步骤耗时分位" class="chart-card">
        <template #header-extra><span class="chart-card__note">来自步骤尝试墙钟 <TermHint term-id="step-duration" /></span></template>
        <ReportChart v-if="metrics.stepLatencyMs.sampleCount > 0" :option="stepLatencyOption" />
        <p v-else class="chart__empty">暂无足够样本</p>
        <p class="chart-card__foot">
          样本 {{ formatCount(metrics.stepLatencyMs.sampleCount) }} 次 · {{ latencyText(metrics.stepLatencyMs) }}
          <template v-if="metrics.executorLatencyMs.sampleCount > 0">
            <br>执行器耗时样本 {{ formatCount(metrics.executorLatencyMs.sampleCount) }} 次，均值 {{ formatMs(metrics.executorLatencyMs.mean) }}
          </template>
        </p>
      </NCard>
      <NCard size="small" title="目标接口耗时分位" class="chart-card">
        <template #header-extra><span class="chart-card__note">来自 network.log 传输层 <TermHint term-id="interface-duration" /></span></template>
        <ReportChart v-if="metrics.interfaceKnown && metrics.interfaceLatencyMs.sampleCount > 0" :option="interfaceLatencyOption" />
        <p v-else-if="!metrics.interfaceKnown" class="chart__empty">暂无真实请求耗时</p>
        <p v-else class="chart__empty">暂无足够样本</p>
        <p class="chart-card__foot">
          样本 {{ formatCount(metrics.interfaceLatencyMs.sampleCount) }} 条 · {{ latencyText(metrics.interfaceLatencyMs) }}
          <template v-if="metrics.interfaceWriteLatencyMs.sampleCount > 0"> · 写请求均值 {{ formatMs(metrics.interfaceWriteLatencyMs.mean) }}</template>
        </p>
      </NCard>
    </section>

    <!-- 结果与停机分类占比：图形之外必须给数值明细，用户要能核对 -->
    <section class="metrics-panel__dist">
      <NCard size="small" title="结果与停机分类占比" class="chart-card">
        <template #header-extra><span class="chart-card__note">按已结束的路径运行统计</span></template>
        <ReportChart v-if="hasResults" :option="resultPieOption" />
        <p v-else class="chart__empty">暂无已结束运行</p>
        <table v-if="resultRows.length > 0" class="detail-table">
          <thead>
            <tr><th>结果</th><th>次数</th><th>占比</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in resultRows" :key="row.name">
              <td><NTag size="small" :bordered="false" round>{{ row.name }}</NTag></td>
              <td class="detail-table__num">{{ formatCount(row.count) }}</td>
              <td class="detail-table__num">{{ formatPercent(row.share) }}</td>
            </tr>
          </tbody>
        </table>
      </NCard>
      <NCard size="small" title="失败分类分布" class="chart-card">
        <template #header-extra><span class="chart-card__note"><TermHint term-id="failure-class" />只统计失败/停止的运行</span></template>
        <ReportChart v-if="hasFailures" :option="failurePieOption" />
        <p v-else class="chart__empty">范围内暂无失败运行</p>
        <table v-if="failureRows.length > 0" class="detail-table">
          <thead>
            <tr><th>分类</th><th>次数</th><th>占比</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in failureRows" :key="row.name">
              <td>{{ row.name }}</td>
              <td class="detail-table__num">{{ formatCount(row.count) }}</td>
              <td class="detail-table__num">{{ formatPercent(row.share) }}</td>
            </tr>
          </tbody>
        </table>
      </NCard>
    </section>

    <p v-if="metrics.sampleCapped" class="metrics-panel__capped">
      <TermHint term-id="sample-cap" />
      接口耗时样本已达采样上限，仅统计最近的运行样本；计数类指标仍为全量。
    </p>
  </div>
</template>

<style scoped>
.metrics-panel { display: grid; gap: 16px; }
.metrics-panel__kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 12px;
}
.kpi__label { margin: 0; color: var(--text-color-3); font-size: 13px; }
.kpi__value { margin: 8px 0; font-size: 28px; line-height: 36px; font-weight: 600; letter-spacing: -.01em; font-variant-numeric: tabular-nums; color: var(--text-color-1); overflow-wrap: anywhere; }
.kpi__hint { margin: 0; color: var(--text-color-3); font-size: 12px; line-height: 1.6; }
.metrics-panel__latency,
.metrics-panel__dist {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 12px;
}
.chart__empty {
  display: flex; align-items: center; justify-content: center;
  height: 220px; margin: 0; color: var(--text-color-3);
}
.chart-card__note { color: var(--text-color-3); font-size: 12px; }
.chart-card__foot { margin: 8px 0 0; color: var(--text-color-3); font-size: 12px; line-height: 1.7; }
/* 图下数值明细：等宽数字右对齐，和图形一一对应，便于人工核对 */
.detail-table { width: 100%; margin-top: 10px; border-collapse: collapse; font-size: 12px; }
.detail-table th,
.detail-table td { padding: 5px 8px; text-align: left; border-bottom: 1px solid var(--divider-color); }
.detail-table__num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  font-family: var(--font-family-mono);
}
.metrics-panel__capped { margin: 0; color: var(--warning-color); font-size: 12px; }
</style>
