import assert from 'node:assert/strict'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import vm from 'node:vm'
import { synchronizeNativeForm, type NativeFormInput } from '../../../executor-service/src/native-form-values.ts'
import { nativeAuthQuery } from '../../../executor-service/src/native-runtime-auth.ts'

const requireWeb = createRequire(new URL('../../../web/package.json', import.meta.url))
const requireRuntime = createRequire(new URL('../../../executor-service/flow-source/package.json', import.meta.url))
const ts = requireWeb('typescript')
const Vue = requireRuntime('vue')
const clone = (value: unknown) => JSON.parse(JSON.stringify(value))
const source = fs.readFileSync(new URL('../../../executor-service/flow-source/src/lib/vue-form-making/dist/FormMaking.common.js', import.meta.url), 'utf8')
const ast = ts.createSourceFile('native.js', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS)
let formMethods: Record<string, { getText: (ast: unknown) => string }>
let report: Record<string, { getText: (ast: unknown) => string }>
const visit = (node: { properties?: Array<{ name?: { text: string }, initializer?: { text: string } }> }) => {
  if (ts.isObjectLiteralExpression(node)) {
    const props = Object.fromEntries(node.properties!.filter(item => item.name).map(item => [item.name!.text, item]))
    if (!formMethods && props.getComponent && props.refresh && props.getValues) formMethods = props
    if (!report && props.name?.initializer?.text === 'generate-report') report = props
  }
  ts.forEachChild(node, visit)
}
visit(ast)
const nativeContext: Record<string, unknown> = { Promise, _objectSpread: (...items: object[]) => Object.assign({}, ...items) }
nativeContext.lodash__WEBPACK_IMPORTED_MODULE_34___default = { a: { cloneDeep: clone } }
for (const name of source.match(/\b[A-Za-z_$][\w$]*__WEBPACK_IMPORTED_MODULE_\d+__\b/g) || []) if (name.includes('toConsumableArray')) nativeContext[name] = { a: (value: unknown[]) => [...value] }
vm.createContext(nativeContext)
const native = (props: typeof formMethods, name: string) => vm.runInContext(`({${props[name]!.getText(ast)}}).${name}`, nativeContext)

// 生产函数以源码串在独立页面环境执行；测试不向它注入模块闭包，避免通过测试但 page.evaluate 找不到依赖。
const execute = (form: object, input: NativeFormInput) => vm.runInNewContext(`(${synchronizeNativeForm.toString()})`, {
  window: { __executorNative: { initFlow: { $refs: { generateForm: form } } } }
})(input)

function createNativeForm () {
  const Base = Vue.extend()
  const Report = Vue.extend({ name: 'generate-report', inject: ['generateComponentInstance', 'deleteComponentInstance'], beforeUnmount: native(report, 'beforeUnmount') })
  const template = { list: [{ type: 'report', model: 'amountBlock', name: '金额区', options: {}, rows: [{ columns: [{ list: [
    { type: 'select', model: 'currency', name: '币种', options: { defaultValue: 'CNY' } },
    { type: 'number', model: 'amount', name: '外币金额', options: { defaultValue: 0 } },
    { type: 'number', model: 'localAmount', name: '人民币金额', options: { defaultValue: 0 } },
    { type: 'radio', model: 'method', name: '谈判方式', options: { defaultValue: '公开招标' } },
    { type: 'input', model: 'user', name: '申请人', options: { defaultValue: '{"id":"operator","name":"当前用户"}' } }
  ] }] }] }], config: {} }
  const form = new Base({ data: () => ({ data: template, formValue: {}, models: {}, dynamicValueData: [], dynamicHideFields: {}, instanceObject: {} }) })
  for (const name of ['_initForm', 'generateModel', '_setSubDisabled', 'setData', 'getComponent', 'generateComponentInstance', 'deleteComponentInstance']) form[name] = native(formMethods, name)
  Object.assign(form, { _generateRules() {}, _generateDataSource() {}, loadDataSource() {}, loadDynamicValueFx() {}, getValues() { return clone(this.models) }, refresh() { return this._initForm() } })
  let currentReport
  let fieldChanges = 0
  const provider = new Base({ provide: { generateComponentInstance: form.generateComponentInstance.bind(form), deleteComponentInstance: form.deleteComponentInstance.bind(form) } })
  // 使用真实 Vue 响应式监听模拟原生字段联动；v-if 隐藏旧表单后，不再按旧控件的用户修改处理新模型。
  const nextTick = form.$nextTick.bind(form)
  form.$nextTick = callback => nextTick(() => {
    if (!form.formShow && currentReport) { currentReport.$destroy(); currentReport = null }
    callback?.()
    if (form.formShow && !currentReport) {
      currentReport = new Report({ parent: provider })
      currentReport.element = form.formData.list[0]
      native(report, 'mounted').call(currentReport)
      form.$children = [currentReport]
      currentReport.$watch(() => form.models.currency, (value, previous) => {
        if (form.formShow && value !== previous) {
          fieldChanges++
          // 对应这次事故中由用户切币种触发的金额搬移；装载不应该走进这段分支。
          void form.setData({ localAmount: form.models.amount, amount: 0 })
        }
      })
    }
  })
  form.loadEvents = () => { form.eventFunction.refresh = () => {
    form.getComponent('amountBlock').element.currency = form.models.currency
    form.getComponent('amountBlock').element.attachmentSection = form.models.method
    form.models.derived = `人民币 ${form.models.localAmount}`
  } }
  return { form, get fieldChanges() { return fieldChanges }, dispose() { currentReport?.$destroy(); provider.$destroy(); form.$destroy() } }
}

