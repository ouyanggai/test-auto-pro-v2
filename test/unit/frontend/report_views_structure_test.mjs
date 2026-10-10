import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

// F-042 本轮：报告中心页面结构门禁（只读源码文本，不启动浏览器）。
// 锁定三件事：① 首页假数据常量不再回来；② 层级收敛——L1/L2 用摘要，整套指标面板只在首页；
// ③ 名词解释入口在三个页面都在，图表走懒加载，分布图下有数值明细。
const here = dirname(fileURLToPath(import.meta.url))
const viewDir = join(here, '../../../web/src/views/')
const featureDir = join(here, '../../../web/src/features/reports/')

// readView 读一个报告视图的源码文本。
function readView(name) {
  return readFileSync(join(viewDir, name), 'utf8')
}

// readFeature 读一个报告中心组件的源码文本。
function readFeature(name) {
  return readFileSync(join(featureDir, name), 'utf8')
}

// 首轮之前的假数据常量名：一旦回来就说明报告中心又变回写死数字。
const fakeConstants = ['const kpis =', 'const latencyMetrics =', 'const planQualityList', 'const rootCauseBreakdown =', 'const auditRows', 'const timeRange =', 'const planFilter =']

test('三个报告页面都没有假数据常量，全部读真实接口', () => {
  for (const view of ['ReportsView.vue', 'ReportDetailView.vue', 'PathReportView.vue']) {
    const source = readView(view)
    for (const constant of fakeConstants) {
      assert.ok(!source.includes(constant), `${view} 不应再出现假数据常量：${constant}`)
    }
    assert.ok(source.includes('fetchReport') || source.includes('fetchPlanReport') || source.includes('fetchPathReport'), `${view} 必须调用真实报告接口`)
  }
})

test('层级收敛：整套指标面板只在首页，L1/L2 只用核心指标摘要', () => {
  const overview = readView('ReportsView.vue')
  assert.ok(overview.includes('ReportMetricsPanel'), '首页应当渲染完整专业指标面板')
  assert.ok(!overview.includes('ReportMetricSummary'), '首页不用摘要组件，摘要属于下一级')

  for (const view of ['ReportDetailView.vue', 'PathReportView.vue']) {
    const source = readView(view)
    assert.ok(source.includes('ReportMetricSummary'), `${view} 应当只渲染核心指标摘要`)
    assert.ok(!source.includes('ReportMetricsPanel'), `${view} 不应再重复整套指标面板（层级收敛）`)
  }
})

test('名词解释入口：三个页面都带行内图标与右上角总入口', () => {
  for (const view of ['ReportsView.vue', 'ReportDetailView.vue', 'PathReportView.vue']) {
    const source = readView(view)
    assert.ok(source.includes('<TermHint'), `${view} 应有行内名词解释图标`)
    assert.ok(source.includes('<ReportGlossaryEntry'), `${view} 应有右上角「指标口径说明」总入口`)
  }
})

test('图表懒加载、分布带数值明细、每张比率卡带分子分母', () => {
  const chart = readFeature('ReportChart.vue')
  assert.ok(chart.includes('defineAsyncComponent'), '图表必须懒加载，首屏不等 ECharts')
  assert.ok(chart.includes('./echarts'), '图表包只能从 echarts.ts 引入')

  const panel = readFeature('ReportMetricsPanel.vue')
  assert.ok(panel.includes('<ReportChart'), '首页图表走懒加载组件')
  assert.ok(panel.includes('distributionRows'), '结果/失败分类必须给数值明细表，不能只有图形')
  assert.ok(panel.includes('ratioText'), '比率卡必须带分子分母')
  assert.ok(panel.includes('sampleCount'), '分位卡必须带样本数')

  // 首页的数据截至时间由页面小节标题给出，不进面板。
  const overview = readView('ReportsView.vue')
  assert.ok(overview.includes('generatedAtText'), '首页应显示数据截至时间')

  const summary = readFeature('ReportMetricSummary.vue')
  assert.ok(summary.includes('ratioText') && summary.includes('latencyText'), '摘要必须给比率与时延口径')
  assert.ok(summary.includes('generatedAtText'), '摘要必须显示数据截至时间')
})

test('路径详情运行实例表给出耗时分解与未知降级，不用 0 冒充', () => {
  const source = readView('PathReportView.vue')
  assert.ok(source.includes('interfaceText'), '接口耗时要按 known 布尔降级')
  assert.ok(source.includes('executorText'), '执行器耗时要按 known 布尔降级')
  assert.ok(source.includes('orchestrationText'), '工具端编排耗时要按 known 布尔降级')
  assert.ok(source.includes('暂无真实请求耗时'), '无 network.log 时必须显示「暂无真实请求耗时」')
  assert.ok(source.includes('report-table__scroll'), '宽表必须可横向滚动')
})
