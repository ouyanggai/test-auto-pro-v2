const FORBIDDEN_WRITE_PATHS = [
  /\/web\/flowInstanceApi\/(submit|draft|save)/i,
  /\/web\/flowOperate(?:Api|Service)?\//i,
  /\/web\/flowNodeProxyApi\/(save|update|delete)/i,
  /\/web\/formDataApi\/(save|update|delete)/i
]

// targetURL 只对当前会话核实的目标网关请求附加 SID，其他源保持浏览器默认行为。
function targetURL (raw, baseURL, sid) {
  const text = String(raw)
  // 目标表单组件大量使用 /web/... 相对地址；独立 iframe 必须把这类请求解析到本次后端核实的目标网关，
  // 但不能把运行时自身的脚本、样式或其他第三方请求错误改写到目标平台。
  const useTargetBase = Boolean(baseURL) && /^\/?web\//i.test(text)
  const targetRequest = useTargetBase
    ? `${String(baseURL).replace(/\/$/, '')}/${text.replace(/^\//, '')}`
    : text
  const url = new URL(targetRequest, window.location.href)
  if (!baseURL) return url
  const base = new URL(baseURL)
  if (url.origin !== base.origin) return url
  if (FORBIDDEN_WRITE_PATHS.some(pattern => pattern.test(url.pathname))) {
    throw new Error('F-007 配置阶段禁止调用目标流程或业务写接口')
  }
  if (sid) url.searchParams.set('sid', sid)
  return url
}

// installReadOnlyRequestPolicy 在会话内给目标读取请求附加 SID，并阻断已知流程/业务写端点。
// 返回的清理函数会恢复原生网络对象，SID 因而只存在于本 iframe 当前会话闭包中。
export function installReadOnlyRequestPolicy ({ sid, baseURL }) {
  const originalOpen = XMLHttpRequest.prototype.open
  const originalSend = XMLHttpRequest.prototype.send
  const originalFetch = window.fetch
  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    const resolved = targetURL(url, baseURL, sid)
    this.__f007TargetRequest = baseURL && resolved.origin === new URL(baseURL).origin
    return originalOpen.call(this, method, resolved.toString(), ...rest)
  }
  XMLHttpRequest.prototype.send = function (body) {
    if (this.__f007TargetRequest && sid) this.setRequestHeader('sid', sid)
    return originalSend.call(this, body)
  }
  window.fetch = function (input, init) {
    const raw = typeof input === 'string' || input instanceof URL ? input : input.url
    const resolved = targetURL(raw, baseURL, sid)
    const headers = new Headers(init && init.headers || (input instanceof Request ? input.headers : undefined))
    if (baseURL && resolved.origin === new URL(baseURL).origin && sid) headers.set('sid', sid)
    return originalFetch.call(window, resolved.toString(), { ...(init || {}), headers })
  }
  return () => {
    XMLHttpRequest.prototype.open = originalOpen
    XMLHttpRequest.prototype.send = originalSend
    window.fetch = originalFetch
  }
}
