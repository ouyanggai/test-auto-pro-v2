// F-015 成功断言与运行准备的前端类型，字段与后端 DTO 一一对应，前端不自造取值。

// SuccessAssertionStatusOption 是目标平台真实实例状态与它自己的中文标签。
export interface SuccessAssertionStatusOption {
  value: string
  label: string
}

// SuccessAssertionEndNodeCandidate 是断言可选的结束节点，全部由真实结构推导。
export interface SuccessAssertionEndNodeCandidate {
  nodeKey: string
  name: string
  // arrivalCount 大于 1 时必须指定第几次到达。
  arrivalCount: number
}

// PathSuccessAssertion 是已保存的成功断言。
export interface PathSuccessAssertion {
  endNodeKey: string
  endNodeName: string
  expectedStatus: string
  expectedStatusLabel: string
  arrivalOrdinal: number
  revision: number
  updatedAt: string
}

// PathConfigAffectedItem 是只读复验给出的问题项，文案由后端给出，前端不改写。
export interface PathConfigAffectedItem {
  kind: string
  name: string
  reason: string
}

// SuccessAssertionWorkspace 是断言卡片一次读取所需的全部内容。
export interface SuccessAssertionWorkspace {
  endNodeCandidates: SuccessAssertionEndNodeCandidate[]
  statusOptions: SuccessAssertionStatusOption[]
  assertion?: PathSuccessAssertion
  issues: PathConfigAffectedItem[]
}

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