for (const currency of ['CNY', 'GBP', 'USD']) test(`初始化 ${currency} 及重复装载保持金额和谈判方式，不冒充用户修改`, async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    const patch = { currency, amount: 5000001, localAmount: 600, method: '直签' }
    for (let count = 0; count < 3; count++) {
      const result = await execute(fixture.form, { phase: 'hydrate', patch, keys: Object.keys(patch) })
      const verified = await execute(fixture.form, { phase: 'verify', expected: result.expected })
      assert.equal(verified.values.amount, 5000001)
      assert.equal(verified.values.localAmount, 600)
      assert.equal(verified.values.method, '直签')
      assert.equal(verified.values.derived, '人民币 600')
      assert.equal(fixture.form.getComponent('amountBlock').element.attachmentSection, '直签')
      assert.equal(Array.isArray(fixture.form.getComponent('amountBlock')), false)
      assert.equal(fixture.fieldChanges, 0)
    }
  } finally { fixture.dispose() }
})

test('旧的逐项 setData 路径能复现金额搬移，新初始化回填可以保留同一份数据', async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    const patch = { currency: 'GBP', amount: 5000001, localAmount: 600, method: '直签' }
    await fixture.form.setData(patch)
    await new Promise(resolve => setTimeout(resolve, 0))
    assert.equal(fixture.form.models.amount, 0)
    assert.equal(fixture.form.models.localAmount, 5000001)
    const result = await execute(fixture.form, { phase: 'hydrate', patch, keys: Object.keys(patch) })
    await execute(fixture.form, { phase: 'verify', expected: result.expected })
    assert.equal(result.values.amount, 5000001)
    assert.equal(result.values.localAmount, 600)
  } finally { fixture.dispose() }
})

test('允许字段、人员伴生键和当前用户保护保持；没有授权的字段不写入', async () => {
  let values = { user: '{"id":"operator","name":"当前用户"}', name: '', receiver: '', receiver__formPersonId: '' }
  const form = { getValues: () => clone(values), setData: patch => { Object.assign(values, patch) } }
  const result = await execute(form, { phase: 'hydrate', identity: { userId: 'operator', companyId: 'company' },
    keys: ['user', 'name', 'receiver'], patch: { user: '{"id":"old"}', name: '说明', receiver: '接收人', receiver__formPersonId: 'receiver-id', forbidden: '禁止写入' } })
  await execute(form, { phase: 'verify', expected: result.expected })
  assert.equal(values.user, '{"id":"operator","name":"当前用户"}')
  assert.equal(values.receiver__formPersonId, 'receiver-id')
  assert.equal(values.forbidden, undefined)
})

test('原生重建保留虚拟名、只读人员伴生键和未建模正文，初始化即可读取它们', async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    const patch = { currency: 'GBP', currency__virtualName: '英镑', user__formPersonId: 'operator', metadata: { source: '原生正文' } }
    const result = await execute(fixture.form, { phase: 'hydrate', patch, keys: ['currency', 'user__formPersonId', 'metadata'] })
    await execute(fixture.form, { phase: 'verify', expected: result.expected })
    assert.equal(result.values.currency__virtualName, '英镑')
    assert.equal(result.values.user__formPersonId, 'operator')
    assert.deepEqual(clone(result.values.metadata), { source: '原生正文' })
  } finally { fixture.dispose() }
})

