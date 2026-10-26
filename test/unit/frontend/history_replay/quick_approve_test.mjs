import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import vm from 'node:vm'
import { preparationOutcome } from '../../../../web/src/features/history-replay/preparation.ts'
import * as pathLogic from '../../../../web/src/features/execution-paths/logic.ts'
const { isExecutionPathRunnable } = pathLogic

const path = (id, configurationStatus = 'configured', dataStatus = 'ready') => ({ id: String(id), sequenceNo: id, name: `路径 ${id}`, configurationStatus, dataStatus })
const item = (pathId, issues = [], status = 'ready') => ({ pathId, status, issues })

test('默认勾选和计划列表运行入口接受与服务端一致的两种数据状态', () => {
  for (const dataStatus of ['ready', 'base_ready']) assert.equal(isExecutionPathRunnable(path(1, 'configured', dataStatus)), true)
  for (const dataStatus of ['empty', 'needs_input', 'affected']) assert.equal(isExecutionPathRunnable(path(1, 'configured', dataStatus)), false)
  for (const status of ['pending', 'partial', 'affected']) assert.equal(isExecutionPathRunnable(path(1, status)), false)
})

test('准备完成只核对本次任务路径，不把另一条未选路径算进来', () => {
  const result = preparationOutcome([path(1, 'configured', 'base_ready'), path(2, 'pending', 'empty')], [item(1)])
  assert.equal(result.tone, 'success')
  assert.match(result.message, /1 条路径已准备/)
  assert.match(result.message, /检查并运行/)
  assert.deepEqual(result.details, [])
})

test('数据成功但节点失败、人员不足和已删除路径都不能报告全部成功', () => {
  const failure = '节点 财务会签 至少需要选择 3 名人员'
  const result = preparationOutcome([path(1), path(2, 'pending'), path(3, 'configured', 'needs_input')], [
    item(1), item(2, [{ code: 'AUTO_ACTION_CONFIGURE_FAILED', message: failure, blocking: false }]),
    item(3, [{ code: 'REQUIRED_FIELD_MISSING', message: '金额为必填字段', blocking: true }], 'needs_input'), item(4),
  ])
  assert.equal(result.tone, 'warning')
  assert.match(result.message, /1 条已准备，3 条仍需处理/)
  assert.deepEqual(result.details.map(detail => detail.reason), [failure, '金额为必填字段', '路径已删除或状态未能读取'])
})

test('原配置可运行但本次替换失败时也不得误报快速方案已准备', () => {
  const result = preparationOutcome([path(1)], [item(1, [{ code: 'AUTO_ACTION_CONFIGURE_FAILED', message: '当前处理人规则读取失败', blocking: false }])])
  assert.equal(result.tone, 'warning')
  assert.match(result.details[0].reason, /处理人规则读取失败/)
})

