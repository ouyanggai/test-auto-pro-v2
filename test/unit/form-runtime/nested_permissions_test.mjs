import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { createRequire } from 'node:module'
import test from 'node:test'
import { hiddenFieldKeys, prepareTemplate } from '../../../form-runtime/src/runtime/formTemplate.js'

const dataTypes = ['table', 'subform', 'group', 'dialog']
const layoutTypes = ['grid', 'report', 'inline', 'card', 'tabs', 'collapse']

// 原生库从运行时依赖中只读提取，不复制参考源码，也不启动浏览器或访问业务写接口。
const require = createRequire(new URL('../../../web/package.json', import.meta.url))
const ts = require('typescript')
const source = fs.readFileSync(new URL('../../../form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js', import.meta.url), 'utf8')
const ast = ts.createSourceFile('native.js', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS)
let nativeProperty
// 仅选择包含完整表单公开方法的对象，排除同名子组件方法。
function visitNative (node) {
  if (ts.isObjectLiteralExpression(node)) {
    const props = Object.fromEntries(node.properties.filter(p => p.name).map(p => [p.name.text, p]))
    if (props.refresh && props.getValues && props._setSubDisabled) nativeProperty = props._setSubDisabled
  }
  ts.forEachChild(node, visitNative)
}
visitNative(ast)
assert.ok(nativeProperty)
const nativeContext = {}
for (const name of source.match(/\b[A-Za-z_$][\w$]*__WEBPACK_IMPORTED_MODULE_\d+__\b/g) || []) {
  if (name.includes('toConsumableArray')) nativeContext[name] = { a: value => [...value] }
}
const nativeSetDisabled = vm.runInNewContext(`({${nativeProperty.getText(ast)}})._setSubDisabled`, nativeContext)

// 六种布局只包裹展示结构，不改变业务字段路径。
function layout (type, children) {
  const result = { type, model: `layout_${type}`, options: {} }
  if (type === 'grid') result.columns = [{ list: children }]
  else if (type === 'report') result.rows = [{ columns: [{ list: children }] }]
  else if (type === 'tabs' || type === 'collapse') result.tabs = [{ list: children }]
  else result.list = children
  return result
}

// 四种容器复用相同字段名，防止修复成为合同主体或某种控件的特例。
function container (type, model = 'business', innerLayout) {
  const children = [
    { type: 'select', model: 'select', options: { disabled: false, required: true }, rules: [{ required: true }] },
    { type: 'input', model: 'input', options: { disabled: false, required: true }, rules: [{ required: true }] },
    { type: 'input', model: 'computed', options: { disabled: true } }
  ]
  return { type, model, options: { disabled: false },
    [type === 'table' ? 'tableColumns' : 'list']: innerLayout ? [layout(innerLayout, children)] : children }
}

// 按模型定位用于断言原始 options，避免测试另写一套权限算法。
function find (node, model) {
  if (!node || typeof node !== 'object') return undefined
  if (node.model === model) return node
  for (const value of Object.values(node)) {
    const found = find(value, model)
    if (found) return found
  }
}