test('重建保持宿主业务选项及节点禁用权限，新初始化的条件显隐覆盖旧条件', async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    const fields = fixture.form.formData.list[0].rows[0].columns[0].list
    fields.find(item => item.model === 'amount').options.extendProps = { companyId: 'company', businessId: '' }
    fixture.form.$parent = { disabledData: ['amount', 'method'], enableData: ['method'] }
    fixture.form.dynamicHideFields.permissionHidden = true
    fixture.form.dynamicHideFields.conditionalHidden = true
    const originalEvents = fixture.form.loadEvents
    fixture.form.loadEvents = () => {
      originalEvents()
      const refresh = fixture.form.eventFunction.refresh
      fixture.form.eventFunction.refresh = () => { refresh(); fixture.form.$set(fixture.form.dynamicHideFields, 'conditionalHidden', false) }
    }
    await execute(fixture.form, { phase: 'hydrate', patch: { amount: 25 }, keys: ['amount'] })
    const next = fixture.form.formData.list[0].rows[0].columns[0].list
    assert.equal(next.find(item => item.model === 'amount').options.disabled, true)
    assert.equal(next.find(item => item.model === 'method').options.disabled, false)
    assert.equal(next.find(item => item.model === 'amount').options.extendProps.companyId, 'company')
    assert.equal(fixture.form.dynamicHideFields.permissionHidden, true)
    assert.equal(fixture.form.dynamicHideFields.conditionalHidden, false)
    fixture.form.$parent = undefined
  } finally { fixture.dispose() }
})

test('0、false、空串、空数组及非空默认值都不能在核对中被当成正确值放行', async () => {
  for (const [expected, actual] of [[12, 0], [false, true], ['', '默认'], [[], ['默认']], ['直签', '公开招标'], [null, 0]]) {
    const form = { getValues: () => ({ value: actual }), setData() {} }
    await assert.rejects(() => execute(form, { phase: 'verify', expected: { value: expected } }), /表单值已偏离路径配置.*value/)
  }
})

test('异步联动改错或装载后改错会阻断；核对只读且保留额外派生字段', async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    const result = await execute(fixture.form, { phase: 'hydrate', patch: { amount: 25 }, keys: ['amount'] })
    fixture.form.models.extraDerived = '原生派生'
    await execute(fixture.form, { phase: 'verify', expected: result.expected })
    fixture.form.models.amount = 0
    await assert.rejects(() => execute(fixture.form, { phase: 'verify', expected: result.expected }), /外币金额.*预期 25，实际 0/)
    assert.equal(fixture.form.models.amount, 0)
  } finally { fixture.dispose() }
})

test('清理已经销毁且不再挂载的旧容器，同时保留合法的多个活实例', async () => {
  const fixture = createNativeForm()
  try {
    await fixture.form.refresh()
    fixture.form.instanceObject.dead = { _isDestroyed: true }
    fixture.form.instanceObject.multiple = [{ _isDestroyed: false }, { _isDestroyed: false }]
    await execute(fixture.form, { phase: 'hydrate', patch: { amount: 25 }, keys: ['amount'] })
    assert.equal(fixture.form.instanceObject.dead, undefined)
    assert.equal(fixture.form.instanceObject.multiple.length, 2)
  } finally { fixture.dispose() }
})

test('嵌套子表的点路径按原生逐行语义回填，且不限制未输入的派生列', async () => {
  let values = { rows: [{ amount: 1 }, { amount: 2 }] }
  const form = { getValues: () => clone(values), setData(patch) { values.rows.forEach(row => { row.amount = patch['rows.amount']; row.derived = '计算值' }) } }
  const result = await execute(form, { phase: 'hydrate', patch: { 'rows.amount': 30 }, keys: ['rows.amount'] })
  await execute(form, { phase: 'verify', expected: result.expected })
  assert.deepEqual(values.rows.map(row => row.amount), [30, 30])
})

test('明确输入的零值、false 和清空意图保留；不存在的嵌套路径明确拒绝', async () => {
  let values = { amount: 10, enabled: true, note: '旧值', empty: '旧值', rows: [1] }
  const form = { getValues: () => clone(values), setData: patch => { Object.assign(values, patch) } }
  const patch = { amount: 0, enabled: false, note: '', empty: null, rows: [] }
  const result = await execute(form, { phase: 'hydrate', patch, keys: Object.keys(patch) })
  await execute(form, { phase: 'verify', expected: result.expected })
  assert.deepEqual(clone(result.values), patch)
  await assert.rejects(() => execute(form, { phase: 'hydrate', patch: { 'missing.amount': 1 }, keys: ['missing.amount'] }), /字段路径不存在/)
})

