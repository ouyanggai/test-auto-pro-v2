const SAFE_HTTP_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])
const WRITE_SEGMENT_PREFIXES = [
  'submit', 'draft', 'save', 'update', 'delete', 'remove', 'upload', 'import',
  'create', 'insert', 'modify', 'edit', 'bind', 'unbind', 'operate', 'approve',
  'reject', 'withdraw', 'terminate', 'abandon', 'complete', 'execute', 'publish',
  'cancel', 'trigger', 'send', 'reset', 'copy', 'move', 'rename', 'change',
  'switch', 'close', 'replace'
]
const READ_SEGMENT_PREFIXES = [
  'find', 'get', 'list', 'query', 'search', 'read', 'load', 'fetch', 'select',
  'lookup', 'count', 'check', 'validate', 'preview', 'download', 'statistical',
  'statistics'
]
const READ_SEGMENT_SUFFIXES = ['list', 'tree', 'detail', 'details', 'options', 'children', 'page']
const EXPLICIT_FORBIDDEN_PATHS = [
  /\/web\/flowOperate(?:Api|Service)?(?:\/|$)/i,
  /\/web\/user\/api\/login\/user\/(?:login|loginOut|switchLinkage)(?:\/|$)/i
]
const KNOWN_TARGET_ORIGINS = new Set([
  'http://192.168.1.220:28081',
  'http://192.168.1.220:38081',
  'http://192.168.1.218:8077',
  'https://iserver.runshihua.com'
])

// pathSegments 统一解码目标路径并保留层级，供写语义优先判断。
function pathSegments (pathname) {
  try {
    return decodeURIComponent(pathname).split('/').filter(Boolean).map(segment => segment.toLowerCase())
  } catch (_) {
    return String(pathname).split('/').filter(Boolean).map(segment => segment.toLowerCase())
  }
}

// targetRequestAllowed 只允许浏览器安全方法和具备明确查询动词的 POST。
// 目标平台大量查询使用 POST，因此不能按方法一刀切；反过来，未命中查询语义的 POST 也不能因路径未知而静默放行。
export function targetRequestAllowed (method, pathname) {
  const normalizedMethod = String(method || 'GET').toUpperCase()
  const segments = pathSegments(pathname)
  if (EXPLICIT_FORBIDDEN_PATHS.some(pattern => pattern.test(pathname))) return false
  if (segments.some(segment => WRITE_SEGMENT_PREFIXES.some(prefix => segment.includes(prefix)))) return false
  if (SAFE_HTTP_METHODS.has(normalizedMethod)) return true
  if (normalizedMethod !== 'POST') return false
  const action = segments[segments.length - 1] || ''
  return READ_SEGMENT_PREFIXES.some(prefix => action.startsWith(prefix)) ||
    READ_SEGMENT_SUFFIXES.some(suffix => action.endsWith(suffix))
}

// targetPath 判断请求是否属于目标平台网关的 /web 或 /api/web 路径。
function targetPath (pathname) {
  return /^\/?(?:api\/)?web(?:\/|$)/i.test(String(pathname || ''))
}

