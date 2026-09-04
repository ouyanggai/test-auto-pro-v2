const CONTAINER_TYPES = new Set(['grid', 'report', 'table', 'subform', 'inline', 'dialog', 'card', 'group', 'tabs', 'collapse'])
const STANDARD_TYPES = new Set([
  'input', 'textarea', 'number', 'date', 'time', 'select', 'radio', 'checkbox', 'switch', 'cascader', 'fileupload',
  'text', 'html', 'divider', 'blank', 'link', 'button', 'component', ...CONTAINER_TYPES
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
      // FormMaking 的 component 是内置模板片段（options.template），只有 custom 或未知 type 才需要目标组件注册。
      const needsTargetRegistration = type === 'custom' || !STANDARD_TYPES.has(type)
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

// fieldAncestors 建立"字段模型 → 祖先容器模型链"映射：联动脚本按容器键整体显隐，
// 判断字段可见性必须沿祖先链回溯，而不是只看字段自身的 hidden 标记。
export function fieldAncestors (template) {
  const ancestors = new Map()
  const visit = (list, chain) => {
    for (const component of Array.isArray(list) ? list : []) {
      if (!component || typeof component !== 'object') continue
      const model = String(component.model || '').trim()
      const next = model ? chain.concat([model]) : chain
      if (model && !ancestors.has(model)) ancestors.set(model, next)
      for (const children of componentLists(component)) visit(children, next)
      if (Array.isArray(component.rows)) {
        for (const row of component.rows) {
          for (const column of Array.isArray(row && row.columns) ? row.columns : []) visit([column], next)
        }
      }
    }
  }
  visit(template && template.list, [])
  return ancestors
}

// hiddenFieldKeys 汇总静态隐藏字段与联动隐藏容器，返回当前实际不可见的字段模型集合。
// 动态显隐由目标模板脚本通过 FormMaking 的 dynamicHideFields 按容器键控制，祖先链上任一容器被隐藏即视为不可见。
export function hiddenFieldKeys (form, template, staticHiddenFields = []) {
  const hiddenContainers = new Set(Array.isArray(staticHiddenFields) ? staticHiddenFields : [])
  const dynamic = form && form.dynamicHideFields
  if (dynamic && typeof dynamic === 'object') {
    for (const [key, value] of Object.entries(dynamic)) {
      if (value === true && key) hiddenContainers.add(key)
    }
  }
  if (hiddenContainers.size === 0) return new Set()
  const hidden = new Set(hiddenContainers)
  for (const [field, chain] of fieldAncestors(template)) {
    if (chain.some(key => hiddenContainers.has(key))) hidden.add(field)
  }
  return hidden
}

// formRuntimeStats 只根据真实 getValues 和组件生效后的编辑权限计算当前填写统计；
// 隐藏字段（静态隐藏容器或联动隐藏区域内）用户既看不到也无法填写，必须从两项统计中剔除，
// 否则其他合同类型封面页的必填附件会被误报成"仍需手工"。
export function formRuntimeStats (values, editableFields, requiredEditableFields, hiddenFields = []) {
	const getPath = (input, path) => String(path || '').split('.').filter(Boolean).reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, input)
	const editable = new Set((Array.isArray(editableFields) ? editableFields : []).map(normalizeFieldPath))
	// hiddenFieldKeys 返回 Set，这里同时兼容数组与集合入参，避免调用方额外展开。
	const hidden = new Set([...(Array.isArray(hiddenFields) ? hiddenFields : (hiddenFields instanceof Set ? hiddenFields : []))].map(normalizeFieldPath))
	let filledEditable = 0
	for (const field of editable) {
		if (hidden.has(field)) continue
		if (!isEmptyModelValue(getPath(values, field))) filledEditable++
	}
	let manualPending = 0
	for (const field of new Set((Array.isArray(requiredEditableFields) ? requiredEditableFields : []).map(normalizeFieldPath))) {
		if (hidden.has(field)) continue
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

// ============ 选项型字段补丁协调 ============
// 目标平台的分支条件常按"显示名称"声明，最小补丁只改名称字段；而选项型控件真正绑定的是取值
// （Id、路径数组或选项值），不同步就会保留历史绑定值，控件按选项匹配继续显示旧名称。
// 这里的协调逻辑对所有选项型控件通用：枚举模板声明的控件与绑定路径，等待各自远程选项就绪，
// 通过 FormMaking 公共选项 API 读取真实选项，按名称唯一匹配后回填绑定值并同步名称与虚拟显示字段，
// 再重放原模板声明的 onChange 联动；无法唯一匹配时产生阻断问题，绝不猜测绑定值。

const OPTION_WIDGET_TYPES = new Set(['select', 'radio', 'checkbox', 'cascader'])
// 子表单是唯一把行数组写进模型值的容器；grid/report/table/tabs 等只是布局容器，不改变取值路径。
const VALUE_GROUP_TYPES = new Set(['subform'])
// 目标约定选项型控件绑定 Id 后缀字段，同前缀 Name 字段保存显示名称；这是平台级结构约定而非业务字段名。
const ID_SUFFIX_PATTERN = /Id$/
// 多选控件的名称字段可能以数组或分隔符文本保存，逐名解析后按顺序同步完整取值数组。
const MULTI_NAME_SEPARATOR = /[、,，;；]/

// optionFieldDescriptors 枚举模板中全部选项型控件及其实际绑定路径，含嵌套布局与子表单列。
export function optionFieldDescriptors (template) {
  const descriptors = []
  const visit = (list, group) => {
    for (const component of Array.isArray(list) ? list : []) {
      const type = String(component && component.type || '').trim()
      const model = String(component && component.model || '').trim()
      if (VALUE_GROUP_TYPES.has(type)) {
        visit(component.list, model || group)
        continue
      }
      const options = (component && component.options) || {}
      if (OPTION_WIDGET_TYPES.has(type) && model && options.dataBind !== false) {
        descriptors.push({
          model,
          type,
          group,
          label: String(component.name || ''),
          multiple: options.multiple === true,
          remote: options.remote === true,
          staticOptions: buildStaticOptions(options, type),
        })
      }
      for (const children of componentLists(component)) visit(children, group)
    }
  }
  visit(template && template.list, '')
  return descriptors
}

// buildStaticOptions 归一化模板静态选项；展示口径与渲染层一致：showLabel 时显示 label，否则直接显示 value。
// 级联的静态选项是取值/名称/子级的树，按声明 props 归一化后与远程选项同构。
function buildStaticOptions (options, type) {
  if (!options || !Array.isArray(options.options)) return []
  if (type === 'cascader') {
    const props = options.props && typeof options.props === 'object' ? options.props : { value: 'value', label: 'label' }
    const walk = nodes => (Array.isArray(nodes) ? nodes : []).filter(Boolean).map(node => ({
      value: node[props.value || 'value'],
      label: node[props.label || 'label'],
      children: walk(node.children),
    }))
    return walk(options.options)
  }
  const showLabel = options.showLabel === true || options.remote === true
  return options.options.filter(Boolean).map(option => ({
    value: option.value,
    label: showLabel ? (option.label == null ? option.value : option.label) : option.value,
  }))
}

// displayNameField 按目标"控件绑 Id、名称存同前缀 Name 字段"的结构约定推导成对名称字段；
// 多选控件按平台惯例使用复数 Ids/Names，同样成对出现。
function displayNameField (model) {
  if (/Ids$/.test(model) && model.length > 3) return model.slice(0, -3) + 'Names'
  if (ID_SUFFIX_PATTERN.test(model) && model.length > 2) return model.slice(0, -2) + 'Name'
  return ''
}

// virtualNameField 是 FormMaking 为每个选项型控件维护的虚拟显示字段，也是兜底渲染选项的标签来源。
function virtualNameField (model) {
  return model + '__virtualName'
}

// descriptorTriggered 判断补丁是否触及该控件的名称或虚拟显示字段；只有被补丁波及的控件才需要协调，
// 未被补丁波及的历史字段本身自洽，不做多余检查也不产生误导性问题。
function descriptorTriggered (descriptor, triggers) {
  if (!Array.isArray(triggers) || triggers.length === 0) return false
  const nameField = displayNameField(descriptor.model)
  const virtualField = virtualNameField(descriptor.model)
  const prefix = descriptor.group ? descriptor.group + '.' : ''
  return triggers.some(trigger => {
    const path = String(trigger || '').trim()
    return path !== '' && (path === prefix + nameField || path === prefix + virtualField)
  })
}

// optionEntries 展开一个控件绑定路径下的全部绑定点：普通字段一个，子表单列按行展开。
function optionEntries (values, descriptor) {
  if (!descriptor.group) {
    return [{ path: descriptor.model, container: values }]
  }
  const rows = values[descriptor.group]
  if (!Array.isArray(rows)) return []
  return rows.map((row, index) => (row && typeof row === 'object'
    ? { path: descriptor.group + '.' + descriptor.model + '[' + index + ']', container: row }
    : null)).filter(Boolean)
}

// entryValue 读取容器内字段值，兼容顶层键与嵌套路径两种形态。
function entryValue (container, key) {
  if (!container || !key) return undefined
  if (Object.prototype.hasOwnProperty.call(container, key)) return container[key]
  return modelValue(container, key)
}

// fieldOptions 读取控件当前真实选项：优先 FormMaking 公共选项 API（getOptionData 深拷贝，
// 异步刷新后总是最新），静态选项回落模板声明，最后才是包装层组件引用，避免空选项误判。
function fieldOptions (form, descriptor) {
  try {
    const options = form && typeof form.getOptionData === 'function' ? form.getOptionData(descriptor.model) : null
    if (Array.isArray(options) && options.length > 0) return options
  } catch (_) {
    // 字段实例尚未完成挂载时静默回落，不能中断只读历史回放。
  }
  if (!descriptor.remote && descriptor.staticOptions.length > 0) return descriptor.staticOptions
  const context = formItemContext(form, descriptor.model)
  const nested = context && context.$refs && context.$refs.generateElementItem
  const remoteOptions = nested && nested.remoteOptions
  return Array.isArray(remoteOptions) ? remoteOptions : []
}

// optionEntryState 计算一个绑定点的当前一致性状态。
// consistent：绑定值按真实选项显示出的名称就是补丁目标名称（含"值不在选项里按虚拟名称兜底显示"）；
// contradiction：显示出的仍是历史名称或绑定值为空；空选项时不能下结论，交由等待循环继续等数据源。
function optionEntryState (descriptor, bound, virtual, options, desired) {
  const expected = String(desired)
  if (descriptor.type === 'cascader') return cascaderEntryState(bound, options, expected)
  if (descriptor.multiple) return multipleEntryState(bound, options, expected)
  if (isEmptyModelValue(bound)) return { state: 'contradiction', actual: '' }
  const matched = options.find(option => option && String(option.value) === String(bound))
  if (matched) {
    return String(matched.label ?? '') === expected
      ? { state: 'consistent', actual: String(matched.label ?? '') }
      : { state: 'contradiction', actual: String(matched.label ?? '') }
  }
  return String(virtual ?? '') === expected
    ? { state: 'consistent', actual: String(virtual ?? '') }
    : { state: 'contradiction', actual: String(virtual ?? '') }
}

// multipleEntryState 多选按标签顺序整体比对：el-select 多选没有虚拟名称兜底，任一元素解析不出标签即显示异常。
function multipleEntryState (bound, options, expected) {
  const boundArray = Array.isArray(bound) ? bound : []
  if (boundArray.length === 0) return { state: 'contradiction', actual: '' }
  const labels = []
  for (const value of boundArray) {
    const matched = options.find(option => option && String(option.value) === String(value))
    if (!matched) return { state: 'contradiction', actual: labels.join('、') }
    labels.push(String(matched.label ?? ''))
  }
  return labels.join('、') === expected
    ? { state: 'consistent', actual: labels.join('、') }
    : { state: 'contradiction', actual: labels.join('、') }
}

// cascaderEntryState 级联按完整取值路径逐级解析标签；级联提交的是路径数组，只看叶子名称不够。
function cascaderEntryState (bound, options, expected) {
  if (isEmptyModelValue(bound)) return { state: 'contradiction', actual: '' }
  const trail = Array.isArray(bound) ? bound.map(String) : [String(bound)]
  const labels = []
  let nodes = options
  for (const value of trail) {
    const node = Array.isArray(nodes) ? nodes.find(item => item && String(item.value) === value) : null
    if (!node) break
    labels.push(String(node.label ?? ''))
    nodes = Array.isArray(node.children) ? node.children : []
  }
  if (labels.length !== trail.length) return { state: 'contradiction', actual: labels.join('/') }
  return labels[labels.length - 1] === expected
    ? { state: 'consistent', actual: expected }
    : { state: 'contradiction', actual: labels.join('/') }
}

// desiredNameList 把补丁写入的显示名称展开为待匹配列表：单选一个，多选按数组或分隔符文本展开保持顺序。
function desiredNameList (descriptor, desired) {
  if (!descriptor.multiple) return [String(desired)]
  if (Array.isArray(desired)) return desired.map(item => String(item).trim()).filter(Boolean)
  return String(desired).split(MULTI_NAME_SEPARATOR).map(item => item.trim()).filter(Boolean)
}

// resolveOptionEntry 在当前真实选项中按名称唯一匹配补丁目标：单选与多选查平铺选项，
// 级联查叶子并还原完整取值路径；找不到或多条同名一律拒绝解析，绝不猜测绑定值。
function resolveOptionEntry (descriptor, options, desiredNames) {
  if (descriptor.type === 'cascader') {
    const leaves = cascaderLeafPaths(options, [])
    const trails = []
    for (const name of desiredNames) {
      const matches = leaves.filter(leaf => leaf.label === name)
      if (matches.length !== 1) return { resolved: false, reason: matches.length === 0 ? 'not-found' : 'ambiguous' }
      trails.push(matches[0].trail)
    }
    return { resolved: true, value: descriptor.multiple ? trails : trails[0] }
  }
  const values = []
  for (const name of desiredNames) {
    const matches = options.filter(option => option && String(option.label ?? '') === name)
    if (matches.length !== 1) return { resolved: false, reason: matches.length === 0 ? 'not-found' : 'ambiguous' }
    values.push(matches[0].value)
  }
  return { resolved: true, value: descriptor.multiple ? values : values[0] }
}

// cascaderLeafPaths 深度收集叶子名称到取值路径的映射，供级联按名称还原完整路径。
function cascaderLeafPaths (nodes, prefix) {
  const paths = []
  for (const node of Array.isArray(nodes) ? nodes : []) {
    if (!node) continue
    const trail = prefix.concat([node.value])
    if (Array.isArray(node.children) && node.children.length > 0) {
      paths.push(...cascaderLeafPaths(node.children, trail))
    } else {
      paths.push({ label: String(node.label ?? ''), trail })
    }
  }
  return paths
}

// evaluateOptionPatches 只读评估全部被补丁波及的选项型绑定点。
// apply=true 时把可唯一解析项写进 values（根字段进 patches，子表单列原地写行对象），否则只产出阻断问题。
// 返回 waiting 表示仍有远程选项未就绪，需要外层继续等待后重新评估。
function evaluateOptionPatches (form, descriptors, triggers, values, apply) {
  const issues = []
  const rootPatch = {}
  const replayModels = new Set()
  const touchedGroups = new Set()
  const unavailable = []
  let waiting = false
  for (const descriptor of descriptors) {
    if (!descriptorTriggered(descriptor, triggers)) continue
    const options = fieldOptions(form, descriptor)
    const nameField = displayNameField(descriptor.model)
    const virtualField = virtualNameField(descriptor.model)
    for (const entry of optionEntries(values, descriptor)) {
      const nameRaw = entryValue(entry.container, nameField)
      const virtual = entryValue(entry.container, virtualField)
      const desired = (nameRaw == null || nameRaw === '') ? virtual : nameRaw
      if (desired == null || desired === '') continue
      const bound = entryValue(entry.container, descriptor.model)
      const state = optionEntryState(descriptor, bound, virtual, options, desired)
      if (state.state === 'consistent') continue
      if (options.length === 0 && descriptor.remote) {
        waiting = true
        // 远程选项始终为空时数据源大概率故障；先记入待报清单，等待重试用尽后统一阻断提示。
        unavailable.push({ path: entry.path, label: descriptor.label, desired: String(desired), actual: String(virtual ?? '') })
        continue
      }
      const resolution = resolveOptionEntry(descriptor, options, desiredNameList(descriptor, desired))
      if (!resolution.resolved) {
        issues.push({
          code: 'OPTION_PATCH_UNRESOLVED', status: 'blocked', source: 'iframe_runtime',
          fieldPath: entry.path, fieldLabel: descriptor.label, operator: '',
          expected: String(desired), actual: state.actual,
          relatedFields: [nameField, virtualField].filter(Boolean),
          message: resolution.reason === 'ambiguous'
            ? '选项中存在多个同名项，无法唯一确定该字段绑定值，请手工核对该字段'
            : '选项中找不到该名称对应的选项，无法完成补丁映射，请手工核对该字段',
          canRetry: true,
        })
        continue
      }
      if (!apply) continue
      entry.container[descriptor.model] = resolution.value
      entry.container[virtualField] = desired
      if (nameField && nameRaw !== undefined) entry.container[nameField] = desired
      if (descriptor.group) {
        touchedGroups.add(descriptor.group)
      } else {
        rootPatch[descriptor.model] = resolution.value
        rootPatch[virtualField] = desired
        if (nameField && nameRaw !== undefined) rootPatch[nameField] = desired
        replayModels.add(descriptor.model)
      }
    }
  }
  return { issues, waiting, unavailable, rootPatch, replayModels: [...replayModels], touchedGroups: [...touchedGroups] }
}

// refreshOptionFields 主动刷新被协调控件自己的远程选项数据源，避免首次刷新只完成挂载而选项仍为空。
async function refreshOptionFields (form, descriptors) {
  if (typeof form?.refreshFieldOptionData !== 'function') return false
  const models = descriptors.filter(descriptor => descriptor.remote).map(descriptor => descriptor.model)
  if (models.length === 0) return false
  try {
    await form.refreshFieldOptionData(models)
  } catch (_) {
    // 选项接口失败时保留原始值，不能让只读历史回放因辅助显示数据不可用而白屏。
  }
  return true
}

// coordinateOptionPatches 是选项型字段补丁协调入口：等待控件自己的远程选项、数据源加载完成后，
// 在真实选项里按名称唯一匹配并回填绑定值，重放原模板声明的 onChange 联动并等待异步派生完成，
// 再读取最终表单值。无法唯一匹配且显示仍停留在历史值的字段产生阻断问题：
// 绝不猜测绑定值，也不保留"路径提示已是新名称、控件显示历史值"的矛盾状态。
export async function coordinateOptionPatches (form, template, values, triggers = [], retries = 20) {
  const current = clonePlain(values || {})
  const patchTriggers = Array.isArray(triggers) ? triggers : []
  if (!form || typeof form.setData !== 'function') return { values: current, issues: [] }
  const descriptors = optionFieldDescriptors(template)
  if (descriptors.length === 0 || patchTriggers.length === 0) return { values: current, issues: [] }
  const attempts = Math.max(1, Number(retries) || 1)
  let refreshed = false
  let issues = []
  for (let attempt = 0; attempt < attempts; attempt++) {
    if (!refreshed) refreshed = await refreshOptionFields(form, descriptors)
    const evaluation = evaluateOptionPatches(form, descriptors, patchTriggers, current, true)
    if (evaluation.replayModels.length > 0 || evaluation.touchedGroups.length > 0) {
      const patch = clonePlain(evaluation.rootPatch)
      for (const group of evaluation.touchedGroups) patch[group] = clonePlain(current[group])
      if (Object.keys(patch).length > 0) await form.setData(patch)
      // 回填绑定值后必须重放目标模板为该字段声明的 onChange 联动，补齐 setData 不会触发的派生字段计算。
      await replayFieldChangeEvents(form, evaluation.replayModels)
      await waitForFormUpdate(form)
      if (typeof form.getValues === 'function') Object.assign(current, clonePlain(form.getValues() || {}))
      continue
    }
    issues = evaluation.issues
    if (evaluation.waiting && attempt + 1 < attempts) {
      await new Promise(resolve => setTimeout(resolve, 100))
      continue
    }
    // 重试用尽后远程选项仍为空：数据源故障会让"按名称映射"永远无法核对，必须阻断而不是静默留空。
    for (const item of evaluation.unavailable) {
      issues.push({
        code: 'OPTION_SOURCE_UNAVAILABLE', status: 'blocked', source: 'iframe_runtime',
        fieldPath: item.path, fieldLabel: item.label, operator: '',
        expected: item.desired, actual: item.actual, relatedFields: [],
        message: '该字段的选项数据源未加载出任何选项，无法完成补丁映射，请检查目标平台对应服务后重试',
        canRetry: true,
      })
    }
    break
  }
  return { values: current, issues }
}

// optionCoordinationIssues 只做只读一致性检查，不等待也不写入，供每次捕获值时刷新阻断问题：
// 用户手工把字段改成一致取值后，阻断应立即解除而不是残留旧问题。
export function optionCoordinationIssues (form, template, values, triggers = []) {
  const patchTriggers = Array.isArray(triggers) ? triggers : []
  if (!form || patchTriggers.length === 0) return []
  const descriptors = optionFieldDescriptors(template)
  if (descriptors.length === 0) return []
  const evaluation = evaluateOptionPatches(form, descriptors, patchTriggers, clonePlain(values || {}), false)
  const issues = [...evaluation.issues]
  // 只读检查不等待数据源，远程选项此刻为空即视为暂不可用，与装载期阻断口径一致。
  for (const item of evaluation.unavailable) {
    issues.push({
      code: 'OPTION_SOURCE_UNAVAILABLE', status: 'blocked', source: 'iframe_runtime',
      fieldPath: item.path, fieldLabel: item.label, operator: '',
      expected: item.desired, actual: item.actual, relatedFields: [],
      message: '该字段的选项数据源尚未加载出选项，无法核对该字段的补丁映射，请稍后重试或检查目标平台服务',
      canRetry: true,
    })
  }
  return issues
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

// modelValue 读取嵌套路径值，兼容顶层键与点分路径两种取值口径。
function modelValue (values, path) {
  if (values && Object.prototype.hasOwnProperty.call(values, path)) return values[path]
  return String(path || '').split('.').filter(Boolean)
    .reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, values)
}

// isEmptyModelValue 判断值是否为空形态；初始默认模型里的空键不能误判为人工覆盖。
function isEmptyModelValue (value) {
  if (value === undefined || value === null) return true
  if (typeof value === 'string') return value.trim() === ''
  if (Array.isArray(value)) return value.length === 0
  if (typeof value === 'object') return Object.keys(value).length === 0
  return false
}