for (const type of dataTypes) {
  for (const layoutType of layoutTypes) {
    test(`${type} 在 ${layoutType} 内父级授权与原生方法一致，布局不成为权限字段`, () => {
      const raw = { list: [layout(layoutType, [container(type)])] }
      const native = structuredClone(raw)
      const context = { $set: (object, key, value) => { object[key] = value }, _setSubDisabled: nativeSetDisabled }
      nativeSetDisabled.call(context, native.list, ['business'], false)
      const result = prepareTemplate(raw, [{ field: 'business', power: 'edit' }], false)
      for (const model of ['select', 'input', 'computed']) {
        assert.equal(find(result.template, model).options.disabled, find(native, model).options.disabled)
      }
      assert.ok(result.editableFields.includes('business.select'))
      assert.ok(!result.allFields.some(field => field.includes('layout_')))
      assert.equal(find(raw, 'input').options.disabled, false)
    })
    if (type !== 'table') test(`${type} 内穿透 ${layoutType} 布局，内部输入与选择可用`, () => {
      const result = prepareTemplate({ list: [container(type, 'business', layoutType)] }, [{ field: 'business', power: 'edit' }], false)
      assert.equal(find(result.template, 'input').options.disabled, false)
      assert.equal(find(result.template, 'select').options.disabled, false)
      assert.equal(find(result.template, 'computed').options.disabled, true)
      assert.equal(find(result.template, 'input').options.required, true)
      assert.ok(result.requiredEditableFields.includes('business.input'))
    })
  }

  for (const power of ['only_read', 'hide', undefined]) test(`${type} 父级 ${power} 不被子字段 edit 绕过`, () => {
    const permissions = [{ field: 'business.input', power: 'edit' }]
    if (power) permissions.push({ field: 'business', power })
    const result = prepareTemplate({ list: [container(type)] }, permissions, false)
    assert.equal(find(result.template, 'input').options.disabled, true)
    assert.deepEqual(result.editableFields, [])
    if (power === 'hide') assert.ok(find(result.template, 'input').options.hidden)
  })

  test(`${type} 全表只读且原模板禁用不能因父级 edit 消失`, () => {
    const result = prepareTemplate({ list: [container(type)] }, [{ field: 'business', power: 'edit' }], true)
    assert.equal(find(result.template, 'input').options.disabled, true)
    assert.equal(find(result.template, 'computed').options.disabled, true)
  })

  test(`${type} 完整路径可控制子字段，裸名与其他容器的同名字段不污染`, () => {
    const raw = { list: [container(type), container(type, 'other')] }
    const permissions = [
      { field: 'business', power: 'edit' }, { field: 'other', power: 'edit' },
      { field: 'input', power: 'hide' }, { field: 'business_$$_input', power: 'only_read' },
      { field: 'business.computed', power: 'edit' }
    ]
    const result = prepareTemplate(raw, permissions, false)
    assert.equal(find(result.template.list[0], 'input').options.disabled, true)
    assert.equal(find(result.template.list[1], 'input').options.disabled, false)
    assert.equal(find(result.template.list[1], 'input').options.hidden, false)
    assert.equal(find(result.template.list[0], 'computed').options.disabled, false)
    assert.equal(find(result.template.list[1], 'computed').options.disabled, true)
  })
}

test('合同主体相同 subform-report 结构保留输入状态、事件与整份原数据', () => {
  const raw = { list: [container('subform', 'mainBodySubform', 'report')], config: { eventScript: [{ key: 'refresh', func: 'this.display(["mainBodySubform"])' }] } }
  const before = structuredClone(raw)
  const result = prepareTemplate(raw, [{ field: 'mainBodySubform', power: 'edit' }], false)
  assert.equal(find(result.template, 'select').options.disabled, false)
  assert.equal(find(result.template, 'input').options.disabled, false)
  assert.deepEqual(result.template.config.eventScript, raw.config.eventScript)
  assert.deepEqual(raw, before)
})

test('多层数据容器在布局之间拼接路径，隐藏路径不会污染另一个同名字段', () => {
  const inner = container('group', 'nested', 'tabs')
  const outer = { type: 'subform', model: 'outer', options: {}, list: [layout('report', [inner])] }
  const result = prepareTemplate({ list: [outer, container('group', 'other')] }, [
    { field: 'outer', power: 'edit' }, { field: 'other', power: 'edit' },
    { field: 'outer.nested.input', power: 'hide' }
  ], false)
  assert.equal(find(result.template.list[0], 'select').options.disabled, false)
  assert.ok(result.allFields.includes('outer.nested.input'))
  const hidden = hiddenFieldKeys({ dynamicHideFields: { 'outer.nested.layout_tabs': true } }, result.template, result.hiddenFields)
  assert.ok(hidden.has('outer.nested.select'))
  assert.ok(!hidden.has('other.select'))
})

for (const outerType of dataTypes) for (const innerType of dataTypes) {
  test(`${outerType} 嵌套 ${innerType} 的完整路径禁用与原生方法一致`, () => {
    const outer = { type: outerType, model: 'outer', options: {},
      [outerType === 'table' ? 'tableColumns' : 'list']: [container(innerType, 'inner')] }
    const raw = { list: [outer] }
    const native = structuredClone(raw)
    const context = { $set: (object, key, value) => { object[key] = value }, _setSubDisabled: nativeSetDisabled }
    nativeSetDisabled.call(context, native.list, ['outer', 'inner', 'input'], true)
    const result = prepareTemplate(raw, [
      { field: 'outer', power: 'edit' }, { field: 'outer.inner.input', power: 'only_read' }
    ], false)
    assert.equal(find(result.template, 'input').options.disabled, find(native, 'input').options.disabled)
    assert.equal(find(result.template, 'select').options.disabled, find(native, 'select').options.disabled)
    assert.ok(result.allFields.includes('outer.inner.select'))
    assert.ok(!result.editableFields.includes('outer.inner.computed'))
  })
}
