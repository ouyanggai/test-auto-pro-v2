import assert from 'node:assert/strict'
import test from 'node:test'

process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(['person-mulSelect', 'custom-upload-excel'])
const { diffManualPaths, prepareTemplate } = await import('../../../form-runtime/src/runtime/formTemplate.js')
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

test('真实入口已注册的目标组件不再被统一标记为 unsupported', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'component', model: 'members', name: '人员多选', options: { componentName: 'person-mulSelect' } },
  ] }, [{ field: 'members', power: 'edit' }], false)
  assert.deepEqual(prepared.unsupported, [])
  assert.equal(prepared.template.list[0].options.disabled, false)
})

test('版本化消息拒绝旧版本、空会话和未知命令', () => {
  const valid = { version: FORM_RUNTIME_VERSION, sessionId: 'session-a', requestId: 'request-a', type: 'getValues', payload: {} }
  assert.equal(isRuntimeCommand(valid), true)
  assert.equal(isRuntimeCommand({ ...valid, version: 'old' }), false)
  assert.equal(isRuntimeCommand({ ...valid, sessionId: '' }), false)
  assert.equal(isRuntimeCommand({ ...valid, type: 'submit' }), false)
})

test('目标写请求由 XHR 和 fetch 统一阻断，已证明只读 POST 仍带 SID', async () => {
  const opened = []
  const sentHeaders = []
  const fetched = []
  class FakeXHR {
    open(method, url) { opened.push([method, url]) }
    setRequestHeader(name, value) { sentHeaders.push([name, value]) }
    send() {}
  }
  const originalWindow = globalThis.window
  const originalXHR = globalThis.XMLHttpRequest
  globalThis.XMLHttpRequest = FakeXHR
  const nativeFetch = async (...args) => {
    fetched.push(args)
    return new Response('{}')
  }
  globalThis.window = {
    location: { href: 'http://127.0.0.1:19001/' },
    fetch: nativeFetch,
  }
  try {
    const restore = installReadOnlyRequestPolicy({ sid: 'memory-only-sid', baseURL: 'http://target.test/api' })
    const request = new XMLHttpRequest()
    request.open('POST', '/web/form/read')
    request.send('{}')
    assert.match(opened[0][1], /^http:\/\/target\.test\/api\/web\/form\/read\?sid=memory-only-sid$/)
    assert.deepEqual(sentHeaders, [['sid', 'memory-only-sid']])
    assert.throws(() => request.open('POST', '/web/flowInstanceApi/submit'), /不支持未证明为只读/)
    assert.throws(() => request.open('POST', '/api/web/file/api/file/uploadFile'), /不支持未证明为只读/)
    assert.throws(() => request.open('POST', '/web/file/api/relationFile/saveBatch'), /不支持未证明为只读/)
    assert.throws(() => request.open('POST', '/web/custom/api/runAction'), /不支持未证明为只读/)
    assert.throws(() => request.open('POST', '/web/custom/api/getOrCreate'), /不支持未证明为只读/)
    await assert.rejects(window.fetch('/web/measuring/api/contractInvoicing/uploadFile', { method: 'POST' }), /不支持未证明为只读/)
    await assert.rejects(window.fetch('/web/file/api/relationFile/deleteByRelationIdAndFileIds', { method: 'POST' }), /不支持未证明为只读/)
    await window.fetch('/web/flowProxy/findById', { method: 'POST', body: '{}' })
    assert.equal(fetched.length, 1)
    assert.equal(fetched[0][0], 'http://target.test/api/web/flowProxy/findById?sid=memory-only-sid')
    assert.equal(fetched[0][1].headers.get('sid'), 'memory-only-sid')
    restore()
    assert.equal(window.fetch, nativeFetch)
    const afterDestroy = new XMLHttpRequest()
    afterDestroy.open('GET', '/web/form/read')
    assert.equal(opened.at(-1)[1], '/web/form/read')
  }
  finally {
    globalThis.window = originalWindow
    globalThis.XMLHttpRequest = originalXHR
  }
})
