import assert from 'node:assert/strict'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import vm from 'node:vm'
import test from 'node:test'

const requireWeb = createRequire(new URL('../../../web/package.json', import.meta.url))
const ts = requireWeb('typescript')
const cache = new Map()
const load = name => {
  if (cache.has(name)) return cache.get(name)
  const file = new URL(`../../../web/src/features/runs/${name}.ts`, import.meta.url)
  const exports = {}
  const code = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText
  vm.runInNewContext(code, { exports, require: path => load(path.slice(2)) })
  cache.set(name, exports)
  return exports
}
const { stepOutcome, stepNoActionSummary, stepElapsedText } = load('actionOutcome')
const fact = changes => ({ stepNo: 3, source: 'user_action', status: 'succeeded', statusName: '执行成功', action: 'approve', actionName: '同意', actorName: '欧阳改', nodeKey: 'node', durationMs: 0, startedAt: '', finishedAt: '', attempts: [], ...changes })

test('截图中的跳过事实不显示成功审批，即使有只读请求耗时', () => {
  const step = fact({ status: 'skipped', attempts: [{ durationMs: 0, requests: [{ durationMs: 578 }] }] })
  assert.equal(stepOutcome(step), '已跳过')
  assert.match(stepNoActionSummary(step, '发起人部门主管领导'), /目标平台自动跳过.*未执行「同意」动作/)
  assert.equal(stepElapsedText(step), '未执行动作，无动作耗时')
})

test('只读导航和真实审批使用不同结论；缺失测量不能显示0毫秒', () => {
  const step = fact({ source: 'system_navigation' })
  assert.equal(stepOutcome(step), '节点已确认')
  assert.match(stepNoActionSummary(step, '分支'), /只读确认.*未执行审批动作/)
  assert.equal(stepElapsedText(step), '只读确认，耗时未记录')
  assert.equal(stepElapsedText(fact()), '总耗时未记录')
  assert.equal(stepOutcome(fact()), '成功')
  assert.equal(stepNoActionSummary(fact(), '审批'), '')
  assert.equal(stepOutcome(fact({ status: 'failed' })), '失败')
  assert.equal(stepOutcome(fact({ status: 'uncertain' })), '结果待确认')
  assert.equal(stepOutcome(fact({ status: 'failed' }), true), '阻塞')
})

test('耗时优先步骤测量，起止可证明时计算差值，尝试只标为部分事实', () => {
  assert.equal(stepElapsedText(fact({ durationMs: 12782 })), '总耗时 12.8 秒')
  assert.equal(stepElapsedText(fact({ startedAt: '2026-10-26T00:00:00Z', finishedAt: '2026-10-26T00:00:08.256Z' })), '总耗时 8.3 秒')
  assert.equal(stepElapsedText(fact({ attempts: [{ durationMs: 8256 }] })), '总耗时未记录；已记录尝试耗时 8.3 秒')
  for (const changes of [{ durationMs: -1 }, { durationMs: NaN }, { startedAt: 'invalid' }, { startedAt: '2026-10-26T00:00:01Z', finishedAt: '2026-10-26T00:00:00Z' }]) {
    assert.equal(stepElapsedText(fact(changes)), '总耗时未记录')
  }
})

test('真实动作面板接入结果与耗时展示，跳过不再声称某人执行了同意', () => {
  const { parse, compileScript } = requireWeb('vue/compiler-sfc')
  const { effectScope } = requireWeb('vue')
  const source = fs.readFileSync(new URL('../../../web/src/features/runs/RunNodePanel.vue', import.meta.url), 'utf8')
  const descriptor = parse(source).descriptor
  const script = compileScript(descriptor, { id: 'f052-outcome-panel' })
  const code = ts.transpileModule(script.content, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText
  const exports = {}
  vm.runInNewContext(code, { exports, require: name => name === 'naive-ui' ? { useMessage: () => ({}) } : name.startsWith('./') ? load(name.slice(2)) : requireWeb(name) })
  const scope = effectScope()
  try {
    const panel = scope.run(() => exports.default.setup({ detail: { steps: [], nodeStates: {} }, nodeName: '审批节点', nodeKey: 'node' }, { expose() {}, emit() {} }))
    panel.stepDialog.value = fact({ status: 'skipped' })
    assert.equal(panel.stepOutcomeTitle.value, '已跳过')
    assert.equal(panel.stepOutcomeTone.value, 'info')
    assert.doesNotMatch(panel.stepPlainSummary.value, /欧阳改.*执行了/)
    assert.match(panel.stepPlainSummary.value, /未执行「同意」动作/)
    panel.stepDialog.value = fact({ source: 'system_navigation' })
    assert.equal(panel.stepOutcomeTitle.value, '节点已确认')
    assert.match(panel.stepPlainSummary.value, /只读确认/)
    panel.stepDialog.value = fact({ durationMs: 12782 })
    assert.match(panel.stepPlainSummary.value, /欧阳改.*执行了「同意」/)
    assert.equal(panel.stepElapsedText(panel.stepDialog.value), '总耗时 12.8 秒')
    assert.match(descriptor.template.content, /stepElapsedText\(stepDialog\)/)
    assert.match(descriptor.template.content, /stepElapsedText\(timelineStep\(item.stepNo\)!\)/)
    assert.doesNotMatch(descriptor.template.content, /共用时|formatElapsed\(stepDialog.durationMs\)/)
  } finally { scope.stop() }
})
