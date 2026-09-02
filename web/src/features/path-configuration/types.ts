import type { HistoryDataIssue, HistoryDataSource } from '../history-replay/types.ts'

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

export interface PathConfigurationDataWorkspace {
  path: PathConfigPath
  revision: number
  nodeRevision: number
  dataRevision: number
  actionRevision: number
  nodeStatus: string
  dataStatus: 'empty' | 'needs_input' | 'ready' | 'affected'
  historySource: HistoryDataSource
  runtimeType: 'formmaking' | 'vue_custom' | 'unknown'
  template: Record<string, unknown>
  vuePage?: PathVueCustomPageRule | null
  permissions: Array<{ field: string, power: 'edit' | 'only_read' | 'hide' }>
  readRequests: PathFormReadRequest[]
  ruleVersion: string
  effectiveFormData: Record<string, unknown>
  branchPatches: PathConfigurationBranchPatch[]
  runtimeValidation: PathConfigurationRuntimeValidation
  issues: HistoryDataIssue[]
  actions: unknown[]
  compiledScenario: unknown[]
}

export interface PathConfigurationRuntimeValidation {
  accepted: boolean
  issues: HistoryDataIssue[]
}

export interface PathConfigurationBranchPatch {
  path: string
  before: unknown
  after: unknown
  reason: string
  branchKey: string
}

export interface PathConfigurationDataInput {
  revision: number
  values: Record<string, unknown>
  runtimeValidation: PathConfigurationRuntimeValidation
  confirmationToken?: string
}

export interface PathConfigurationRouteChange {
  from: PathConfigPath
  to: PathConfigPath
  overwritesData: boolean
  affected: Array<{ kind: string, name: string, reason: string }>
  warning: string
}

export interface PathConfigurationDataResult extends Omit<PathConfigurationDataWorkspace, 'historySource' | 'nodeRevision' | 'actionRevision' | 'nodeStatus' | 'actions' | 'compiledScenario'> {
  routeChanged: boolean
  requiresConfirmation: boolean
  confirmationToken?: string
  routeChange?: PathConfigurationRouteChange | null
}

// PathActionKey 是 F-012 允许编排的稳定动作语义键，不携带目标临时身份。
export type PathActionKey =
  | 'save_draft'
  | 'submit'
  | 'resubmit'
  | 'storage_form_data'
  | 'add_sign'
  | 'transfer'
  | 'approve'
  | 'reject'
  | 'rollback_previous'
  | 'retrieve'
  | 'withdraw'
  | 'urge'
  | 'forward'
  | 'follow'
  | 'unfollow'

// PathActionScope 描述动作作用于发起实例、当前待办、已办任务或实例旁支的范围。
export type PathActionScope = 'initiator' | 'task' | 'completed_task' | 'instance'

// PathActionConfigurationInput 是新动作端点的最小请求体，只提交独立动作记录。
export interface PathActionConfigurationInput {
  revision: number
  persons?: PathConfigPersonStrategyInput[]
  actions: PathConfiguredActionInput[]
}

// PathConfiguredActionInput 是一条可独立排序、删除和重试的 F-012 动作记录。
export interface PathConfiguredActionInput {
  key: string
  action: PathActionKey
  scope: PathActionScope
  nodeKey?: string
  order: number
  parameters?: Record<string, unknown>
  actorPolicy?: string
  note?: string
}

// PathActionConfigurationResult 是服务端重编译后的动作保存和只读预览结果。
export interface PathActionConfigurationResult {
  path: PathConfigPath
  revision: number
  nodeRevision: number
  actionRevision: number
  status: string
  actions: PathConfiguredActionInput[]
  compiledScenario: unknown[]
  issues: Array<{ index: number, action?: string, actionId?: string, code: string, message: string, blocking: boolean }>
}

export interface PathConfigPreparation {
  preparedNodes: number
  pendingItems: number
  included: boolean
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

export interface PathFormConditionBinding {
  key: string
  nodeName: string
  branchName: string
  expression: string
  fields: string[]
  selected: boolean
  locked: boolean
  needsReview: boolean
  verified: boolean
}

export interface PathFormFieldRule {
  field: string
  disabled: boolean
  conditionKeys: string[]
}

export interface PathFormConfiguration {
  revision: number
  status: PathFormStatus
  statusName: string
  readOnly: boolean
  ruleVersion: string
  readRequests: PathFormReadRequest[]
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
  conditionBindings: PathFormConditionBinding[]
  conditionReviews: string[]
  fieldRules: PathFormFieldRule[]
  renderType?: 'formmaking' | 'vue_custom' | 'unknown'
  vuePage?: PathVueCustomPageRule | null
}

export interface PathFormReadRequest {
  method: string
  path: string
  source: string
}

export interface PathVueCustomPageRule {
  status: 'complete' | 'partial' | 'blocked'
  pageName: string
  componentName: string
  route: string
  fields: PathVueCustomFieldRule[]
  issues: string[]
}

export interface PathVueCustomFieldRule {
  path: string
  name: string
  valueType: string
  valueShape: string
  serialization: string
  required: boolean
  readOnly: boolean
  hidden: boolean
  disabled: boolean
  nested: boolean
  collection: boolean
  candidateKind: string
  candidateSource: string
  defaultValue?: unknown
  dataSource?: string
  format?: string
  validation: string[]
  validationCapability: string[]
  evidence: string
  options: Array<{ label: string, value: unknown }>
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
  conditionBindings: PathFormConditionBinding[]
  conditionReviews: string[]
  fieldRules: PathFormFieldRule[]
  generationState: 'complete' | 'partial' | 'blocked'
  issues: PathFormGenerationIssue[]
  routeVerification: PathFormRouteVerification
}

export interface PathFormGenerationIssue {
  field: string
  reason: string
  blocking: boolean
  code?: string
  status?: string
  source?: string
  fieldPath?: string
  fieldLabel?: string
  operator?: string
  expected?: unknown
  actual?: unknown
  relatedFields?: string[]
  message?: string
  canRetry?: boolean
}

export interface PathFormRouteVerification {
  matched: boolean
  reason: string
  source?: string
}

export interface PathFormRuntimeSession {
  sid: string
  baseURL: string
  accountName: string
  userId: string
  companyId: string
  customerCode: string
  companyName: string
  departmentId: string
  departmentName: string
}

export interface RunInputSnapshot {
  version: string
  planId: number
  pathId: number
  sequenceNo: number
  accountRef: string
  flowSource: string
  targetObjectRef: string
  renderType: 'formmaking' | 'vue_custom' | 'unknown'
  templateRuleVersion: string
  formTemplateVersion: string
  shapeDigest: string
  snapshotDigest: string
  configVersion: number
  configRevision: number
  nodeRevision: number
  formRevision: number
  pathChoices: Array<{ routeNodeId: string, branchId: string }>
  nodeFieldValues: Record<string, Record<string, string>>
  actionValues: Record<string, string>
  confirmedNodeKeys: string[]
  formValues: Record<string, unknown>
  capturedAt: string
}

export interface RunInputPreflightResult {
  status: 'ready' | 'blocked'
  snapshot: RunInputSnapshot
  target: {
    method: string
    path: string
    payloadKeys: string[]
    payloadDigest: string
    successChecks: string[]
  }
  issues: Array<{ code: string, source: string, fieldPath?: string, message: string, canRetry: boolean }>
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
  parameters?: Record<string, unknown>
  actorPolicy?: string
  note?: string
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
  parameters?: Record<string, unknown>
  actorPolicy?: string
  note?: string
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
