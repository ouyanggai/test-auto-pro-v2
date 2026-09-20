<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NDataTable,
  NIcon,
  NProgress,
  NRadioGroup,
  NRadioButton,
  NSelect,
  NTag,
  useThemeVars,
  type DataTableColumns,
} from 'naive-ui'
import {
  ArrowForwardOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  ShieldCheckmarkOutline,
  PulseOutline,
  TimeOutline,
  FlashOutline,
  SpeedometerOutline,
  GitBranchOutline,
  RefreshOutline,
} from '@vicons/ionicons5'

const router = useRouter()
const themeVars = useThemeVars()

// 过滤与时间控制
const timeRange = ref('30d')
const planFilter = ref('all')

const planFilterOptions = [
  { label: '全部投产计划 (3 个计划 / 10 条路径)', value: 'all' },
  { label: '投资立项常规分支全量校验', value: 'plan-1' },
  { label: '风控合规加签与会签组合验证', value: 'plan-2' },
  { label: '出纳复核异常驳回与取回链路', value: 'plan-3' },
]

// 核心测试度量 KPI 数据 (包含通过率、覆盖率、稳定性、拦截量)
const kpis = [
  {
    title: '累计执行路径',
    value: '486',
    unit: '条次',
    subtext: '覆盖 10 条业务拓扑分支',
    badge: '覆盖度 94.2%',
    type: 'default',
  },
  {
    title: '放行一次性通过率 (FPY)',
    value: '96.3',
    unit: '%',
    subtext: '468 步终局放行 / 18 次阻断',
    badge: '环比 +1.8%',
    type: 'success',
  },
  {
    title: '步骤断言通过率',
    value: '99.1',
    unit: '%',
    subtext: '共 3,420 处前置/后置断言校验',
    badge: '31 处边界异常',
    type: 'info',
  },
  {
    title: '安全停机防御拦截',
    value: '14',
    unit: '次',
    subtext: '规避无效加签与脏写写出',
    badge: '100% 成功阻断',
    type: 'warning',
  },
]

// 耗时阶梯（时延性能洞察）
const latencyMetrics = [
  { label: '平均历时 (Avg)', value: '4.2s', desc: '全链路执行均值', status: 'optimal' },
  { label: '中位数 (P50)', value: '3.4s', desc: '典型快速路径响应', status: 'optimal' },
  { label: '长尾延迟 (P90)', value: '6.8s', desc: '含多级动态加签流转', status: 'acceptable' },
  { label: '峰值延迟 (P95)', value: '8.6s', desc: '涉及会签与重新发起回读', status: 'warning' },
]

// 计划维度深度度量模型
interface PlanQualityMetric {
  id: string
  name: string
  version: string
  totalRuns: number
  pathCoverage: number
  passRate: number
  avgDuration: string
  p95Duration: string
  flakyRate: string
  health: 'healthy' | 'warning'
  lastRunTime: string
}

const planQualityList: PlanQualityMetric[] = [
  {
    id: 'plan-1',
    name: '投资立项常规分支全量校验',
    version: 'v2.4.0',
    totalRuns: 218,
    pathCoverage: 100,
    passRate: 98.6,
    avgDuration: '3.8s',
    p95Duration: '6.2s',
    flakyRate: '0.4%',
    health: 'healthy',
    lastRunTime: '10分钟前',
  },
  {
    id: 'plan-2',
    name: '风控合规加签与会签组合验证',
    version: 'v1.8.2',
    totalRuns: 164,
    pathCoverage: 91.5,
    passRate: 94.5,
    avgDuration: '5.4s',
    p95Duration: '9.8s',
    flakyRate: '2.1%',
    health: 'warning',
    lastRunTime: '35分钟前',
  },
  {
    id: 'plan-3',
    name: '出纳复核异常驳回与取回链路',
    version: 'v3.1.0',
    totalRuns: 104,
    pathCoverage: 88.0,
    passRate: 93.2,
    avgDuration: '3.6s',
    p95Duration: '7.4s',
    flakyRate: '1.2%',
    health: 'healthy',
    lastRunTime: '2小时前',
  },
]

const filteredPlans = computed(() => {
  if (planFilter.value === 'all') return planQualityList
  return planQualityList.filter(p => p.id === planFilter.value)
})

