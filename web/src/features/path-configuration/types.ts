export type PathConfigFieldType = 'text' | 'number' | 'date' | 'dateTime' | 'singleSelect' | 'multiSelect' | 'switch'

export interface PathConfigPath {
  sequenceNo: number
  name: string
}

export interface PathConfiguration {
  path: PathConfigPath
  revision: number
  nodeRevision: number
  status: 'pending' | 'configured' | 'affected'
  progress: PathConfigProgress
  nextNodeKey: string
  groups: PathConfigGroup[]
  warnings: string[]
  form: PathFormConfiguration
}

export type PathFormStatus = 'empty' | 'draft' | 'valid' | 'affected' | 'unsupported'

export interface PathFormConfiguration {
  revision: number
  status: PathFormStatus
  statusName: string
  readOnly: boolean
  template: Record<string, unknown>
  permissions: Array<{ field: string, power: 'edit' | 'only_read' | 'hide' }>
  values: Record<string, unknown>
  seed: number
  generatedFieldPaths: string[]
  manualOverridePaths: string[]
  sampleSummary: PathFormSampleSummary
  validated: boolean
  unsupported: string[]
  affected: Array<{ kind: string, name: string, reason: string }>
  autoFilled: number
  manualPending: number
}

export interface PathFormSampleSummary {
  saved: boolean
  defaults: number
  recent: number
  fallback: number
}

export interface PathFormGenerateResult {
  revision: number
  status: 'draft'
  values: Record<string, unknown>
  seed: number
  generatedFieldPaths: string[]
  manualOverridePaths: string[]
  sampleSummary: PathFormSampleSummary
  autoFilled: number
  manualPending: number
  unsupported: string[]
}

export interface PathFormRuntimeSession {
  sid: string
  baseURL: string
  accountName: string
}

export interface PathConfigProgress {
  total: number
  completed: number
  pending: number
}

export interface PathConfigGroup {
  title: string
  kind: string
  nodes: PathConfigNode[]
}

export interface PathConfigNode {
  key: string
  name: string
  typeName: string
  kind: string
  status: 'not_required' | 'pending' | 'partial' | 'configured' | 'runtime' | 'affected'
  statusName: string
  fields: PathConfigField[]
  persons: PathConfigPerson[]
  gaps: PathConfigGap[]
  requirements: PathConfigRequirement[]
  actions: PathConfigAction[]
  actionPlan: PathConfigActionPlan
  lineBlocked: boolean
}

export interface PathConfigPerson {
  key: string
  title: string
  mode: 'fixed' | 'select' | 'runtime' | 'review'
  detail: string
  items: PathConfigPersonDisplayItem[]
  editable: boolean
  multiple: boolean
  required: boolean
  minCount: number
  maxCount: number
  selected: string[]
  defaultSelected: string[]
  options: PathConfigPersonOption[]
  strategy: PathConfigPersonStrategy
  strategySeed: number
  strategies: PathConfigPersonStrategyOption[]
  affected: boolean
  note: string
}

export interface PathConfigPersonDisplayItem {
  category: string
  name: string
  count: number
}

export interface PathConfigPersonOption {
  label: string
  value: string
}

export type PathConfigPersonStrategy = 'target_default' | 'manual' | 'random' | 'all'

export interface PathConfigPersonStrategyOption {
  value: PathConfigPersonStrategy
  label: string
}

export interface PathConfigPersonStrategyInput {
  key: string
  strategy: PathConfigPersonStrategy
  seed: number
  selected: string[]
}

export interface PathConfigRequirement {
  category: string
  title: string
  detail: string
  status: string
}

export interface PathConfigField {
  key: string
  name: string
  type: PathConfigFieldType
  required: boolean
  value: string
  options: PathConfigOption[]
  editable: boolean
  affected: boolean
  note: string
}

export interface PathConfigOption {
  label: string
  value: string
}

export interface PathConfigGap {
  name: string
  reason: string
}

export interface PathConfigAction {
  key: string
  kind: 'submit' | 'agree_disagree'
  label: string
  current: string
  default: string
  options: PathConfigActionOption[]
  disagreeWarning: string
}

export interface PathConfigActionOption {
  value: string
  label: string
}

export type PathConfigActionKind = 'submit' | 'approve_pass' | 'reject_no_pass' | 'draft_save' | 'rollback_previous' | 'add_sign' | 'transfer_approver'

export interface PathConfigActionPlan {
  catalog: PathConfigActionCatalogItem[]
  rollbackTargets: PathConfigActionOption[]
  arrivals: PathConfigArrivalPlan[]
  maxArrivals: number
  maxPathSteps: number
  affected: boolean
  note: string
}

export interface PathConfigActionCatalogItem {
  kind: PathConfigActionKind
  label: string
  description: string
  enabled: boolean
  disabledReason: string
  maxCount: number
  allowsOpinion: boolean
  requiresTarget: boolean
  requiresPerson: boolean
  person?: PathConfigPerson
}

export interface PathConfigActionRow {
  kind: PathConfigActionKind
  count: number
  target: string
  person?: PathConfigPersonStrategyInput
}

export interface PathConfigArrivalPlan {
  visit: number
  steps: PathConfigActionStep[]
}

export interface PathConfigActionStep {
  kind: PathConfigActionKind
  label: string
  opinion: string
  target: string
  person?: PathConfigPersonStrategyInput
}

export interface PathConfigArrivalInput {
  visit: number
  steps: PathConfigActionStepInput[]
}

export interface PathConfigActionStepInput {
  kind: PathConfigActionKind
  opinion: string
  target: string
  person?: PathConfigPersonStrategyInput
}

export interface PathConfigFieldValue {
  key: string
  value: string
}

export interface PathConfigActionValue {
  key: string
  action: string
}

export interface PathConfigSaveResult {
  path: PathConfigPath
  revision: number
  nodeRevision: number
  formRevision: number
  status: string
}

export interface PathConfigDraft {
  fields: Record<string, string>
  actions: Record<string, string>
  persons: Record<string, string[]>
  personStrategies: Record<string, PathConfigPersonStrategyInput>
  arrivals: Record<string, PathConfigArrivalInput[]>
}

export interface PathConfigNodeSavePayload {
  persons: PathConfigPersonStrategyInput[]
  arrivals: PathConfigArrivalInput[]
}

export type PathConfigPagePhase =
  | 'loading'
  | 'ready'
  | 'saving'
  | 'invalid'
  | 'error'
