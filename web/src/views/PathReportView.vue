<script setup lang="ts">
import { NButton, NCard, NSpin, NTag } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import ReportGlossaryEntry from '../features/reports/ReportGlossaryEntry.vue'
import ReportMetricSummary from '../features/reports/ReportMetricSummary.vue'
import TermHint from '../features/reports/TermHint.vue'
import { fetchPathReport, formatCount, formatMs, formatTime, generatedAtText, resultTagType, ReportApiError } from '../features/reports/api'
import { pathTermIds } from '../features/reports/glossary'
import type { ReportPathDetail, ReportRunInstance } from '../features/reports/api'

// PathReportView 是报告中心 L2 路径详情（分）：路径头部、路径核心指标摘要与运行实例列表。
// 运行实例表是本级主体：每行一次运行，重点是「结果 + 耗时分解」；列做合并与右对齐，
// 外包横向滚动 + 粘性表头，避免十几列挤在一行看不清。点击行进 L3 现有运行详情。
const route = useRoute()
const router = useRouter()
const detail = ref<ReportPathDetail | null>(null)
const loading = ref(false)
const errorText = ref('')
let controller: AbortController | null = null

// currentIds 从路由取计划与路径标识。
function currentIds(): { planId: string, pathId: string } {
  return { planId: String(route.params.planId ?? ''), pathId: String(route.params.pathId ?? '') }
}

