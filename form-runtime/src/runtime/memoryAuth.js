const values = new Map()

// normalize 把 rsh-flow-components 预期的 storage 值收敛为字符串，不落浏览器长期存储。
function normalize (value) {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : JSON.stringify(value)
}

// localstorageSet 保持目标源码原有调用接口，但只写当前 iframe 内存。
export function localstorageSet (name, content) {
  if (!name) return
  values.set(String(name), normalize(content))
}

// localstorageGet 从当前 iframe 内存读取认证与组件上下文。
export function localstorageGet (name) {
  return values.get(String(name)) || ''
}

// localstorageRemove 删除当前 iframe 内存键。
export function localstorageRemove (name) {
  values.delete(String(name))
}

// setRuntimeAuth 原子装载本次会话上下文，禁止残留上一计划或账号。
export function setRuntimeAuth (payload = {}) {
  clearRuntimeAuth()
  Object.entries(payload).forEach(([key, value]) => localstorageSet(key, value))
}

// clearRuntimeAuth 销毁本次会话全部认证上下文。
export function clearRuntimeAuth () {
  values.clear()
}

// 以下权限缓存接口兼容目标源码，全部限定在同一内存会话。
export function clearPermissionCache () {}
export function isLoginUserChanged () { return false }
export function clearPermissionCacheOnUserChange () { return false }
export function setOriginPermissionCache (permissions) { localstorageSet('originPermission', permissions) }
export function getOriginPermissionCache () {
  const content = localstorageGet('originPermission')
  if (!content) return null
  try { return JSON.parse(content) } catch (_) { return null }
}
