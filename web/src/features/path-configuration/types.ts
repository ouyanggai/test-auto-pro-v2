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
  actionCycles: PathConfigActionCycle[]
  preparation: PathConfigPreparation
}

export interface PathConfigPreparation {
  preparedNodes: number
  pendingItems: number
  included: boolean
}

export type PathConfigPresetScope = 'current' | 'selected' | 'compatible'

export interface PathConfigPresetNodeItem {
  nodeKey: string
  nodeName: string
  action: string
  status: 'write' | 'keep' | 'skip' | 'manual'
  detail: string
}

export interface PathConfigPresetPath {
  path: PathConfigPath
  items: PathConfigPresetNodeItem[]
}

export interface PathConfigPresetPreview {
  scope: PathConfigPresetScope
  paths: PathConfigPresetPath[]
}

export interface PathConfigPresetApplyResult {
  preview: PathConfigPresetPreview
  written: number
  kept: number
  skipped: number
  manual: number
}

export interface PathConfigActionCycle {
  key: string
  type: 'restart_from_initiator' | 'redo_previous_task'
  endNodeKey: string
  label: string
  count: number
  members: string[]
  summary: string
}

export interface PathConfigActionCycleInput {
  key: string
  type: 'restart_from_initiator' | 'redo_previous_task'
  endNodeKey: string
  count: number
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
  conditionHints: Array<{ key: string, nodeName: string, branchName: string, field: string, fields: string[], unmappedFields: string[], text: string, protected: boolean, active: boolean, activeKnown: boolean, mapped: boolean }>
  fieldRules: Array<{ field: string, disabled: boolean, conditionHints: string[] }>
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
  conditionHints: Array<{ key: string, nodeName: string, branchName: string, field: string, fields: string[], unmappedFields: string[], text: string, protected: boolean, active: boolean, activeKnown: boolean, mapped: boolean }>
  fieldRules: Array<{ field: string, disabled: boolean, conditionHints: string[] }>
}

export interface PathFormRuntimeSession {
  sid: string
  baseURL: string
  accountName: string
  userId: string
  companyId: string
  customerCode: string
  companyName: string
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
  actionConfiguration: PathConfigActionConfiguration
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

export type PathConfigActionKind = 'reject_no_pass' | 'draft_save' | 'rollback_previous' | 'add_sign'

export interface PathConfigActionBase {
  kind: 'submit' | 'approve_pass'
  label: string
  count: 1
  detail: string
}

export interface PathConfigActionConfiguration {
  base?: PathConfigActionBase
  catalog: PathConfigActionCatalogItem[]
  actions: PathConfigConfiguredAction[]
  affected: boolean
  note: string
}

export interface PathConfigConfiguredAction {
  key: string
  kind: PathConfigActionKind
  label: string
  count: number
  person?: PathConfigPersonStrategyInput
}

export interface PathConfigActionCatalogItem {
  kind: PathConfigActionKind
  label: string
  description: string
  enabled: boolean
  disabledReason: string
  requiresPerson: boolean
  person?: PathConfigPerson
}

export interface PathConfigConfiguredActionInput {
  key: string
  kind: PathConfigActionKind
  count: number
  person?: PathConfigPersonStrategyInput
}

export interface PathConfigFieldValue {
  key: string
  value: string
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
  persons: Record<string, string[]>
  personStrategies: Record<string, PathConfigPersonStrategyInput>
  actionConfigurations: Record<string, PathConfigConfiguredActionInput[]>
}

export interface PathConfigNodeSavePayload {
  persons: PathConfigPersonStrategyInput[]
  actions: PathConfigConfiguredActionInput[]
  actionCycles?: PathConfigActionCycleInput[]
}

export type PathConfigPagePhase =
  | 'loading'
  | 'ready'
  | 'saving'
  | 'invalid'
  | 'error'
