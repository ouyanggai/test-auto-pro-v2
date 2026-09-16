// 发起模板精确选择（P1-2 修复后的权威实现）。
// 纪律：选择判据只认 trace 里 /web/flowTemplateApi/list 的真实响应；
// 没有真实响应、集合为空、绑定 ID 不在集合中一律阻塞，绝不在空列表上调 getFlowFindById 硬闯；
// 原生行为依据 InitFlowDialog.vue：mounted 只在 params.auditWay 存在时查询；
// 0 条 → 「暂无流程权限」；1 条 → 直接 getFlowFindById；多条 → chooseFlowDialog + confirm(id)。
export class InitiateBlockedError extends Error {
  constructor (message: string) {
    super(message)
    this.name = 'InitiateBlockedError'
  }
}

// TemplateSelectionPlan 是选择计划：blocked=阻塞（无权限/绑定缺失）；single=等原生自行装载；
// confirm=调用原生 confirm(boundId)（多模板）。
export type TemplateSelectionPlan =
  | { action: 'blocked', reason: string }
  | { action: 'single', boundId: string }
  | { action: 'confirm', boundId: string }

// planTemplateSelection 按真实候选集合给出选择动作。
// candidateIds=null 表示列表请求失败或没有 fulfilled 响应——无法证明权限，必须阻塞。
export function planTemplateSelection (candidateIds: string[] | null, boundId: string): TemplateSelectionPlan {
  const bound = String(boundId ?? '').trim()
  if (!bound) return { action: 'blocked', reason: '发起装载缺少计划绑定模板 ID' }
  if (candidateIds === null) return { action: 'blocked', reason: '模板列表请求失败或没有响应，无法证明模板权限' }
  if (candidateIds.length === 0) return { action: 'blocked', reason: '模板列表为空：目标报告该账号暂无流程权限' }
  if (!candidateIds.includes(bound)) {
    return { action: 'blocked', reason: `计划绑定模板 ${bound} 不在可发起集合 ${JSON.stringify(candidateIds)} 中` }
  }
  if (candidateIds.length === 1) return { action: 'single', boundId: bound }
  return { action: 'confirm', boundId: bound }
}
