import type {
  PlanAction,
  PlanFilters,
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
