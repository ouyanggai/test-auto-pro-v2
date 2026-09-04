// F-015 成功断言与运行准备的前端类型，字段与后端 DTO 一一对应，前端不自造取值。

// RunReadinessItem 是一条阻塞或提醒；anchor 让界面能定位到具体面板。
export interface RunReadinessItem {
  kind: string
  name: string
  reason: string
  anchor: string
}

// PathRunReadiness 是一条路径的运行准备结论。
export interface PathRunReadiness {
  pathId: string
  pathName: string
  sequenceNo: number
  runnable: boolean
  summary: string
  blocks: RunReadinessItem[]
  reminders: RunReadinessItem[]
}

// PlanRunReadiness 是一个计划的运行准备结论。
export interface PlanRunReadiness {
  summary: string
  totalCount: number
  runnableCount: number
  blockedCount: number
  paths: PathRunReadiness[]
}
