import assert from 'node:assert/strict'
import test from 'node:test'
import { prepareTemplate } from '../../../form-runtime/src/runtime/formTemplate.js'

// 只构造权限边界，不携带服务器业务快照或复制参考源码。
function template (customClass = '') {
  return { list: [{ type: 'table', model: 'performanceSplitItems', options: { disabled: false, customClass }, tableColumns: [
    { type: 'select', model: 'year', options: { disabled: false, required: true }, rules: [{ required: true }] },
    { type: 'number', model: 'performanceAmountRmb', options: { disabled: false, required: true }, rules: [{ required: true }] },
    { type: 'input', model: 'performanceRatio', options: { disabled: true } },
    { type: 'input', model: 'remark', options: { disabled: false } }
  ] }] }
}

test('父表可编辑时年度金额备注保持可填，占比保持模板只读且不改原模板', () => {
  const raw = template()
  const before = structuredClone(raw)
  const result = prepareTemplate(raw, [{ field: 'performanceSplitItems', power: 'edit' }], false)
  assert.deepEqual(result.template.list[0].tableColumns.map(c => c.options.disabled), [false, false, true, false])
  assert.ok(result.editableFields.includes('performanceSplitItems.year'))
  assert.ok(!result.editableFields.includes('performanceSplitItems.performanceRatio'))
  assert.deepEqual(raw, before)
})

for (const power of ['only_read', 'hide', undefined]) {
  test(`父表权限 ${power} 不允许子列 edit 绕过`, () => {
    const permissions = [{ field: 'performanceSplitItems.year', power: 'edit' }]
    if (power) permissions.push({ field: 'performanceSplitItems', power })
    const result = prepareTemplate(template(), permissions, false)
    assert.ok(result.template.list[0].tableColumns.every(c => c.options.disabled))
    assert.deepEqual(result.editableFields, [])
    if (power === 'hide') assert.ok(result.template.list[0].options.hidden)
  })
}

test('全表只读不能被父表或子列 edit 放宽', () => {
  const result = prepareTemplate(template(), [{ field: 'performanceSplitItems', power: 'edit' }], true)
  assert.ok(result.template.list[0].tableColumns.every(c => c.options.disabled))
})

test('完整列路径权限生效，顶层同名权限不污染表格列', () => {
  const result = prepareTemplate(template(), [
    { field: 'performanceSplitItems', power: 'edit' },
    { field: 'year', power: 'hide' },
    { field: 'performanceSplitItems_$$_remark', power: 'only_read' },
    { field: 'performanceSplitItems.performanceAmountRmb', power: 'hide' }
  ], false)
  const columns = result.template.list[0].tableColumns
  assert.equal(columns[0].options.disabled, false)
  assert.equal(columns[0].options.hidden, false)
  assert.equal(columns[1].options.hidden, true)
  assert.equal(columns[3].options.disabled, true)
})

test('_col 仅控制必填，缺少列必填授权不应禁止填写', () => {
  const result = prepareTemplate(template(), [
    { field: 'performanceSplitItems', power: 'edit' }, { field: 'year_col', power: 'edit' }
  ], false)
  const columns = result.template.list[0].tableColumns
  assert.equal(columns[0].options.required, true)
  assert.equal(columns[1].options.disabled, false)
  assert.equal(columns[1].options.required, false)
  assert.deepEqual(columns[1].rules, [])
})

test('原生免列权限标记保留模板必填，布局容器不引入模型路径', () => {
  const raw = template('subFormNoNeedPermission')
  raw.list = [{ type: 'grid', model: 'layout', columns: [{ list: raw.list }] }]
  const result = prepareTemplate(raw, [{ field: 'performanceSplitItems', power: 'edit' }], false)
  const columns = result.template.list[0].columns[0].list[0].tableColumns
  assert.equal(columns[0].options.required, true)
  assert.equal(columns[1].options.required, true)
  assert.equal(columns[0].options.disabled, false)
})
