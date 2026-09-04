const values = new Map()
let restoreStorageFacade = null
const STORAGE_PREFIX = 'invest-power-system-'

// normalize 把 rsh-flow-components 预期的 storage 值收敛为字符串，不落浏览器长期存储。
function normalize (value) {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : JSON.stringify(value)
}

// targetStorageKey 保持 rsh-flow-components 原 auth.js 的命名空间，兼容模板直接访问完整 localStorage 键。
function targetStorageKey (name) {
  return `${STORAGE_PREFIX}${String(name)}`
}

// localstorageSet 保持目标源码原有调用接口，但只写当前 iframe 内存。
export function localstorageSet (name, content) {
  if (!name) return
  values.set(targetStorageKey(name), normalize(content))
}

// localstorageGet 从当前 iframe 内存读取认证与组件上下文。
export function localstorageGet (name) {
  return values.get(targetStorageKey(name)) || ''
}

// localstorageRemove 删除当前 iframe 内存键。
export function localstorageRemove (name) {
  values.delete(targetStorageKey(name))
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
  const target = {}
  // Storage 的方法不可枚举；模板遍历 localStorage 时只能看到会话数据键，不能把 facade 方法误当成业务上下文。
  Object.defineProperties(target, {
    length: { configurable: true, get: () => values.size },
    key: { configurable: true, value: index => [...values.keys()][Number(index)] || null },
    getItem: { configurable: true, value: key => values.has(String(key)) ? values.get(String(key)) : null },
    setItem: { configurable: true, value: (key, value) => { values.set(String(key), normalize(value)) } },
    removeItem: { configurable: true, value: key => { values.delete(String(key)) } },
    clear: { configurable: true, value: () => values.clear() }
  })
  // 目标模板存在 for...in localStorage 的既有写法；Proxy 只暴露当前内存会话的目标命名空间键，不接触浏览器持久化存储。
  const facade = new Proxy(target, {
    ownKeys (object) { return [...Reflect.ownKeys(object), ...values.keys()] },
    getOwnPropertyDescriptor (object, key) {
      if (values.has(String(key))) return { configurable: true, enumerable: true, writable: true, value: values.get(String(key)) }
      return Reflect.getOwnPropertyDescriptor(object, key)
    },
    get (object, key, receiver) {
      if (values.has(String(key))) return values.get(String(key))
      return Reflect.get(object, key, receiver)
    },
    set (object, key, value, receiver) {
      if (typeof key === 'string' && key.startsWith(STORAGE_PREFIX)) {
        values.set(key, normalize(value))
        return true
      }
      return Reflect.set(object, key, value, receiver)
    },
    deleteProperty (object, key) {
      if (values.has(String(key))) return values.delete(String(key))
      return Reflect.deleteProperty(object, key)
    }
  })
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
