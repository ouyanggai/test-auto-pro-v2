import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(['person-mulSelect', 'custom-upload-excel', 'custome-info-select'])
const { captureFormValues, componentRuntimeName, diffManualPaths, formRuntimeStats, prepareTemplate, refreshPreparedForm } = await import('../../../form-runtime/src/runtime/formTemplate.js')
const { clearRuntimeAuth, installRuntimeStorageFacade, localstorageGet } = await import('../../../form-runtime/src/runtime/memoryAuth.js')
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
    config: { beforeSubmitAndDraft: 'writeBusinessData()', eventScript: [{ name: 'beforeSubmit', func: 'saveTarget()' }] },
  }, [{ field: 'title', power: 'edit' }, { field: 'hiddenValue', power: 'hide' }], false)
  const fields = prepared.template.list[0].columns[0].list
  assert.equal(fields[0].options.disabled, false)
  assert.equal(fields[0].options.required, true)
  assert.equal(fields[1].options.hidden, true)
  assert.equal(fields[1].options.disabled, true)
  assert.equal(fields[1].options.required, false)
  assert.ok(prepared.unsupported.some(item => item.includes('合同业务组件')))
  assert.equal(prepared.unsupported.some(item => item.includes('业务提交钩子')), false)
  assert.deepEqual(prepared.isolatedHooks, ['beforeSubmitAndDraft', 'eventScript'])
  assert.equal(Object.hasOwn(prepared.template.config, 'beforeSubmitAndDraft'), false)
  assert.equal(Object.hasOwn(prepared.template.config, 'eventScript'), false)
  assert.deepEqual(prepared.allFields.sort(), ['contract', 'hiddenValue', 'title'])
  assert.deepEqual(prepared.editableFields, ['title'])
  assert.deepEqual(prepared.hiddenFields, ['hiddenValue'])
})

test('隐藏必填字段不计入人工待办且身份 storage 只在当前运行时内存存在', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'input', model: 'hiddenRequired', options: { required: true, hidden: true } },
    { type: 'input', model: 'visibleRequired', options: { required: true } },
  ] }, [{ field: 'hiddenRequired', power: 'edit' }, { field: 'visibleRequired', power: 'edit' }], false)
  assert.deepEqual(prepared.hiddenFields, ['hiddenRequired'])
  assert.deepEqual(prepared.requiredEditableFields, ['visibleRequired'])

  const originalWindow = globalThis.window
  globalThis.window = {}
  try {
    const restore = installRuntimeStorageFacade({ currentDepartment: '财务部', currentCompanyName: '测试公司' })
    assert.equal(localstorageGet('currentDepartment'), '财务部')
    restore()
    clearRuntimeAuth()
    assert.equal(localstorageGet('currentDepartment'), '')
  } finally {
    globalThis.window = originalWindow
    clearRuntimeAuth()
  }
})