// 运行实际页面的 setup 和异步事件处理，只替代网络、定时器与挂载钩子，不启动浏览器。
function pageFixture(options = {}) {
  const requireWeb = createRequire(new URL('../../../../web/package.json', import.meta.url))
  const vue = requireWeb('vue'), ts = requireWeb('typescript')
  const { parse, compileScript } = requireWeb('vue/compiler-sfc')
  const source = readFileSync(new URL('../../../../web/src/views/PlanPathsView.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'quick-approve-page' })
  const code = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const route = vue.reactive({ params: { id: '1' } }), messages = [], requests = [], timers = new Map()
  let sequence = 0, resolveCreate
  const job = { id: 'job-1', actionMode: 'approve_only', status: 'queued', total: 1, pending: 1, running: 0 }
  const modules = {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {} },
    'vue-router': { useRoute: () => route, useRouter: () => ({ push: () => {} }) },
    'naive-ui': { useMessage: () => Object.fromEntries(['success', 'warning', 'error'].map(tone => [tone, text => messages.push({ tone, text })])) },
    '../features/execution-paths/logic': pathLogic,
    '../features/execution-paths/api': { fetchExecutionPaths: async () => [path(1, 'configured', 'base_ready')] },
    '../features/flow-graph/api': { fetchFlowGraph: async () => ({ nodes: [], edges: [], entryNodeIds: [] }) },
    '../features/plans/persistence': { fetchPlan: async id => ({ id, name: '本次计划', status: 'not_started' }) },
    '../features/history-replay/preparation': { preparationOutcome },
    '../features/history-replay/api': {
      fetchActiveHistoryReplay: async () => options.activeMode ? { ...job, actionMode: options.activeMode } : null,
      createHistoryReplay: (...args) => { requests.push(args); return new Promise(resolve => { resolveCreate = resolve }) },
      fetchHistoryReplay: async () => ({ ...job, status: 'completed', pending: 0, ready: 1 }),
      fetchHistoryReplayItems: async () => ({ items: options.incomplete ? [] : [item(1, options.issues ?? [])] }),
    },
    '../features/plans/selection': { flowSourceLabels: {} },
    '../utils/uuid': { createUuid: () => '123e4567-e89b-12d3-a456-426614175050' },
  }
  const exports = {}
  vm.runInNewContext(code, { exports, AbortController, window: { requestAnimationFrame: fn => { fn(); return 1 } }, setTimeout: fn => { timers.set(++sequence, fn); return sequence }, clearTimeout: id => timers.delete(id),
    require: name => modules[name] ?? (name.endsWith('.vue') ? { default: {} } : requireWeb(name)),
  })
  const scope = vue.effectScope()
  const page = scope.run(() => exports.default.setup({}, { expose() {} }))
  return { page, route, requests, messages, job, timers, resolve: value => resolveCreate(value ?? job), tick: vue.nextTick, stop: () => scope.stop() }
}

test('实际页面默认全同意，请求等待时冻结勾选，完成后核对明细再报告已准备', async () => {
  const e = pageFixture()
  try {
    await e.page.loadPage()
    assert.equal(e.page.oneClickMode.value, 'approve_only')
    assert.deepEqual([...e.page.selectedRunPathIDs.value], ['1'])
    const pending = e.page.startSelectedPreparation()
    assert.equal(e.page.preparationBusy.value, true)
    e.page.setAllRunPathSelections(false)
    assert.deepEqual([...e.page.selectedRunPathIDs.value], ['1'])
    assert.equal(e.requests[0][0], '1')
    assert.deepEqual(Array.from(e.requests[0][1]), [1])
    assert.equal(e.requests[0][4], 'approve_only')
    e.resolve()
    await pending
    assert.equal(e.timers.size, 1)
    await [...e.timers.values()][0]()
    assert.equal(e.page.preparationJob.value, null)
    assert.equal(e.page.preparationResult.value.tone, 'success')
    assert.match(e.messages[0].text, /检查并运行/)
  } finally { e.stop() }
})

test('实际页面保留完整节点错误；明细未读全时保留任务供重试', async () => {
  for (const incomplete of [false, true]) {
    const issue = { code: 'AUTO_ACTION_CONFIGURE_FAILED', message: '节点 财务审批 候选人数不足', blocking: false }
    const e = pageFixture({ incomplete, issues: [issue] })
    try {
      await e.page.loadPage()
      const pending = e.page.startSelectedPreparation()
      e.resolve()
      await pending
      await [...e.timers.values()][0]()
      assert.equal(e.messages.some(message => message.tone === 'success'), false)
      if (incomplete) {
        assert.equal(e.page.preparationJob.value.id, e.job.id)
        assert.match(e.page.pathSelectionError.value, /未读取完整/)
      } else {
        assert.equal(e.page.preparationResult.value.tone, 'warning')
        assert.equal(e.page.preparationResult.value.details[0].reason, issue.message)
      }
    } finally { e.stop() }
  }
})

test('实际页面切换计划后，迟到的准备响应不能覆盖新计划状态', async () => {
  const e = pageFixture()
  try {
    await e.page.loadPage()
    const pending = e.page.startSelectedPreparation()
    e.route.params.id = '2'
    await e.tick()
    await e.tick()
    e.resolve()
    await pending
    assert.equal(e.page.plan.value.id, '2')
    assert.equal(e.page.preparationJob.value, null)
    assert.equal(e.page.preparationBusy.value, false)
    assert.equal(e.timers.size, 0)
  } finally { e.stop() }
})

test('实际页面刷新恢复任务时显示任务真正的方案', async () => {
  const e = pageFixture({ activeMode: 'random' })
  try {
    await e.page.loadPage()
    await e.tick()
    assert.equal(e.page.oneClickMode.value, 'random')
    assert.equal(e.page.preparationJob.value.actionMode, 'random')
    assert.equal(e.page.preparationBusy.value, true)
  } finally { e.stop() }
})
