const CONTAINER_TYPES = new Set(['grid', 'report', 'table', 'subform', 'inline', 'dialog', 'card', 'group', 'tabs', 'collapse'])
const STANDARD_TYPES = new Set([
  'input', 'textarea', 'number', 'date', 'time', 'select', 'radio', 'checkbox', 'switch',
  'text', 'html', 'divider', 'blank', 'link', 'button', ...CONTAINER_TYPES
])

// clonePlain 在 postMessage 与 Vue 观察对象边界复制纯数据，禁止代理对象进入 FormMaking。
export function clonePlain (value) {
  return JSON.parse(JSON.stringify(value == null ? null : value))
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

// prepareTemplate 在完整模板副本上应用字段权限，并把尚未独立适配的目标自定义组件明确标记为 unsupported。
export function prepareTemplate (rawTemplate, permissions, readOnly) {
  const template = clonePlain(rawTemplate || {})
  const permissionByField = new Map((Array.isArray(permissions) ? permissions : []).map(item => [String(item.field || ''), item.power]))
  const unsupported = new Set()
  const visit = (list) => {
    for (const component of Array.isArray(list) ? list : []) {
      const type = String(component && component.type || '').trim()
      const model = String(component && component.model || '').trim()
      const customName = String(component && component.options && (component.options.componentName || component.options.component) || '').trim()
      if (!STANDARD_TYPES.has(type) || type === 'component' || customName) {
        unsupported.add(`${component.name || model || type || '未知组件'}：依赖 rsh-flow-components 宿主业务适配`)
      }
      if (model) {
        const power = permissionByField.get(model) || (readOnly ? 'only_read' : 'edit')
        component.options = component.options || {}
        component.options.hidden = power === 'hide'
        component.options.disabled = readOnly || power !== 'edit'
        if (component.options.disabled) {
          component.options.required = false
          if (Array.isArray(component.rules)) component.rules = component.rules.filter(rule => !rule || !rule.required)
        }
      }
      for (const children of componentLists(component)) visit(children)
    }
  }
  visit(template.list)
  const config = template.config || {}
  if (config.beforeSubmitAndDraft || config.beforeSubmit || config.eventScript) {
    unsupported.add('表单业务提交钩子：配置阶段禁止执行外部业务写入')
  }
  return { template, unsupported: [...unsupported] }
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
