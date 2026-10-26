import assert from 'node:assert/strict'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import test from 'node:test'
import vm from 'node:vm'
import { refreshPreparedForm, seedFormRefreshValues } from '../../../form-runtime/src/runtime/formTemplate.js'
import { formComponentLifecycle } from '../../../form-runtime/src/runtime/formComponentLifecycle.js'

const runtimeRequire = createRequire(new URL('../../../form-runtime/package.json', import.meta.url))
const webRequire = createRequire(new URL('../../../web/package.json', import.meta.url))
const Vue = runtimeRequire('vue')
const ts = webRequire('typescript')
const clone = value => JSON.parse(JSON.stringify(value))
const source = fs.readFileSync(new URL('../../../form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js', import.meta.url), 'utf8')
const ast = ts.createSourceFile('native.js', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS)
const properties = node => Object.fromEntries(node.properties.filter(item => item.name).map(item => [item.name.text, item]))
let nativeMethods
const containers = new Map()
const visit = node => {
  if (ts.isObjectLiteralExpression(node)) {
    const props = properties(node)
    if (!nativeMethods && props.refresh && props.getValues && props.getComponent) nativeMethods = props
    if (props.beforeUnmount && props.inject?.getText(ast).includes('deleteComponentInstance')) containers.set(props.name.initializer.text, props)
  }
  ts.forEachChild(node, visit)
}
visit(ast)

// 直接执行同步库的实际方法；只提供压缩包闭包依赖，不手写一份刷新或注册实现。
const nativeContext = { Promise, _objectSpread: (...values) => Object.assign({}, ...values) }
nativeContext.lodash__WEBPACK_IMPORTED_MODULE_34___default = { a: { cloneDeep: clone } }
for (const name of source.match(/\b[A-Za-z_$][\w$]*__WEBPACK_IMPORTED_MODULE_\d+__\b/g) || []) {
  if (name.includes('toConsumableArray')) nativeContext[name] = { a: value => [...value] }
}
vm.createContext(nativeContext)
const native = (props, name) => vm.runInContext(`({${props[name].getText(ast)}}).${name}`, nativeContext)
const newRegistry = () => {
  const registry = { instanceObject: {} }
  for (const name of ['getComponent', 'generateComponentInstance', 'deleteComponentInstance']) registry[name] = native(nativeMethods, name)
  return registry
}

// 所有动态布局共用同一机制；合成模板同时验证金额标题/显隐和非金额的付款区布局，不绑定某张业务表。
const makeForm = initial => {
  const registry = newRegistry()
  const template = {
    list: [{ type: 'report', model: 'amounts', options: {}, rows: [{ columns: [{ list: [
      { type: 'select', model: 'currency', options: { defaultValue: 'CNY' } },
      { type: 'number', model: 'amount', options: { defaultValue: 0 } },
      { type: 'number', model: 'localAmount', options: { defaultValue: 0 } },
      { type: 'select', model: 'payment', options: { defaultValue: 'out' } }
    ] }] }], title: '人民币', localAmountHidden: true, paymentSection: '付款' }], config: {}
  }
  const report = containers.get('generate-report')
  const Report = Vue.extend({
    name: 'generate-report', inject: ['generateComponentInstance', 'deleteComponentInstance'],
    mixins: [formComponentLifecycle], beforeUnmount: native(report, 'beforeUnmount')
  })
  let currentReport
  const initializationReads = []
  const form = Object.assign(registry, {
    data: template, formValue: {}, models: clone(initial), dynamicValueData: [], dynamicHideFields: {},
    $set: (object, key, value) => { object[key] = value },
    // 模拟 Vue 更新边界：原生 _initForm 先隐藏旧表单，再于下一轮挂载新表单。销毁使用真实 Vue 2。
    $nextTick(callback) {
      return Promise.resolve().then(() => {
        if (!this.formShow && currentReport) { currentReport.$destroy(); currentReport = null }
        const result = callback?.()
        if (this.formShow && !currentReport) {
          currentReport = new Report({ parent: provider })
          currentReport.element = this.formData.list[0]
          native(report, 'mounted').call(currentReport)
        }
        return result
      })
    },
    getValues() { return clone(this.models) },
    _generateRules() {}, _generateDataSource() {}, loadDataSource() {}, loadDynamicValueFx() {},
    loadEvents() {
      this.eventFunction.refresh = () => {
        initializationReads.push(clone(this.models))
        const element = this.getComponent('amounts').element
        element.title = this.models.currency
        element.localAmountHidden = this.models.currency === 'CNY'
        element.paymentSection = this.models.payment === 'in' ? '收款' : '付款'
      }
    },
    refresh() { return this._initForm() }
  })
  const provider = new Vue({ provide: {
    generateComponentInstance: form.generateComponentInstance.bind(form),
    deleteComponentInstance: form.deleteComponentInstance.bind(form)
  } })
  for (const name of ['_initForm', 'generateModel', 'setData']) form[name] = native(nativeMethods, name)
  return { form, initializationReads, dispose() { currentReport?.$destroy(); provider.$destroy() } }
}

