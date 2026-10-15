<script setup lang="ts">
import { NButton, NCard, NSpin, NTag } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import ReportGlossaryEntry from '../features/reports/ReportGlossaryEntry.vue'
import ReportMetricsPanel from '../features/reports/ReportMetricsPanel.vue'
import TermHint from '../features/reports/TermHint.vue'
import { fetchReportOverview, formatCount, formatPercent, formatTime, generatedAtText, resultTagType, ReportApiError } from '../features/reports/api'
import { overviewTermIds } from '../features/reports/glossary'
import type { ReportOverview, ReportPlanCard } from '../features/reports/api'

// ReportsView 是报告中心 L0 首页（总）：上半计划情况计数与计划维度表，下半全局专业指标。
// 全部数字来自真实运行记录聚合接口，无任何硬编码统计常量；空/错/加载三态分别用文字状态呈现。
// 本轮排版：计数、计划表、专业指标三段各带小标题；结果列用状态标签；右上角常驻「指标口径说明」。
const router = useRouter()
const overview = ref<ReportOverview | null>(null)
const loading = ref(false)
const errorText = ref('')
let controller: AbortController | null = null

// loadOverview 拉取首页总览聚合；重复进入时中止上一请求，避免竞态覆盖。
async function loadOverview(): Promise<void> {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorText.value = ''
  try {
    overview.value = await fetchReportOverview(controller.signal)
  }
  catch (error) {
    if (controller.signal.aborted) return
    errorText.value = error instanceof ReportApiError ? error.message : '暂时无法读取报告数据，请重试'
    overview.value = null
  }
  finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

// openPlan 下钻到 L1 计划详情。
function openPlan(plan: ReportPlanCard): void {
  router.push(`/reports/${plan.planId}`)
}

// distributionText 把计划的结果分布映射拼成「成功 3 · 失败 1」的紧凑文本；空映射显示占位符。
function distributionText(distribution: Record<string, number>): string {
  const entries = Object.entries(distribution)
  if (entries.length === 0) return '—'
  return entries.map(([label, count]) => `${label} ${count}`).join(' · ')
}

onMounted(() => { void loadOverview() })
onBeforeUnmount(() => { controller?.abort() })
</script>

<template>
  <section class="report-view">
    <header class="report-view__header">
      <div>
        <h1>报告中心</h1>
        <p class="report-view__hint">从计划全貌逐级下钻到单条运行实例，全部数据来自真实运行记录，只读、不造数。点计划看路径，点路径看每次运行。</p>
      </div>
      <div class="report-view__actions">
        <ReportGlossaryEntry :term-ids="overviewTermIds" />
        <NButton class="app-button--solid-primary" type="primary" :loading="loading" @click="loadOverview">刷新</NButton>
      </div>
    </header>

    <p v-if="errorText" class="report-view__error" role="alert">{{ errorText }}</p>

    <div v-if="loading && !overview" class="report-view__loading"><NSpin size="small" /><span>正在读取报告数据……</span></div>

    <template v-else-if="overview">
      <AppEmptyState
        v-if="overview.counts.plansTotal === 0"
        size="large"
        title="还没有测试计划"
        description="创建测试计划并运行后，报告中心会按真实运行记录汇总这里的统计与图表。"
      >
        <NButton class="app-button--solid-primary" type="primary" @click="router.push('/plans')">前往测试计划</NButton>
      </AppEmptyState>

      <template v-else>
        <!-- 上半部分第一级：全局计划情况计数 -->
        <h2 class="report-view__section-title">计划情况 <span class="report-view__stamp">{{ generatedAtText(overview.generatedAt) }}</span></h2>
        <section class="report-view__counts">
          <NCard size="small" class="count"><p class="count__label">计划总数</p><p class="count__value">{{ overview.counts.plansTotal }}</p></NCard>
          <NCard size="small" class="count"><p class="count__label">有运行记录的计划</p><p class="count__value">{{ overview.counts.plansWithRuns }}</p></NCard>
          <NCard size="small" class="count">
            <p class="count__label">累计运行次数 <TermHint term-id="run" /></p>
            <p class="count__value">{{ overview.counts.runsTotal }}</p>
          </NCard>
          <NCard size="small" class="count">
            <p class="count__label">累计路径运行 <TermHint term-id="path-run" /></p>
            <p class="count__value">{{ formatCount(overview.counts.pathRunsTotal) }}</p>
          </NCard>
          <NCard size="small" class="count"><p class="count__label">进行中运行</p><p class="count__value">{{ overview.counts.runsRunning }}</p></NCard>
        </section>

        <!-- 计划维度表：点击任一行进入 L1 -->
        <NCard size="small" title="计划维度统计" class="report-view__plans">
          <template #header-extra><span class="report-table__note">点击任一行进入该计划的路径指标</span></template>
          <AppEmptyState
            v-if="overview.plans.length === 0"
            title="暂无计划统计"
            description="计划列表为空。"
          />
          <div v-else class="report-table__scroll">
            <table class="report-table">
              <thead>
                <tr>
                  <th>计划</th>
                  <th>运行次数 <TermHint term-id="run" /></th>
                  <th>路径运行 <TermHint term-id="path-run" /></th>
                  <th>成功率 <TermHint term-id="success-rate" /></th>
                  <th>结果分布</th>
                  <th>路径覆盖度 <TermHint term-id="path-coverage" /></th>
                  <th>最近运行</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="plan in overview.plans" :key="plan.planId" class="report-table__row" @click="openPlan(plan)">
                  <td class="report-table__name">{{ plan.planName || `计划 ${plan.planId}` }}</td>
                  <td class="report-table__num">{{ plan.runCount }}</td>
                  <td class="report-table__num">{{ formatCount(plan.pathRunCount) }}</td>
                  <td class="report-table__num">
                    <template v-if="plan.finishedPathRuns > 0">
                      {{ formatPercent(plan.successRate) }}<span class="report-table__sub">（{{ plan.succeededPathRuns }}/{{ plan.finishedPathRuns }}）</span>
                    </template>
                    <template v-else>—</template>
                  </td>
                  <td>{{ distributionText(plan.resultDistribution) }}</td>
                  <td class="report-table__num">
                    <template v-if="plan.totalPaths > 0">
                      {{ formatPercent(plan.pathCoverage) }}<span class="report-table__sub">（{{ plan.coveredPaths }}/{{ plan.totalPaths }}）</span>
                    </template>
                    <template v-else>—</template>
                  </td>
                  <td>
                    <template v-if="plan.lastRunNo">
                      <NTag size="small" :bordered="false" round :type="resultTagType(plan.lastRunResultName || '')">#{{ plan.lastRunNo }} {{ plan.lastRunResultName }}</NTag>
                      <template v-if="plan.lastRunFinishedAt"><span class="report-table__sub">{{ formatTime(plan.lastRunFinishedAt) }}</span></template>
                    </template>
                    <template v-else>未运行</template>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </NCard>

        <!-- 下半部分：全局专业指标 -->
        <h2 class="report-view__section-title">
          专业指标
          <span class="report-view__stamp">{{ generatedAtText(overview.generatedAt) }}</span>
        </h2>
        <ReportMetricsPanel :metrics="overview.metrics" />
      </template>
    </template>
  </section>
</template>

<style scoped>
.report-view { display: grid; gap: 16px; padding: 20px 24px; align-content: start; }
.report-view__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.report-view__header h1 { margin: 0; font-size: 20px; }
.report-view__hint { margin: 6px 0 0; opacity: 0.75; max-width: 720px; }
.report-view__actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.report-view__error { color: var(--error-color); margin: 0; }
.report-view__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }
.report-view__section-title { display: flex; align-items: baseline; gap: 10px; margin: 4px 0 0; font-size: 16px; }
.report-view__stamp { color: var(--text-color-3); font-size: 12px; font-weight: 400; }
.report-view__counts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 12px;
}
.count__label { margin: 0; color: var(--text-color-3); font-size: 13px; }
.count__value { margin: 6px 0 0; font-size: 26px; font-weight: 600; color: var(--text-color-1); }
.report-view__plans :deep(.n-card__content) { padding-top: 8px; }
.report-table__scroll { overflow-x: auto; }
.report-table { width: 100%; border-collapse: collapse; }
.report-table th,
.report-table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid var(--divider-color); white-space: nowrap; }
.report-table th { position: sticky; top: 0; z-index: 1; background: var(--card-color); color: var(--text-color-3); font-weight: 500; }
.report-table__row { cursor: pointer; }
.report-table__row:hover { background: color-mix(in srgb, var(--card-color) 92%, var(--primary-color)); }
.report-table__name { font-weight: 500; }
.report-table__note { color: var(--text-color-3); font-size: 12px; }
.report-table__num { text-align: right; font-variant-numeric: tabular-nums; }
.report-table__sub { margin-left: 6px; color: var(--text-color-3); font-size: 12px; }
</style>
