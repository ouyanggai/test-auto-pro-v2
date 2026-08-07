export type PathConfigFieldType = 'text' | 'number' | 'dateTime' | 'singleSelect' | 'multiSelect' | 'switch'

export interface PathConfigPath {
  sequenceNo: number
  name: string
}

export interface PathConfiguration {
  path: PathConfigPath
  revision: number
  status: 'configured' | 'affected'
  groups: PathConfigGroup[]
  warnings: string[]
}

export interface PathConfigGroup {
  title: string
  kind: string
  nodes: PathConfigNode[]
}

export interface PathConfigNode {
  name: string
  typeName: string
  kind: string
  fields: PathConfigField[]
  gaps: PathConfigGap[]
  actions: PathConfigAction[]
  lineBlocked: boolean
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
}

export type PathConfigPagePhase =
  | 'loading'
  | 'ready'
  | 'saving'
  | 'invalid'
  | 'error'
