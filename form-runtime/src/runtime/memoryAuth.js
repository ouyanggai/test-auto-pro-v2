const values = new Map()
let restoreStorageFacade = null

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

// installRuntimeStorageFacade 把宿主模板直接访问的 localStorage 限定为当前 iframe 内存，销毁时恢复原生对象。
export function installRuntimeStorageFacade (payload = {}) {
  if (restoreStorageFacade) restoreStorageFacade()
  const descriptor = Object.getOwnPropertyDescriptor(window, 'localStorage')
  const facade = {
    get length () { return values.size },
    key (index) { return [...values.keys()][Number(index)] || null },
    getItem (key) { const value = localstorageGet(key); return value === '' ? null : value },
    setItem (key, value) { localstorageSet(key, value) },
    removeItem (key) { localstorageRemove(key) },
    clear () { values.clear() }
  }
  try {
    Object.defineProperty(window, 'localStorage', { configurable: true, value: facade })
    restoreStorageFacade = () => {
      if (descriptor) Object.defineProperty(window, 'localStorage', descriptor)
      else delete window.localStorage
      restoreStorageFacade = null
    }
  } catch (_) {
    // 浏览器禁止重定义时仍使用模块内存认证，不能写入真实持久化 storage。
    restoreStorageFacade = () => { restoreStorageFacade = null }
  }
  Object.entries(payload).forEach(([key, value]) => localstorageSet(key, value))
  return restoreStorageFacade
}

// clearRuntimeAuth 销毁本次会话全部认证上下文。
export function clearRuntimeAuth () {
	if (restoreStorageFacade) restoreStorageFacade()
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
