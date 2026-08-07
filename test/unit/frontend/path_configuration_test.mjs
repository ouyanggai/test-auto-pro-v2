import assert from 'node:assert/strict'
import test from 'node:test'

import {
  allEditableFieldsFilled,
  buildPathConfigSavePayload,
  encodePathConfigValue,
  hasPathConfigDraftChanges,
  initPathConfigDraft,
  parsePathConfigValue,
} from '../../../web/src/features/path-configuration/logic.ts'

const configuration = {
  path: { sequenceNo: 1, name: '财务路径' },
  revision: 2,
  status: 'configured',
  warnings: [],
  groups: [
    {
      title: '主线',
      kind: 'main',
      nodes: [
        {
          name: '发起',
          typeName: '发起',
          kind: 'start',
          lineBlocked: false,
          fields: [],
          gaps: [],
          actions: [
            { key: 'action-submit', kind: 'submit', label: '发起动作', current: 'submit', default: 'submit', options: [{ value: 'submit', label: '提交' }], disagreeWarning: '' },
          ],
        },
        {
          name: '财务审批',
          typeName: '审批',
          kind: 'common',
          lineBlocked: false,
          gaps: [],
          fields: [
            { key: 'field-amount', name: '申请金额', type: 'number', required: true, value: '2500', options: [], editable: true, affected: false, note: '' },
            { key: 'field-type', name: '类型', type: 'singleSelect', required: false, value: '"a"', options: [{ label: 'A', value: 'a' }, { label: 'B', value: 'b' }], editable: true, affected: false, note: '' },
            { key: 'field-tags', name: '标签', type: 'multiSelect', required: false, value: '["a"]', options: [{ label: 'A', value: 'a' }], editable: true, affected: false, note: '' },
            { key: 'field-subform', name: '明细', type: 'text', required: false, value: '', options: [], editable: false, affected: false, note: '' },
          ],
          actions: [
            { key: 'action-approve', kind: 'agree_disagree', label: '处理结果', current: 'agree', default: 'agree', options: [{ value: 'agree', label: '同意' }, { value: 'disagree', label: '不同意' }], disagreeWarning: '会改变后续线路' },
          ],
        },
      ],
    },
  ],
}

test('初始化草稿只包含可编辑字段与全部动作', () => {
  const draft = initPathConfigDraft(configuration)
  assert.deepEqual(draft.fields, {
    'field-amount': '2500',
    'field-type': '"a"',
    'field-tags': '["a"]',
  })
  assert.equal(Object.prototype.hasOwnProperty.call(draft.fields, 'field-subform'), false)
  assert.deepEqual(draft.actions, { 'action-submit': 'submit', 'action-approve': 'agree' })
})

test('草稿无变化不提示保存，修改后提示', () => {
  const draft = initPathConfigDraft(configuration)
  assert.equal(hasPathConfigDraftChanges(configuration, draft), false)
  draft.fields['field-amount'] = '3000'
  assert.equal(hasPathConfigDraftChanges(configuration, draft), true)
  draft.fields['field-amount'] = '2500'
  draft.actions['action-approve'] = 'disagree'
  assert.equal(hasPathConfigDraftChanges(configuration, draft), true)
})

test('字段值按控件类型往返编码', () => {
  const amount = configuration.groups[0].nodes[1].fields[0]
  assert.equal(parsePathConfigValue(amount, '2500'), 2500)
  assert.equal(encodePathConfigValue(amount, 3000), '3000')
  const tags = configuration.groups[0].nodes[1].fields[2]
  assert.deepEqual(parsePathConfigValue(tags, '["a","b"]'), ['a', 'b'])
  assert.equal(encodePathConfigValue(tags, []), '[]')
  assert.equal(encodePathConfigValue(tags, undefined), '[]')
})

test('保存载荷包含全部可编辑字段与动作且不含不可编辑项', () => {
  const draft = initPathConfigDraft(configuration)
  const payload = buildPathConfigSavePayload(configuration, draft)
  assert.equal(payload.fields.length, 3)
  assert.equal(payload.actions.length, 2)
  assert.deepEqual(payload.fields.map((item) => item.key).sort(), ['field-amount', 'field-tags', 'field-type'])
  assert.equal(payload.fields.some((item) => item.key === 'field-subform'), false)
  assert.deepEqual(payload.actions.map((item) => item.action).sort(), ['agree', 'submit'])
})

test('必填字段缺失时提示具体中文名称', () => {
  const draft = initPathConfigDraft(configuration)
  draft.fields['field-amount'] = '""'
  const result = allEditableFieldsFilled(configuration, draft)
  assert.equal(result.complete, false)
  assert.deepEqual(result.missing, ['申请金额'])
  draft.fields['field-amount'] = '3500'
  assert.equal(allEditableFieldsFilled(configuration, draft).complete, true)
})
