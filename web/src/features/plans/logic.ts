import type {
  PlanAction,
  PlanFilters,
  PlanRow,
  PlanStatus,
} from './types.ts'

export const planActionByStatus: Record<PlanStatus, PlanAction> = {
  not_started: { label: '继续配置', intent: 'configure' },
  running: { label: '查看运行', intent: 'view_running' },
  completed: { label: '查看结果', intent: 'view_result' },
}

export const planStatusLabels: Record<PlanStatus, string> = {
  not_started: '未运行',
  running: '运行中',
  completed: '已完成',
}

export const planStatusOptions = Object.entries(planStatusLabels).map(([value, label]) => ({ value, label }))

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
