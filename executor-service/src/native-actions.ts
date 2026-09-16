const BUSINESS_ACTIONS = ['save_draft', 'submit', 'storage_form_data', 'approve', 'reject', 'resubmit', 'forward'] as const
export type BusinessAction = (typeof BUSINESS_ACTIONS)[number]
const CHOICE_ERRORS = ['custom_choose', 'parallel_choose', 'run_node_choose'] as const

export class ActionBlockedError extends Error {
  constructor (message: string) {
    super(message)
    this.name = 'ActionBlockedError'
  }
}

export interface NativeActionHost {
  sumbitFlow?: (kind: 'draft' | 'submit') => unknown
  storageFormData?: () => unknown
  formMakingFormBusiness?: (kind: 'pass' | 'no_pass') => unknown
  reSubmit?: () => unknown
  confirmTranspond?: () => unknown
  handleSaveBranchChooseNode?: () => unknown
  handleSaveParallelChooseNode?: () => unknown
  getSelectPerson?: (payload: unknown) => unknown
}

export interface ChoiceState {
  continued: boolean
}

export function dispatchNativeAction (host: NativeActionHost, action: string): unknown {
  switch (action) {
    case 'save_draft':
      return host.sumbitFlow?.('draft')
    case 'submit':
      return host.sumbitFlow?.('submit')
    case 'storage_form_data':
      return host.storageFormData?.()
    case 'approve':
      return host.formMakingFormBusiness?.('pass')
    case 'reject':
      return host.formMakingFormBusiness?.('no_pass')
    case 'resubmit':
      return host.reSubmit?.()
    case 'forward':
      return host.confirmTranspond?.()
    default:
      throw new ActionBlockedError(`未知动作 ${action}`)
  }
}

export function canContinueChoice (errorType: string, state: ChoiceState, complete: boolean): boolean {
  if (!CHOICE_ERRORS.includes(errorType as typeof CHOICE_ERRORS[number])) return false
  if (!complete) throw new ActionBlockedError('选择数据不完整，禁止继续')
  if (state.continued) throw new ActionBlockedError('选择继续只能发送一次')
  state.continued = true
  return true
}

export function dispatchChoiceContinue (host: NativeActionHost, errorType: string, payload?: unknown): unknown {
  if (errorType === 'custom_choose') return host.handleSaveBranchChooseNode?.()
  if (errorType === 'parallel_choose') return host.handleSaveParallelChooseNode?.()
  if (errorType === 'run_node_choose') return host.getSelectPerson?.(payload)
  throw new ActionBlockedError(`不是选择错误：${errorType}`)
}

export { BUSINESS_ACTIONS, CHOICE_ERRORS }
