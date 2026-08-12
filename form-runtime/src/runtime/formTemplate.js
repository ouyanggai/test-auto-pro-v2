const CONTAINER_TYPES = new Set(['grid', 'report', 'table', 'subform', 'inline', 'dialog', 'card', 'group', 'tabs', 'collapse'])
const STANDARD_TYPES = new Set([
  'input', 'textarea', 'number', 'date', 'time', 'select', 'radio', 'checkbox', 'switch',
  'text', 'html', 'divider', 'blank', 'link', 'button', ...CONTAINER_TYPES
])
const TARGET_COMPONENT_NAMES = new Set(JSON.parse(process.env.VUE_APP_TARGET_COMPONENT_NAMES || '[]'))
const SUBMIT_HOOK_NAMES = ['beforeSubmitAndDraft', 'beforeSubmit', 'eventScript']

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

// prepareTemplate 在完整模板副本上应用字段权限，并把尚未独立适配的目标自定义组件明确标记为 unsupported。
export function prepareTemplate (rawTemplate, permissions, readOnly) {
  const template = clonePlain(rawTemplate || {})
  const permissionByField = new Map((Array.isArray(permissions) ? permissions : []).map(item => [normalizeFieldPath(item.field), item.power]))
  const unsupported = new Set()
  const allFields = new Set()
  const editableFields = new Set()
  const hiddenFields = new Set()
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
      if (model) {
        // 目标页面先禁用整张表单，再只开放流程节点明确授权的字段；缺少权限不能默认可编辑。
        const field = normalizeFieldPath(model)
        const power = permissionByField.get(field) || 'only_read'
        allFields.add(field)
        if (!readOnly && power === 'edit') editableFields.add(field)
        if (power === 'hide') hiddenFields.add(field)
        component.options = component.options || {}
        component.options.hidden = power === 'hide'
        component.options.disabled = readOnly || power !== 'edit'
        if (component.options.disabled) {
          component.options.required = false
          // 未开放字段必须移除整组运行时校验，目标页面也是先按权限清理规则再 refresh。
          if (Array.isArray(component.rules)) component.rules = []
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
    // F-007 只保存 V2 配置，提交/草稿/业务事件钩子必须在 FormMaking 装载前清除，不能执行也不能误判成不支持。
    delete config[hook]
  }
  return {
    template,
    unsupported: [...unsupported],
    isolatedHooks,
    allFields: [...allFields],
    editableFields: [...editableFields],
    hiddenFields: [...hiddenFields]
  }
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

// refreshPreparedForm 只刷新已预先写入权限的模板，避免目标自定义组件因缺少 disabledElement 被统一运行时禁用调用击穿。
export async function refreshPreparedForm (form) {
  if (!form || typeof form.refresh !== 'function') throw new Error('目标 FormMaking 运行时缺少 refresh 能力')
  await form.refresh()
}

// diffManualPaths 递归比较生成基线与 getValues 结果，得到换一组时必须保留的人工覆盖路径。
export function diffManualPaths (generated, current) {
  const paths = new Set()
  const walk = (left, right, prefix) => {
    if (JSON.stringify(left) === JSON.stringify(right)) return
    if (left && right && typeof left === 'object' && typeof right === 'object' && !Array.isArray(left) && !Array.isArray(right)) {
      const keys = new Set([...Object.keys(left), ...Object.keys(right)])
      for (const key of keys) walk(left[key], right[key], prefix ? `${prefix}.${key}` : key)
      return
    }
    if (prefix) paths.add(prefix)
  }
  walk(generated || {}, current || {}, '')
  return [...paths].sort()
}
