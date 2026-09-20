<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NDataTable,
  NIcon,
  NTag,
  useThemeVars,
  type DataTableColumns,
} from 'naive-ui'
import {
  ArrowBackOutline,
  SpeedometerOutline,
  CheckmarkCircleOutline,
  ShieldCheckmarkOutline,
} from '@vicons/ionicons5'

const route = useRoute()
const router = useRouter()
const themeVars = useThemeVars()

const planId = computed(() => (route.params.id as string) || 'plan-1')

// 计划细化定义
const planDetailMap: Record<string, {
  name: string
  version: string
  templateProxyId: string
  description: string
  totalRuns: number
  passRate: number
  avgDuration: string
  p50Duration: string
  p90Duration: string
  p95Duration: string
  pathsCount: number
  flakyRate: string
}> = {
  'plan-1': {
    name: '投资立项常规分支全量校验',
    version: 'v2.4.0',
    templateProxyId: 'FP-INV-2026-004',
    description: '涵盖立项发起、主部门审查、财务风险评估与投资决策会签同意的全链路流程验证。',
    totalRuns: 218,
    passRate: 98.6,
    avgDuration: '3.8s',
    p50Duration: '3.1s',
    p90Duration: '5.2s',
    p95Duration: '6.2s',
    pathsCount: 4,
    flakyRate: '0.4%',
  },
  'plan-2': {
    name: '风控合规加签与会签组合验证',
    version: 'v1.8.2',
    templateProxyId: 'FP-RISK-2026-012',
    description: '针对风控动态加签节点、多级会签并行、加签人员防错过滤等边界分支的自动化深测。',
    totalRuns: 164,
    passRate: 94.5,
    avgDuration: '5.4s',
    p50Duration: '4.6s',
    p90Duration: '8.4s',
    p95Duration: '9.8s',
    pathsCount: 3,
    flakyRate: '2.1%',
  },
  'plan-3': {
    name: '出纳复核异常驳回与取回链路',
    version: 'v3.1.0',
    templateProxyId: 'FP-FIN-2026-088',
    description: '检验出纳初审驳回至经办人再提交、审批人在下一节点处理前主动取回并重新决策的动作组。',
    totalRuns: 104,
    passRate: 93.2,
    avgDuration: '3.6s',
    p50Duration: '2.9s',
    p90Duration: '5.9s',
    p95Duration: '7.4s',
    pathsCount: 3,
    flakyRate: '1.2%',
  },
}

const currentPlan = computed(() => planDetailMap[planId.value] || planDetailMap['plan-1'])

// 路径细分拓扑
interface PathDetailRow {
  pathId: string
  pathName: string
  pathTopology: string
  execCount: number
  passRate: number
  p50Duration: string
  p95Duration: string
  flakyScore: string
  status: 'optimal' | 'attention'
  lastIssue: string
}

const pathDetailRows: PathDetailRow[] = [
  {
    pathId: 'PATH-01',
    pathName: '主线标准放行路径',
    pathTopology: '发起 -> 主管审批 -> 财务总监 -> 归档',
    execCount: 112,
    passRate: 100,
    p50Duration: '2.8s',
    p95Duration: '3.2s',
    flakyScore: '0.0%',
    status: 'optimal',
    lastIssue: '—',
  },
  {
    pathId: 'PATH-02',
    pathName: '大额投资合规加签路径',
    pathTopology: '发起 -> 主管加签 -> 合规风控审查 -> 插入节点放行',
    execCount: 56,
    passRate: 96.4,
    p50Duration: '4.2s',
    p95Duration: '5.8s',
    flakyScore: '1.8%',
    status: 'optimal',
    lastIssue: '加签人员名单包含不可登录账号 (已由名单排除收敛)',
  },
  {
    pathId: 'PATH-03',
    pathName: '前置驳回重发回流分支',
    pathTopology: '发起 -> 财务初审驳回 -> 重新发起(改值) -> 重新提交',
    execCount: 32,
    passRate: 93.8,
    p50Duration: '3.6s',
    p95Duration: '4.6s',
    flakyScore: '3.1%',
    status: 'attention',
    lastIssue: '目标后端代理树更新延迟，重新发起弹窗短时无响应',
  },
  {
    pathId: 'PATH-04',
    pathName: '多维度条件分支分流路径',
    pathTopology: '发起 -> 预算金额分支判定 -> 专家评审委员会',
    execCount: 18,
    passRate: 100,
    p50Duration: '3.0s',
    p95Duration: '3.9s',
    flakyScore: '0.0%',
    status: 'optimal',
    lastIssue: '—',
  },
]

