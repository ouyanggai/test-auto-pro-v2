import type {
  PathConfiguration,
  PathConfigurationDataInput,
  PathConfigurationDataResult,
  PathConfigurationDataWorkspace,
  PathConfigurationRouteChange,
  PathActionConfigurationInput,
  PathActionConfigurationResult,
  PathConfigSaveResult,
  PathFormConditionBinding,
  PathFormConfiguration,
  RunInputPreflightResult,
  PathFormGenerateResult,
  PathFormSampleSummary,
  PathFormRuntimeSession,
} from './types.ts'

interface ApiSuccess<T> {
  success: true
  data: T
}

// fetchRunInputPreflight 重读当前路径并返回只读运行输入预检，不启动目标流程。
export function fetchRunInputPreflight(planId: string, pathId: string, signal?: AbortSignal): Promise<RunInputPreflightResult> {
  return request<RunInputPreflightResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/run-input/preflight`, { method: 'GET' }, signal)
}

interface ApiFailure {
  success: false
  error: {
    code: string
    message: string
    retryable: boolean
    details?: unknown
  }
}

export class PathConfigApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly details: Array<{ kind: string, name: string, reason: string }>
  readonly routeChange: PathConfigurationRouteChange | null
  readonly confirmationToken: string

  constructor(message: string, code = 'PLAN_STORAGE_UNAVAILABLE', retryable = false, details: unknown = undefined) {
    super(message)
    this.name = 'PathConfigApiError'
    this.code = code
    this.retryable = retryable
    const source = details && typeof details === 'object' && !Array.isArray(details) ? details as Record<string, unknown> : null
    this.routeChange = source?.routeChange && typeof source.routeChange === 'object' ? source.routeChange as PathConfigurationRouteChange : null
    this.confirmationToken = typeof source?.confirmationToken === 'string' ? source.confirmationToken : ''
    this.details = Array.isArray(details) ? details.filter(item => item && typeof item === 'object').map(item => {
      const value = item as Record<string, unknown>
      return { kind: String(value.kind ?? ''), name: String(value.name ?? ''), reason: String(value.reason ?? '') }
    }) : []
  }
}

export async function fetchPathConfiguration(planId: string, pathId: string, signal: AbortSignal): Promise<PathConfiguration> {
  const result = await request<PathConfiguration>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration`, { method: 'GET' }, signal)
  return normalizePathConfiguration(result)
}

// fetchPathConfigurationData 读取目标原始表单数据和复制 runtime 的完整加载协议。
export function fetchPathConfigurationData(planId: string, pathId: string, signal?: AbortSignal): Promise<PathConfigurationDataWorkspace> {
  return request<PathConfigurationDataWorkspace>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/data`, { method: 'GET' }, signal).then(normalizePathConfigurationData)
}

// savePathConfigurationData 保存 runtime 捕获的原始 values，服务端负责重算实际路径与换路门禁。
export function savePathConfigurationData(planId: string, pathId: string, idempotencyKey: string, input: PathConfigurationDataInput, signal?: AbortSignal): Promise<PathConfigurationDataResult> {
  return request<PathConfigurationDataResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/data`, {
    method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(input),
  }, signal).then(normalizePathConfigurationDataResult)
}

// normalizePathConfigurationData 只为模板渲染补齐数组默认值，不改写目标原始 values。
function normalizePathConfigurationData(value: PathConfigurationDataWorkspace): PathConfigurationDataWorkspace {
  return {
    ...value,
    template: value?.template && typeof value.template === 'object' ? value.template : {},
    vuePage: value?.vuePage ?? null,
    permissions: Array.isArray(value?.permissions) ? value.permissions.map(permission => ({ field: String(permission?.field ?? ''), power: permission?.power === 'edit' || permission?.power === 'hide' ? permission.power : 'only_read' })) : [],
    readRequests: Array.isArray(value?.readRequests) ? value.readRequests.map(request => ({ method: String(request?.method ?? 'GET').toUpperCase(), path: String(request?.path ?? ''), source: String(request?.source ?? '') })).filter(request => request.path) : [],
    effectiveFormData: value?.effectiveFormData && typeof value.effectiveFormData === 'object' ? value.effectiveFormData : {},
    branchPatches: Array.isArray(value?.branchPatches) ? value.branchPatches : [],
    runtimeValidation: { accepted: value?.runtimeValidation?.accepted === true, issues: Array.isArray(value?.runtimeValidation?.issues) ? value.runtimeValidation.issues : [] },
    issues: Array.isArray(value?.issues) ? value.issues : [],
  }
}

