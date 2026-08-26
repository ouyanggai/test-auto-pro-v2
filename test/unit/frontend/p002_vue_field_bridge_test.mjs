import assert from 'node:assert/strict'
import test from 'node:test'

import { buildValuesEnvelope } from '../../../form-runtime/src/runtime/formTemplate.js'
import { captureVueFieldValues, writeVueFieldValues } from '../../../form-runtime/src/runtime/vueFieldBridge.js'

const field = (path, valueShape = 'scalar') => ({ path, name: path, valueShape })

test('15 个 Vue 声明字段逐一 setData 到 capture 完整往返', () => {
  const fields = [
    field('form.f01'), field('form.f02'), field('form.f03'), field('form.f04'), field('form.f05'),
    field('form.f06', 'number'), field('form.f07', 'number'), field('form.f08', 'array'), field('form.f09', 'array'), field('form.f10', 'object'),
    field('form.f11'), field('form.f12'), field('form.f13'), field('form.f14'), field('form.f15')
  ]
  const initial = {
    f01: '', f02: '', f03: '', f04: '', f05: '', f06: 0, f07: 0,
    f08: [], f09: [], f10: {}, f11: '', f12: '', f13: '', f14: '', f15: ''
  }
  const root = { $data: { form: initial }, $children: [], $refs: {} }
  const values = { form: {
    f01: '一', f02: '二', f03: '三', f04: '四', f05: '五', f06: 6, f07: 7,
    f08: ['八'], f09: [{ id: 9 }], f10: { id: 10 }, f11: '十一', f12: '十二', f13: '十三', f14: '十四', f15: '十五'
  } }
  const written = writeVueFieldValues(root, fields, values)
  const captured = captureVueFieldValues(root, fields, {})
  assert.deepEqual(written.issues, [])
  assert.equal(written.writtenFieldPaths.length, 15)
  assert.deepEqual(captured.issues, [])
  assert.deepEqual(captured.values, values)
})

test('Vue 声明字段缺失不会静默成功', () => {
  const root = { $data: { form: { present: '值' } }, $children: [], $refs: {} }
  const fields = [field('form.present'), field('form.missing')]
  const written = writeVueFieldValues(root, fields, { form: { present: '新值', missing: '不能写入' } })
  const captured = captureVueFieldValues(root, fields, {})
  assert.equal(written.issues.some(issue => issue.code === 'vue_field_not_found' && issue.fieldPath === 'form.missing' && issue.operator === 'write'), true)
  assert.equal(captured.issues.some(issue => issue.code === 'vue_field_not_found' && issue.fieldPath === 'form.missing' && issue.operator === 'read'), true)
  assert.equal(captured.values.form.missing, undefined)
})

test('Vue 字段路径歧义与深度超限分别 blocked 和 partial', () => {
  const child = { $data: { code: '子级' }, $children: [], $refs: {} }
  const ambiguousRoot = { $data: { code: '根级' }, $children: [child], $refs: {} }
  const ambiguous = captureVueFieldValues(ambiguousRoot, [field('code')], {})
  assert.equal(ambiguous.issues.some(issue => issue.code === 'vue_field_ambiguous' && issue.status === 'blocked' && issue.actual === 2), true)

  let deep = { $data: { target: '深层值' }, $children: [], $refs: {} }
  for (let index = 0; index < 9; index++) deep = { $data: {}, $children: [deep], $refs: {} }
  const truncated = captureVueFieldValues(deep, [field('target')], {})
  assert.equal(truncated.issues.some(issue => issue.code === 'vue_field_depth_exceeded' && issue.status === 'partial'), true)
})

test('两种渲染器共享稳定 values envelope', () => {
  const input = {
    values: { nested: [{ id: 1 }], custom: '{"id":"p1"}' }, validated: true,
    unsupported: [], dirty: true, generatedFieldPaths: ['custom', 'nested'], manualOverridePaths: ['custom'],
    issues: [{ code: 'sample', status: 'partial' }], stats: { autoFilled: 1, manualPending: 0 }
  }
  const formMaking = buildValuesEnvelope(input)
  const vueCustom = buildValuesEnvelope(input)
  assert.deepEqual(formMaking, vueCustom)
  assert.notEqual(formMaking.values, input.values)
  assert.deepEqual(Object.keys(formMaking).sort(), ['dirty', 'generatedFieldPaths', 'issues', 'manualOverridePaths', 'stats', 'unsupported', 'validated', 'values'])
})
