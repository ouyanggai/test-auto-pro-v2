const CONTAINER_TYPES = new Set(['grid', 'report', 'table', 'subform', 'inline', 'dialog', 'card', 'group', 'tabs', 'collapse'])
const STANDARD_TYPES = new Set([
  'input', 'textarea', 'number', 'date', 'time', 'select', 'radio', 'checkbox', 'switch', 'cascader', 'fileupload',
  'text', 'html', 'divider', 'blank', 'link', 'button', ...CONTAINER_TYPES
])
const TARGET_COMPONENT_NAMES = new Set(JSON.parse(process.env.VUE_APP_TARGET_COMPONENT_NAMES || '[]'))
const SUBMIT_HOOK_NAMES = ['beforeSubmitAndDraft', 'beforeSubmit']
const CONTRACT_COMPONENT_NAMES = new Set(['legal-contract-doctable', 'contract-seal-review-business'])
const RUNTIME_CONTEXT_KEYS = [
  'companyId', 'companyName', 'departmentId', 'departmentName', 'userId', 'accountName', 'customerCode'
]

// clonePlain 在 postMessage 与 Vue 观察对象边界复制纯数据，禁止代理对象进入 FormMaking。
export function clonePlain (value) {
  return JSON.parse(JSON.stringify(value == null ? null : value))
}

// normalizeFieldPath 对齐目标页面的嵌套字段权限编码，避免 edit 权限因 _$$_ 分隔符失配而被错误禁用。
function normalizeFieldPath (value) {
  return String(value || '').trim().replaceAll('_$$_', '.')
}

// componentLists 枚举目标 FormMaking 的真实嵌套容器结构。
function componentLists (component) {
  const lists = []
  if (Array.isArray(component.list)) lists.push(component.list)
  if (Array.isArray(component.tableColumns)) lists.push(component.tableColumns)
  for (const column of Array.isArray(component.columns) ? component.columns : []) {
    if (Array.isArray(column && column.list)) lists.push(column.list)
  }
  for (const row of Array.isArray(component.rows) ? component.rows : []) {
    if (Array.isArray(row && row.list)) lists.push(row.list)
    for (const column of Array.isArray(row && row.columns) ? row.columns : []) {
      if (Array.isArray(column && column.list)) lists.push(column.list)
    }
  }
  return lists
}

// componentRuntimeName 按目标模板真实字段优先级解析组件注册名，避免 type=custom 掩盖 el。
export function componentRuntimeName (component) {
  const options = component && component.options
  return String(
    (component && component.el) ||
    (options && options.componentName) ||
    (options && options.component) ||
    (component && component.componentName) ||
    (component && component.component) ||
    ''
  ).trim()
}

// buildComponentExtendProps 为每个目标组件补齐当前会话身份；合同组件的发起态必须重置业务单号，避免把模板历史业务带入新表单。
function buildComponentExtendProps (componentName, existingProps, runtimeContext) {
  const props = { ...existingProps }
  for (const key of RUNTIME_CONTEXT_KEYS) {
    if (props[key] === undefined && runtimeContext[key] !== undefined && runtimeContext[key] !== null) {
      props[key] = String(runtimeContext[key])
    }
  }
  if (CONTRACT_COMPONENT_NAMES.has(componentName)) {
    props.isFlowInitiate = true
    props.businessId = ''
    if (runtimeContext.companyId !== undefined && runtimeContext.companyId !== null) {
      props.companyId = String(runtimeContext.companyId)
    }
  }
  return props
}

