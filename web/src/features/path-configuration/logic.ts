import type {
  PathConfigActionValue,
  PathConfiguration,
  PathConfigDraft,
  PathConfigField,
  PathConfigFieldValue,
} from './types.ts'

// parsePathConfigValue 把后端 JSON 文本值按字段类型解析为前端控件值；无法解析时退回原始文本。
export function parsePathConfigValue(field: PathConfigField, raw: string): unknown {
  if (raw === '') return field.type === 'multiSelect' ? [] : ''
  try {
    return JSON.parse(raw)
  }
  catch {
    return raw
  }
}

// encodePathConfigValue 把前端控件值编码为后端 JSON 文本；多选保持数组。
export function encodePathConfigValue(field: PathConfigField, value: unknown): string {
  if (field.type === 'multiSelect' && !Array.isArray(value)) return '[]'
  if (value === '' || value === undefined || value === null) return '""'
  return JSON.stringify(value)
}

// initPathConfigDraft 从配置模型生成可编辑草稿；字段用不透明键、动作用不透明键保存当前值。
export function initPathConfigDraft(configuration: PathConfiguration): PathConfigDraft {
  const fields: Record<string, string> = {}
  const actions: Record<string, string> = {}
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (field.editable) fields[field.key] = field.value
      }
      for (const action of node.actions) {
        actions[action.key] = action.current
      }
    }
  }
  return { fields, actions }
}

// hasPathConfigDraftChanges 判断草稿相对配置模型是否有真实变化；仅用于保存按钮可用性。
export function hasPathConfigDraftChanges(configuration: PathConfiguration, draft: PathConfigDraft): boolean {
  const baseline = initPathConfigDraft(configuration)
  const baselineFields = JSON.stringify(baseline.fields)
  const baselineActions = JSON.stringify(baseline.actions)
  return baselineFields !== JSON.stringify(draft.fields) || baselineActions !== JSON.stringify(draft.actions)
}

// buildPathConfigSavePayload 把草稿收敛为后端最小回写体，只包含当前配置中的可编辑项。
export function buildPathConfigSavePayload(configuration: PathConfiguration, draft: PathConfigDraft): { fields: PathConfigFieldValue[], actions: PathConfigActionValue[] } {
  const fields: PathConfigFieldValue[] = []
  const actions: PathConfigActionValue[] = []
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (field.editable && Object.prototype.hasOwnProperty.call(draft.fields, field.key)) {
          fields.push({ key: field.key, value: draft.fields[field.key] })
        }
      }
      for (const action of node.actions) {
        if (Object.prototype.hasOwnProperty.call(draft.actions, action.key)) {
          actions.push({ key: action.key, action: draft.actions[action.key] })
        }
      }
    }
  }
  return { fields, actions }
}

// allEditableFieldsFilled 判断必填字段是否都有值，用于保存前即时提示。
export function allEditableFieldsFilled(configuration: PathConfiguration, draft: PathConfigDraft): { missing: string[], complete: boolean } {
  const missing: string[] = []
  for (const group of configuration.groups) {
    for (const node of group.nodes) {
      for (const field of node.fields) {
        if (!field.editable || !field.required) continue
        const value = parsePathConfigValue(field, draft.fields[field.key] ?? '')
        const empty = Array.isArray(value) ? value.length === 0 : value === '' || value === undefined || value === null
        if (empty) missing.push(field.name)
      }
    }
  }
  return { missing, complete: missing.length === 0 }
}

// disagreesInDraft 统计草稿中不同意动作数量，用于线路影响提示。
export function disagreesInDraft(draft: PathConfigDraft): number {
  let count = 0
  for (const key of Object.keys(draft.actions)) {
    if (draft.actions[key] === 'disagree') count++
  }
  return count
}