test('运行时身份覆盖宿主常用公司部门用户键', () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/App.vue', import.meta.url), 'utf8')
  assert.match(source, /topCompanyId:/)
  assert.match(source, /user:\s*\{/)
  assert.match(source, /userDepartmentId:/)
  assert.match(source, /currentDepName:/)
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

test('新发起只开放 edit 字段且已发待发保持全表只读', () => {
  const template = { list: [
    { type: 'input', model: 'basic.title', options: { required: true } },
    { type: 'input', model: 'readonlyValue', options: { required: true } },
  ] }
  const editable = prepareTemplate(template, [
    { field: 'basic_$$_title', power: 'edit' },
    { field: 'readonlyValue', power: 'only_read' },
  ], false)
  assert.equal(editable.template.list[0].options.disabled, false)
  assert.equal(editable.template.list[0].options.required, true)
  assert.equal(editable.template.list[1].options.disabled, true)
  assert.equal(editable.template.list[1].options.required, false)
  assert.deepEqual(editable.editableFields, ['basic.title'])

  const readonly = prepareTemplate(template, [{ field: 'basic_$$_title', power: 'edit' }], true)
  assert.equal(readonly.template.list[0].options.disabled, true)
  assert.equal(readonly.template.list[0].options.required, false)
  assert.deepEqual(readonly.editableFields, [])
})

test('路径条件规则在真实组件装载前禁用精确字段，统计按真实权限和值重算', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'number', model: 'amount', options: { required: true } },
    { type: 'number', model: 'mirrorAmount', options: { required: true } },
    { type: 'input', model: 'title', options: { required: true } },
  ] }, [{ field: 'amount', power: 'edit' }, { field: 'mirrorAmount', power: 'edit' }, { field: 'title', power: 'edit' }], false, [
    { field: 'amount', disabled: true, conditionKeys: ['申请金额大于等于 3000'] },
    { field: 'mirrorAmount', disabled: true, conditionKeys: ['申请金额等于对比金额'] },
    { field: 'missing', disabled: true, conditionKeys: ['不得按名称猜测'] },
  ])
  assert.equal(prepared.template.list[0].options.disabled, true)
  assert.equal(prepared.template.list[0].options.required, false)
  assert.equal(prepared.template.list[1].options.disabled, true)
  assert.equal(prepared.template.list[1].options.required, false)
  assert.equal(prepared.template.list[2].options.disabled, false)
  assert.deepEqual(prepared.protectedFields, ['amount', 'mirrorAmount'])
  assert.deepEqual(prepared.editableFields, ['title'])
  assert.deepEqual(formRuntimeStats({ amount: 3000, mirrorAmount: 3000, title: '自动生成' }, ['amount', 'mirrorAmount', 'title'], [], prepared.editableFields, prepared.protectedFields, prepared.requiredEditableFields), { autoFilled: 3, manualPending: 0 })
  assert.deepEqual(formRuntimeStats({ amount: 3000, mirrorAmount: 3000, title: '' }, ['amount', 'mirrorAmount', 'title'], [], prepared.editableFields, prepared.protectedFields, prepared.requiredEditableFields), { autoFilled: 2, manualPending: 1 })
  assert.deepEqual(formRuntimeStats({ amount: 3000, mirrorAmount: 3000, title: '人工填写' }, ['amount', 'mirrorAmount', 'title'], ['title'], prepared.editableFields, prepared.protectedFields, prepared.requiredEditableFields), { autoFilled: 2, manualPending: 0 })
})

test('刷新已预置权限的模板不会统一调用自定义组件 disabledElement', async () => {
  let refreshed = false
  let disabledCalled = false
  await refreshPreparedForm({
    async refresh() { refreshed = true },
    disabled() {
      disabledCalled = true
      throw new TypeError('n.disabledElement is not a function')
    },
  })
  assert.equal(refreshed, true)
  assert.equal(disabledCalled, false)
})

test('刷新会回填已填数据，避免 FormMaking 重新初始化清空 models', async () => {
  const form = {
    model: { title: '人工填写', amount: 2500 },
    getValues() { return this.model },
    async refresh() { this.model = {} },
    async setData(values) { this.model = values },
  }
  await refreshPreparedForm(form)
  assert.deepEqual(form.model, { title: '人工填写', amount: 2500 })
})

test('校验只使用 getData 而完整保存值来自 getValues 人工输入与虚拟字段', async () => {
  let validated = false
  const values = await captureFormValues({
    async getData(strict) { validated = strict },
    getValues() { return { title: '人工填写', title__virtualName: '人工填写', nested: { enabled: true } } },
  }, true)
  assert.equal(validated, true)
  assert.deepEqual(values, { title: '人工填写', title__virtualName: '人工填写', nested: { enabled: true } })
})

test('真实入口已注册的目标组件不再被统一标记为 unsupported', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'component', model: 'members', name: '人员多选', options: { componentName: 'person-mulSelect' } },
  ] }, [{ field: 'members', power: 'edit' }], false)
  assert.deepEqual(prepared.unsupported, [])
  assert.equal(prepared.template.list[0].options.disabled, false)
})