// prepareTemplate 在完整模板副本上应用目标权限，并把尚未独立适配的目标自定义组件明确标记为 unsupported。
// 分支条件由服务端对原始数据重算；runtime 不接收字段映射或生成规则，避免把历史正文改造成工具状态。
export function prepareTemplate (rawTemplate, permissions, readOnly, runtimeContext = {}) {
	const template = clonePlain(rawTemplate || {})
	const permissionByField = new Map((Array.isArray(permissions) ? permissions : []).map(item => [normalizeFieldPath(item.field), item.power]))
  const unsupported = new Set()
  const allFields = new Set()
  const editableFields = new Set()
	const hiddenFields = new Set()
	const requiredEditableFields = new Set()
  const visit = (list) => {
    for (const component of Array.isArray(list) ? list : []) {
      const type = String(component && component.type || '').trim()
      const model = String(component && component.model || '').trim()
      const targetComponentName = componentRuntimeName(component) || (!STANDARD_TYPES.has(type) ? type : '')
      const needsTargetRegistration = type === 'custom' || type === 'component' || !STANDARD_TYPES.has(type)
      // 真实上游 main.js 已注册的目标组件交给原生 FormMaking 渲染；只有未注册组件才阻止错误宣称支持。
      if (needsTargetRegistration && !TARGET_COMPONENT_NAMES.has(targetComponentName)) {
        unsupported.add(`${component.name || model || type || '未知组件'}：依赖 rsh-flow-components 宿主业务适配`)
      }
      if (needsTargetRegistration) {
        component.options = component.options || {}
        const existingProps = component.options.extendProps && typeof component.options.extendProps === 'object'
          ? component.options.extendProps
          : {}
        component.options.extendProps = buildComponentExtendProps(targetComponentName, existingProps, runtimeContext)
      }
      if (model) {
        // 目标页面先禁用整张表单，再只开放流程节点明确授权的字段；缺少权限不能默认可编辑。
		const field = normalizeFieldPath(model)
		const power = permissionByField.get(field) || 'only_read'
		allFields.add(field)
			if (!readOnly && power === 'edit') editableFields.add(field)
			const staticallyHidden = power === 'hide' || component.hidden === true || component.options && (component.options.hidden === true || component.options.display === false)
			if (staticallyHidden) hiddenFields.add(field)
			component.options = component.options || {}
			component.options.hidden = staticallyHidden
			component.options.disabled = readOnly || power !== 'edit'
			if (component.options.disabled) {
          component.options.required = false
          // 未开放字段必须移除整组运行时校验，目标页面也是先按权限清理规则再 refresh。
				if (Array.isArray(component.rules)) component.rules = []
			} else if (!staticallyHidden && (component.options.required || Array.isArray(component.rules) && component.rules.some(rule => rule && rule.required))) {
				requiredEditableFields.add(field)
        }
      }
      for (const children of componentLists(component)) visit(children)
    }
  }
  visit(template.list)
  const config = template.config = template.config || {}
  const isolatedHooks = []
  for (const hook of SUBMIT_HOOK_NAMES) {
    if (config[hook]) isolatedHooks.push(hook)
    // F-007 只保存 V2 配置，提交与草稿钩子必须在 FormMaking 装载前清除；eventScript 是目标表单初始化与字段联动的一部分，必须保留。
    delete config[hook]
  }
  return {
    template,
    unsupported: [...unsupported],
    isolatedHooks,
    allFields: [...allFields],
		editableFields: [...editableFields],
		hiddenFields: [...hiddenFields],
		requiredEditableFields: [...requiredEditableFields]
	}
}

// formRuntimeStats 只根据真实 getValues 和组件生效后的编辑权限计算当前填写统计。
export function formRuntimeStats (values, editableFields, requiredEditableFields) {
	const getPath = (input, path) => String(path || '').split('.').filter(Boolean).reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, input)
	const editable = new Set((Array.isArray(editableFields) ? editableFields : []).map(normalizeFieldPath))
	let filledEditable = 0
	for (const field of editable) if (!isEmptyModelValue(getPath(values, field))) filledEditable++
	let manualPending = 0
	for (const field of new Set((Array.isArray(requiredEditableFields) ? requiredEditableFields : []).map(normalizeFieldPath))) {
		if (editable.has(field) && isEmptyModelValue(getPath(values, field))) manualPending++
	}
	return { filledEditable, manualPending }
}

// captureFormValues 使用目标运行时 getData 仅做校验，并以 getValues 返回包含虚拟字段的完整对象。
export async function captureFormValues (form, validate) {
  if (!form) throw new Error('目标 FormMaking 尚未就绪')
  if (validate) {
    try {
      await form.getData(true)
    } catch (_) {
      throw new Error('请先完成表单中的必填项')
    }
  }
  if (typeof form.getValues !== 'function') throw new Error('目标 FormMaking 运行时缺少 getValues 能力')
  return clonePlain(form.getValues())
}