for (const currency of ['CNY', 'GBP', 'USD']) {
  test(`原生初始化与多次刷新读取当前 ${currency} 数据，所有布局及路径金额保持`, async () => {
    const values = { currency, amount: 5000001, localAmount: 600, payment: 'in' }
    const { form, initializationReads, dispose } = makeForm(values)
    try {
      for (let count = 0; count < 3; count++) {
        await refreshPreparedForm(form)
        assert.deepEqual(form.getValues(), values)
        assert.deepEqual(initializationReads.at(-1), values)
        const component = form.getComponent('amounts')
        assert.equal(Array.isArray(component), false)
        assert.equal(component._isDestroyed, false)
        assert.equal(component.element.title, currency)
        assert.equal(component.element.localAmountHidden, currency === 'CNY')
        assert.equal(component.element.paymentSection, '收款')
      }
    } finally { dispose() }
  })
}

test('整份数据替换与恢复时先更新原生快照，宿主直接刷新也按新数据联动', async () => {
  const { form, dispose } = makeForm({ currency: 'CNY', amount: 10, localAmount: 0, payment: 'out' })
  try {
    await refreshPreparedForm(form)
    for (const values of [
      { currency: 'GBP', amount: 5000001, localAmount: 600, payment: 'in' },
      { currency: 'CNY', amount: 10, localAmount: 0, payment: 'out' }
    ]) {
      seedFormRefreshValues(form, values)
      assert.notEqual(form.formValue, values)
      await form.refresh()
      assert.deepEqual(form.getValues(), values)
      assert.equal(form.getComponent('amounts').element.title, values.currency)
      assert.equal(form.getComponent('amounts').element.paymentSection, values.payment === 'in' ? '收款' : '付款')
    }
  } finally { dispose() }
})

test('初始化不冒充用户 onChange；原生金额搬移规则只在实际用户切换时执行', async () => {
  const values = { currency: 'GBP', amount: 5000001, localAmount: 600, payment: 'out' }
  const { form, dispose } = makeForm(values)
  let userChanges = 0
  form.onChange = () => { userChanges++; form.models.localAmount = form.models.amount; form.models.amount = null }
  try {
    await refreshPreparedForm(form)
    assert.equal(userChanges, 0)
    assert.deepEqual(form.getValues(), values)
    form.onChange()
    assert.equal(userChanges, 1)
    assert.equal(form.models.amount, null)
    assert.equal(form.models.localAmount, 5000001)
  } finally { dispose() }
})

test('全部同类原生容器在真实 Vue 2 销毁后注销，再注册只得到当前实例', () => {
  assert.deepEqual([...containers.keys()].sort(), ['generate-card', 'generate-col-item', 'generate-collapse', 'generate-inline', 'generate-report', 'generate-tab-item'])
  for (const [name, props] of containers) {
    const registry = newRegistry()
    const provider = new Vue({ provide: {
      generateComponentInstance: registry.generateComponentInstance.bind(registry),
      deleteComponentInstance: registry.deleteComponentInstance.bind(registry)
    } })
    const Container = Vue.extend({
      name, inject: ['generateComponentInstance', 'deleteComponentInstance'], mixins: [formComponentLifecycle],
      beforeUnmount: native(props, 'beforeUnmount')
    })
    for (let count = 0; count < 3; count++) {
      const component = new Container({ parent: provider })
      component.element = { model: 'block' }; component.fieldNode = 'rows.0'
      native(props, 'mounted').call(component)
      assert.equal(registry.getComponent('rows.0.block'), component, name)
      component.$destroy()
      assert.equal(registry.instanceObject['rows.0.block'], undefined, name)
    }
    provider.$destroy()
  }
})

test('清理桥接不执行无关组件的 beforeUnmount', () => {
  let cleanups = 0
  const Component = Vue.extend({ mixins: [formComponentLifecycle], beforeUnmount() { cleanups++ } })
  new Component().$destroy()
  assert.equal(cleanups, 0)
})

test('实际 App 的整份 setData 在宿主观察器刷新前同步初始化数据', async () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/App.vue', import.meta.url), 'utf8')
  const script = source.match(/<script>([\s\S]*?)<\/script>/)[1].replace(/^import .*$/gm, '').replace('export default', 'module.exports =')
  const context = vm.createContext({ module: { exports: {} }, HostVuePage: {}, seedFormRefreshValues, clonePlain: clone })
  vm.runInContext(script, context)
  const { form, dispose } = makeForm({ currency: 'CNY', amount: 1, localAmount: 0, payment: 'out' })
  try {
    await refreshPreparedForm(form)
    const values = { currency: 'GBP', amount: 5000001, localAmount: 600, payment: 'in' }
    let seen
    let refreshed
    const host = { set editData(value) { seen = clone(form.formValue); refreshed = form.refresh() } }
    const app = { renderType: 'formmaking', $refs: { formHost: host }, form: () => form,
      assertActiveSession() {}, bindFormChange() {}, $nextTick: () => Promise.resolve() }
    await context.module.exports.methods.setData.call(app, values, {})
    await refreshed
    assert.deepEqual(seen, values)
    assert.deepEqual(form.getValues(), values)
    assert.equal(form.getComponent('amounts').element.title, 'GBP')
    const load = source.slice(source.indexOf("if (this.renderType === 'formmaking')"), source.indexOf('await this.refresh(loadContext)'))
    assert.ok(load.indexOf('seedFormRefreshValues(form, this.values)') < load.indexOf('host.editData ='))
    assert.match(fs.readFileSync(new URL('../../../form-runtime/src/main.js', import.meta.url), 'utf8'), /Vue\.mixin\(formComponentLifecycle\)/)
  } finally { dispose() }
})
