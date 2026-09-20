// 加签人员排除名单 API 封装。
// 排除名单里的人员不会被一键配置的加签/移交随机选中，用于规避「目录里存在但无法登录目标平台」
// 的人员被选为被加签人后，运行到其审批节点时因无法登录而安全停机的整条路径失败。

// AddSignExclusionItem 是人员排除列表的统一项：手动名单与系统自动排除合并。
// source: manual（手动，可删除）| auto（系统默认排除，只读）。
// autoReason 仅 auto 时有值：disabled（账号已停用）| no_platform_role（未分配当前平台权限）。
export interface AddSignExclusionItem {
  personId: string
  personName: string
  personPhone: string
  reason: string
  source: 'manual' | 'auto'
  autoReason?: string
  updatedAt?: string
}

export interface AddSignExclusionListResult {
  items: AddSignExclusionItem[]
  total: number
  manualTotal: number
  autoTotal: number
}

export interface AddSignExclusionInput {
  personId: string
  personName: string
  personPhone?: string
  reason?: string
}

// AddSignCandidate 是按姓名检索目标人员目录的结果项。enabled=false 表示该账号在目标平台已停用，
// 停用账号无法登录，是「加签后运行停机」的主因，检索结果里会重点标出。
export interface AddSignCandidate {
  personId: string
  personName: string
  phone: string
  enabled: boolean
}

export interface AddSignCandidateResult {
  items: AddSignCandidate[]
  total: number
}

interface SuccessEnvelope<T> { success: true, data: T }
interface FailureEnvelope { success: false, error: { code: string, message: string } }

// exclusionRequest 统一解析排除名单 API，保留后端稳定错误文案。
async function exclusionRequest<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...init.headers } })
  const envelope = await response.json() as SuccessEnvelope<T> | FailureEnvelope
  if (!response.ok || !envelope.success) throw new Error((envelope as FailureEnvelope).error?.message || '排除名单服务不可用')
  return envelope.data
}

// fetchAddSignExclusions 读取排除名单，可选按姓名模糊过滤。
export function fetchAddSignExclusions(name?: string, signal?: AbortSignal) {
  const query = name && name.trim() ? `?name=${encodeURIComponent(name.trim())}` : ''
  return exclusionRequest<AddSignExclusionListResult>(`/api/settings/add-sign-exclusion${query}`, { method: 'GET' }, signal)
}

// addAddSignExclusion 新增或更新一条排除记录（personId 相同则更新）。返回值前端不消费，用宽松类型。
export function addAddSignExclusion(input: AddSignExclusionInput) {
  return exclusionRequest<Record<string, unknown>>('/api/settings/add-sign-exclusion', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

// removeAddSignExclusion 按 personId 删除一条排除记录。
export function removeAddSignExclusion(personId: string) {
  return exclusionRequest<{ message: string }>(`/api/settings/add-sign-exclusion/${encodeURIComponent(personId)}`, {
    method: 'DELETE',
  })
}

// searchAddSignCandidates 按姓名检索目标人员目录，供点选加入排除名单。
export function searchAddSignCandidates(name: string, signal?: AbortSignal) {
  const query = name.trim() ? `?name=${encodeURIComponent(name.trim())}` : ''
  return exclusionRequest<AddSignCandidateResult>(`/api/settings/add-sign-exclusion/candidates${query}`, { method: 'GET' }, signal)
}
