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

// pathSegments 统一解码目标路径并保留层级，供写语义优先判断。
function pathSegments (pathname) {
  try {
    return decodeURIComponent(pathname).split('/').filter(Boolean).map(segment => segment.toLowerCase())
  } catch (_) {
    return String(pathname).split('/').filter(Boolean).map(segment => segment.toLowerCase())
  }
}

// classifyTargetRequest 返回当前启发式策略的判定及命中原因，供影子覆盖率统计使用，不改变现有放行规则。
export function classifyTargetRequest (method, pathname) {
  const normalizedMethod = String(method || 'GET').toUpperCase()
  const segments = pathSegments(pathname)
  if (EXPLICIT_FORBIDDEN_PATHS.some(pattern => pattern.test(pathname))) return { allowed: false, reason: 'explicit_forbidden' }
  if (segments.some(segment => WRITE_SEGMENT_PREFIXES.some(prefix => segment.includes(prefix)))) return { allowed: false, reason: 'write_segment' }
  if (SAFE_HTTP_METHODS.has(normalizedMethod)) return { allowed: true, reason: 'safe_method' }
  if (normalizedMethod !== 'POST') return { allowed: false, reason: 'unsupported_method' }
  const action = segments[segments.length - 1] || ''
  const readAction = READ_SEGMENT_PREFIXES.some(prefix => action.startsWith(prefix)) ||
    READ_SEGMENT_SUFFIXES.some(suffix => action.endsWith(suffix))
  return readAction ? { allowed: true, reason: 'read_action' } : { allowed: false, reason: 'unknown_post' }
}

// targetRequestAllowed 只允许浏览器安全方法和具备明确查询动词的 POST。
// 目标平台大量查询使用 POST，因此不能按方法一刀切；反过来，未命中查询语义的 POST 也不能因路径未知而静默放行。
export function targetRequestAllowed (method, pathname) {
  return classifyTargetRequest(method, pathname).allowed
}

