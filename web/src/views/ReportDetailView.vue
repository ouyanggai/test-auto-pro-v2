<script setup lang="ts">
import { NButton, NCard, NSpin, NTag } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import ReportGlossaryEntry from '../features/reports/ReportGlossaryEntry.vue'
import ReportMetricSummary from '../features/reports/ReportMetricSummary.vue'
import TermHint from '../features/reports/TermHint.vue'
import { fetchPlanReport, formatMs, formatPercent, formatTime, generatedAtText, resultTagType, ReportApiError } from '../features/reports/api'
import { planTermIds } from '../features/reports/glossary'
import type { ReportPathCard, ReportPlanDetail } from '../features/reports/api'

// ReportDetailView 是报告中心 L1 计划详情（分）：计划头部、计划核心指标摘要与各路径指标表。
// 本轮层级收敛：这里只放摘要 + 各路径指标表，不再渲染首页那整套指标面板——
// 图表与分布占比是首页（总）的一等信息，三级重复会让用户分不清每级该看什么。
const route = useRoute()
const router = useRouter()
const detail = ref<ReportPlanDetail | null>(null)
const loading = ref(false)
const errorText = ref('')
let controller: AbortController | null = null

// planId 从路由取；切换计划时重新加载。
function currentPlanId(): string {
  return String(route.params.id ?? '')
}

// loadDetail 拉取计划详情聚合。
async function loadDetail(): Promise<void> {
  const planId = currentPlanId()
  if (!planId) return
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorText.value = ''
  try {
    detail.value = await fetchPlanReport(planId, controller.signal)
  }
  catch (error) {
    if (controller.signal.aborted) return
    errorText.value = error instanceof ReportApiError ? error.message : '暂时无法读取计划报告，请重试'
    detail.value = null
  }
  finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

// openPath 下钻到 L2 路径详情。
function openPath(path: ReportPathCard): void {
  router.push(`/reports/${currentPlanId()}/paths/${path.executionPathId}`)
}

watch(() => route.params.id, () => { void loadDetail() })
onMounted(() => { void loadDetail() })
onBeforeUnmount(() => { controller?.abort() })
</script>

<template>
  <section class="report-detail">
    <header class="report-detail__header">
      <div>
        <NButton text type="primary" @click="router.push('/reports')">← 返回报告中心</NButton>
        <h1>{{ detail?.planName || '计划详情' }}</h1>
      </div>
      <div class="report-detail__actions">
        <ReportGlossaryEntry :term-ids="planTermIds" />
        <NButton type="primary" :loading="loading" @click="loadDetail">刷新</NButton>
      </div>
    </header>

    <p v-if="errorText" class="report-detail__error" role="alert">{{ errorText }}</p>

    <div v-if="loading && !detail" class="report-detail__loading"><NSpin size="small" /><span>正在读取计划报告……</span></div>

    <template v-else-if="detail">
      <!-- 计划头部 -->
      <NCard size="small" class="report-detail__meta">
        <div class="meta-grid">
          <div><span class="meta__label">账号</span><span class="meta__value">{{ detail.account || '—' }}</span></div>
          <div><span class="meta__label">流程来源</span><span class="meta__value">{{ detail.flowSource || '—' }}</span></div>
          <div><span class="meta__label">目标对象</span><span class="meta__value">{{ detail.targetObjectName || '—' }}</span></div>
          <div><span class="meta__label">路径总数</span><span class="meta__value">{{ detail.totalPaths }}</span></div>
          <div><span class="meta__label">累计运行次数</span><span class="meta__value">{{ detail.runCount }}</span></div>
          <div><span class="meta__label">数据截至</span><span class="meta__value">{{ generatedAtText(detail.generatedAt) }}</span></div>
        </div>
      </NCard>

      <!-- 计划核心指标摘要：比率与时延口径，图形与分布占比回首页看 -->
      <h2 class="report-detail__section-title">计划核心指标</h2>
      <ReportMetricSummary :metrics="detail.metrics" :generated-at="detail.generatedAt" :term-ids="planTermIds" />

      <!-- 各路径指标表：点击进入 L2 -->
      <NCard size="small" title="各执行路径指标" class="report-detail__paths">
        <template #header-extra><span class="report-table__note">点击任一行进入该路径的运行实例</span></template>
        <AppEmptyState
          v-if="detail.paths.length === 0"
          title="该计划还没有执行路径"
          description="在计划的路径页保存执行路径后，这里会按路径汇总运行指标。"
        />
        <div v-else class="report-table__scroll">
          <table class="report-table">
            <thead>
              <tr>
                <th>序号</th>
                <th>路径名称</th>
                <th>运行次数</th>
                <th>成功率 <TermHint term-id="success-rate" /></th>
                <th>平均步骤数 <TermHint term-id="step" /></th>
                <th>平均步骤耗时 <TermHint term-id="step-duration" /></th>
                <th>平均接口耗时 <TermHint term-id="interface-duration" /></th>
                <th>最近结果</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="path in detail.paths" :key="path.executionPathId" class="report-table__row" @click="openPath(path)">
                <td class="report-table__num">{{ path.sequenceNo }}</td>
                <td class="report-table__name">{{ path.name }}</td>
                <td class="report-table__num">{{ path.pathRunCount }}</td>
                <td class="report-table__num">
                  <template v-if="path.finishedPathRuns > 0">
                    {{ formatPercent(path.successRate) }}<span class="report-table__sub">（{{ path.succeededPathRuns }}/{{ path.finishedPathRuns }}）</span>
                  </template>
                  <template v-else>—</template>
                </td>
                <td class="report-table__num">{{ path.avgSteps > 0 ? path.avgSteps.toFixed(1) : '—' }}</td>
                <td class="report-table__num">{{ path.stepLatencyMs.sampleCount > 0 ? formatMs(path.stepLatencyMs.mean) : '暂无样本' }}</td>
                <td class="report-table__num">{{ path.interfaceLatencyMs.sampleCount > 0 ? formatMs(path.interfaceLatencyMs.mean) : '暂无真实请求耗时' }}</td>
                <td>
                  <NTag v-if="path.lastResultName" size="small" :bordered="false" round :type="resultTagType(path.lastResultName)">{{ path.lastResultName }}</NTag>
                  <template v-else>未运行</template>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="report-detail__legend">
          步骤耗时来自步骤尝试墙钟，接口耗时来自 network.log 传输层，两者口径不同分开展示；样本不足时按「暂无样本/暂无真实请求耗时」如实显示，不用 0 冒充。
        </p>
      </NCard>
    </template>
  </section>
</template>

<style scoped>
.report-detail { display: grid; gap: 16px; padding: 20px 24px; align-content: start; }
.report-detail__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.report-detail__header h1 { margin: 6px 0 0; font-size: 20px; }
.report-detail__actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.report-detail__error { color: var(--error-color, #d03050); margin: 0; }
.report-detail__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }
.report-detail__meta :deep(.n-card__content) { padding-top: 12px; }
.meta-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
.meta__label { display: block; color: var(--text-color-3); font-size: 12px; }
.meta__value { font-size: 15px; font-weight: 500; color: var(--text-color-1); }
.report-detail__section-title { margin: 4px 0 0; font-size: 16px; }
.report-detail__paths :deep(.n-card__content) { padding-top: 8px; }
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
.report-detail__legend { margin: 12px 0 0; color: var(--text-color-3); font-size: 12px; line-height: 1.6; }
</style>
