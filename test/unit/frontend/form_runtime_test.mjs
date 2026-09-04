import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const runtimeSource = fs.readFileSync(new URL('../../../form-runtime/runtime-source/src/main.js', import.meta.url), 'utf8')
const registeredRuntimeComponentNames = [...runtimeSource.matchAll(/name:\s*['"]([^'"]+)['"]\s*,\s*component:/g)].map(match => match[1])
process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(registeredRuntimeComponentNames)
const { captureFormValues, componentRuntimeName, coordinateOptionPatches, formRuntimeStats, hiddenFieldKeys, optionCoordinationIssues, prepareTemplate, refreshPreparedForm, replayFieldChangeEvents } = await import('../../../form-runtime/src/runtime/formTemplate.js')
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

test('远程下拉按真实选项同步名称对应的绑定值、虚拟显示与名称字段', async () => {
  const template = { list: [{ type: 'select', model: 'paymentId', name: '付款单位', options: { remote: true } }] }
  const form = {
    model: { paymentId: 'old-id', paymentId__virtualName: '旧付款单位', paymentName: '新付款单位' },
    // 真实 FormMaking 的 getComponent 返回 el-select；字段定义和选项属于 formItemContexts 包装组件。
    getComponent() { return { value: 'old-id' } },
    formItemContexts: {
      paymentId: {
        widget: { type: 'select', model: 'paymentId', options: { remote: true } },
        $refs: { generateElementItem: { remoteOptions: [{ value: 'new-id', label: '新付款单位' }] } },
      },
    },
    async setData(values) { Object.assign(this.model, values) },
    getValues() { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['paymentName'], 1)
  assert.equal(coordination.issues.length, 0)
  assert.equal(coordination.values.paymentId, 'new-id')
  assert.equal(coordination.values.paymentId__virtualName, '新付款单位')
  assert.equal(coordination.values.paymentName, '新付款单位')
})

test('远程下拉选项为空时先刷新数据源再同步分支值', async () => {
  const template = { list: [{ type: 'select', model: 'paymentId', options: { remote: true } }] }
  const select = { remoteOptions: [] }
  const form = {
    model: { paymentId: 'old-id', paymentId__virtualName: '旧付款单位', paymentName: '新付款单位' },
    formItemContexts: {
      paymentId: {
        widget: { type: 'select', model: 'paymentId', options: { remote: true } },
        $refs: { generateElementItem: select },
      },
    },
    async refreshFieldOptionData () {
      select.remoteOptions = [{ value: 'new-id', label: '新付款单位' }]
    },
    async setData (values) { Object.assign(this.model, values) },
    getValues () { return this.model },
  }

  const coordination = await coordinateOptionPatches(form, template, form.model, ['paymentName'], 2)
  assert.equal(coordination.values.paymentId, 'new-id')
})

test('选项读取优先 FormMaking 公共选项 API 而非包装层引用', async () => {
  let refreshed = 0
  const template = { list: [{ type: 'select', model: 'applicationFundsVo_payCompanyId', options: { remote: true } }] }
  const form = {
    model: {
      applicationFundsVo_payCompanyId: 'old-company-id',
      applicationFundsVo_payCompanyId__virtualName: '旧公司',
      applicationFundsVo_payCompanyName: '新公司',
    },
    formItemContexts: {
      applicationFundsVo_payCompanyId: {
        widget: { type: 'select', model: 'applicationFundsVo_payCompanyId', options: { remote: true } },
      },
    },
    async refreshFieldOptionData () { refreshed++ },
    getOptionData (field) {
      assert.equal(field, 'applicationFundsVo_payCompanyId')
      return [{ value: 'new-company-id', label: '新公司' }]
    },
    async setData (values) { Object.assign(this.model, values) },
    getValues () { return this.model },
  }

  const coordination = await coordinateOptionPatches(form, template, form.model, ['applicationFundsVo_payCompanyName'], 1)
  assert.equal(refreshed, 1)
  assert.equal(coordination.values.applicationFundsVo_payCompanyId, 'new-company-id')
  assert.equal(coordination.values.applicationFundsVo_payCompanyId__virtualName, '新公司')
})

test('服务端已按主数据回填且值不在选项里时靠虚拟名称兜底，不产生问题也不改写', async () => {
  const template = { list: [{ type: 'select', model: 'payCompanyId', options: { remote: true } }] }
  let optionReads = 0
  const form = {
    model: {
      payCompanyId: 'real-new-id',
      payCompanyId__virtualName: '新公司',
      payCompanyName: '新公司',
    },
    formItemContexts: { payCompanyId: { widget: { type: 'select', model: 'payCompanyId', options: { remote: true } } } },
    async refreshFieldOptionData () {},
    getOptionData () { optionReads++; return [{ value: 'other-id', label: '其他公司' }] },
    async setData () { throw new Error('一致状态不允许写值') },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['payCompanyName'], 1)
  assert.equal(optionReads, 1)
  assert.equal(coordination.issues.length, 0)
  assert.equal(coordination.values.payCompanyId, 'real-new-id')
})

test('多选按名称顺序同步完整取值数组', async () => {
  const template = { list: [{ type: 'select', model: 'budgetIds', name: '预算项', options: { multiple: true, showLabel: true, options: [
    { value: 'b1', label: '甲项' }, { value: 'b2', label: '乙项' }, { value: 'b3', label: '丙项' },
  ] } }] }
  const form = {
    model: { budgetIds: ['b3'], budgetIds__virtualName: '丙项', budgetNames: ['甲项', '乙项'] },
    async setData (values) { Object.assign(this.model, values) },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['budgetNames'], 1)
  assert.deepEqual(coordination.values.budgetIds, ['b1', 'b2'])
  assert.deepEqual(coordination.values.budgetIds__virtualName, ['甲项', '乙项'])
  assert.deepEqual(coordination.values.budgetNames, ['甲项', '乙项'])
})

test('级联按叶子名称唯一匹配并同步完整取值路径', async () => {
  const template = { list: [{ type: 'cascader', model: 'regionPath', options: { options: [
    { value: 'gd', label: '广东', children: [
      { value: 'gza', label: '广州' },
      { value: 'sz', label: '深圳' },
    ] },
    { value: 'sx', label: '山西', children: [{ value: 'ymsn', label: '深圳' }] },
  ] } }] }
  const form = {
    model: { regionPath: ['gd', 'gza'], regionPath__virtualName: '广州' },
    async setData (values) { Object.assign(this.model, values) },
    getValues () { return this.model },
  }
  // 补丁把虚拟名称改为“深圳”：树里“深圳”叶子出现两次，同名歧义必须阻断且绝不能猜测路径。
  const patched = { regionPath: ['gd', 'gza'], regionPath__virtualName: '深圳' }
  const ambiguous = await coordinateOptionPatches(form, template, patched, ['regionPath__virtualName'], 1)
  assert.equal(ambiguous.issues.length, 1)
  assert.equal(ambiguous.issues[0].code, 'OPTION_PATCH_UNRESOLVED')
  assert.equal(ambiguous.issues[0].status, 'blocked')
  assert.deepEqual(ambiguous.values.regionPath, ['gd', 'gza'], '歧义时保留原路径等待人工处理')

  // 同一名称在树里唯一时，级联按完整取值路径回填。
  const template2 = { list: [{ type: 'cascader', model: 'regionPath', options: { options: [
    { value: 'gd', label: '广东', children: [
      { value: 'gza', label: '广州' },
      { value: 'sz', label: '深圳' },
    ] },
  ] } }] }
  const resolved = await coordinateOptionPatches(form, template2, { regionPath: ['gd', 'gza'], regionPath__virtualName: '深圳' }, ['regionPath__virtualName'], 1)
  assert.equal(resolved.issues.length, 0)
  assert.deepEqual(resolved.values.regionPath, ['gd', 'sz'])
  assert.equal(resolved.values.regionPath__virtualName, '深圳')
})

test('选项里找不到补丁名称且控件仍显示历史值时明确阻断', async () => {
  const template = { list: [{ type: 'select', model: 'payCompanyId', options: { remote: true } }] }
  const form = {
    model: {
      payCompanyId: 'old-id',
      payCompanyId__virtualName: '旧公司',
      payCompanyName: '不在选项里的公司',
    },
    formItemContexts: { payCompanyId: { widget: { type: 'select', model: 'payCompanyId', options: { remote: true } } } },
    async refreshFieldOptionData () {},
    getOptionData () { return [{ value: 'old-id', label: '旧公司' }] },
    async setData () { throw new Error('找不到选项时绝不能写值') },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['payCompanyName'], 1)
  assert.equal(coordination.issues.length, 1)
  assert.equal(coordination.issues[0].code, 'OPTION_PATCH_UNRESOLVED')
  assert.equal(coordination.issues[0].status, 'blocked')
  assert.equal(coordination.issues[0].expected, '不在选项里的公司')
  assert.equal(coordination.issues[0].actual, '旧公司')
  assert.equal(coordination.values.payCompanyId, 'old-id', '阻断时必须保留原值等待人工处理')
})

test('选项中存在多个同名项时阻断且不猜测绑定值', async () => {
  const template = { list: [{ type: 'select', model: 'companyId', options: { options: [
    { value: 'c1', label: '同名公司' }, { value: 'c2', label: '同名公司' },
  ] } }] }
  const form = {
    model: { companyId: 'c1', companyId__virtualName: '旧公司', companyName: '同名公司' },
    async setData () { throw new Error('同名歧义时绝不能写值') },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['companyName'], 1)
  assert.equal(coordination.issues.length, 1)
  assert.equal(coordination.issues[0].status, 'blocked')
  assert.equal(coordination.values.companyId, 'c1')
})

test('阻断问题随当前取值状态实时刷新', async () => {
  const template = { list: [{ type: 'select', model: 'payCompanyId', options: { remote: true } }] }
  const form = {
    formItemContexts: { payCompanyId: { widget: { type: 'select', model: 'payCompanyId', options: { remote: true } } } },
    async refreshFieldOptionData () {},
    getOptionData () { return [{ value: 'old-id', label: '旧公司' }] },
    getValues () { return this.model },
  }
  // 补丁名称在选项里不存在且绑定值仍显示历史公司：只读检查必须报阻断。
  const blocked = optionCoordinationIssues(form, template, {
    payCompanyId: 'old-id', payCompanyId__virtualName: '旧公司', payCompanyName: '新公司',
  }, ['payCompanyName'])
  assert.equal(blocked.length, 1)
  assert.equal(blocked[0].status, 'blocked')
  assert.equal(blocked[0].code, 'OPTION_PATCH_UNRESOLVED')
  // 用户把名称改回选项里存在的历史公司后，显示与绑定值重新一致，阻断立即解除。
  const cleared = optionCoordinationIssues(form, template, {
    payCompanyId: 'old-id', payCompanyId__virtualName: '旧公司', payCompanyName: '旧公司',
  }, ['payCompanyName'])
  assert.equal(cleared.length, 0)
})

test('子表单列按行协调且补丁波及行字段才触发', async () => {
  const template = { list: [{ type: 'subform', model: 'expenseBudgetList', list: [
    { type: 'select', model: 'expenseCompanyId', options: { remote: true } },
  ] }] }
  const form = {
    model: { expenseBudgetList: [
      { expenseCompanyId: 'old-id', expenseCompanyId__virtualName: '旧公司', expenseCompanyName: '新公司' },
      { expenseCompanyId: 'new-id', expenseCompanyId__virtualName: '新公司', expenseCompanyName: '新公司' },
    ] },
    getOptionData (field) {
      assert.equal(field, 'expenseCompanyId')
      return [{ value: 'new-id', label: '新公司' }, { value: 'keep-id', label: '保留公司' }]
    },
    async setData (values) { this.model.expenseBudgetList = values.expenseBudgetList },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['expenseBudgetList.expenseCompanyName'], 1)
  assert.equal(coordination.values.expenseBudgetList[0].expenseCompanyId, 'new-id')
  assert.equal(coordination.values.expenseBudgetList[1].expenseCompanyId, 'new-id', '已一致行保持原值')
})

test('回填绑定值后重放目标 onChange 联动并等待异步派生完成', async () => {
  const template = { list: [{ type: 'select', model: 'payCompanyId', options: { remote: true } }] }
  const replayed = []
  const form = {
    model: { payCompanyId: 'old-id', payCompanyName: '新公司' },
    formItemContexts: {
      payCompanyId: {
        widget: { type: 'select', model: 'payCompanyId', options: { remote: true }, events: { onChange: 'companyChanged' } },
        currentOptions: { fieldNode: 'payCompanyId' },
      },
    },
    async refreshFieldOptionData () {},
    getOptionData () { return [{ value: 'new-id', label: '新公司' }] },
    $nextTick () { return new Promise(resolve => setTimeout(resolve, 0)) },
    async setData (values) {
      Object.assign(this.model, values)
      // 模拟异步派生：联动脚本改写派生字段的时机晚于 setData 返回。
      await new Promise(resolve => setTimeout(resolve, 0))
      this.model.derivedFromCompany = '已联动'
    },
    getValues () { return this.model },
    eventFunction: { companyChanged () { replayed.push(this.getValue ? this.getValue('payCompanyId') : 'missing') } },
    getValue (key) { return this.model[key] },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['payCompanyName'], 1)
  assert.deepEqual(replayed, ['new-id'])
  assert.equal(coordination.values.derivedFromCompany, '已联动')
})

test('未波及的选项字段不参与协调', async () => {
  const template = { list: [
    { type: 'select', model: 'payCompanyId', options: { remote: true } },
    { type: 'select', model: 'typeId', name: '类型', options: { remote: true } },
  ] }
  let optionReads = 0
  const form = {
    model: { payCompanyId: 'old-id', payCompanyName: '新公司', typeId: 't1', typeId__virtualName: '类型一', typeName: '类型一' },
    formItemContexts: {
      payCompanyId: { widget: { type: 'select', model: 'payCompanyId', options: { remote: true } } },
      typeId: { widget: { type: 'select', model: 'typeId', options: { remote: true } } },
    },
    async refreshFieldOptionData () {},
    getOptionData (field) {
      optionReads++
      return field === 'payCompanyId' ? [{ value: 'new-id', label: '新公司' }] : [{ value: 't1', label: '类型一' }]
    },
    async setData (values) { Object.assign(this.model, values) },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['payCompanyName'], 1)
  assert.equal(coordination.issues.length, 0)
  assert.equal(optionReads, 1, '只读取被补丁波及控件的选项')
  assert.equal(coordination.values.typeId, 't1')
})

test('分支补丁字段会重放目标 onChange 以刷新派生值', async () => {
  let changed = ''
  const component = { widget: { events: { onChange: 'amountChanged' } }, currentOptions: { fieldNode: 'amount' } }
  const form = {
    getComponent() { return { value: 1999 } },
    formItemContexts: { amount: component },
    eventFunction: { amountChanged(options) { changed = options.fieldNode } },
  }
  await replayFieldChangeEvents(form, ['amount'])
  assert.equal(changed, 'amount')
})

test('重放异步 setData 后才能读取最新派生值', async () => {
  const form = {
    model: { amount: 1999, amountText: '旧金额' },
    formItemContexts: {
      amount: { widget: { events: { onChange: 'amountChanged' } }, currentOptions: { fieldNode: 'amount' } },
    },
    $nextTick () {
      return new Promise(resolve => setTimeout(resolve, 0))
    },
    setData (values) {
      return this.$nextTick().then(() => Object.assign(this.model, values))
    },
    getValues () {
      return this.model
    },
    eventFunction: {
      amountChanged () {
        // 目标模板脚本不会返回 setData 的 Promise，运行时必须主动等待 Vue 更新。
        void this.setData({ amountText: '壹仟玖佰玖拾玖元整' })
      },
    },
  }

  await replayFieldChangeEvents(form, ['amount'])
  assert.equal(form.getValues().amountText, '壹仟玖佰玖拾玖元整')
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

test('无表单审批方式使用复制运行时的真实页面注册入口', () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/runtime/hostVuePages.js', import.meta.url), 'utf8')
  for (const key of ['contract_review', 'contract_pay_request', 'buy_plan', 'buy_demand', 'buy_order', 'invoice_apply', 'travel_expense', 'request_funds', 'loan']) {
    assert.match(source, new RegExp(`${key}:`))
  }
  assert.match(source, /NoFormFlow:\s*HostNoFormPage/)
  assert.match(source, /CompanyAmountAdjustForm/)
  assert.match(source, /ManagePerformence/)
  assert.match(source, /HOST_VUE_PAGES\.NoFormFlow/)
})

test('无表单页面以数据工作区模式回显模型并隐藏目标操作栏', () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/HostVuePage.vue', import.meta.url), 'utf8')
  assert.match(source, /hydrateInitialValues\(this\.\$refs\.page, this\.values/)
  assert.match(source, /collectPageValues\(this\.\$refs\.page/)
  assert.match(source, /const FORM_MODEL_KEYS = new Set\(/)
  assert.match(source, /const workspaceMode = this\.readOnly \? 'preview' : 'edit'/)
  assert.match(source, /selectFlowType: ''/)
  assert.match(source, /function isVueInstance \(value\)/)
  assert.match(source, /function hasOwnPropertySafe \(value, key\)/)
  assert.match(source, /function isConfigObject \(value\)/)
  assert.match(source, /\.host-vue-page \.footer-bt, \.host-vue-page \.botton-group/)
})

test('路径字段提示有中文标签时不显示技术字段路径', () => {
  const source = fs.readFileSync(new URL('../../../web/src/features/path-configuration/FormDataHintsPanel.vue', import.meta.url), 'utf8')
  assert.match(source, /function fieldLabel\(field: PathConfigKeyField\)/)
  assert.doesNotMatch(source, /<code>\{\{ field\.path \}\}<\/code>/)
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
    assert.equal(sentHeaders.some(([name]) => /^(origin|referer)$/i.test(name)), false)
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
    assert.equal(fetched[0][1].headers.has('origin'), false)
    assert.equal(fetched[0][1].headers.has('referer'), false)
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


test('统计剔除静态隐藏容器与联动隐藏区域内的字段', () => {
  const template = { list: [
    { type: 'report', model: 'report_zhiqian_showIfPermission', options: { hidden: true }, list: [
      { type: 'fileupload', model: 'zhiqian_file1', options: { required: true } },
    ] },
    { type: 'report', model: 'report_GKzhaobiao_showIfPermission', list: [
      { type: 'fileupload', model: 'GKzhaobiao_file1', options: { required: true } },
    ] },
    { type: 'input', model: 'contractName', options: { required: true } },
  ] }
  const prepared = prepareTemplate(template, [
    { field: 'zhiqian_file1', power: 'edit' },
    { field: 'GKzhaobiao_file1', power: 'edit' },
    { field: 'contractName', power: 'edit' },
  ], false)
  const form = { dynamicHideFields: { report_GKzhaobiao_showIfPermission: false, report_zhiqian_showIfPermission: true } }
  // 直签封面页被联动隐藏，公开招标封面页可见但没填，合同名称已填。
  const hidden = hiddenFieldKeys(form, template, prepared.hiddenFields)
  const stats = formRuntimeStats(
    { zhiqian_file1: [{ name: '直签文件' }], contractName: '某合同' },
    prepared.editableFields,
    prepared.requiredEditableFields,
    hidden,
  )
  assert.deepEqual(stats, { filledEditable: 1, manualPending: 1 }, '隐藏区域剔除后只统计可见字段')
})

test('被补丁波及的远程字段选项数据源未加载时明确阻断', async () => {
  const template = { list: [{ type: 'select', model: 'classificationId', name: '合同分类', options: { remote: true } }] }
  const form = {
    model: {
      classificationId: ['old-category-id'],
      classificationId__virtualName: '施工类',
      classificationName: '工程服务类',
    },
    formItemContexts: { classificationId: { widget: { type: 'select', model: 'classificationId', options: { remote: true } } } },
    async refreshFieldOptionData () {},
    getOptionData () { return [] },
    async setData () { throw new Error('数据源不可用时绝不能写值') },
    getValues () { return this.model },
  }
  const coordination = await coordinateOptionPatches(form, template, form.model, ['classificationId__virtualName'], 2)
  assert.equal(coordination.issues.length, 1)
  assert.equal(coordination.issues[0].code, 'OPTION_SOURCE_UNAVAILABLE')
  assert.equal(coordination.issues[0].status, 'blocked')
  assert.equal(coordination.issues[0].fieldPath, 'classificationId')
  assert.deepEqual(coordination.values.classificationId, ['old-category-id'], '数据源不可用时保留原值，不改动模型')
})
