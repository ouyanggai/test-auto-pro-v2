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
  instanceActionKey: string
  instanceActions: PathConfigActionConfiguration
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
  effectiveFormData: Record<string, unknown>
  branchPatches: PathConfigurationBranchPatch[]
  runtimeValidation: PathConfigurationRuntimeValidation
  issues: HistoryDataIssue[]
  keyFields: PathConfigKeyField[]
  actions: unknown[]
  compiledScenario: PathCompiledActionStep[]
}

// PathConfigKeyField 是决定当前执行路径的条件字段，只用于界面提示，不参与保存。
export interface PathConfigKeyField {
  path: string
  label?: string
  hasCurrent: boolean
  current?: unknown
  candidates?: unknown[]
  operators?: string[]
  branches?: string[]
  decisive: boolean
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
  compiledScenario: PathCompiledActionStep[]
  issues: PathActionConfigurationIssue[]
}

export interface PathActionConfigurationIssue {
  index: number
  action?: string
  actionId?: string
  code: string
  message: string
  blocking: boolean
}

export type PathFormStatus = 'empty' | 'affected' | 'ready'

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

export type PathConfigActionKind = PathActionKey | 'system_automatic'

export interface PathConfigActionBase {
  kind: PathActionKey
  label: string
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
  person?: PathConfigPersonStrategyInput
  parameters?: Record<string, unknown>
  actorPolicy?: string
  note?: string
}

// PathActionCategory 是目标平台动作分类，系统自动项只读展示。
export type PathActionCategory = 'lifecycle' | 'current_todo' | 'done_recovery' | 'instance_management' | 'system_automatic'

export interface PathConfigActionParameter {
  name: string
  value?: string
  required: boolean
  description: string
}

export interface PathConfigActionPrecondition {
  key: string
  label: string
  required: boolean
  present: boolean
}

export interface PathConfigActionCatalogItem {
  kind: PathConfigActionKind
  category: PathActionCategory
  scope: PathActionScope
  label: string
  description: string
  enabled: boolean
  disabledReason: string
  requiresPerson: boolean
  person?: PathConfigPerson
  targetOperation: string
  parameters: string[]
  parameterDetails: PathConfigActionParameter[]
  preconditions: PathConfigActionPrecondition[]
  expectedEffect: string
  requiresReload: boolean
  reloadRequirements: string[]
  systemOnly: boolean
  systemNodeType: string
  systemInserted?: boolean
  systemInsertedReason?: string
  runtimeNote: string
}

// PathActionStepSource 区分用户配置动作、系统恢复步骤和系统导航步骤。
export type PathActionStepSource = 'user' | 'system_recovery' | 'system_navigation'

// PathCompiledActionStep 是服务端编译的只读场景步骤，浏览器不能提交。
export interface PathCompiledActionStep {
  sequence: number
  source: PathActionStepSource
  sourceActionKey?: string
  action: PathActionKey | 'system_automatic'
  scope: PathActionScope
  actorPolicy?: string
  nodeKey?: string
  parameters?: Record<string, unknown>
  precondition: string
  expectedEffect: string
  stopOnFailure: string
  recoveryPolicy: string
  reloadRequired: boolean
}

// PathActionContainer 是一个可独立编排的动作容器：语义节点或实例上下文。
export interface PathActionContainer {
  key: string
  persons: PathConfigPerson[]
  actionConfiguration: PathConfigActionConfiguration
}

export interface PathConfigConfiguredActionInput {
  key: string
  kind: PathConfigActionKind
  person?: PathConfigPersonStrategyInput
  parameters?: Record<string, unknown>
  actorPolicy?: string
  note?: string
}

export interface PathConfigDraft {
  fields: Record<string, string>
  persons: Record<string, string[]>
  personStrategies: Record<string, PathConfigPersonStrategyInput>
  actionConfigurations: Record<string, PathConfigConfiguredActionInput[]>
}

export type PathConfigPagePhase =
  | 'loading'
  | 'ready'
  | 'saving'
  | 'invalid'
  | 'error'
