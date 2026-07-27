import type { FlowSource, PlanRow, PlanStatus, SelectOption } from './types.ts'

export const planStatusLabels: Record<PlanStatus, string> = {
  pending_configuration: '待配置',
  ready: '可运行',
  running: '运行中',
  completed: '已完成',
}

export const planStatusOptions = Object.entries(planStatusLabels).map(([value, label]) => ({ value, label }))

export const accountOptions: SelectOption[] = [
  { label: '张敏（zhangmin）', value: 'zhangmin' },
  { label: '李伟（liwei）', value: 'liwei' },
  { label: '测试专员（tester01）', value: 'tester01' },
]

export const flowSourceOptions: Array<SelectOption & { value: FlowSource }> = [
  { label: '新发起', value: 'new' },
  { label: '已发', value: 'started' },
  { label: '待发', value: 'pending' },
]

const flowOptionsByAccountAndSource: Record<string, SelectOption[]> = {
  'zhangmin:new': [
    { label: '采购申请审批', value: 'purchase-apply' },
    { label: '合同用印审批', value: 'contract-seal' },
  ],
  'zhangmin:started': [{ label: '费用报销审批', value: 'expense-started' }],
  'zhangmin:pending': [{ label: '设备领用审批', value: 'equipment-pending' }],
  'liwei:new': [{ label: '项目立项审批', value: 'project-create' }],
  'liwei:started': [{ label: '采购申请审批', value: 'purchase-started' }],
  'liwei:pending': [{ label: '合同用印审批', value: 'contract-pending' }],
  'tester01:new': [{ label: '费用报销审批', value: 'expense-create' }],
  'tester01:started': [{ label: '项目立项审批', value: 'project-started' }],
  'tester01:pending': [{ label: '设备领用审批', value: 'equipment-tester-pending' }],
}

export function getTargetFlowOptions(accountId: string | null, source: FlowSource | null): SelectOption[] {
  if (!accountId || !source) return []
  return flowOptionsByAccountAndSource[`${accountId}:${source}`] ?? []
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
