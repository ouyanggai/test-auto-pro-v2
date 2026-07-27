export type PlanStatus = 'pending_configuration' | 'ready' | 'running' | 'completed'
export type PlanRunMode = 'serial' | 'parallel'
export type FlowSource = 'new' | 'started' | 'pending'
export type AccountVerificationState = 'idle' | 'verifying' | 'verified' | 'invalid' | 'failed'
export type FlowCandidateKind = 'template' | 'submitted' | 'due'

export interface PlanRow {
  id: string
  name: string
  flowName: string
  accountName: string
  pathCount: number
  runMode: PlanRunMode
  scheduledAt: string | null
  status: PlanStatus
  lastRunResult: string
}

export interface PlanFilters {
  name: string
  status: PlanStatus | null
}

export interface PlanAction {
  label: string
  intent: 'configure' | 'start' | 'view_running' | 'view_result'
}

export interface PlanFormValue {
  name: string
  account: string
  flowSource: FlowSource
  templateId: string | null
  submittedFlowId: string | null
  dueFlowId: string | null
  runMode: PlanRunMode
  maxConcurrency: number | null
  scheduleEnabled: boolean
  scheduledAt: number | null
}

interface FlowCandidateBase {
  key: string
  kind: FlowCandidateKind
  accountName: string
  [key: string]: unknown
}

export interface FlowTemplateCandidate extends FlowCandidateBase {
  kind: 'template'
  templateId: string
  flowName: string
  typeName: string
  groupName: string
  statusText: string
  updateTime: string
}

export interface VerifiedTargetAccount {
  account: string
  displayName: string
  companyName: string
}

export interface SubmittedFlowCandidate extends FlowCandidateBase {
  kind: 'submitted'
  id: string
  name: string
  status: string
  createDate: string
  currentNodeName: string
  currentAuditUserNames: string
}

export interface DueFlowCandidate extends FlowCandidateBase {
  kind: 'due'
  flowInstanceId: string
  flowInstanceName: string
  statusName: string
  initiator: string
  initiatorDate: string
}

export type FlowCandidate = FlowTemplateCandidate | SubmittedFlowCandidate | DueFlowCandidate
