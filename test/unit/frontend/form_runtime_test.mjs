import assert from 'node:assert/strict'
import test from 'node:test'

import { diffManualPaths, prepareTemplate } from '../../../form-runtime/src/runtime/formTemplate.js'
import { FORM_RUNTIME_VERSION, isRuntimeCommand } from '../../../form-runtime/src/runtime/protocol.js'
import { installReadOnlyRequestPolicy } from '../../../form-runtime/src/runtime/requestPolicy.js'

test('目标表单模板递归应用权限且复杂组件不降级', () => {
  const prepared = prepareTemplate({
    list: [
      { type: 'grid', columns: [{ list: [
        { type: 'input', model: 'title', name: '标题', options: { required: true } },
        { type: 'input', model: 'hiddenValue', name: '隐藏值', options: { required: true } },
      ] }] },
      { type: 'component', model: 'contract', name: '合同业务组件', options: { componentName: 'contract-seal-review' } },
    ],
    config: { beforeSubmitAndDraft: 'writeBusinessData()' },
  }, [{ field: 'title', power: 'edit' }, { field: 'hiddenValue', power: 'hide' }], false)
  const fields = prepared.template.list[0].columns[0].list
  assert.equal(fields[0].options.disabled, false)
  assert.equal(fields[0].options.required, true)
  assert.equal(fields[1].options.hidden, true)
  assert.equal(fields[1].options.disabled, true)
  assert.equal(fields[1].options.required, false)
  assert.ok(prepared.unsupported.some(item => item.includes('合同业务组件')))
  assert.ok(prepared.unsupported.some(item => item.includes('业务提交钩子')))
})

test('未显式授权字段默认只读且人工覆盖路径递归稳定', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'input', model: 'granted', options: {} },
    { type: 'input', model: 'ungranted', options: { required: true } },
  ] }, [{ field: 'granted', power: 'edit' }], false)
  assert.equal(prepared.template.list[0].options.disabled, false)
  assert.equal(prepared.template.list[1].options.disabled, true)
  assert.equal(prepared.template.list[1].options.required, false)
  assert.deepEqual(diffManualPaths(
    { title: '生成标题', nested: { amount: 10 }, rows: [{ id: 1 }] },
    { title: '人工标题', nested: { amount: 10 }, rows: [{ id: 2 }] },
  ), ['rows', 'title'])
})

test('版本化消息拒绝旧版本、空会话和未知命令', () => {
  const valid = { version: FORM_RUNTIME_VERSION, sessionId: 'session-a', requestId: 'request-a', type: 'getValues', payload: {} }
  assert.equal(isRuntimeCommand(valid), true)
  assert.equal(isRuntimeCommand({ ...valid, version: 'old' }), false)
  assert.equal(isRuntimeCommand({ ...valid, sessionId: '' }), false)
  assert.equal(isRuntimeCommand({ ...valid, type: 'submit' }), false)
})

test('SID 仅附加到核实目标请求且销毁策略后清除', () => {
  const opened = []
  const sentHeaders = []
  class FakeXHR {
    open(method, url) { opened.push([method, url]) }
    setRequestHeader(name, value) { sentHeaders.push([name, value]) }
    send() {}
  }
  const originalWindow = globalThis.window
  const originalXHR = globalThis.XMLHttpRequest
  globalThis.XMLHttpRequest = FakeXHR
  globalThis.window = {
    location: { href: 'http://127.0.0.1:19001/' },
    fetch: async () => new Response('{}'),
  }
  try {
    const restore = installReadOnlyRequestPolicy({ sid: 'memory-only-sid', baseURL: 'http://target.test/api' })
    const request = new XMLHttpRequest()
    request.open('POST', '/web/form/read')
    request.send('{}')
    assert.match(opened[0][1], /^http:\/\/target\.test\/api\/web\/form\/read\?sid=memory-only-sid$/)
    assert.deepEqual(sentHeaders, [['sid', 'memory-only-sid']])
    assert.throws(() => request.open('POST', '/web/flowInstanceApi/submit'), /禁止调用目标流程或业务写接口/)
    restore()
    const afterDestroy = new XMLHttpRequest()
    afterDestroy.open('GET', '/web/form/read')
    assert.equal(opened.at(-1)[1], '/web/form/read')
  }
  finally {
    globalThis.window = originalWindow
    globalThis.XMLHttpRequest = originalXHR
  }
})
