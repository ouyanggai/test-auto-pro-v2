export type PlanStatus = 'pending_configuration' | 'ready' | 'running' | 'completed'
export type PlanRunMode = 'serial' | 'parallel'
export type FlowSource = 'new' | 'started' | 'pending'

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
  accountId: string | null
  flowSource: FlowSource | null
  flowId: string | null
  runMode: PlanRunMode
  maxConcurrency: number | null
  scheduledAt: number | null
}

export type PlanFormField = 'name' | 'accountId' | 'flowSource' | 'flowId' | 'maxConcurrency'
export type PlanFormErrors = Partial<Record<PlanFormField, string>>

export interface SelectOption {
  label: string
  value: string
}