// 根因分析分类
const rootCauseBreakdown = [
  {
    category: '目标平台会话失效 (AUTH_401)',
    count: 9,
    ratio: 50.0,
    severity: '高',
    defense: '重定向拦截，零脏写',
    color: '#f0a020',
  },
  {
    category: '加签节点人员不可登录 (安全停机)',
    count: 4,
    ratio: 22.2,
    severity: '中',
    defense: '排除名单拦截规避',
    color: '#2080f0',
  },
  {
    category: '表单字段必填校验未放行 (业务校验)',
    count: 3,
    ratio: 16.7,
    severity: '低',
    defense: '执行前基础数据探测',
    color: '#d03050',
  },
  {
    category: '取回动作已办事实滞后 (异步竞态)',
    count: 2,
    ratio: 11.1,
    severity: '中',
    defense: '轮询事实确认，安全等待',
    color: '#8a2be2',
  },
]

// 审计流水
interface AuditRow {
  runId: string
  planName: string
  pathName: string
  status: 'passed' | 'failed' | 'stopped'
  stepsCount: number
  p95Delta: string
  duration: string
  executedAt: string
}

const auditRows: AuditRow[] = [
  {
    runId: 'RUN-20260919-08',
    planName: '风控合规加签与会签组合验证',
    pathName: '分支 A：加签经办人 -> 插入审批点 -> 终审放行',
    status: 'passed',
    stepsCount: 9,
    p95Delta: '-1.4s',
    duration: '4.8s',
    executedAt: '2026-09-19 15:42:10',
  },
  {
    runId: 'RUN-20260919-07',
    planName: '出纳复核异常驳回与取回链路',
    pathName: '主路径：提交立项 -> 财务初审 -> 模拟取回重审',
    status: 'passed',
    stepsCount: 6,
    p95Delta: '-0.8s',
    duration: '3.2s',
    executedAt: '2026-09-19 15:10:04',
  },
  {
    runId: 'RUN-20260919-06',
    planName: '风控合规加签与会签组合验证',
    pathName: '分支 B：多级会签并发 -> 会签人排除名单核验',
    status: 'stopped',
    stepsCount: 5,
    p95Delta: '+1.2s',
    duration: '6.1s',
    executedAt: '2026-09-19 14:22:31',
  },
  {
    runId: 'RUN-20260919-05',
    planName: '投资立项常规分支全量校验',
    pathName: '全路径：发起 -> 部门核验 -> 风险审查 -> 会签同意',
    status: 'passed',
    stepsCount: 11,
    p95Delta: '-0.3s',
    duration: '5.2s',
    executedAt: '2026-09-19 13:58:19',
  },
  {
    runId: 'RUN-20260919-04',
    planName: '出纳复核异常驳回与取回链路',
    pathName: '分支 C：驳回后重新发起 -> 会话时效探测',
    status: 'failed',
    stepsCount: 4,
    p95Delta: '-2.1s',
    duration: '2.1s',
    executedAt: '2026-09-19 11:30:45',
  },
]

function openPlanReport(planId: string) {
  void router.push(`/reports/${planId}`)
}

const auditColumns: DataTableColumns<AuditRow> = [
  { title: '运行流水号', key: 'runId', width: 170 },
  { title: '所属测试计划', key: 'planName', minWidth: 170, ellipsis: { tooltip: true } },
  { title: '执行路径拓扑链路', key: 'pathName', minWidth: 260, ellipsis: { tooltip: true } },
  {
    title: '放行状态',
    key: 'status',
    width: 105,
    render: (row) => {
      if (row.status === 'passed') return '成功通过'
      if (row.status === 'failed') return '执行失败'
      return '安全停机'
    },
  },
  { title: '校验步骤', key: 'stepsCount', width: 90, render: (row) => `${row.stepsCount} 步` },
  { title: '时延表现', key: 'duration', width: 95 },
  { title: 'P95 偏离度', key: 'p95Delta', width: 100 },
  { title: '执行记录时间', key: 'executedAt', width: 170 },
]
</script>