// 执行目标适配器原文，验证 URL 初始化是否会清空注入的身份，而不只检查查询参数字符串。
test('原生 URL 初始化保留完整身份，避免仅 companyId 导致 SID 和当前用户清空', () => {
  const payload = { sid: 'test-session', customerCode: 'tenant', userId: 'operator', accountName: '欧阳改', companyId: 'company', companyName: '公司', departmentName: '部门' }
  const runAdapter = (params: URLSearchParams) => {
    const storage = new Map(Object.entries({ 'invest-power-system-token': 'test-session', 'invest-power-system-userId': 'operator' }))
    const context = vm.createContext({ window: { localStorage: { setItem: (k, v) => storage.set(k, v), removeItem: k => storage.delete(k), getItem: k => storage.get(k) || null } },
      URLSearchParams, store: { commit() {} }, router: {}, setConfig() {}, getLocationParam: key => params.get(key) || '',
      localstorageGet: key => storage.get(`invest-power-system-${key}`) || null,
      localstorageSet: (key, value) => storage.set(`invest-power-system-${key}`, String(value)), localstorageRemove: key => storage.delete(`invest-power-system-${key}`) })
    const auth = fs.readFileSync(new URL('../../../executor-service/flow-source/src/utils/runtimeAuthSync.js', import.meta.url), 'utf8').replace(/^import .*$/gm, '').replace(/export /g, '')
    vm.runInContext(auth, context)
    const adapter = fs.readFileSync(new URL('../../../executor-service/flow-source/src/adapters/postMessage.js', import.meta.url), 'utf8').replace(/^import .*$/gm, '').replace(/export function/g, 'function').replace(/export \{ MESSAGE_TYPES \};/, '')
    // 目标自己的 getLocationParam 使用 location；把实际 URL 交给它。
    context.window.location = { search: '', hash: '#/flow/initiate?' + params.toString() }
    vm.runInContext(adapter + '\ninitFromUrlParams()', context)
    return storage
  }
  assert.equal(runAdapter(new URLSearchParams({ companyId: 'company' })).get('invest-power-system-userId'), undefined)
  const storage = runAdapter(nativeAuthQuery(payload))
  assert.equal(storage.get('invest-power-system-userId'), 'operator')
  assert.equal(storage.get('invest-power-system-token'), 'test-session')
  assert.equal(storage.get('invest-power-system-userName'), '欧阳改')
})

test('实际服务装载后及分发前都调用只读核对；旧空值恢复链已移除', () => {
  const service = fs.readFileSync(new URL('../../../executor-service/src/server.ts', import.meta.url), 'utf8')
  assert.ok(service.includes("session.page.evaluate(synchronizeNativeForm, { phase: 'verify' as const"))
  assert.ok(service.indexOf("begin('form_values_before_dispatch')") < service.indexOf("begin('native_dispatch_evaluate')"))
  assert.ok(!service.includes('restoreIntendedPatchViaPage'))
  assert.equal((service.match(/const params = nativeAuthQuery\(payload\)/g) || []).length, 3)
})

test('生产 URL 构造在发起、详情和转发模式都携带完整身份，taskContext 保持 JSON 类型', () => {
  const source = fs.readFileSync(new URL('../../../executor-service/src/server.ts', import.meta.url), 'utf8')
  const parsed = ts.createSourceFile('server.ts', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const declaration = parsed.statements.find(statement => ts.isFunctionDeclaration(statement) && statement.name?.text === 'nativeURL')
  const js = ts.transpileModule(declaration.getText(parsed), { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText
  const url = vm.runInNewContext(js + '\nnativeURL', { URLSearchParams, nativeAuthQuery })
  for (const mode of ['initiate', 'approve', 'forward']) {
    const href = url('http://127.0.0.1/flow/', { mode, sid: 'test-session', userId: 'operator', accountName: '当前用户', customerCode: 'tenant', companyId: 'company', companyName: '公司', departmentName: '部门', instanceId: 'instance' })
    const params = new URLSearchParams(href.split('?')[1])
    assert.equal(params.get('sid'), 'test-session')
    assert.equal(params.get('userId'), 'operator')
    assert.equal(params.get('userDepartmentName'), '部门')
    if (mode === 'approve') assert.equal(JSON.parse(params.get('taskContext')).id, 'instance')
  }
})
