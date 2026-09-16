export class InitiateBlockedError extends Error {
  constructor (message: string) {
    super(message)
    this.name = 'InitiateBlockedError'
  }
}

export interface FlowTemplateRow {
  id: string
}

export interface NativeInitiateHandle {
  params?: { auditWay?: unknown, flowTemplateId?: unknown }
  flowList?: FlowTemplateRow[]
  confirm?: (id: string) => void
  getFlowFindById?: (id: string) => void
}

// selectBoundTemplate 在原生 $initFlow 实例上精确选中计划绑定模板。
// 没有 auditWay 不得假定 flowTemplateId 已完成选择；多模板不得取第一项。
export function selectBoundTemplate (handle: NativeInitiateHandle, boundTemplateId: string): { selectedId: string, via: 'single' | 'direct' | 'confirm' } {
  const auditWay = handle.params?.auditWay
  if (auditWay === undefined || auditWay === null || String(auditWay).trim() === '') {
    throw new InitiateBlockedError('发起装载缺少 auditWay，不能只传 flowTemplateId')
  }
  const boundId = String(boundTemplateId ?? '').trim()
  if (!boundId) {
    throw new InitiateBlockedError('发起装载缺少计划绑定模板 ID')
  }
  const list = Array.isArray(handle.flowList) ? handle.flowList : []
  if (list.length === 0) {
    handle.getFlowFindById?.(boundId)
    return { selectedId: boundId, via: 'direct' }
  }
  const matched = list.filter(item => String(item?.id ?? '') === boundId)
  if (matched.length !== 1) {
    throw new InitiateBlockedError(`查询结果不含计划绑定模板 ${boundId}`)
  }
  if (list.length === 1) {
    handle.getFlowFindById?.(boundId)
    return { selectedId: boundId, via: 'single' }
  }
  handle.confirm?.(boundId)
  return { selectedId: boundId, via: 'confirm' }
}
