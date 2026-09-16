export class DetailBlockedError extends Error {
  constructor (message: string) {
    super(message)
    this.name = 'DetailBlockedError'
  }
}

export type DetailMode = 'audit' | 'edit' | 'view'

export interface TaskContext {
  id?: string
  flowInstanceId?: string
  jobTaskId?: string
  auditWay?: string
  initiatorId?: string
  createrId?: string
  formProxyId?: string
  flowProxyId?: string
  [key: string]: unknown
}

export interface ActorIdentity {
  userId: string
  name?: string
}

export interface NativeSemantics {
  mode: DetailMode
  isExamine: boolean
  isReInitiate: boolean
  operaType: 'examine' | 'reEdit' | 'check'
  actionType: 'examine' | 'edit' | 'preview'
  taskStatus: string
}

// resolveDetailSemantics 对齐 FlowDetail.openFlowDialog 的 audit/edit/view 映射。
export function resolveDetailSemantics (mode: string, taskContext: TaskContext | null): NativeSemantics {
  if (mode !== 'audit' && mode !== 'edit' && mode !== 'view') {
    throw new DetailBlockedError(`详情装载不支持 mode=${mode}`)
  }
  if (mode === 'audit') {
    if (!taskContext || String(taskContext.jobTaskId ?? '').trim() === '') {
      throw new DetailBlockedError('审批装载缺少 jobTaskId')
    }
    return {
      mode,
      isExamine: true,
      isReInitiate: false,
      operaType: 'examine',
      actionType: 'examine',
      taskStatus: 'pending'
    }
  }
  if (mode === 'edit') {
    return {
      mode,
      isExamine: false,
      isReInitiate: true,
      operaType: 'reEdit',
      actionType: 'edit',
      taskStatus: 'edit'
    }
  }
  return {
    mode: 'view',
    isExamine: false,
    isReInitiate: false,
    operaType: 'check',
    actionType: 'preview',
    taskStatus: ''
  }
}

// splitIdentities 把当前审批人与原发起人分开保存，禁止互相覆盖。
export function splitIdentities (current: ActorIdentity, initiator: ActorIdentity): { current: ActorIdentity, initiator: ActorIdentity } {
  if (!current.userId) throw new DetailBlockedError('缺少当前操作人')
  if (!initiator.userId) throw new DetailBlockedError('缺少原发起人')
  return {
    current: { userId: current.userId, name: current.name },
    initiator: { userId: initiator.userId, name: initiator.name }
  }
}

export function requireNewContextOnActorChange (previousUserId: string, nextUserId: string): boolean {
  return previousUserId !== nextUserId
}
