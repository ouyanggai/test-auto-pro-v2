import type {
  PathConfigActionConfiguration,
  PathConfiguration,
  PathConfigurationDataInput,
  PathConfigurationDataResult,
  PathConfigurationDataWorkspace,
  PathConfigurationRouteChange,
  PathActionConfigurationInput,
  PathActionConfigurationResult,
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
    keyFields: Array.isArray(value?.keyFields)
      ? value.keyFields.map(field => ({ ...field, fillNodeName: typeof field?.fillNodeName === 'string' ? field.fillNodeName : '', fillableAtStart: field?.fillableAtStart === true }))
      : [],
    // 节点视图只在服务端按目标节点声明生成；前端不猜权限，缺失时退化为只有发起人视图。
    nodeViews: Array.isArray(value?.nodeViews)
      ? value.nodeViews.map(view => ({
        nodeName: String(view?.nodeName ?? ''),
        isInitiator: view?.isInitiator === true,
        permissions: Array.isArray(view?.permissions)
          ? view.permissions.map(permission => ({ field: String(permission?.field ?? ''), power: permission?.power === 'edit' || permission?.power === 'hide' ? permission.power : 'only_read' }))
          : [],
      }))
      : [],
  }
}

// normalizePathConfigurationDataResult 保留服务端换路结果的路径和原始值，不合并旧表单状态。
function normalizePathConfigurationDataResult(value: PathConfigurationDataResult): PathConfigurationDataResult {
  return { ...normalizePathConfigurationData(value as unknown as PathConfigurationDataWorkspace), routeChanged: value?.routeChanged === true, requiresConfirmation: value?.requiresConfirmation === true, confirmationToken: typeof value?.confirmationToken === 'string' ? value.confirmationToken : '', routeChange: value?.routeChange ?? null }
}

// normalizePathConfiguration 在 API 边界补齐节点数组，表单运行时数据由独立工作区接口承载。
function normalizePathConfiguration(value: PathConfiguration): PathConfiguration {
  return {
    ...value,
    groups: Array.isArray(value?.groups) ? value.groups : [],
    warnings: Array.isArray(value?.warnings) ? value.warnings : [],
    instanceActionKey: typeof value?.instanceActionKey === 'string' ? value.instanceActionKey : '',
    instanceActions: normalizePathActionConfiguration(value?.instanceActions),
  }
}

// normalizePathActionConfiguration 补齐动作容器数组，缺失目录时页面显示空目录而不是崩溃。
function normalizePathActionConfiguration(value: PathConfigActionConfiguration | undefined): PathConfigActionConfiguration {
  return {
    ...(value?.base ? { base: value.base } : {}),
    catalog: Array.isArray(value?.catalog) ? value.catalog : [],
    actions: Array.isArray(value?.actions) ? value.actions : [],
    affected: value?.affected === true,
    note: typeof value?.note === 'string' ? value.note : '',
  }
}

// fetchPathCompiledScenario 只读取服务端编译的完整场景步骤，浏览器不提交任何步骤正文。
export function fetchPathCompiledScenario(planId: string, pathId: string, signal?: AbortSignal): Promise<PathActionConfigurationResult> {
  return request<PathActionConfigurationResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/compiled-scenario`, { method: 'GET' }, signal).then(normalizePathActionConfigurationResult)
}

// normalizePathActionConfigurationResult 补齐编译预览数组，保存返回和只读预览共用同一投影。
function normalizePathActionConfigurationResult(value: PathActionConfigurationResult): PathActionConfigurationResult {
  return {
    ...value,
    actions: Array.isArray(value?.actions) ? value.actions : [],
    compiledScenario: Array.isArray(value?.compiledScenario) ? value.compiledScenario : [],
    issues: Array.isArray(value?.issues) ? value.issues : [],
  }
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
  }).then(normalizePathActionConfigurationResult)
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