// buildValuesEnvelope 只回传 runtime 捕获的原始 values 和结构化校验摘要，不附带额外业务元数据。
export function buildValuesEnvelope ({ values, validated, unsupported, dirty, stats, issues, renderType }) {
  return {
    renderType: String(renderType || 'formmaking'),
    values: clonePlain(values || {}),
		validated: Boolean(validated),
		unsupported: Array.isArray(unsupported) ? unsupported.map(String) : [],
		dirty: Boolean(dirty),
		issues: Array.isArray(issues) ? clonePlain(issues) : [],
		stats: stats && typeof stats === 'object' ? clonePlain(stats) : { filledEditable: 0, manualPending: 0 }
	}
}

// refreshPreparedForm 只刷新已预先写入权限的模板，避免目标自定义组件因缺少 disabledElement 被统一运行时禁用调用击穿。
// FormMaking 的 refresh 会重新初始化模板并清空 models；因此刷新前捕获当前值、刷新后回填，避免用户已填数据被清空。
export async function refreshPreparedForm (form) {
  if (!form || typeof form.refresh !== 'function') throw new Error('目标 FormMaking 运行时缺少 refresh 能力')
  const current = typeof form.getValues === 'function' ? form.getValues() : null
  await form.refresh()
  if (current != null && typeof form.setData === 'function') await form.setData(current)
}

// waitForFormUpdate 等待 FormMaking 内部 setData 通过 Vue 更新模型，避免事件脚本刚返回就读取到旧派生值。
async function waitForFormUpdate (form) {
  if (typeof form?.$nextTick === 'function') {
    const result = form.$nextTick()
    if (result && typeof result.then === 'function') {
      await result
      return
    }
  }
  await new Promise(resolve => setTimeout(resolve, 0))
}

// reconcileLinkedSelectValues 在目标下拉选项就绪后按显示名称回填对应 ID，
// 解决分支补丁只改变名称字段时，FormMaking 仍按历史 ID 显示旧选项的问题。
export async function reconcileLinkedSelectValues (form, values, retries = 20) {
  if (!form || typeof form.setData !== 'function') return clonePlain(values || {})
  const current = clonePlain(values || {})
  if (!hasLinkedSelectCandidate(form, current)) return current
  const attempts = Math.max(1, Number(retries) || 1)
  let optionsRequested = false
  for (let attempt = 0; attempt < attempts; attempt++) {
    if (!optionsRequested) optionsRequested = await refreshLinkedSelectOptions(form)
    const patches = linkedSelectPatches(form, current)
    if (Object.keys(patches).length > 0) {
      await form.setData(patches)
      await replayFieldChangeEvents(form, Object.keys(patches))
      Object.assign(current, patches)
      if (typeof form.getValues === 'function') Object.assign(current, clonePlain(form.getValues() || {}))
      return current
    }
    if (attempt + 1 < attempts) await new Promise(resolve => setTimeout(resolve, 100))
  }
  return current
}

// refreshLinkedSelectOptions 主动等待远程下拉数据源，避免首次刷新只完成表单挂载而选项仍为空。
async function refreshLinkedSelectOptions (form) {
  if (typeof form?.refreshFieldOptionData !== 'function') return false
  const fields = formItemContextEntries(form)
    .filter(({ field, context }) => {
      return String(context?.widget?.type || '').trim() === 'select' &&
        field.endsWith('Id') && context?.widget?.options?.remote === true
    })
    .map(({ field }) => field)
  if (fields.length === 0) return false
  try {
    await form.refreshFieldOptionData(fields)
  } catch (_) {
    // 选项接口失败时保留原始值，不能让只读历史回放因辅助显示数据不可用而白屏。
  }
  return true
}