<template>
  <div class="report-view">
    <!-- 头部：全局控制与报告范围标识 -->
    <header class="report-header">
      <div class="report-header__intro">
        <div class="header-badge-row">
          <span class="report-tag">QA Quality Dashboard</span>
          <span class="report-tag report-tag--muted">自动化测试度量体系</span>
        </div>
        <h1 class="report-title">自动化测试报告中心</h1>
        <p class="report-desc">
          提供跨计划全貌、路径放行率（FPY）、时延阶梯（P50/P90/P95）及异常阻断根因分析，实现从宏观覆盖度到微观执行事实的完整洞察。
        </p>
      </div>

      <div class="report-header__actions">
        <n-radio-group v-model:value="timeRange" size="small">
          <n-radio-button value="7d">近 7 天</n-radio-button>
          <n-radio-button value="30d">近 30 天</n-radio-button>
          <n-radio-button value="90d">近 90 天</n-radio-button>
        </n-radio-group>
        <n-select
          v-model:value="planFilter"
          size="small"
          :options="planFilterOptions"
          class="plan-filter"
        />
      </div>
    </header>

    <!-- 第一层：总览核心 KPI 矩阵 (4 列等高卡片) -->
    <section class="kpi-row">
      <div v-for="(kpi, idx) in kpis" :key="idx" class="kpi-panel">
        <div class="kpi-panel__top">
          <span class="kpi-panel__title">{{ kpi.title }}</span>
          <span class="kpi-panel__badge" :class="`badge--${kpi.type}`">{{ kpi.badge }}</span>
        </div>
        <div class="kpi-panel__number-row">
          <span class="kpi-panel__number">{{ kpi.value }}</span>
          <span class="kpi-panel__unit">{{ kpi.unit }}</span>
        </div>
        <div class="kpi-panel__footer">{{ kpi.subtext }}</div>
      </div>
    </section>

    <!-- 第二层：时延分位性能看板 (时延表现/阶梯剖析：P50/P90/P95) -->
    <section class="latency-banner">
      <div class="latency-banner__title-col">
        <div class="latency-banner__badge">
          <n-icon :component="SpeedometerOutline" />
          <span>时延阶梯洞察 (Latency Percentiles)</span>
        </div>
        <p class="latency-banner__caption">从单步 API 调用到整路径流转的端到端耗时分位分析</p>
      </div>

      <div class="latency-banner__metrics">
        <div v-for="(item, idx) in latencyMetrics" :key="idx" class="latency-cell">
          <div class="latency-cell__label">{{ item.label }}</div>
          <div class="latency-cell__val">{{ item.value }}</div>
          <div class="latency-cell__desc">{{ item.desc }}</div>
        </div>
      </div>
    </section>

    <!-- 第三层：分维拆解（计划执行矩阵 VS 阻断与停机根因分类） -->
    <section class="content-split-grid">
      <!-- 计划维度分析 -->
      <n-card size="small" class="grid-card">
        <template #header>
          <div class="card-head">
            <div class="card-head__text">
              <span class="card-head__title">测试计划执行与放行全貌</span>
              <span class="card-head__sub">按投产流程模板聚合，点击条目可下钻查看详细路径拓扑</span>
            </div>
          </div>
        </template>

        <div class="plan-list">
          <div
            v-for="plan in filteredPlans"
            :key="plan.id"
            class="plan-card-item"
            @click="openPlanReport(plan.id)"
          >
            <div class="plan-card-item__info">
              <div class="plan-card-item__heading">
                <span class="plan-name">{{ plan.name }}</span>
                <span class="plan-version">{{ plan.version }}</span>
                <n-tag
                  size="small"
                  :bordered="false"
                  :type="plan.health === 'healthy' ? 'success' : 'warning'"
                  class="plan-health-tag"
                >
                  {{ plan.health === 'healthy' ? '健康放行' : '存在偶发停机' }}
                </n-tag>
              </div>

              <div class="plan-card-item__stats">
                <div class="stat-col">
                  <span class="stat-col__lbl">累计验证</span>
                  <span class="stat-col__val"><strong>{{ plan.totalRuns }}</strong> 次</span>
                </div>
                <div class="stat-col">
                  <span class="stat-col__lbl">平均历时</span>
                  <span class="stat-col__val">{{ plan.avgDuration }}</span>
                </div>
                <div class="stat-col">
                  <span class="stat-col__lbl">P95 时延</span>
                  <span class="stat-col__val">{{ plan.p95Duration }}</span>
                </div>
                <div class="stat-col">
                  <span class="stat-col__lbl">Flaky 抖动率</span>
                  <span class="stat-col__val text-mono">{{ plan.flakyRate }}</span>
                </div>
              </div>
            </div>

            <div class="plan-card-item__bars">
              <div class="bar-group">
                <div class="bar-label">
                  <span>路径分支覆盖度</span>
                  <span>{{ plan.pathCoverage }}%</span>
                </div>
                <n-progress
                  type="line"
                  :percentage="plan.pathCoverage"
                  :show-indicator="false"
                  status="info"
                  :height="6"
                />
              </div>

              <div class="bar-group">
                <div class="bar-label">
                  <span>放行通过率 (FPY)</span>
                  <span>{{ plan.passRate }}%</span>
                </div>
                <n-progress
                  type="line"
                  :percentage="plan.passRate"
                  :show-indicator="false"
                  :status="plan.passRate >= 95 ? 'success' : 'warning'"
                  :height="6"
                />
              </div>
            </div>

            <div class="plan-card-item__enter">
              <span>下钻</span>
              <n-icon :component="ArrowForwardOutline" />
            </div>
          </div>
        </div>
      </n-card>

      <!-- 阻断根因分类与防御机制有效性 -->
      <n-card size="small" class="grid-card">
        <template #header>
          <div class="card-head">
            <div class="card-head__text">
              <span class="card-head__title">阻断与停机根因分布</span>
              <span class="card-head__sub">18 次阻断/停机事实按防御机制与平台表现分类归因</span>
            </div>
          </div>
        </template>

        <div class="cause-breakdown-list">
          <div v-for="(item, idx) in rootCauseBreakdown" :key="idx" class="cause-row">
            <div class="cause-row__header">
              <span class="cause-name">{{ item.category }}</span>
              <span class="cause-data">
                <strong>{{ item.count }}</strong> 次 ({{ item.ratio }}%)
              </span>
            </div>
            <div class="cause-track">
              <div class="cause-fill" :style="{ width: `${item.ratio}%`, backgroundColor: item.color }" />
            </div>
            <div class="cause-defense-line">
              <span class="defense-label">应对机制：</span>
              <span class="defense-val">{{ item.defense }}</span>
            </div>
          </div>
        </div>

        <div class="defense-assessment-box">
          <div class="defense-box-title">
            <n-icon :component="ShieldCheckmarkOutline" />
            <span>执行引擎安全评价</span>
          </div>
          <p class="defense-box-desc">
            在累计 486 条次全链路执行中，所有 14 次非预期停机均由引擎前置门禁在 <strong>0 写入</strong> 状态下安全收口，未对目标平台造成无效脏实例。加签排除名单投产后，人员异常阻断率降低 <strong>85%</strong>。
          </p>
        </div>
      </n-card>
    </section>

    <!-- 第四层：执行流水事实 (总：全局透视到微观) -->
    <section class="audit-stream">
      <n-card size="small" class="grid-card">
        <template #header>
          <div class="card-head">
            <div class="card-head__text">
              <span class="card-head__title">典型执行路径事实流水</span>
              <span class="card-head__sub">覆盖加签插入、模拟取回重审与多级分支放行实时审计</span>
            </div>
          </div>
        </template>

        <n-data-table
          :columns="auditColumns"
          :data="auditRows"
          :bordered="false"
          size="small"
        />
      </n-card>
    </section>
  </div>
