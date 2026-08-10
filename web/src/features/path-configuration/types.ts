export type PathConfigFieldType = 'text' | 'number' | 'date' | 'dateTime' | 'singleSelect' | 'multiSelect' | 'switch'

export interface PathConfigPath {
  sequenceNo: number
  name: string
}

export interface PathConfiguration {
  path: PathConfigPath
  revision: number
  status: 'pending' | 'configured' | 'affected'
  progress: PathConfigProgress
  nextNodeKey: string
  groups: PathConfigGroup[]
  warnings: string[]
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
  lineBlocked: boolean
}

export interface PathConfigPerson {
  key: string
  title: string
  mode: 'fixed' | 'select' | 'runtime' | 'review'
  detail: string
  editable: boolean
  multiple: boolean
  required: boolean
  minCount: number
  selected: string[]
  options: PathConfigPersonOption[]
  affected: boolean
  note: string
}

export interface PathConfigPersonOption {
  label: string
  value: string
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
  status: string
}

export interface PathConfigDraft {
  fields: Record<string, string>
  actions: Record<string, string>
  persons: Record<string, string[]>
}

export type PathConfigPagePhase =
  | 'loading'
  | 'ready'
  | 'saving'
  | 'invalid'
  | 'error'
