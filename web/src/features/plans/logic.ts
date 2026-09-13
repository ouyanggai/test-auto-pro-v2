import type {
  PlanFilters,
  PlanRow,
  PlanStatus,
} from './types.ts'

export const planStatusLabels: Record<PlanStatus, string> = {
  not_started: '未运行',
  running: '运行中',
  // F-032：completed 表示“已有运行记录”而非“运行成功”，最近一次运行也可能是失败、停止或结果待确认。
  completed: '已运行',
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
