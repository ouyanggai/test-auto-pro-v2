import type {
  DueFlowCandidate,
  FlowCandidate,
  FlowSource,
  FlowTemplateCandidate,
  PlanRow,
  PlanStatus,
  SubmittedFlowCandidate,
} from './types.ts'

export const planStatusLabels: Record<PlanStatus, string> = {
  pending_configuration: '待配置',
  ready: '可运行',
  running: '运行中',
  completed: '已完成',
}

export const planStatusOptions = Object.entries(planStatusLabels).map(([value, label]) => ({ value, label }))

const templateNames = ['采购申请审批', '合同用印审批', '费用报销审批', '项目立项审批', '设备领用审批', '预算调整审批']
const submittedNames = ['采购申请单', '合同用印申请', '差旅费报销单', '项目立项申请', '设备采购申请', '预算调整申请']
const dueNames = ['采购申请草稿', '合同用印草稿', '费用报销退回单', '项目立项草稿', '设备领用草稿', '预算调整退回单']

const flowTemplates: FlowTemplateCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `template-${index + 1}`,
  kind: 'template',
  accountName: '',
  templateId: `template-${index + 1}`,
  flowName: `${templateNames[index % templateNames.length]}${index >= templateNames.length ? ` ${Math.floor(index / templateNames.length) + 1}` : ''}`,
  typeName: index % 2 === 0 ? '经营管理' : '综合办公',
  groupName: index % 3 === 0 ? '常用流程' : '业务流程',
  statusText: '可发起',
  updateTime: `2026-07-${String(26 - (index % 8)).padStart(2, '0')} 10:${String(index % 6).padStart(2, '0')}0`,
}))

const submittedFlows: SubmittedFlowCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `submitted-${index + 1}`,
  kind: 'submitted',
  accountName: '',
  id: `submitted-${index + 1}`,
  name: `${submittedNames[index % submittedNames.length]} #${20260700 + index + 1}`,
  status: ['审批中', '已完结', '已撤销'][index % 3],
  createDate: `2026-07-${String(26 - (index % 9)).padStart(2, '0')} ${String(9 + (index % 8)).padStart(2, '0')}:20`,
  currentNodeName: index % 3 === 1 ? '完结' : ['部门负责人审批', '财务复核', '分管领导审批'][index % 3],
  currentAuditUserNames: index % 3 === 1 ? '完结' : ['王静', '赵磊', '陈晨'][index % 3],
}))

const dueFlows: DueFlowCandidate[] = Array.from({ length: 18 }, (_, index) => ({
  key: `due-${index + 1}`,
  kind: 'due',
  accountName: '',
  flowInstanceId: `due-${index + 1}`,
  flowInstanceName: `${dueNames[index % dueNames.length]} #${20260750 + index + 1}`,
  statusName: ['草稿', '驳回', '撤销'][index % 3],
  initiator: ['张敏', '李伟', '测试专员'][index % 3],
  initiatorDate: `2026-07-${String(25 - (index % 8)).padStart(2, '0')} ${String(8 + (index % 9)).padStart(2, '0')}:40`,
}))

export function getMockFlowCandidates(source: FlowSource, accountName: string): FlowCandidate[] {
  const candidates: readonly FlowCandidate[] = source === 'new'
    ? flowTemplates
    : source === 'started'
      ? submittedFlows
      : dueFlows
  return candidates.map((candidate) => ({ ...candidate, accountName }))
}

export const mockPlans: PlanRow[] = [
  {
    id: 'plan-001',
    name: '采购申请主流程回归',
    flowName: '采购申请审批',
    accountName: '张敏（zhangmin）',
    pathCount: 0,
    runMode: 'serial',
    scheduledAt: null,
    status: 'pending_configuration',
    lastRunResult: '尚未运行',
  },
  {
    id: 'plan-002',
    name: '合同用印条件分支检查',
    flowName: '合同用印审批',
    accountName: '李伟（liwei）',
    pathCount: 3,
    runMode: 'parallel',
    scheduledAt: '2026-07-28 09:30',
    status: 'ready',
    lastRunResult: '尚未运行',
  },
  {
    id: 'plan-003',
    name: '费用报销日常巡检',
    flowName: '费用报销审批',
    accountName: '测试专员（tester01）',
    pathCount: 4,
    runMode: 'serial',
    scheduledAt: null,
    status: 'running',
    lastRunResult: '2 / 4 路径已完成',
  },
  {
    id: 'plan-004',
    name: '项目立项完整路径验证',
    flowName: '项目立项审批',
    accountName: '李伟（liwei）',
    pathCount: 5,
    runMode: 'parallel',
    scheduledAt: '2026-07-26 18:00',
    status: 'completed',
    lastRunResult: '成功 5，失败 0',
  },
]
