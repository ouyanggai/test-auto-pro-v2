import type {
  PlanAction,
  PlanFilters,
  PlanFormErrors,
  PlanFormValue,
  PlanRow,
  PlanStatus,
} from './types.ts'

export const planActionByStatus: Record<PlanStatus, PlanAction> = {
  pending_configuration: { label: '继续配置', intent: 'configure' },
  ready: { label: '开始运行', intent: 'start' },
  running: { label: '查看运行', intent: 'view_running' },
  completed: { label: '查看结果', intent: 'view_result' },
}

export function filterPlans(plans: readonly PlanRow[], filters: PlanFilters): PlanRow[] {
  const normalizedName = filters.name.trim().toLocaleLowerCase('zh-CN')

  return plans.filter((plan) => {
    const nameMatches = !normalizedName || plan.name.toLocaleLowerCase('zh-CN').includes(normalizedName)
    const statusMatches = !filters.status || plan.status === filters.status
    return nameMatches && statusMatches
  })
}

export function getPlanAction(status: PlanStatus): PlanAction {
  return planActionByStatus[status]
}

export function shouldShowMaxConcurrency(runMode: PlanFormValue['runMode']): boolean {
  return runMode === 'parallel'
}

export function validatePlanForm(form: PlanFormValue): PlanFormErrors {
  const errors: PlanFormErrors = {}

  if (!form.name.trim()) errors.name = '请输入计划名称'
  if (!form.accountId) errors.accountId = '请选择真实账号'
  if (!form.flowSource) errors.flowSource = '请选择唯一流程来源'
  if (!form.flowId) errors.flowId = '请选择目标流程'
  if (form.runMode === 'parallel' && (!form.maxConcurrency || form.maxConcurrency < 2 || form.maxConcurrency > 20)) {
    errors.maxConcurrency = '并行最大并发数应为 2 至 20'
  }

  return errors
}

export function hasPlanFormErrors(errors: PlanFormErrors): boolean {
  return Object.keys(errors).length > 0
}