</template>

<style scoped>
.report-view {
  max-width: 1200px;
  margin: 0 auto;
  padding: 8px 0 40px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

/* 顶部头部 */
.report-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  flex-wrap: wrap;
}

.header-badge-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.report-tag {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(32, 128, 240, 0.1);
  color: #2080f0;
}

.report-tag--muted {
  background: v-bind('themeVars.actionColor');
  color: var(--n-text-color-3);
}

.report-title {
  margin: 0 0 8px;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--n-text-color);
}

.report-desc {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--n-text-color-3);
  max-width: 780px;
}

.report-header__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 14px;
}

.plan-filter {
  width: 270px;
}

/* KPI 卡片网格 */
.kpi-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.kpi-panel {
  background: v-bind('themeVars.cardColor');
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 10px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}

.kpi-panel__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.kpi-panel__title {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--n-text-color-3);
}

.kpi-panel__badge {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
}

.badge--success { background: rgba(24, 160, 88, 0.12); color: #18a058; }
.badge--info { background: rgba(32, 128, 240, 0.12); color: #2080f0; }
.badge--warning { background: rgba(240, 160, 32, 0.12); color: #f0a020; }
.badge--default { background: v-bind('themeVars.actionColor'); color: var(--n-text-color-2); }

.kpi-panel__number-row {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.kpi-panel__number {
  font-size: 30px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: var(--n-text-color);
  line-height: 1;
}

.kpi-panel__unit {
  font-size: 13px;
  color: var(--n-text-color-3);
  font-weight: 500;
}

.kpi-panel__footer {
  font-size: 12px;
  color: var(--n-text-color-3);
}

/* 时延横幅看板 (P50/P90/P95) */
.latency-banner {
  background: v-bind('themeVars.cardColor');
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 10px;
  padding: 16px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

.latency-banner__title-col {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 280px;
}

.latency-banner__badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--n-text-color);
}

.latency-banner__caption {
  margin: 0;
  font-size: 12px;
  color: var(--n-text-color-3);
  line-height: 1.5;
}

.latency-banner__metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 28px;
  flex-grow: 1;
}

.latency-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border-left: 2px solid v-bind('themeVars.borderColor');
  padding-left: 14px;
}

.latency-cell__label {
  font-size: 11.5px;
  color: var(--n-text-color-3);
}

.latency-cell__val {
  font-size: 20px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: var(--n-text-color);
}

.latency-cell__desc {
  font-size: 11.5px;
  color: var(--n-text-color-3);
}

/* 拆分卡片与栅格 */
.content-split-grid {
  display: grid;
  grid-template-columns: 1.6fr 1.1fr;
  gap: 20px;
}

.grid-card {
  border-radius: 12px;
}

.card-head {
  padding: 2px 0;
}

.card-head__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.card-head__title {
  font-size: 15.5px;
  font-weight: 600;
  color: var(--n-text-color);
}

.card-head__sub {
  font-size: 12px;
  color: var(--n-text-color-3);
}

/* 计划列表 */
.plan-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plan-card-item {
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 8px;
  padding: 12px 16px;
  display: grid;
  grid-template-columns: 1.8fr 1.3fr auto;
  align-items: center;
  gap: 16px;
  background: v-bind('themeVars.actionColor');
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
}

.plan-card-item:hover {
  background: v-bind('themeVars.cardColor');
  border-color: v-bind('themeVars.primaryColorHover');
  transform: translateY(-1.5px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.plan-card-item__heading {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.plan-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--n-text-color);
}

.plan-version {
  font-size: 11px;
  padding: 1px 6px;
  background: v-bind('themeVars.tagColor');
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: var(--n-text-color-3);
}

.plan-card-item__stats {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-col {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.stat-col__lbl {
  font-size: 11px;
  color: var(--n-text-color-3);
}

.stat-col__val {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.text-mono {
  font-family: ui-monospace, SFMono-Regular, monospace;
}

.plan-card-item__bars {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.bar-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.bar-label {
  display: flex;
  justify-content: space-between;
  font-size: 11.5px;
  color: var(--n-text-color-2);
}

.plan-card-item__enter {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--n-text-color-3);
  transition: color 0.15s ease;
}

.plan-card-item:hover .plan-card-item__enter {
  color: v-bind('themeVars.primaryColor');
}

/* 阻断归因 */
.cause-breakdown-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0 10px;
}

.cause-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cause-row__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12.5px;
}

.cause-name {
  color: var(--n-text-color-2);
  font-weight: 500;
}

.cause-data {
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: var(--n-text-color-3);
}

.cause-track {
  height: 6px;
  border-radius: 3px;
  background: v-bind('themeVars.actionColor');
  overflow: hidden;
}

.cause-fill {
  height: 100%;
  border-radius: 3px;
}

.cause-defense-line {
  font-size: 11.5px;
  color: var(--n-text-color-3);
}

.defense-val {
  color: var(--n-text-color-2);
}

.defense-assessment-box {
  margin-top: 8px;
  padding: 12px 14px;
  background: v-bind('themeVars.actionColor');
  border-radius: 8px;
  border-left: 3px solid #18a058;
}

.defense-box-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  font-weight: 600;
  color: #18a058;
  margin-bottom: 4px;
}

.defense-box-desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--n-text-color-2);
}

/* 审计流水 */
.audit-stream {
  width: 100%;
}

@media (max-width: 960px) {
  .kpi-row {
    grid-template-columns: repeat(2, 1fr);
  }
  .latency-banner {
    flex-direction: column;
    align-items: flex-start;
  }
  .latency-banner__metrics {
    width: 100%;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }
  .content-split-grid {
    grid-template-columns: 1fr;
  }
  .plan-card-item {
    grid-template-columns: 1fr;
  }
}
</style>
