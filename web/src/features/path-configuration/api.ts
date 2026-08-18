import type {
  PathConfiguration,
  PathConfigNodeSavePayload,
	PathConfigPresetApplyResult,
	PathConfigPresetPreview,
	PathConfigPresetScope,
  PathConfigSaveResult,
  PathFormConditionBinding,
  PathFormConfiguration,
  PathFormGenerateResult,
  PathFormSampleSummary,
  PathFormRuntimeSession,
} from './types.ts'

interface ApiSuccess<T> {
  success: true
  data: T
}

interface ApiFailure {
  success: false
  error: {
    code: string
    message: string
    retryable: boolean
    details?: Array<{ kind: string, name: string, reason: string }>
  }
}

export class PathConfigApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly details: Array<{ kind: string, name: string, reason: string }>

  constructor(message: string, code = 'PLAN_STORAGE_UNAVAILABLE', retryable = false, details: Array<{ kind: string, name: string, reason: string }> = []) {
    super(message)
    this.name = 'PathConfigApiError'
    this.code = code
    this.retryable = retryable
    this.details = details
  }
}

export async function fetchPathConfiguration(planId: string, pathId: string, signal: AbortSignal): Promise<PathConfiguration> {
  const result = await request<PathConfiguration>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration`, { method: 'GET' }, signal)
  return normalizePathConfiguration(result)
}

// normalizePathConditionBinding 将外部 JSON 的可选条件字段归一为稳定前端模型，避免 null 进入模板表达式。
function normalizePathConditionBinding(value: Partial<PathFormConditionBinding> | null | undefined): PathFormConditionBinding {
  return {
    key: String(value?.key ?? ''),
    nodeName: String(value?.nodeName ?? ''),
    branchName: String(value?.branchName ?? ''),
    expression: String(value?.expression ?? ''),
    fields: Array.isArray(value?.fields) ? value.fields.map(String) : [],
    selected: value?.selected === true,
    locked: value?.locked === true,
    needsReview: value?.needsReview === true,
    verified: value?.verified === true,
  }
}

// normalizePathFormConfiguration 统一配置读取与生成回传的条件绑定、字段规则和提示数组默认值。
function normalizePathFormConfiguration(value: PathFormConfiguration | null | undefined): PathFormConfiguration {
  const form = value ?? {} as PathFormConfiguration
  return {
    ...form,
    conditionBindings: Array.isArray(form.conditionBindings) ? form.conditionBindings.map(normalizePathConditionBinding) : [],
    conditionReviews: Array.isArray(form.conditionReviews) ? form.conditionReviews.map(String) : [],
    fieldRules: Array.isArray(form.fieldRules) ? form.fieldRules.map(rule => ({
      field: String(rule?.field ?? ''),
      disabled: rule?.disabled === true,
      conditionKeys: Array.isArray(rule?.conditionKeys) ? rule.conditionKeys.map(String) : [],
    })) : [],
  }
}

// normalizePathConfiguration 在 API 边界修复条件提示数组，页面只消费归一后的业务模型。
function normalizePathConfiguration(value: PathConfiguration): PathConfiguration {
  return { ...value, form: normalizePathFormConfiguration(value?.form) }
}

// previewPathConfigurationPreset 计算每个节点的随机动作预设结果，不产生任何写入。
export function previewPathConfigurationPreset(planId: string, pathId: string, scope: PathConfigPresetScope): Promise<PathConfigPresetPreview> {
  return request<PathConfigPresetPreview>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/preset/preview`, { method: 'POST', body: JSON.stringify({ scope }) })
}

// applyPathConfigurationPreset 应用随机动作预设，不生成循环或覆盖人工配置。
export function applyPathConfigurationPreset(planId: string, pathId: string, scope: PathConfigPresetScope): Promise<PathConfigPresetApplyResult> {
  return request<PathConfigPresetApplyResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/preset/apply`, { method: 'POST', body: JSON.stringify({ scope }) })
}

// copyPathConfigurationCycles 复制来源路径已保存的循环，服务端会再次核对完整结构签名。
export function copyPathConfigurationCycles(planId: string, targetPathId: string, sourcePathId: string, idempotencyKey: string): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(targetPathId)}/configuration/cycles/copy`, {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ sourcePathId }),
  })
}

// savePathConfigurationNode 只保存当前节点人员和动作，避免一次点击覆盖整条路径。
export function savePathConfigurationNode(
  planId: string,
  pathId: string,
  nodeKey: string,
  revision: number,
  payload: PathConfigNodeSavePayload,
  idempotencyKey: string,
): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/nodes/${encodeURIComponent(nodeKey)}`, {
    method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ revision, ...payload }),
  })
}

// savePathConfigurationSelection 只保存路径是否纳入本次测试，不触发节点动作或目标平台请求。
export function savePathConfigurationSelection(planId: string, pathId: string, revision: number, included: boolean, idempotencyKey: string): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/selection`, {
    method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ revision, included }),
  })
}

// generatePathFormData 请求服务端按当前模板、样本和路径条件生成草稿，不产生保存事实。
export function generatePathFormData(
  planId: string,
  pathId: string,
	seed: number,
	values: Record<string, unknown>,
	manualOverridePaths: string[],
	nextGroup: boolean,
	signal?: AbortSignal,
): Promise<PathFormGenerateResult> {
  return request<PathFormGenerateResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/form/generate`, {
		method: 'POST', body: JSON.stringify({ seed, values, manualOverridePaths, nextGroup }),
	}, signal).then(result => ({
    ...result,
    conditionBindings: Array.isArray(result.conditionBindings) ? result.conditionBindings.map(normalizePathConditionBinding) : [],
    conditionReviews: Array.isArray(result.conditionReviews) ? result.conditionReviews.map(String) : [],
    fieldRules: Array.isArray(result.fieldRules) ? result.fieldRules.map(rule => ({
      field: String(rule?.field ?? ''),
      disabled: rule?.disabled === true,
      conditionKeys: Array.isArray(rule?.conditionKeys) ? rule.conditionKeys.map(String) : [],
    })) : [],
  }))
}

// savePathFormData 保存真实 getValues 返回的完整对象与生成元数据。
export function savePathFormData(
  planId: string,
  pathId: string,
  idempotencyKey: string,
  payload: {
    revision: number
    values: Record<string, unknown>
    seed: number
    generatedFieldPaths: string[]
    manualOverridePaths: string[]
    sampleSummary: PathFormSampleSummary
    validated: boolean
    unsupported: string[]
  },
  signal?: AbortSignal,
): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/form`, {
    method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(payload),
  }, signal)
}

// fetchPathFormRuntimeSession 取得当前账号缓存的短期 SID；调用方只保存在 iframe 会话内。
export function fetchPathFormRuntimeSession(planId: string, pathId: string, signal?: AbortSignal): Promise<PathFormRuntimeSession> {
  return request<PathFormRuntimeSession>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/runtime-session`, { method: 'GET' }, signal)
}

async function request<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      signal,
      headers: { 'Content-Type': 'application/json', ...init.headers },
    })
  }
  catch (error) {
    if (signal?.aborted) throw error
    throw new PathConfigApiError('路径配置服务暂不可用，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new PathConfigApiError('路径配置数据格式异常，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new PathConfigApiError(
      failure.error?.message || '路径配置操作失败，请重试',
      failure.error?.code,
      failure.error?.retryable,
      failure.error?.details || [],
    )
  }
  return envelope.data
}