test('初始默认模型的空值不会被误判为人工覆盖，真实非空修改才会', () => {
  const emptyDefaults = { myCompanyName: '', myDepName: '', myUserName: '', time: [], vacateReason: '', vacateType: '', vacateTime: null }
  assert.deepEqual(diffManualPaths({}, emptyDefaults), [])
  const realEdit = { ...emptyDefaults, vacateReason: '个人事务需要处理' }
  assert.deepEqual(diffManualPaths({}, realEdit), ['vacateReason'])
})

test('目标模板 type custom 优先按 el 匹配真实注册组件', () => {
  const supported = prepareTemplate({ list: [
    { type: 'custom', el: 'custome-info-select', model: 'generalInfo', name: '通用信息选择', options: {} },
  ] }, [{ field: 'generalInfo', power: 'edit' }], false)
  assert.equal(componentRuntimeName(supported.template.list[0]), 'custome-info-select')
  assert.deepEqual(supported.unsupported, [])
  assert.equal(supported.template.list[0].options.disabled, false)

  const unknown = prepareTemplate({ list: [
    { type: 'custom', el: 'unknown-host-component', model: 'unknownValue', name: '未知宿主组件', options: {} },
  ] }, [{ field: 'unknownValue', power: 'edit' }], false)
  assert.equal(unknown.unsupported.length, 1)
  assert.match(unknown.unsupported[0], /未知宿主组件/)
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
  const sentBodies = []
  const fetched = []
  const observations = []
  class FakeXHR {
    open(method, url) { opened.push([method, url]) }
    setRequestHeader(name, value) { sentHeaders.push([name, value]) }
    send(body) { sentBodies.push(body) }
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
    const restore = installReadOnlyRequestPolicy({
      sid: 'memory-only-sid',
      baseURL: 'http://target.test/api',
      shadowContext: { renderType: 'formmaking', componentName: '' },
      onDecision: observation => observations.push(observation),
    })
    const request = new XMLHttpRequest()
    request.open('POST', '/web/form/read')
    request.send('{"data":{"flag":"3"}}')
    assert.match(opened[0][1], /^http:\/\/target\.test\/api\/web\/form\/read\?sid=memory-only-sid$/)
    assert.deepEqual(sentHeaders[0], ['sid', 'memory-only-sid'])
    // 目标网关只在请求体携带 SID 时才认可会话；JSON 请求体必须合并 SID。
    assert.deepEqual(JSON.parse(sentBodies[0]), { data: { flag: '3' }, sid: 'memory-only-sid' })
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
    assert.deepEqual(JSON.parse(fetched[0][1].body), { sid: 'memory-only-sid' })

    await window.fetch('http://192.168.1.220:28081/api/web/user/api/company/children?flag=3', { method: 'POST', body: '{}' })
    assert.equal(fetched[1][0], 'http://target.test/api/web/user/api/company/children?flag=3&sid=memory-only-sid')
    assert.equal(fetched[1][1].headers.get('sid'), 'memory-only-sid')
    await assert.rejects(window.fetch('http://192.168.1.220:28081/api/web/file/api/relationFile/saveBatchFile', { method: 'POST' }), /不支持未证明为只读/)

    await window.fetch('https://cdn.example.test/assets/form.css')
    assert.equal(fetched[2][0], 'https://cdn.example.test/assets/form.css')
    assert.equal(fetched[2][1].headers.has('sid'), false)
    assert.ok(observations.some(item => item.pathname === '/api/web/form/read' && item.allowed === true))
    assert.ok(observations.some(item => item.pathname === '/api/web/flowInstanceApi/submit' && item.allowed === false))
    assert.ok(observations.every(item => item.renderType === 'formmaking' && !Object.hasOwn(item, 'sid') && !Object.hasOwn(item, 'body') && !Object.hasOwn(item, 'url')))
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
