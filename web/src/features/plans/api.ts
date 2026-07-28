import type {
  DueFlowCandidate,
  FlowCandidate,
  FlowSource,
  FlowTemplateCandidate,
  SubmittedFlowCandidate,
  VerifiedTargetAccount,
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
  }
}

type ApiEnvelope<T> = ApiSuccess<T> | ApiFailure

export class TargetApiError extends Error {
  readonly code?: string
  readonly retryable: boolean
  readonly status: number

  constructor(message: string, options: { code?: string, retryable?: boolean, status?: number } = {}) {
    super(message)
    this.name = 'TargetApiError'
    this.code = options.code
    this.retryable = options.retryable ?? false
    this.status = options.status ?? 0
  }
}

interface VerifyResponse {
  verified: boolean
  account: VerifiedTargetAccount
}

interface PageResponse<T> {
  account: string
  items: T[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
}

interface TemplateDTO {
  id: string
  flowName: string
  code: string
  groupName: string
  flowStatus: string
  statusText: string
	typeName: string
	updateDate: string
	createDate: string
	remark: string
	flowCreateType: string
	formExist: string
	formTemplateCount: number
}

interface SubmittedDTO {
  id: string
  name: string
  formName: string
  title: string
	status: string
	statusName: string
	createDate: string
  currentNodeName: string
  currentAuditUserNames: string
}

interface DueDTO {
  flowInstanceId: string
  flowInstanceName: string
  formName: string
  title: string
  flowStatus: string
  statusName: string
  initiator: string
  initiatorDate: string
}

export interface CandidatePage {
  account: string
  items: FlowCandidate[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
}

const errorMessages: Record<string, string> = {
	INVALID_ARGUMENT: '请求参数不正确',
	TARGET_CONFIG_MISSING: '服务配置不完整，请联系维护人员',
	TARGET_LOGIN_REJECTED: '账号验证失败，请核对账号',
	TARGET_SESSION_EXPIRED: '账号验证已失效，请重新验证',
	TARGET_RESPONSE_INVALID: '流程数据异常，请重试',
	TARGET_UNAVAILABLE: '暂时无法读取流程，请重试',
	TARGET_TIMEOUT: '读取流程超时，请重试',
}

const submittedStatusNames: Record<string, string> = {
	await_sent: '待发',
	run: '审批中',
	withdraw: '撤销',
	termination: '终止',
	abandon: '丢弃',
	rejected: '驳回',
	end: '完结',
	draft: '草稿',
}

export function submittedStatusName(status: string): string {
	return submittedStatusNames[status] || '状态未知'
}

export function targetApiErrorMessage(code?: string): string {
	return code ? errorMessages[code] || '请求失败，请重试' : '请求失败，请重试'
}

async function request<T>(path: string, init: RequestInit, signal: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...init.headers } })
  }
  catch (error) {
    if (signal.aborted) throw error
		throw new TargetApiError('服务暂不可用，请稍后重试', { code: 'TARGET_UNAVAILABLE', retryable: true })
  }

  let envelope: ApiEnvelope<T>
  try {
    envelope = await response.json() as ApiEnvelope<T>
  }
  catch {
		throw new TargetApiError('服务返回数据异常，请重试', { code: 'TARGET_RESPONSE_INVALID', retryable: true, status: response.status })
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
		throw new TargetApiError(targetApiErrorMessage(failure.error?.code), {
      code: failure.error?.code,
      retryable: failure.error?.retryable,
      status: response.status,
    })
  }
  return envelope.data
}

export async function verifyTargetAccount(account: string, signal: AbortSignal): Promise<VerifiedTargetAccount> {
  const result = await request<VerifyResponse>('/api/target/accounts/verify', {
    method: 'POST',
    body: JSON.stringify({ account }),
  }, signal)
  return result.account
}

export async function fetchTargetCandidates(params: {
  account: string
  source: FlowSource
  query: string
  page: number
  pageSize: number
  signal: AbortSignal
}): Promise<CandidatePage> {
  const search = new URLSearchParams({
    account: params.account,
    query: params.query,
    page: String(params.page),
    pageSize: String(params.pageSize),
  })
  if (params.source === 'new') {
    const page = await request<PageResponse<TemplateDTO>>(`/api/target/flow-templates?${search}`, { method: 'GET' }, params.signal)
    return {
      ...page,
      items: page.items.map((item): FlowTemplateCandidate => ({
        key: `template:${item.id}`,
        kind: 'template',
        accountName: page.account,
        templateId: item.id,
        flowName: item.flowName,
        typeName: item.typeName,
        groupName: item.groupName,
        statusText: item.statusText || item.flowStatus,
		updateTime: item.updateDate || item.createDate,
		code: item.code,
		remark: item.remark,
			flowCreateType: item.flowCreateType,
			formExist: item.formExist,
			formTemplateCount: Number.isInteger(item.formTemplateCount) && item.formTemplateCount > 0 ? item.formTemplateCount : 0,
		})),
    }
  }

  search.set('source', params.source === 'started' ? 'submitted' : 'due')
  if (params.source === 'started') {
    const page = await request<PageResponse<SubmittedDTO>>(`/api/target/flow-instances?${search}`, { method: 'GET' }, params.signal)
    return {
      ...page,
      items: page.items.map((item): SubmittedFlowCandidate => ({
        key: `submitted:${item.id}`,
        kind: 'submitted',
        accountName: page.account,
        id: item.id,
        name: item.title || item.name || item.formName,
        status: item.status,
		statusName: item.statusName || submittedStatusName(item.status),
        createDate: item.createDate,
        currentNodeName: item.currentNodeName,
        currentAuditUserNames: item.currentAuditUserNames,
      })),
    }
  }

  const page = await request<PageResponse<DueDTO>>(`/api/target/flow-instances?${search}`, { method: 'GET' }, params.signal)
  return {
    ...page,
    items: page.items.map((item): DueFlowCandidate => ({
      key: `due:${item.flowInstanceId}`,
      kind: 'due',
      accountName: page.account,
      flowInstanceId: item.flowInstanceId,
      flowInstanceName: item.title || item.flowInstanceName || item.formName,
      statusName: item.statusName || item.flowStatus,
      initiator: item.initiator,
      initiatorDate: item.initiatorDate,
    })),
  }
}
