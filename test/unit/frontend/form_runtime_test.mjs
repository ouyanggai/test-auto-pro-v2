import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const runtimeSource = fs.readFileSync(new URL('../../../form-runtime/runtime-source/src/main.js', import.meta.url), 'utf8')
const registeredRuntimeComponentNames = [...runtimeSource.matchAll(/name:\s*['"]([^'"]+)['"]\s*,\s*component:/g)].map(match => match[1])
process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(registeredRuntimeComponentNames)
const { captureFormValues, componentRuntimeName, formRuntimeStats, prepareTemplate, refreshPreparedForm } = await import('../../../form-runtime/src/runtime/formTemplate.js')
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
    config: {
      beforeSubmitAndDraft: 'writeBusinessData()',
      beforeSubmit: 'submitTarget()',
      eventScript: [{ name: 'refresh', func: 'setOptionData("currentDepartment", departments)' }],
    },
  }, [{ field: 'title', power: 'edit' }, { field: 'hiddenValue', power: 'hide' }], false)
  const fields = prepared.template.list[0].columns[0].list
  assert.equal(fields[0].options.disabled, false)
  assert.equal(fields[0].options.required, true)
  assert.equal(fields[1].options.hidden, true)
  assert.equal(fields[1].options.disabled, true)
  assert.equal(fields[1].options.required, false)
  assert.ok(prepared.unsupported.some(item => item.includes('合同业务组件')))
  assert.equal(prepared.unsupported.some(item => item.includes('业务提交钩子')), false)
  assert.deepEqual(prepared.isolatedHooks, ['beforeSubmitAndDraft', 'beforeSubmit'])
  assert.equal(Object.hasOwn(prepared.template.config, 'beforeSubmitAndDraft'), false)
  assert.equal(Object.hasOwn(prepared.template.config, 'beforeSubmit'), false)
  assert.deepEqual(prepared.template.config.eventScript, [{ name: 'refresh', func: 'setOptionData("currentDepartment", departments)' }])
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
    assert.equal(window.localStorage.getItem('invest-power-system-currentDepartment'), '财务部')
    assert.deepEqual(Object.keys(window.localStorage).sort(), [
      'invest-power-system-currentCompanyName',
      'invest-power-system-currentDepartment',
    ])
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

test('未显式授权字段默认只读且统计只依据真实可编辑值', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'input', model: 'granted', options: {} },
    { type: 'input', model: 'ungranted', options: { required: true } },
  ] }, [{ field: 'granted', power: 'edit' }], false)
  assert.equal(prepared.template.list[0].options.disabled, false)
  assert.equal(prepared.template.list[1].options.disabled, true)
  assert.equal(prepared.template.list[1].options.required, false)
  assert.deepEqual(formRuntimeStats({ granted: '已填写', ungranted: '不计入' }, prepared.editableFields, prepared.requiredEditableFields), { filledEditable: 1, manualPending: 0 })
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

