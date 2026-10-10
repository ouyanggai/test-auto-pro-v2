import assert from 'node:assert/strict'
import test from 'node:test'

// F-042 本轮：报告中心「数据细致权威」门禁——每个数字都要说得清来历。
// 锁定四件事：比率一律带分子分母（分母为 0 不显示 0%）、千分位、
// 数据截至时间缺失时如实说未知（不用当前时间冒充）、分布明细按次数倒序并带占比。
import {
  distributionRows,
  formatCount,
  formatMs,
  formatPercent,
  generatedAtText,
  latencyText,
  ratioText,
  resultTagType,
} from '../../../web/src/features/reports/api.ts'

test('比率带分子分母，分母为 0 时显示占位而不是 0%', () => {
  assert.equal(ratioText(2 / 3, 2, 3), '66.7%（2/3）')
  assert.equal(ratioText(1, 1, 1), '100.0%（1/1）')
  assert.equal(ratioText(0, 0, 0), '暂无样本')
  assert.equal(ratioText(0.5, 1, 2), '50.0%（1/2）')
})

test('大数字带千分位，毫秒跨秒进位到秒', () => {
  assert.equal(formatCount(118919), '118,919')
  assert.equal(formatCount(0), '0')
  assert.equal(formatMs(999), '999 ms')
  assert.equal(formatMs(1000), '1.00 s')
  assert.equal(formatPercent(0.1234), '12.3%')
})

test('数据截至时间：缺失与异常都如实说明，不用当前时间冒充', () => {
  assert.equal(generatedAtText(undefined), '数据截至时间未知')
  assert.equal(generatedAtText(''), '数据截至时间未知')
  assert.match(generatedAtText('2026-10-10T09:30:00Z'), /^数据截至 \d{4}\//)
  assert.match(generatedAtText('not-a-time'), /^数据截至 时间异常$/)
})

test('分布明细按次数倒序并带占比，空分布返回空数组', () => {
  const rows = distributionRows({ 成功: 1, 失败: 3, 已停止: 0 })
  assert.deepEqual(rows.map((row) => row.name), ['失败', '成功'], '零次项不出现，其余按次数倒序')
  assert.equal(rows[0].share, 0.75)
  assert.equal(rows[1].share, 0.25)
  assert.deepEqual(distributionRows(undefined), [])
})

test('分位文本无样本时显示暂无足够样本，有样本给四个数', () => {
  assert.equal(latencyText({ sampleCount: 0, p50: 0, p90: 0, p95: 0, mean: 0 }), '暂无足够样本')
  assert.match(
    latencyText({ sampleCount: 12, p50: 100, p90: 200, p95: 300, mean: 150 }),
    /^P50 100 ms · P90 200 ms · P95 300 ms · 均值 150 ms$/,
  )
})

test('结果名映射为状态标签类型，未知结果不假装成功', () => {
  assert.equal(resultTagType('成功'), 'success')
  assert.equal(resultTagType('失败'), 'error')
  assert.equal(resultTagType('已停止'), 'warning')
  assert.equal(resultTagType('已取消'), 'default')
  assert.equal(resultTagType('结果待确认'), 'info')
  assert.equal(resultTagType(''), 'default')
})