// loadDetail 拉取路径详情聚合。
async function loadDetail(): Promise<void> {
  const { planId, pathId } = currentIds()
  if (!planId || !pathId) return
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  errorText.value = ''
  try {
    detail.value = await fetchPathReport(planId, pathId, controller.signal)
  }
  catch (error) {
    if (controller.signal.aborted) return
    errorText.value = error instanceof ReportApiError ? error.message : '暂时无法读取路径报告，请重试'
    detail.value = null
  }
  finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

// openRun 下钻到 L3：复用现有运行详情页，不复制第二套明细。
function openRun(run: ReportRunInstance): void {
  router.push(`/runs/${run.runId}/paths/${run.pathRunId}`)
}

// interfaceText / executorText / orchestrationText 按 known 布尔显示真实值或「未知」，绝不用 0 冒充。
function interfaceText(run: ReportRunInstance): string {
  return run.interfaceDurationKnown ? formatMs(run.interfaceDurationMs) : '暂无真实请求耗时'
}
function executorText(run: ReportRunInstance): string {
  return run.executorDurationKnown ? formatMs(run.executorDurationMs) : '未知'
}
function orchestrationText(run: ReportRunInstance): string {
  return run.orchestrationKnown ? formatMs(run.orchestrationDurationMs) : '未知'
}

watch(() => [route.params.planId, route.params.pathId], () => { void loadDetail() })
onMounted(() => { void loadDetail() })
onBeforeUnmount(() => { controller?.abort() })
</script>

<template>
  <section class="path-report">
    <header class="path-report__header">
      <div>
        <NButton text type="primary" @click="router.push(`/reports/${currentIds().planId}`)">← 返回计划详情</NButton>
        <h1>{{ detail ? `路径 ${detail.sequenceNo}：${detail.name}` : '路径详情' }}</h1>
      </div>
      <div class="path-report__actions">
        <ReportGlossaryEntry :term-ids="pathTermIds" />
        <NButton class="app-button--solid-primary" type="primary" :loading="loading" @click="loadDetail">刷新</NButton>
      </div>
    </header>

    <p v-if="errorText" class="path-report__error" role="alert">{{ errorText }}</p>

    <div v-if="loading && !detail" class="path-report__loading"><NSpin size="small" /><span>正在读取路径报告……</span></div>

    <template v-else-if="detail">
      <!-- 路径头部 -->
      <NCard size="small" class="path-report__meta">
        <div class="meta-grid">
          <div><span class="meta__label">路径序号</span><span class="meta__value">{{ detail.sequenceNo }}</span></div>
          <div><span class="meta__label">路径名称</span><span class="meta__value">{{ detail.name }}</span></div>
          <div><span class="meta__label">配置状态</span><span class="meta__value">{{ detail.configurationStatus || '—' }}</span></div>
          <div><span class="meta__label">运行次数</span><span class="meta__value">{{ detail.runCount }}</span></div>
          <div><span class="meta__label">数据截至</span><span class="meta__value">{{ generatedAtText(detail.generatedAt) }}</span></div>
        </div>
      </NCard>

      <!-- 路径核心指标摘要 -->
      <h2 class="path-report__section-title">路径核心指标</h2>
      <ReportMetricSummary :metrics="detail.metrics" :generated-at="detail.generatedAt" :term-ids="pathTermIds" />

      <!-- 运行实例列表：耗时三分解 + 点击进入 L3 -->
      <NCard size="small" title="运行实例" class="path-report__runs">
        <template #header-extra><span class="report-table__note">每行一次运行；耗时分解口径见「指标口径说明」</span></template>
        <AppEmptyState
          v-if="detail.runs.length === 0"
          title="该路径还没有运行实例"
          description="运行包含此路径的测试计划后，这里会按次列出每条运行实例与耗时分解。"
        />
        <div v-else class="report-table__scroll">
          <table class="report-table">
            <thead>
              <tr>
                <th>运行</th>
                <th>结果</th>
                <th>步骤数</th>
                <th>步骤总耗时</th>
                <th>接口耗时</th>
                <th>执行器耗时</th>
                <th>工具端编排耗时</th>
                <th>接口调用</th>
                <th>目标实例</th>
                <th aria-label="操作" />
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in detail.runs" :key="run.pathRunId">
                <td>
                  <span class="report-table__runno">#{{ run.runNo }}</span>
                  <span class="report-table__sub">{{ formatTime(run.startedAt) }}</span>
                </td>
                <td>
                  <div class="report-table__result">
                    <NTag size="small" :bordered="false" round :type="resultTagType(run.resultName)">{{ run.resultName }}</NTag>
                    <span class="report-table__sub">{{ run.statusName }}</span>
                  </div>
                </td>
                <td class="report-table__num">{{ run.stepCount }}</td>
                <td class="report-table__num">{{ formatMs(run.totalDurationMs) }}</td>
                <td class="report-table__num">
                  {{ interfaceText(run) }}
                  <template v-if="run.interfaceDurationKnown && run.interfaceWriteDurationMs > 0">
                    <span class="report-table__sub">写 {{ formatMs(run.interfaceWriteDurationMs) }}</span>
                  </template>
                </td>
                <td class="report-table__num">{{ executorText(run) }}</td>
                <td class="report-table__num">{{ orchestrationText(run) }}</td>
                <td class="report-table__num">
                  {{ formatCount(run.interfaceCalls) }} 次
                  <span class="report-table__sub">
                    <TermHint term-id="interface-calls" />
                  </span>
                </td>
                <td class="report-table__ref">{{ run.mainInstanceRef || '—' }}</td>
                <td><NButton size="small" @click="openRun(run)">查看明细</NButton></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="path-report__legend">
          <TermHint term-id="orchestration-duration" />
          工具端编排耗时 = 步骤总耗时 − 接口传输耗时 − 执行器耗时；runtime 步骤的执行器耗时天然包含目标等待，相减为负或分量缺失时显示「未知」，不硬凑负数。
          非浏览器执行的动作显示「无执行器环节」，历史无该事实显示「未知」。
        </p>
      </NCard>
    </template>
  </section>
</template>

<style scoped>
.path-report { display: grid; gap: 16px; padding: 20px 24px; align-content: start; }
.path-report__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.path-report__header h1 { margin: 6px 0 0; font-size: 20px; }
.path-report__actions { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.path-report__error { color: var(--error-color); margin: 0; }
.path-report__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }
.path-report__meta :deep(.n-card__content) { padding-top: 12px; }
.meta-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
.meta__label { display: block; color: var(--text-color-3); font-size: 12px; }
.meta__value { font-size: 15px; font-weight: 500; color: var(--text-color-1); }
.path-report__section-title { margin: 4px 0 0; font-size: 16px; }
.path-report__runs :deep(.n-card__content) { padding-top: 8px; }
.path-report__note { color: var(--text-color-3); font-size: 12px; }
.report-table__scroll { overflow-x: auto; }
.report-table { width: 100%; border-collapse: collapse; }
.report-table th,
.report-table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid var(--divider-color); white-space: nowrap; }
.report-table th { position: sticky; top: 0; z-index: 1; background: var(--card-color); color: var(--text-color-3); font-weight: 500; }
.report-table__runno { font-weight: 600; }
.report-table__result { display: flex; align-items: center; gap: 6px; }
.report-table__num { text-align: right; font-variant-numeric: tabular-nums; }
.report-table__sub { margin-left: 6px; color: var(--text-color-3); font-size: 12px; }
.report-table__ref { font-family: var(--font-family-mono); font-size: 12px; }
.path-report__legend { margin: 12px 0 0; color: var(--text-color-3); font-size: 12px; line-height: 1.6; }
</style>