const pathColumns: DataTableColumns<PathDetailRow> = [
  { title: '路径编号', key: 'pathId', width: 95 },
  { title: '执行路径名称', key: 'pathName', minWidth: 160 },
  { title: '拓扑流转关系', key: 'pathTopology', minWidth: 260, ellipsis: { tooltip: true } },
  { title: '累计验证', key: 'execCount', width: 90, render: (row) => `${row.execCount} 次` },
  {
    title: '放行成功率 (FPY)',
    key: 'passRate',
    width: 130,
    render: (row) => `${row.passRate}%`,
  },
  { title: 'P50 时延', key: 'p50Duration', width: 90 },
  { title: 'P95 时延', key: 'p95Duration', width: 90 },
  { title: 'Flaky 抖动', key: 'flakyScore', width: 95 },
  {
    title: '健康评级',
    key: 'status',
    width: 95,
    render: (row) => (row.status === 'optimal' ? '优良' : '需关注'),
  },
  { title: '近期阻断事实', key: 'lastIssue', minWidth: 200, ellipsis: { tooltip: true } },
]
</script>

<template>
  <div class="detail-page">
    <header class="detail-header">
      <div class="back-nav">
        <n-button
          quaternary
          size="small"
          class="back-btn"
          @click="router.push('/reports')"
        >
          <template #icon>
            <n-icon :component="ArrowBackOutline" />
          </template>
          返回报告中心
        </n-button>
      </div>

      <div class="header-main">
        <div class="header-main__title-row">
          <h1 class="plan-title">{{ currentPlan.name }}</h1>
          <span class="version-tag">{{ currentPlan.version }}</span>
          <span class="proxy-tag">流程代理 ID: {{ currentPlan.templateProxyId }}</span>
        </div>
        <p class="plan-desc">{{ currentPlan.description }}</p>
      </div>
    </header>

    <!-- 顶栏 KPI 指标栏 (包含 P50/P90/P95 完整性能台阶) -->
    <section class="stat-deck">
      <div class="stat-deck-card">
        <span class="stat-deck-card__label">覆盖路径分支</span>
        <span class="stat-deck-card__value">{{ currentPlan.pathsCount }} <small>条</small></span>
        <span class="stat-deck-card__sub">全分支拓扑覆盖</span>
      </div>
      <div class="stat-deck-card">
        <span class="stat-deck-card__label">累计执行总次</span>
        <span class="stat-deck-card__value">{{ currentPlan.totalRuns }} <small>次</small></span>
        <span class="stat-deck-card__sub">稳定回放验证</span>
      </div>
      <div class="stat-deck-card">
        <span class="stat-deck-card__label">放行一次成功率 (FPY)</span>
        <span class="stat-deck-card__value text--success">{{ currentPlan.passRate }} <small>%</small></span>
        <span class="stat-deck-card__sub">Flaky 率 {{ currentPlan.flakyRate }}</span>
      </div>
      <div class="stat-deck-card">
        <span class="stat-deck-card__label">平均 / P95 时延</span>
        <span class="stat-deck-card__value">{{ currentPlan.avgDuration }} <small>/ {{ currentPlan.p95Duration }}</small></span>
        <span class="stat-deck-card__sub">P50: {{ currentPlan.p50Duration }} | P90: {{ currentPlan.p90Duration }}</span>
      </div>
    </section>

    <!-- 拓扑细化表格 -->
    <section class="table-section">
      <n-card size="small" class="section-card">
        <template #header>
          <div class="card-title-group">
            <span class="card-main-title">路径分支流转拓扑与质量下钻明细</span>
            <span class="card-sub-title">按各分支节点流转拓扑切分度量 P50/P95 耗时阶梯与阻断事实</span>
          </div>
        </template>

        <n-data-table
          :columns="pathColumns"
          :data="pathDetailRows"
          :bordered="false"
          size="small"
        />
      </n-card>
    </section>

    <!-- 质量评估与建议 -->
    <section class="summary-section">
      <n-card size="small" class="section-card summary-card">
        <div class="summary-header">
          <n-icon :component="ShieldCheckmarkOutline" />
          <span>自动化测试工程评估与建议</span>
        </div>
        <div class="summary-list">
          <div class="summary-row">
            <span class="summary-tag">覆盖度良好</span>
            <p>主线放行路径（PATH-01）与条件分流（PATH-04）成功率达到 100%，耗时 P95 稳定在 4 秒以内，可作为基线持续回归。</p>
          </div>
          <div class="summary-row">
            <span class="summary-tag">安全防错收益</span>
            <p>加签人员排除名单在 PATH-02 已生效，成功过滤 4 次由于测试人员账号停用造成的无效流转中断。</p>
          </div>
          <div class="summary-row">
            <span class="summary-tag">时延注意项</span>
            <p>PATH-03 涉及驳回重填后代理更新，偶有 P95 达 4.6 秒，属于目标平台底层结构写入同步耗时，已加入平滑等待与重试机制。</p>
          </div>
        </div>
      </n-card>
    </section>
  </div>
