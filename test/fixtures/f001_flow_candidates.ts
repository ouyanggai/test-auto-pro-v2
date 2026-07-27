import type {
  DueFlowCandidate,
  FlowCandidate,
  FlowSource,
  FlowTemplateCandidate,
  SubmittedFlowCandidate,
} from '../../web/src/features/plans/types.ts'

const flowTemplates: FlowTemplateCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `template-${index + 1}`,
  kind: 'template',
  accountName: '',
  templateId: `template-${index + 1}`,
  flowName: `测试流程模板 ${index + 1}`,
  typeName: '测试类型',
  groupName: '测试分组',
  statusText: '可发起',
  updateTime: '2026-07-27 10:00',
}))

const submittedFlows: SubmittedFlowCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `submitted-${index + 1}`,
  kind: 'submitted',
  accountName: '',
  id: `submitted-${index + 1}`,
  name: `测试已发流程 ${index + 1}`,
  status: '审批中',
  createDate: '2026-07-27 10:00',
  currentNodeName: '测试节点',
  currentAuditUserNames: '测试处理人',
}))

const dueFlows: DueFlowCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `due-${index + 1}`,
  kind: 'due',
  accountName: '',
  flowInstanceId: `due-${index + 1}`,
  flowInstanceName: `测试待发流程 ${index + 1}`,
  statusName: '草稿',
  initiator: '测试发起人',
  initiatorDate: '2026-07-27 10:00',
}))

export function getMockFlowCandidates(source: FlowSource, accountName: string): FlowCandidate[] {
  const candidates: readonly FlowCandidate[] = source === 'new'
    ? flowTemplates
    : source === 'started'
      ? submittedFlows
      : dueFlows
  return candidates.map((candidate) => ({ ...candidate, accountName }))
}
