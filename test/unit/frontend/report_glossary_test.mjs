import assert from 'node:assert/strict'
import test from 'node:test'

// F-042 本轮：报告中心名词解释术语表的完整性门禁。
// 用户不熟悉测试术语，术语表是「名词解释 info 图标」与「指标口径说明」总入口的唯一来源，
// 因此必须：条目齐全、每条都说清算法/来源/易误解点、三个页面引用的 id 都真实存在、
// 分组覆盖全部条目（不允许有孤儿名词），文案里不残留内部英文键。
import {
  overviewTermIds,
  pathTermIds,
  planTermIds,
  reportTermGroups,
  reportTermById,
  reportTerms,
  reportTermsByIds,
} from '../../../web/src/features/reports/glossary.ts'

// 计划功能文档第 3 节列出的必备名词分组，逐组核对是否全部落地。
const requiredGroups = [
  ['运行事实', ['run', 'path-run', 'step', 'instance-ref', 'generated-at']],
  ['比率', ['success-rate', 'first-pass-rate', 'path-coverage']],
  ['耗时', ['step-duration', 'interface-duration', 'executor-duration', 'orchestration-duration', 'queue-wait']],
  ['统计', ['percentile', 'mean', 'sample-count', 'sample-cap']],
  ['接口与重试', ['read-request', 'write-request', 'interface-calls', 'retry']],
  ['结果', ['result-succeeded', 'result-failed', 'result-stopped', 'result-cancelled', 'result-awaiting', 'failure-class']],
  ['数据来源', ['run-records', 'network-log']],
]

test('必备名词全部落地，一条不少', () => {
  for (const [group, ids] of requiredGroups) {
    for (const id of ids) {
      assert.ok(reportTermById(id), `术语表缺少 ${group} 分组下的名词：${id}`)
    }
  }
  assert.equal(reportTerms.length, requiredGroups.flatMap(([, ids]) => ids).length, '术语条目数应与必备清单一致')
})

test('每条解释都写清算式、来源与易误解点，且不留内部英文键', () => {
  for (const term of reportTerms) {
    assert.ok(term.name && term.summary && term.formula && term.source && term.caveat, `${term.id} 有字段为空`)
    for (const field of [term.summary, term.formula, term.source, term.caveat]) {
      // 内部标识形如 duration_ms / path_runs / P95 之类的下划线键或裸英文键名不允许出现。
      assert.ok(!/[a-z]+_[a-z_]+/.test(field), `${term.id} 的解释里出现内部英文键：${field}`)
    }
  }
})

test('三个页面引用的名词 id 都真实存在，且首页覆盖全部名词', () => {
  for (const id of [...planTermIds, ...pathTermIds]) {
    assert.ok(reportTermById(id), `页面引用了不存在的名词：${id}`)
  }
  const overview = new Set(overviewTermIds)
  assert.equal(overview.size, reportTerms.length, '首页总入口应列出全部名词')
  for (const term of reportTerms) {
    assert.ok(overview.has(term.id), `首页总入口缺少名词：${term.id}`)
  }
  // 计划详情不列只有首页才有的名词（执行器耗时、写请求等属于路径级）。
  assert.ok(!planTermIds.includes('executor-duration'), '计划详情不应列执行器耗时（该级看不到）')
  assert.ok(pathTermIds.includes('executor-duration'), '路径详情必须列执行器耗时')
})

test('分组覆盖全部条目，没有孤儿名词；按 id 查询保持顺序并过滤未知 id', () => {
  const grouped = new Set(reportTermGroups.flatMap((group) => group.ids))
  for (const term of reportTerms) {
    assert.ok(grouped.has(term.id), `名词没有归入任何分组：${term.id}`)
  }
  assert.equal(grouped.size, reportTerms.length, '分组里不应有重复名词')

  const picked = reportTermsByIds(['success-rate', 'not-exist', 'path-coverage'])
  assert.deepEqual(picked.map((term) => term.id), ['success-rate', 'path-coverage'], '应按传入顺序返回并过滤未知 id')
  assert.equal(reportTermById('not-exist'), undefined, '未知 id 必须返回 undefined，由调用方决定不渲染图标')
})

test('计时/未知口径与产品说明一致：未知不用 0ms 冒充', () => {
  // 这三条是用户最容易读错的口径，文案必须包含关键降级说法，避免页面显示 0 造成误解。
  const interfaceTerm = reportTermById('interface-duration')
  assert.ok(interfaceTerm.caveat.includes('暂无真实请求耗时'), '接口耗时必须说明历史无日志时的降级文案')
  const orchestrationTerm = reportTermById('orchestration-duration')
  assert.ok(orchestrationTerm.caveat.includes('未知'), '工具端编排耗时必须说明分量为缺时显示未知')
  const sampleTerm = reportTermById('sample-count')
  assert.ok(sampleTerm.caveat.includes('暂无足够样本'), '样本数必须说明无样本时不显示 0')
})
