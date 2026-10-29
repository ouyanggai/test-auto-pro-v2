import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const runtimeSource = fs.readFileSync(new URL('../../../form-runtime/runtime-source/src/main.js', import.meta.url), 'utf8')
process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(
  [...runtimeSource.matchAll(/name:\s*['"]([^'"]+)['"]\s*,\s*component:/g)].map(match => match[1])
)
const { prepareTemplate, hiddenFieldKeys } = await import('../../../form-runtime/src/runtime/formTemplate.js')

// 使用目标控件结构而非业务字段白名单，确保同后缀选择器和附件均由模板及权限决定显隐。
const template = () => ({ list: [{ type: 'report', model: 'section', options: {}, list: [
  { type: 'custom', el: 'general-list-select-show', model: 'related_flow', options: { hidden: false, required: true }, rules: [{ required: true }] },
  { type: 'fileupload', model: 'attachment_flow', options: { hidden: false } },
  { type: 'fileupload', model: 'attachment', options: {} },
] }], config: { eventScript: [{ key: 'refresh', func: 'this.display(["related_flow"])' }] } })
const permissions = ['related_flow', 'attachment_flow', 'attachment'].map(field => ({ field, power: 'edit' }))

test('可编辑关联选择器与附件不因 _flow 后缀隐藏且不改原模板', () => {
  const original = template()
  const snapshot = structuredClone(original)
  const prepared = prepareTemplate(original, permissions, false)
  for (const field of prepared.template.list[0].list) {
    assert.equal(field.options.hidden, false)
    assert.equal(field.options.disabled, false)
  }
  assert.deepEqual(prepared.unsupported, [])
  assert.deepEqual(prepared.hiddenFields, [])
  assert.deepEqual(prepared.requiredEditableFields, ['related_flow'])
  assert.deepEqual(prepared.template.config.eventScript, original.config.eventScript)
  assert.deepEqual(original, snapshot)
})

for (const readOnly of [false, true]) test(`关联选择器在只读权限下仍显示但不可编辑：readOnly=${readOnly}`, () => {
  const prepared = prepareTemplate(template(), readOnly ? permissions : [], readOnly)
  const selector = prepared.template.list[0].list[0]
  assert.equal(selector.options.hidden, false)
  assert.equal(selector.options.disabled, true)
  assert.equal(selector.options.required, false)
  assert.deepEqual(selector.rules, [])
  assert.deepEqual(prepared.hiddenFields, [])
})

for (const source of ['permission', 'component', 'options', 'display']) test(`目标明确隐藏关联选择器时保持隐藏：${source}`, () => {
  const original = template()
  const selector = original.list[0].list[0]
  if (source === 'component') selector.hidden = true
  if (source === 'options') selector.options.hidden = true
  if (source === 'display') selector.options.display = false
  const powers = permissions.map(item => item.field === selector.model && source === 'permission' ? { ...item, power: 'hide' } : item)
  const prepared = prepareTemplate(original, powers, false)
  assert.equal(prepared.template.list[0].list[0].options.hidden, true)
  assert.deepEqual(prepared.hiddenFields, ['related_flow'])
})

test('关联区域仍服从模板联动隐藏且恢复显示不遗留后缀静态隐藏', () => {
  const prepared = prepareTemplate(template(), permissions, false)
  const form = { dynamicHideFields: { section: true } }
  assert.equal(hiddenFieldKeys(form, prepared.template, prepared.hiddenFields).has('related_flow'), true)
  form.dynamicHideFields.section = false
  assert.equal(hiddenFieldKeys(form, prepared.template, prepared.hiddenFields).has('related_flow'), false)
})