// normalizePathConfigurationDataResult 保留服务端换路结果的路径和原始值，不合并旧表单状态。
function normalizePathConfigurationDataResult(value: PathConfigurationDataResult): PathConfigurationDataResult {
  return { ...normalizePathConfigurationData(value as unknown as PathConfigurationDataWorkspace), routeChanged: value?.routeChanged === true, requiresConfirmation: value?.requiresConfirmation === true, confirmationToken: typeof value?.confirmationToken === 'string' ? value.confirmationToken : '', routeChange: value?.routeChange ?? null }
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
    ruleVersion: String(form.ruleVersion ?? ''),
    readRequests: Array.isArray(form.readRequests) ? form.readRequests.map(request => ({
      method: String(request?.method ?? 'GET').toUpperCase(),
      path: String(request?.path ?? ''),
      source: String(request?.source ?? ''),
    })).filter(request => request.path) : [],
    conditionBindings: Array.isArray(form.conditionBindings) ? form.conditionBindings.map(normalizePathConditionBinding) : [],
    conditionReviews: Array.isArray(form.conditionReviews) ? form.conditionReviews.map(String) : [],
    fieldRules: Array.isArray(form.fieldRules) ? form.fieldRules.map(rule => ({
      field: String(rule?.field ?? ''),
      disabled: rule?.disabled === true,
      conditionKeys: Array.isArray(rule?.conditionKeys) ? rule.conditionKeys.map(String) : [],
    })) : [],
    renderType: form.renderType === 'vue_custom' || form.renderType === 'unknown' ? form.renderType : 'formmaking',
    vuePage: form.vuePage ? {
      status: form.vuePage.status === 'complete' || form.vuePage.status === 'partial' ? form.vuePage.status : 'blocked',
      pageName: String(form.vuePage.pageName ?? ''),
      componentName: String(form.vuePage.componentName ?? ''),
      route: String(form.vuePage.route ?? ''),
      fields: Array.isArray(form.vuePage.fields) ? form.vuePage.fields.map(field => ({
        path: String(field?.path ?? ''), name: String(field?.name ?? ''), valueType: String(field?.valueType ?? 'input'),
        valueShape: String(field?.valueShape ?? 'unknown'), serialization: String(field?.serialization ?? 'runtime_value'),
        required: field?.required === true, readOnly: field?.readOnly === true, hidden: field?.hidden === true,
        disabled: field?.disabled === true, nested: field?.nested === true, collection: field?.collection === true,
        candidateKind: String(field?.candidateKind ?? ''), candidateSource: String(field?.candidateSource ?? ''),
        defaultValue: field?.defaultValue, dataSource: String(field?.dataSource ?? ''),
        format: field?.format ? String(field.format) : undefined,
        validation: Array.isArray(field?.validation) ? field.validation.map(String) : [],
        validationCapability: Array.isArray(field?.validationCapability) ? field.validationCapability.map(String) : [],
        evidence: String(field?.evidence ?? ''),
        options: Array.isArray(field?.options) ? field.options.map(option => ({ label: String(option?.label ?? ''), value: option?.value })) : [],
      })) : [],
      issues: Array.isArray(form.vuePage.issues) ? form.vuePage.issues.map(String) : [],
    } : null,
  }
}

// normalizePathConfiguration 在 API 边界修复条件提示数组，页面只消费归一后的业务模型。
function normalizePathConfiguration(value: PathConfiguration): PathConfiguration {
  return { ...value, form: normalizePathFormConfiguration(value?.form) }
}

// copyPathConfigurationCycles 复制来源路径已保存的循环，服务端会再次核对完整结构签名。
export function copyPathConfigurationCycles(planId: string, targetPathId: string, sourcePathId: string, idempotencyKey: string): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(targetPathId)}/configuration/cycles/copy`, {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ sourcePathId }),
  })
}

// savePathActionConfiguration 只提交 F-012 独立动作记录，由服务端重编译同一主实例场景。
export function savePathActionConfiguration(
  planId: string,
  pathId: string,
  nodeKey: string,
  payload: PathActionConfigurationInput,
  idempotencyKey: string,
): Promise<PathActionConfigurationResult> {
  return request<PathActionConfigurationResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/nodes/${encodeURIComponent(nodeKey)}`, {
    method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(payload),
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
    generationState: result.generationState || 'blocked',
    issues: Array.isArray(result.issues) ? result.issues.map(issue => ({
      field: String(issue?.field ?? '表单数据'),
      reason: String(issue?.reason ?? '需要人工核对'),
      blocking: issue?.blocking === true,
    })) : [],
    routeVerification: {
      matched: result.routeVerification?.matched === true,
      reason: String(result.routeVerification?.reason ?? ''),
    },
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
