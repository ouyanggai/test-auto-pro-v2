// parseJsonValue 统一解析 FormMaking 自定义组件的字符串值；空值或历史脏值必须回落到调用方指定的默认值。
export function parseJsonValue (value, fallback = null) {
  if (value === undefined || value === null || value === '') return fallback
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch (_) {
    return fallback
  }
}

// parseJsonObject 将组件配置值规范为对象，避免首屏空字段触发属性读取异常。
export function parseJsonObject (value, fallback = {}) {
  const parsed = parseJsonValue(value, fallback)
  return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : fallback
}

// parseJsonArray 将组件配置值规范为数组，保证多选组件始终可以安全渲染列表。
export function parseJsonArray (value, fallback = []) {
  const parsed = parseJsonValue(value, fallback)
  return Array.isArray(parsed) ? parsed : fallback
}