// rewriteTargetPath 保持原查询串，把目标 /web 路径收敛到会话网关的 /api 前缀。
function rewriteTargetPath (source, base) {
  const gatewayPrefix = base.pathname.replace(/\/$/, '').endsWith('/api') ? base.pathname.replace(/\/$/, '') : '/api'
  const pathname = source.pathname.replace(/^\/?api(?=\/web(?:\/|$))/i, '').replace(/^\//, '')
  const target = new URL(`${base.origin}${gatewayPrefix}/${pathname}`)
  target.search = source.search
  target.hash = source.hash
  return target
}

// resolveURL 把相对地址和同步源码遗留的目标绝对地址都收敛到当前会话网关，真正第三方资源保持原样。
function resolveURL (raw, baseURL) {
  const text = String(raw)
  if (!baseURL) return new URL(text, window.location.href)
  const base = new URL(baseURL)
  if (/^\/?(?:api\/)?web(?:\/|$)/i.test(text)) {
    return rewriteTargetPath(new URL(text.replace(/^\/?api(?=\/web)/i, ''), base.origin), base)
  }
  const resolved = new URL(text, window.location.href)
  if (!targetPath(resolved.pathname)) return resolved
  // 同步源码的开发环境可能在 axios 初始化时捕获旧 28081；同主机旧端口和已知目标网关都必须改写，不能绕过会话只读策略。
  if (resolved.hostname === base.hostname || KNOWN_TARGET_ORIGINS.has(resolved.origin)) {
    return rewriteTargetPath(resolved, base)
  }
  return resolved
}

// targetURL 只对当前会话核实且已证明只读的目标请求附加 SID，其他源保持浏览器默认行为。
function targetURL (raw, method, baseURL, sid) {
  // 目标表单组件大量使用 /web/... 相对地址；独立 iframe 必须把这类请求解析到本次后端核实的目标网关，
  // 但不能把运行时自身的脚本、样式或其他第三方请求错误改写到目标平台。
  const url = resolveURL(raw, baseURL)
  if (!baseURL) return url
  const base = new URL(baseURL)
  if (url.origin !== base.origin) return url
  if (!targetRequestAllowed(method, url.pathname)) {
    throw new Error('F-007 配置阶段不支持未证明为只读的目标请求')
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
  const targetOrigin = baseURL ? new URL(baseURL).origin : ''

  // withTargetSid 把 SID 并入请求体；目标网关只在请求体携带 SID 时才认可会话（与后端适配层一致）。
  function withTargetSid (body) {
    if (!sid || body === undefined || body === null) return body
    if (typeof body === 'string') {
      try {
        const parsed = JSON.parse(body)
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          parsed.sid = sid
          return JSON.stringify(parsed)
        }
      } catch (_) {
        // 非 JSON 文本保持原样，不能因注入失败破坏原始请求。
      }
      return body
    }
    if (typeof FormData !== 'undefined' && body instanceof FormData) {
      body.set('sid', sid)
      return body
    }
    return body
  }

  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    const resolved = targetURL(url, method, baseURL, sid)
    this.__f007TargetRequest = Boolean(baseURL) && resolved.origin === new URL(baseURL).origin
    return originalOpen.call(this, method, resolved.toString(), ...rest)
  }
  XMLHttpRequest.prototype.send = function (body) {
    if (this.__f007TargetRequest && sid) {
      this.setRequestHeader('sid', sid)
      if (targetOrigin) {
        this.setRequestHeader('origin', targetOrigin)
        this.setRequestHeader('Referer', targetOrigin + '/')
      }
      body = withTargetSid(body)
    }
    return originalSend.call(this, body)
  }
  window.fetch = async function (input, init) {
    const raw = typeof input === 'string' || input instanceof URL ? input : input.url
    const method = (init && init.method) || (input instanceof Request ? input.method : 'GET')
    const resolved = targetURL(raw, method, baseURL, sid)
    const headers = new Headers((init && init.headers) || (input instanceof Request ? input.headers : undefined))
    const nextInit = { ...(init || {}) }
    if (baseURL && resolved.origin === new URL(baseURL).origin && sid) {
      headers.set('sid', sid)
      if (targetOrigin) {
        headers.set('origin', targetOrigin)
        headers.set('Referer', targetOrigin + '/')
      }
      nextInit.body = withTargetSid(init && init.body)
    }
    if (input instanceof Request) {
      const request = new Request(resolved.toString(), input)
      return originalFetch.call(window, request, { ...nextInit, headers })
    }
    return originalFetch.call(window, resolved.toString(), { ...nextInit, headers })
  }
  return () => {
    XMLHttpRequest.prototype.open = originalOpen
    XMLHttpRequest.prototype.send = originalSend
    window.fetch = originalFetch
  }
}
