const DEFAULT_MAX_DEPTH = 7

// cloneBridgeValue 深复制字段值，避免宿主状态与 iframe envelope 共享引用。
function cloneBridgeValue (value) {
  if (value === undefined || value === null || typeof value !== 'object') return value
  return JSON.parse(JSON.stringify(value))
}

// fieldIssue 创建 Vue 字段桥接器统一结构化问题。
function fieldIssue (code, status, field, operator, expected, actual, message, canRetry = false) {
  return {
    code,
    status,
    source: 'vue_field_bridge',
    fieldPath: String(field && field.path ? field.path : ''),
    fieldLabel: String(field && field.name ? field.name : ''),
    operator,
    expected,
    actual,
    relatedFields: [],
    message,
    canRetry
  }
}

// childInstances 合并 Vue children 与 refs，保持对象身份去重。
function childInstances (instance) {
  const refs = []
  for (const value of Object.values((instance && instance.$refs) || {})) {
    if (Array.isArray(value)) refs.push(...value.filter(Boolean))
    else if (value) refs.push(value)
  }
  return [...new Set([...(Array.isArray(instance && instance.$children) ? instance.$children : []), ...refs])]
}

// collectInstances 有界收集宿主实例树，并记录是否因深度限制截断。
function collectInstances (root, maxDepth) {
  const instances = []
  const visited = new Set()
  let truncated = false
  const visit = (instance, depth) => {
    if (!instance || visited.has(instance)) return
    if (depth > maxDepth) {
      truncated = true
      return
    }
    visited.add(instance)
    instances.push(instance)
    for (const child of childInstances(instance)) visit(child, depth + 1)
  }
  visit(root, 0)
  return { instances, truncated }
}

// resolveBinding 把点路径解析到真实 owner/key，便于按对象身份消除重复视图。
function resolveBinding (input, path) {
  const parts = String(path || '').split('.').filter(Boolean)
  if (!input || typeof input !== 'object' || parts.length === 0) return null
  let owner = input
  for (let index = 0; index < parts.length - 1; index++) {
    owner = owner && typeof owner === 'object' ? owner[parts[index]] : undefined
    if (!owner || typeof owner !== 'object') return null
  }
  const key = parts[parts.length - 1]
  return Object.prototype.hasOwnProperty.call(owner, key) ? { owner, key } : null
}

// instanceStates 返回 HostVuePage 已验证的常见表单状态容器。
function instanceStates (instance) {
  const data = (instance && instance.$data) || {}
  return [...new Set([data, data.form, data.initForm, data.editData, data.params, data.param, data.rawData, data.detail].filter(value => value && typeof value === 'object'))]
}

// discoverBindings 查找字段的精确路径；仅在精确路径不存在时使用唯一叶键回退。
function discoverBindings (instances, path) {
  const exact = []
  const leaf = String(path || '').split('.').filter(Boolean).pop()
  const fallback = []
  for (const instance of instances) {
    for (const state of instanceStates(instance)) {
      const binding = resolveBinding(state, path)
      if (binding) exact.push(binding)
      else if (leaf) {
        const leafBinding = resolveBinding(state, leaf)
        if (leafBinding) fallback.push(leafBinding)
      }
    }
  }
  return uniqueBindings(exact.length > 0 ? exact : fallback)
}

// uniqueBindings 按 owner/key 去重同一状态从父子容器产生的重复视图。
function uniqueBindings (bindings) {
  const owners = new Map()
  const result = []
  for (const binding of bindings) {
    let keys = owners.get(binding.owner)
    if (!keys) {
      keys = new Set()
      owners.set(binding.owner, keys)
    }
    if (keys.has(binding.key)) continue
    keys.add(binding.key)
    result.push(binding)
  }
  return result
}

// readPath 读取 iframe values 中的点路径。
function readPath (input, path) {
  return String(path || '').split('.').filter(Boolean).reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, input)
}

// writePath 写入 iframe values 中的点路径。
function writePath (input, path, value) {
  const parts = String(path || '').split('.').filter(Boolean)
  let current = input
  for (let index = 0; index < parts.length - 1; index++) {
    if (!current[parts[index]] || typeof current[parts[index]] !== 'object') current[parts[index]] = {}
    current = current[parts[index]]
  }
  if (parts.length) current[parts[parts.length - 1]] = value
}

// valueMatchesShape 校验规则目录声明的最小值形态，不执行页面业务脚本。
function valueMatchesShape (value, shape) {
  if (value === undefined || value === null || shape === '' || shape === 'unknown') return true
  if (shape === 'array') return Array.isArray(value)
  if (shape === 'object') return typeof value === 'object' && !Array.isArray(value)
  if (shape === 'number') return typeof value === 'number' && Number.isFinite(value)
  if (shape === 'json_string') {
    if (typeof value !== 'string') return false
    try {
      JSON.parse(value)
      return true
    } catch (_) {
      return false
    }
  }
  if (shape === 'scalar') return typeof value !== 'object'
  return true
}

