<script setup lang="ts">
import { NCard } from 'naive-ui'

import ReportGlossaryEntry from './ReportGlossaryEntry.vue'
import TermHint from './TermHint.vue'
import { generatedAtText, latencyText, ratioText } from './api'
import type { ReportMetrics } from './api'

// ReportMetricSummary 是 L1/L2 共用的「核心指标摘要」：本级只看比率、时延、接口与重试四类事实，
// 图表和分布占比属于首页（L0）的一等信息，不在每一级重复出现（本轮层级收敛的核心）。
// 每张卡都带名词解释图标，分子分母直接写在卡上，用户不用猜这个数怎么来的。
defineProps<{ metrics: ReportMetrics, generatedAt?: string, termIds: string[] }>()
</script>

<template>
  <NCard size="small" class="metric-summary">
    <template #header-extra>
      <div class="metric-summary__extra">
        <span class="metric-summary__stamp">{{ generatedAtText(generatedAt) }}</span>
        <ReportGlossaryEntry :term-ids="termIds" />
      </div>
    </template>

    <section class="metric-summary__kpis">
      <div class="metric-summary__kpi">
        <p class="metric-summary__label">路径运行成功率 <TermHint term-id="success-rate" /></p>
        <p class="metric-summary__value">
          {{ ratioText(metrics.successRate, metrics.succeededPathRuns, metrics.finishedPathRuns) }}
        </p>
      </div>
      <div class="metric-summary__kpi">
        <p class="metric-summary__label">一次通过率（FPY） <TermHint term-id="first-pass-rate" /></p>
        <p class="metric-summary__value">
          {{ ratioText(metrics.firstPassRate, metrics.firstPassRuns, metrics.finishedPathRuns) }}
        </p>
      </div>
      <div class="metric-summary__kpi">
        <p class="metric-summary__label">路径覆盖度 <TermHint term-id="path-coverage" /></p>
        <p class="metric-summary__value">
          {{ ratioText(metrics.pathCoverage, metrics.coveredPaths, metrics.totalPaths) }}
        </p>
      </div>
    </section>

    <dl class="metric-summary__facts">
      <div class="metric-summary__fact">
        <dt>步骤耗时 <TermHint term-id="step-duration" /></dt>
        <dd>
          {{ latencyText(metrics.stepLatencyMs) }}
          <span class="metric-summary__sample">样本 {{ metrics.stepLatencyMs.sampleCount }} 次</span>
        </dd>
      </div>
      <div class="metric-summary__fact">
        <dt>目标接口耗时 <TermHint term-id="interface-duration" /></dt>
        <dd>
          <template v-if="metrics.interfaceKnown">{{ latencyText(metrics.interfaceLatencyMs) }}</template>
          <template v-else>暂无真实请求耗时</template>
          <span class="metric-summary__sample">样本 {{ metrics.interfaceLatencyMs.sampleCount }} 条</span>
        </dd>
      </div>
      <div class="metric-summary__fact">
        <dt>目标接口调用 <TermHint term-id="interface-calls" /></dt>
        <dd>
          共 {{ metrics.interfaceCalls }} 次（读 {{ metrics.interfaceReadCalls }} · 写 {{ metrics.interfaceWriteCalls }}）
        </dd>
      </div>
      <div class="metric-summary__fact">
        <dt>重试 <TermHint term-id="retry" /></dt>
        <dd>
          {{ metrics.retryAttempts }} 次，涉及 {{ metrics.retriedPathRuns }} 条路径运行
        </dd>
      </div>
    </dl>
  </NCard>
</template>

<style scoped>
.metric-summary :deep(.n-card__content) {
  display: grid;
  gap: 12px;
}
.metric-summary__extra {
  display: flex;
  align-items: center;
  gap: 10px;
}
.metric-summary__stamp {
  color: var(--text-color-3);
  font-size: 12px;
}
.metric-summary__kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}
.metric-summary__label {
  margin: 0;
  color: var(--text-color-3);
  font-size: 12px;
}
.metric-summary__value {
  margin: 4px 0 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-color-1);
}
.metric-summary__facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 8px 16px;
  margin: 0;
  padding-top: 10px;
  border-top: 1px solid var(--divider-color);
  font-size: 13px;
}
.metric-summary__fact dt {
  color: var(--text-color-3);
  font-size: 12px;
}
.metric-summary__fact dd {
  margin: 2px 0 0;
}
.metric-summary__sample {
  margin-left: 6px;
  color: var(--text-color-3);
  font-size: 12px;
}
</style>