</template>

<style scoped>
.detail-page {
  max-width: 1180px;
  margin: 0 auto;
  padding: 8px 0 40px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.back-nav {
  margin-bottom: 8px;
}

.back-btn {
  padding-left: 0;
  color: var(--n-text-color-2);
  font-weight: 500;
  transition: all 0.2s ease;
}

.back-btn:hover {
  color: var(--n-primary-color);
  transform: translateX(-2px);
}

.header-main__title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.plan-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--n-text-color);
}

.version-tag {
  font-size: 11px;
  padding: 2px 7px;
  background: v-bind('themeVars.tagColor');
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: var(--n-text-color-2);
}

.proxy-tag {
  font-size: 12px;
  color: var(--n-text-color-3);
  font-family: ui-monospace, SFMono-Regular, monospace;
}

.plan-desc {
  margin: 0;
  font-size: 13px;
  color: var(--n-text-color-3);
  max-width: 800px;
  line-height: 1.6;
}

/* 顶栏四列卡片 */
.stat-deck {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-deck-card {
  background: v-bind('themeVars.cardColor');
  border: 1px solid v-bind('themeVars.borderColor');
  border-radius: 9px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-deck-card__label {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.stat-deck-card__value {
  font-size: 24px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: var(--n-text-color);
}

.stat-deck-card__value small {
  font-size: 13px;
  font-weight: normal;
  color: var(--n-text-color-3);
}

.stat-deck-card__sub {
  font-size: 11.5px;
  color: var(--n-text-color-3);
}

.text--success {
  color: #18a058;
}

/* 表格卡片 */
.section-card {
  border-radius: 12px;
}

.card-title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.card-main-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--n-text-color);
}

.card-sub-title {
  font-size: 12px;
  color: var(--n-text-color-3);
}

/* 总结建议 */
.summary-card {
  background: v-bind('themeVars.actionColor');
}

.summary-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #18a058;
  margin-bottom: 12px;
}

.summary-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.summary-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-size: 12.5px;
  line-height: 1.55;
}

.summary-tag {
  font-size: 11px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  background: v-bind('themeVars.tagColor');
  color: var(--n-text-color-2);
  flex-shrink: 0;
}

.summary-row p {
  margin: 0;
  color: var(--n-text-color-2);
}

@media (max-width: 860px) {
  .stat-deck {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
