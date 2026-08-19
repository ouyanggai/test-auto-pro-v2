export interface TemplateRuleCatalogSummary {
  total: number
  formmaking: number
  vueCustom: number
  unknown: number
  complete: number
  needsAttention: number
  failed: number
  components: Record<string, number>
  updatedAt?: string
}

export interface TemplateRuleCatalogItem {
  id: number
  flowCode: string
  flowName: string
  templateType: string
  renderType: 'formmaking' | 'vue_custom' | 'unknown'
  status: 'complete' | 'needs_attention' | 'failed'
  sourceVersion: string
  issues: string[]
  analyzedAt?: string
}

export interface TemplateRuleAnalysisJob {
  id: string
  mode: 'incremental' | 'full' | 'retry'
  account: string
  status: 'queued' | 'running' | 'completed' | 'failed'
  total: number
  processed: number
  completed: number
  needsAttention: number
  failed: number
  message?: string
  updatedAt: string
}

interface ApiSuccess<T> { success: true; data: T }
interface ApiFailure { success: false; error?: { message?: string } }

// fetchTemplateRuleSummary 读取本地目录汇总，绝不触发目标平台模板扫描。
export async function fetchTemplateRuleSummary(signal?: AbortSignal): Promise<TemplateRuleCatalogSummary> {
  return request<TemplateRuleCatalogSummary>('/api/settings/template-rules/summary', { method: 'GET' }, signal)
}

// fetchTemplateRuleCatalog 按页读取规则摘要，页面不接收规则正文或宿主源码。
export async function fetchTemplateRuleCatalog(page: number, size: number, query = '', signal?: AbortSignal): Promise<{ items: TemplateRuleCatalogItem[]; total: number }> {
  const params = new URLSearchParams({ page: String(page), size: String(size) })
  if (query.trim()) params.set('query', query.trim())
  return request<{ items: TemplateRuleCatalogItem[]; total: number }>(`/api/settings/template-rules?${params.toString()}`, { method: 'GET' }, signal)
}

// createTemplateRuleAnalysis 只允许创建固定三种本地目录分析任务。
export async function createTemplateRuleAnalysis(account: string, mode: TemplateRuleAnalysisJob['mode']): Promise<TemplateRuleAnalysisJob> {
  return request<TemplateRuleAnalysisJob>('/api/settings/template-rules/jobs', { method: 'POST', body: JSON.stringify({ account, mode }) })
}

// fetchLatestTemplateRuleAnalysis 在页面刷新后恢复当前账号最近一次任务状态。
export async function fetchLatestTemplateRuleAnalysis(account: string, signal?: AbortSignal): Promise<TemplateRuleAnalysisJob | null> {
  return request<TemplateRuleAnalysisJob | null>(`/api/settings/template-rules/jobs/latest?account=${encodeURIComponent(account)}`, { method: 'GET' }, signal)
}

// fetchTemplateRuleAnalysis 读取后台分析任务的真实持久化进度。
export async function fetchTemplateRuleAnalysis(jobID: string, signal?: AbortSignal): Promise<TemplateRuleAnalysisJob> {
  return request<TemplateRuleAnalysisJob>(`/api/settings/template-rules/jobs/${encodeURIComponent(jobID)}`, { method: 'GET' }, signal)
}

// request 统一处理目录 API 的业务失败，不把目标平台地址或内部规则透传到页面。
async function request<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...(init.headers || {}) } })
  const body = await response.json() as ApiSuccess<T> | ApiFailure
  if (!response.ok || body.success !== true) throw new Error(('error' in body && body.error?.message) || '模板规则目录暂时不可用，请重试')
  return body.data
}
