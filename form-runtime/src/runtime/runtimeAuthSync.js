import { clearRuntimeAuth, localstorageGet, setRuntimeAuth } from './memoryAuth'

// setupRuntimeAuthSync 禁止上游默认实现把 SID 写入 localStorage；真实认证由版本化 iframe 会话注入。
export function setupRuntimeAuthSync () {}

// getLocationParam 配置工作区不从 URL 暴露 SID，仅保留兼容接口。
export function getLocationParam () { return '' }

// persistRuntimeAuth 把目标组件要求的账号上下文保存在当前 iframe 内存。
export function persistRuntimeAuth (payload = {}) { setRuntimeAuth(payload) }

// clearRuntimeAuthAliases 清除当前会话认证。
export function clearRuntimeAuthAliases () { clearRuntimeAuth() }

// getRuntimeAuth 返回当前内存认证摘要。
export function getRuntimeAuth () {
  return {
    sid: localstorageGet('sid') || localstorageGet('token'),
    customerCode: localstorageGet('customerCode'),
    userId: localstorageGet('userId'),
    companyId: localstorageGet('companyId'),
    companyName: localstorageGet('companyName')
  }
}

// bootstrapRuntimeAuth 兼容上游启动入口，但不会读取浏览器长期存储。
export function bootstrapRuntimeAuth () { return getRuntimeAuth() }