// canonicalTargetPath 把 /web 与网关重写后的 /api/web 收敛为同一清单键，并丢弃查询串。
function canonicalTargetPath (rawPath) {
  let pathname = String(rawPath || '').trim()
  try {
    pathname = new URL(pathname, 'http://manifest.local').pathname
  } catch (_) {
    pathname = pathname.split(/[?#]/, 1)[0]
  }
  pathname = `/${pathname.replace(/^\/+/, '')}`
  return pathname.replace(/^\/api(?=\/web(?:\/|$))/i, '')
}

// manifestPathMatches 支持静态路径、:id、{id} 与单段 * 占位，避免清单为了实例标识退化为宽泛前缀。
function manifestPathMatches (manifestPath, requestPath) {
  const pattern = canonicalTargetPath(manifestPath)
    .split('/')
    .map(segment => segment === '*' || /^:[^/]+$/.test(segment) || /^\{[^/]+\}$/.test(segment)
      ? '[^/]+'
      : segment.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
    .join('/')
  return new RegExp(`^${pattern}/?$`, 'i').test(canonicalTargetPath(requestPath))
}

// normalizeReadRequestManifest 只保留方法与目标路径完整的清单项，重复项不扩大权限。
function normalizeReadRequestManifest (manifest) {
  const seen = new Set()
  const normalized = []
  for (const entry of Array.isArray(manifest) ? manifest : []) {
    const method = String(entry && entry.method || 'GET').toUpperCase()
    const pathname = canonicalTargetPath(entry && (entry.path || entry.pathname))
    if (!targetPath(pathname) || !['GET', 'HEAD', 'OPTIONS', 'POST'].includes(method)) continue
    const key = `${method}\x00${pathname}`
    if (seen.has(key)) continue
    seen.add(key)
    normalized.push({ method, pathname, source: String(entry && entry.source || '') })
  }
  return normalized
}

// classifyRequestWithManifest 对非空清单默认拒绝未列出的目标请求；空清单仅保留有诊断的过渡启发式判定。
export function classifyRequestWithManifest (method, pathname, manifest) {
  const normalizedMethod = String(method || 'GET').toUpperCase()
  const heuristic = classifyTargetRequest(normalizedMethod, pathname)
  if (heuristic.reason === 'explicit_forbidden' || heuristic.reason === 'write_segment' || heuristic.reason === 'unsupported_method') {
    return { ...heuristic, source: 'hard_block', manifestStatus: 'present' }
  }
  const entries = normalizeReadRequestManifest(manifest)
  if (entries.length === 0) {
    return { ...heuristic, source: 'transition_heuristic', manifestStatus: 'empty' }
  }
  const matched = entries.find(entry => entry.method === normalizedMethod && manifestPathMatches(entry.pathname, pathname))
  if (!matched) return { allowed: false, reason: 'manifest_miss', source: 'read_manifest', manifestStatus: 'present' }
  return { allowed: true, reason: 'manifest_allow', source: matched.source || 'read_manifest', manifestStatus: 'present' }
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
  // 模板中的目标接口可能固化任意历史网关；凡 /web 路径都必须收敛到本次核实的会话网关，不能因旧来源地址绕过只读清单。
  return rewriteTargetPath(resolved, base)
}

// requestPolicyIssue 把策略缺口投影为不含查询串、正文和 SID 的结构化诊断。
function requestPolicyIssue (code, status, message, pathname = '', canRetry = true) {
  return {
    code, status, source: 'request_policy', fieldPath: '', fieldLabel: '', operator: '',
    expected: '只读请求清单命中', actual: canonicalTargetPath(pathname), relatedFields: [], message, canRetry
  }
}

// targetURL 将目标地址改写到当前会话网关并保留 SID；请求策略只记录判定，不阻断目标组件请求。
function targetURL (raw, method, baseURL, sid, readRequestManifest, onDecision, onIssue, shadowContext) {
  // 目标表单组件大量使用 /web/... 相对地址；独立 iframe 必须把这类请求解析到本次后端核实的目标网关，
  // 但不能把运行时自身的脚本、样式或其他第三方请求错误改写到目标平台。
  const url = resolveURL(raw, baseURL)
  if (!baseURL) return url
  const base = new URL(baseURL)
  if (url.origin !== base.origin) return url
  const decision = classifyRequestWithManifest(method, url.pathname, readRequestManifest)
  // 目标组件的查询接口并不总能被静态清单提前收集；配置阶段必须让目标运行时自行完成请求，
  // 因而这里把清单判定作为观察数据保留，不能再把未命中项转换成异常。
  const forwardedDecision = decision.allowed
    ? decision
    : { ...decision, allowed: true, reason: 'passthrough', source: 'passthrough' }
  if (typeof onDecision === 'function') {
    try {
      // 影子记录只使用无查询串路径，禁止把 SID、请求正文或业务响应带入诊断。
      onDecision({ ...(shadowContext || {}), method: String(method || 'GET').toUpperCase(), pathname: url.pathname, ...forwardedDecision })
    } catch (_) {
      // 诊断观察器不能影响真实请求判定。
    }
  }
  if (sid) url.searchParams.set('sid', sid)
  return url
}

// installReadOnlyRequestPolicy 在会话内给目标请求附加 SID并改写网关地址；请求分类仅用于观察，不阻断目标请求。
// 返回的清理函数会恢复原生网络对象，SID 因而只存在于本 iframe 当前会话闭包中。
export function installReadOnlyRequestPolicy ({ sid, baseURL, readRequestManifest, onDecision, onIssue, shadowContext }) {
  const originalOpen = XMLHttpRequest.prototype.open
  const originalSend = XMLHttpRequest.prototype.send
  const originalFetch = window.fetch
  const targetOrigin = baseURL ? new URL(baseURL).origin : ''
  const normalizedManifest = normalizeReadRequestManifest(readRequestManifest)

  if (baseURL && normalizedManifest.length === 0 && typeof onIssue === 'function') {
    try {
      onIssue(requestPolicyIssue('request_manifest_empty', 'partial', '只读请求清单为空，当前会话使用过渡启发式判定'))
    } catch (_) {
      // 诊断观察器不能阻止会话装载。
    }
  }

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
    const resolved = targetURL(url, method, baseURL, sid, normalizedManifest, onDecision, onIssue, shadowContext)
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
    const resolved = targetURL(raw, method, baseURL, sid, normalizedManifest, onDecision, onIssue, shadowContext)
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