// fieldBindings 汇总字段绑定及树深度问题，读写两端共享同一发现语义。
function fieldBindings (root, fields, maxDepth) {
  const discovered = collectInstances(root, maxDepth)
  const result = new Map()
  for (const field of fields) result.set(String(field.path || ''), discoverBindings(discovered.instances, field.path))
  return { bindings: result, truncated: discovered.truncated }
}

// bindingIssues 把缺失、歧义和深度截断转换为字段级结构化问题。
function bindingIssues (field, bindings, truncated, operation) {
  const issues = []
  if (bindings.length === 0) {
    issues.push(fieldIssue('vue_field_not_found', 'partial', field, operation, '宿主实例树中唯一可访问字段', undefined, `声明字段无法在宿主实例树中${operation === 'write' ? '写入' : '读取'}`, true))
  } else if (bindings.length > 1) {
    issues.push(fieldIssue('vue_field_ambiguous', 'blocked', field, operation, '唯一字段绑定', bindings.length, '字段路径在宿主实例树中存在多个候选，已阻止猜测读写'))
  }
  if (truncated && bindings.length === 0) {
    issues.push(fieldIssue('vue_field_depth_exceeded', 'partial', field, operation, `实例深度不超过 ${DEFAULT_MAX_DEPTH}`, '实例树已截断', '字段可能位于安全遍历深度之外，需要补充明确桥接规则', true))
  }
  return issues
}

// writeVueFieldValues 按规则目录字段逐一写入宿主实例，任何缺失或歧义均可观测。
export function writeVueFieldValues (root, fields, values, setter, maxDepth = DEFAULT_MAX_DEPTH) {
  const normalizedFields = Array.isArray(fields) ? fields.filter(field => field && field.path) : []
  const discovered = fieldBindings(root, normalizedFields, maxDepth)
  const issues = []
  const writtenFieldPaths = []
  const setValue = typeof setter === 'function' ? setter : (owner, key, value) => { owner[key] = value }
  for (const field of normalizedFields) {
    const bindings = discovered.bindings.get(String(field.path)) || []
    issues.push(...bindingIssues(field, bindings, discovered.truncated, 'write'))
    if (bindings.length !== 1) continue
    const value = readPath(values, field.path)
    if (value === undefined) continue
    if (!valueMatchesShape(value, String(field.valueShape || 'unknown'))) {
      issues.push(fieldIssue('vue_field_shape_mismatch', 'blocked', field, 'write', field.valueShape, Array.isArray(value) ? 'array' : typeof value, '写入值形态与规则目录不一致'))
      continue
    }
    try {
      setValue(bindings[0].owner, bindings[0].key, cloneBridgeValue(value))
      writtenFieldPaths.push(String(field.path))
    } catch (caught) {
      issues.push(fieldIssue('vue_field_write_failed', 'blocked', field, 'write', '可写宿主状态', caught instanceof Error ? caught.message : '写入异常', '字段写入宿主实例失败', true))
    }
  }
  return { issues, writtenFieldPaths: [...new Set(writtenFieldPaths)].sort() }
}

// captureVueFieldValues 按规则目录字段逐一捕获宿主状态并返回完整 values envelope 内核。
export function captureVueFieldValues (root, fields, baseValues, maxDepth = DEFAULT_MAX_DEPTH) {
  const normalizedFields = Array.isArray(fields) ? fields.filter(field => field && field.path) : []
  const discovered = fieldBindings(root, normalizedFields, maxDepth)
  const values = cloneBridgeValue(baseValues || {}) || {}
  const issues = []
  const capturedFieldPaths = []
  for (const field of normalizedFields) {
    const bindings = discovered.bindings.get(String(field.path)) || []
    issues.push(...bindingIssues(field, bindings, discovered.truncated, 'read'))
    if (bindings.length !== 1) continue
    try {
      const value = cloneBridgeValue(bindings[0].owner[bindings[0].key])
      if (!valueMatchesShape(value, String(field.valueShape || 'unknown'))) {
        issues.push(fieldIssue('vue_field_shape_mismatch', 'blocked', field, 'read', field.valueShape, Array.isArray(value) ? 'array' : typeof value, '读取值形态与规则目录不一致'))
        continue
      }
      writePath(values, field.path, value)
      capturedFieldPaths.push(String(field.path))
    } catch (caught) {
      issues.push(fieldIssue('vue_field_read_failed', 'blocked', field, 'read', '可读宿主状态', caught instanceof Error ? caught.message : '读取异常', '字段读取宿主实例失败', true))
    }
  }
  return { values, issues, capturedFieldPaths: [...new Set(capturedFieldPaths)].sort() }
}

// mergeVueFieldIssues 合并 setData 与 capture 问题并保持稳定首次顺序。
export function mergeVueFieldIssues (...groups) {
  const result = []
  const seen = new Set()
  for (const issue of groups.flat()) {
    if (!issue) continue
    const key = [issue.code, issue.source, issue.fieldPath, issue.operator, issue.message].join('\u0000')
    if (seen.has(key)) continue
    seen.add(key)
    result.push(issue)
  }
  return result
}