test('路径工作区统计按真实权限和值重算', () => {
  const prepared = prepareTemplate({ list: [
    { type: 'number', model: 'amount', options: { required: true } },
    { type: 'number', model: 'mirrorAmount', options: { required: true } },
    { type: 'input', model: 'title', options: { required: true } },
  ] }, [{ field: 'amount', power: 'only_read' }, { field: 'mirrorAmount', power: 'only_read' }, { field: 'title', power: 'edit' }], false)
  assert.equal(prepared.template.list[0].options.disabled, true)
  assert.equal(prepared.template.list[0].options.required, false)
  assert.equal(prepared.template.list[1].options.disabled, true)
  assert.equal(prepared.template.list[1].options.required, false)
  assert.equal(prepared.template.list[2].options.disabled, false)
  assert.deepEqual(prepared.editableFields, ['title'])
  assert.deepEqual(formRuntimeStats({ amount: 3000, mirrorAmount: 3000, title: '已填写' }, prepared.editableFields, prepared.requiredEditableFields), { filledEditable: 1, manualPending: 0 })
  assert.deepEqual(formRuntimeStats({ amount: 3000, mirrorAmount: 3000, title: '' }, prepared.editableFields, prepared.requiredEditableFields), { filledEditable: 0, manualPending: 1 })
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

test('业务自定义组件获得目标发起态上下文且不携带历史业务标识', () => {
  const prepared = prepareTemplate({ list: [
    {
      type: 'custom', el: 'contract-seal-review-business', model: 'custom_contractSealField',
      options: { extendProps: { pageTemplateId: 'template-a', businessId: 'historical-business', companyId: 'historical-company' } },
    },
  ] }, [{ field: 'custom_contractSealField', power: 'only_read' }], false, { companyId: 'company-a' })
  assert.deepEqual(prepared.unsupported, [])
  assert.deepEqual(prepared.template.list[0].options.extendProps, {
    isFlowInitiate: true,
    businessId: '',
    companyId: 'company-a',
    pageTemplateId: 'template-a',
  })
})

test('所有已注册自定义组件共享运行时身份且不误用合同发起态标识', () => {
  const registeredNames = registeredRuntimeComponentNames
  assert.ok(registeredNames.length > 0)
  const prepared = prepareTemplate({ list: registeredNames.map((name, index) => ({
    type: 'custom', el: name, model: `custom_${index}`, options: {}
  })) }, registeredNames.map((_, index) => ({ field: `custom_${index}`, power: 'edit' })), false, {
    companyId: 'company-a', companyName: '测试公司', departmentId: 'department-a',
    departmentName: '测试部门', userId: 'user-a', accountName: '测试用户', customerCode: 'customer-a'
  })
  assert.deepEqual(prepared.unsupported, [])
  for (const [index, name] of registeredNames.entries()) {
    const props = prepared.template.list[index].options.extendProps
    assert.equal(props.companyId, 'company-a')
    assert.equal(props.userId, 'user-a')
    assert.equal(props.departmentId, 'department-a')
    if (name === 'legal-contract-doctable' || name === 'contract-seal-review-business') {
      assert.equal(props.isFlowInitiate, true)
      assert.equal(props.businessId, '')
    } else {
      assert.equal(Object.hasOwn(props, 'isFlowInitiate'), false)
      assert.equal(Object.hasOwn(props, 'businessId'), false)
    }
  }
})

test('宿主 Vue 页面透传统一快照别名并在 setData 后重新初始化', () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/HostVuePage.vue', import.meta.url), 'utf8')
  for (const prop of ['value: values', 'propData: values', 'params: values', 'paramsInfo: values', 'param: values', 'initialValues: values']) {
    assert.match(source, new RegExp(prop.replace(/[.*+?^${}()|[\\]\\]/g, '\\$&')))
  }
  assert.match(source, /:key="`\$\{page\.componentName\}:\$\{valuesVersion\}`"/)
  assert.match(source, /this\.valuesVersion \+= 1/)
})

test('初始默认模型的空值不计入已填写值', () => {
  const emptyDefaults = { myCompanyName: '', myDepName: '', myUserName: '', time: [], vacateReason: '', vacateType: '', vacateTime: null }
  assert.deepEqual(formRuntimeStats(emptyDefaults, Object.keys(emptyDefaults), ['vacateReason']), { filledEditable: 0, manualPending: 1 })
  const realEdit = { ...emptyDefaults, vacateReason: '个人事务需要处理' }
  assert.deepEqual(formRuntimeStats(realEdit, Object.keys(realEdit), ['vacateReason']), { filledEditable: 1, manualPending: 0 })
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

test('目标请求统一透传并保留网关改写与 SID', async () => {
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
      readRequestManifest: [
        { method: 'POST', path: '/web/api/measuring/contract/type/enableTreeList', source: 'formmaking_template' },
        { method: 'POST', path: '/web/flowProxy/findById', source: 'formmaking_template' },
        { method: 'POST', path: '/web/user/api/company/children', source: 'formmaking_template' },
      ],
      shadowContext: { renderType: 'formmaking', componentName: '' },
      onDecision: observation => observations.push(observation),
    })
    const request = new XMLHttpRequest()
    request.open('POST', '/web/api/measuring/contract/type/enableTreeList')
    request.send('{"data":{"flag":"3"}}')
    assert.match(opened[0][1], /^http:\/\/target\.test\/api\/web\/api\/measuring\/contract\/type\/enableTreeList\?sid=memory-only-sid$/)
    assert.deepEqual(sentHeaders[0], ['sid', 'memory-only-sid'])
    // 目标网关只在请求体携带 SID 时才认可会话；JSON 请求体必须合并 SID。
    assert.deepEqual(JSON.parse(sentBodies[0]), { data: { flag: '3' }, sid: 'memory-only-sid' })
    request.open('POST', '/web/flowInstanceApi/submit')
    request.send('{}')
    request.open('POST', '/api/web/file/api/file/uploadFile')
    request.send('{}')
    request.open('POST', '/web/file/api/relationFile/saveBatch')
    request.send('{}')
    request.open('POST', '/web/custom/api/runAction')
    request.send('{}')
    request.open('POST', '/web/custom/api/getOrCreate')
    request.send('{}')
    request.open('GET', '/web/custom/unlisted')
    request.send()
    await window.fetch('/web/measuring/api/contractInvoicing/uploadFile', { method: 'POST' })
    await window.fetch('/web/file/api/relationFile/deleteByRelationIdAndFileIds', { method: 'POST' })
    await window.fetch('/web/flowProxy/findById', { method: 'POST', body: '{}' })
    assert.equal(fetched.length, 3)
    assert.equal(fetched[0][0], 'http://target.test/api/web/measuring/api/contractInvoicing/uploadFile?sid=memory-only-sid')
    assert.equal(fetched[0][1].headers.get('sid'), 'memory-only-sid')
    assert.equal(fetched[1][0], 'http://target.test/api/web/file/api/relationFile/deleteByRelationIdAndFileIds?sid=memory-only-sid')
    assert.equal(fetched[1][1].headers.get('sid'), 'memory-only-sid')
    assert.equal(fetched[2][0], 'http://target.test/api/web/flowProxy/findById?sid=memory-only-sid')
    assert.equal(fetched[2][1].headers.get('sid'), 'memory-only-sid')
    assert.deepEqual(JSON.parse(fetched[2][1].body), { sid: 'memory-only-sid' })

    await window.fetch('http://192.168.1.220:8081/api/web/api/measuring/contract/type/enableTreeList?platformCode=200001', { method: 'POST', body: '{}' })
    assert.equal(fetched[3][0], 'http://target.test/api/web/api/measuring/contract/type/enableTreeList?platformCode=200001&sid=memory-only-sid')
    assert.equal(fetched[3][1].headers.get('sid'), 'memory-only-sid')
    await window.fetch('http://192.168.1.220:28081/api/web/file/api/relationFile/saveBatchFile', { method: 'POST' })

    await window.fetch('https://cdn.example.test/assets/form.css')
    assert.equal(fetched[5][0], 'https://cdn.example.test/assets/form.css')
    assert.equal(fetched[5][1].headers.has('sid'), false)
    assert.ok(observations.some(item => item.pathname === '/api/web/api/measuring/contract/type/enableTreeList' && item.allowed === true))
    assert.ok(observations.some(item => item.pathname === '/api/web/flowInstanceApi/submit' && item.allowed === true && item.reason === 'passthrough'))
    assert.ok(observations.every(item => item.renderType === 'formmaking' && !Object.hasOwn(item, 'sid') && !Object.hasOwn(item, 'body') && !Object.hasOwn(item, 'url')))
    restore()
    assert.equal(window.fetch, nativeFetch)
    const afterDestroy = new XMLHttpRequest()
    afterDestroy.open('GET', '/web/api/measuring/contract/type/enableTreeList')
    assert.equal(opened.at(-1)[1], '/web/api/measuring/contract/type/enableTreeList')
  }
  finally {
    globalThis.window = originalWindow
    globalThis.XMLHttpRequest = originalXHR
  }
})