// replayFieldChangeEvents 重放目标模板为指定字段声明的 onChange 脚本，补齐 setData 不会触发的派生字段计算。
export async function replayFieldChangeEvents (form, fields) {
  if (!form || !form.eventFunction) return
  for (const field of Array.isArray(fields) ? fields : []) {
    const component = formItemContext(form, String(field || '').trim())
    const eventKey = component?.widget?.events?.onChange
    const handler = eventKey && form.eventFunction[eventKey]
    if (typeof handler !== 'function') continue
    // 目标事件脚本通过 this.getValue/this.setData 访问 FormMaking 实例，不能以裸函数调用丢失上下文。
    await handler.call(form, component.currentOptions || {})
    await waitForFormUpdate(form)
  }
}

// formItemContext 读取 FormMaking 为字段包装组件维护的真实上下文；getComponent 返回的是 el-select 等控件实例，不含字段定义。
function formItemContext (form, field) {
  const context = form?.formItemContexts?.[field]
  return Array.isArray(context) ? context[0] : context
}

// formItemContextEntries 展平子表单可能生成的同名字段上下文，并保留根模型路径。
function formItemContextEntries (form) {
  const contexts = form?.formItemContexts
  if (!contexts || typeof contexts !== 'object') return []
  return Object.entries(contexts).flatMap(([field, value]) => (Array.isArray(value) ? value : [value])
    .filter(Boolean).map(context => ({ field, context })))
}

function modelValue (values, path) {
  if (values && Object.prototype.hasOwnProperty.call(values, path)) return values[path]
  return String(path || '').split('.').filter(Boolean)
    .reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, values)
}

// hasLinkedSelectCandidate 判断当前模板是否存在需要等待真实远程选项的 ID 下拉字段，普通表单不增加等待开销。
function hasLinkedSelectCandidate (form, values) {
  return formItemContextEntries(form).some(({ field, context }) => {
    if (String(context?.widget?.type || '').trim() !== 'select' || !field.endsWith('Id')) return false
    const nameModel = `${field.slice(0, -2)}Name`
    return modelValue(values, nameModel) !== undefined || modelValue(values, `${field}__virtualName`) !== undefined
  })
}

// linkedSelectPatches 仅使用 FormMaking 已加载的真实选项建立 Name/Id 关联，找不到唯一标签时保持原值。
function linkedSelectPatches (form, values) {
  const patches = {}
  for (const { field, context } of formItemContextEntries(form)) {
    const widget = context?.widget
    if (String(widget?.type || '').trim() !== 'select' || !field.endsWith('Id')) continue
    const nameModel = `${field.slice(0, -2)}Name`
    const virtualModel = `${field}__virtualName`
    const desired = modelValue(values, nameModel) ?? modelValue(values, virtualModel)
    if (desired === undefined || desired === null || desired === '') continue
    const options = linkedSelectOptions(form, field, context)
    if (options.length === 0) continue
    const matches = options.filter(option => option && String(option.label ?? '') === String(desired))
    if (matches.length !== 1) continue
    const option = matches[0]
    const nextID = option.value
    if (nextID === undefined || nextID === null || nextID === '') continue
    patches[field] = nextID
    patches[virtualModel] = String(option.label ?? desired)
    if (modelValue(values, nameModel) !== undefined) patches[nameModel] = patches[virtualModel]
  }
  return patches
}

// linkedSelectOptions 优先读取 FormMaking 对字段实例公开的真实选项，避免包装组件的 ref 在异步刷新后仍停留在旧值。
function linkedSelectOptions (form, field, context) {
  try {
    const options = form?.getOptionData?.(field)
    if (Array.isArray(options) && options.length > 0) return options
  } catch (_) {
    // 字段实例尚未完成挂载时退回包装层，不能中断只读历史表单回放。
  }
  const options = context?.$refs?.generateElementItem?.remoteOptions
  return Array.isArray(options) ? options : []
}

// isEmptyModelValue 判断值是否为空形态；初始默认模型里的空键不能误判为人工覆盖。
function isEmptyModelValue (value) {
  if (value === undefined || value === null) return true
  if (typeof value === 'string') return value.trim() === ''
  if (Array.isArray(value)) return value.length === 0
  if (typeof value === 'object') return Object.keys(value).length === 0
  return false
}
