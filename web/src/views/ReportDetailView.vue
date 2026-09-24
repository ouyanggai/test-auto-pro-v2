<script setup lang="ts">
import { NButton, NCard, NSpin } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import ReportMetricsPanel from '../features/reports/ReportMetricsPanel.vue'
import { fetchPlanReport, formatMs, formatPercent, latencyText, ReportApiError } from '../features/reports/api'
import type { ReportPathCard, ReportPlanDetail } from '../features/reports/api'

// ReportDetailView 是报告中心 L1 计划详情（分）：计划头部、计划级专业指标与各路径指标表。
// 与 L0 同口径但范围收敛到本计划；点击任一路径进入 L2。
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
      <NButton type="primary" :loading="loading" @click="loadDetail">刷新</NButton>
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
        </div>
      </NCard>

      <!-- 计划级专业指标 -->
      <h2 class="report-detail__section-title">计划级专业指标</h2>
      <ReportMetricsPanel :metrics="detail.metrics" />

      <!-- 各路径指标表：点击进入 L2 -->
      <NCard size="small" title="各执行路径指标" class="report-detail__paths">
        <AppEmptyState
          v-if="detail.paths.length === 0"
          title="该计划还没有执行路径"
          description="在计划的路径页保存执行路径后，这里会按路径汇总运行指标。"
        />
        <table v-else class="report-table">
          <thead>
            <tr>
              <th>序号</th>
              <th>路径名称</th>
              <th>运行次数</th>
              <th>成功率</th>
              <th>平均步骤数</th>
              <th>平均步骤耗时</th>
              <th>平均接口耗时</th>
              <th>最近结果</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="path in detail.paths" :key="path.executionPathId" class="report-table__row" @click="openPath(path)">
              <td>{{ path.sequenceNo }}</td>
              <td class="report-table__name">{{ path.name }}</td>
              <td>{{ path.pathRunCount }}</td>
              <td>
                <template v-if="path.finishedPathRuns > 0">{{ formatPercent(path.successRate) }}</template>
                <template v-else>—</template>
              </td>
              <td>{{ path.avgSteps > 0 ? path.avgSteps.toFixed(1) : '—' }}</td>
              <td>{{ path.stepLatencyMs.sampleCount > 0 ? formatMs(path.stepLatencyMs.mean) : '暂无样本' }}</td>
              <td>{{ path.interfaceLatencyMs.sampleCount > 0 ? formatMs(path.interfaceLatencyMs.mean) : '暂无真实请求耗时' }}</td>
              <td>{{ path.lastResultName || '未运行' }}</td>
            </tr>
          </tbody>
        </table>
        <p class="report-detail__legend">步骤耗时来自步骤尝试墙钟，接口耗时来自 network.log 传输层，两者口径不同分开展示。分位明细：{{ latencyText(detail.metrics.stepLatencyMs) }}。</p>
      </NCard>
    </template>
  </section>
</template>

<style scoped>
.report-detail { display: grid; gap: 16px; padding: 20px 24px; }
.report-detail__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.report-detail__header h1 { margin: 6px 0 0; font-size: 20px; }
.report-detail__error { color: var(--error-color, #d03050); margin: 0; }
.report-detail__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }
.meta-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; }
.meta__label { display: block; color: var(--text-color-3); font-size: 12px; }
.meta__value { font-size: 15px; font-weight: 500; color: var(--text-color-1); }
.report-detail__section-title { margin: 8px 0 0; font-size: 16px; }
.report-detail__legend { margin: 12px 0 0; color: var(--text-color-3); font-size: 12px; line-height: 1.6; }
.report-table { width: 100%; border-collapse: collapse; }
.report-table th,
.report-table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid var(--divider-color); }
.report-table__row { cursor: pointer; }
.report-table__row:hover { background: color-mix(in srgb, var(--card-color) 92%, var(--primary-color)); }
.report-table__name { font-weight: 500; }
</style>
