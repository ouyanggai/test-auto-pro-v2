import type { FlowCandidate, FlowTemplateCandidate } from './types.ts'

export const CANDIDATE_ITEM_SIZE = 96
export const CANDIDATE_VIEWPORT_HEIGHT = 480

function joinMeta(parts: string[]): string {
  return parts.filter(Boolean).join(' · ')
}

export function templateFormSummary(candidate: FlowTemplateCandidate): string {
  if (candidate.formExist === 'noForm') return '无表单流程'
  if (candidate.formTemplateCount > 0) return `有表单 · 关联 ${candidate.formTemplateCount} 个`
  if (candidate.formExist) return '有表单'
  return ''
}

export function candidateTitle(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return candidate.flowName
  if (candidate.kind === 'submitted') return candidate.name
  return candidate.flowInstanceName
}

export function candidateStatus(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return ''
  if (candidate.kind === 'submitted') return candidate.status
  return candidate.statusName
}

export function templateCompanyName(candidate: FlowCandidate): string {
  return candidate.kind === 'template' ? candidate.companyName.trim() : ''
}

export function candidateMeta(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') {
    const classification = [candidate.typeName, candidate.groupName].filter(Boolean).join(' / ')
    return joinMeta([
      classification ? `分类 ${classification}` : '',
      templateFormSummary(candidate),
      candidate.updateTime ? `更新 ${candidate.updateTime}` : '',
    ])
  }
  if (candidate.kind === 'submitted') {
    return joinMeta([
      candidate.createDate ? `提交时间 ${candidate.createDate}` : '',
      candidate.currentNodeName ? `当前节点 ${candidate.currentNodeName}` : '',
    ])
  }
  return joinMeta([
    candidate.initiator ? `发起人 ${candidate.initiator}` : '',
    candidate.initiatorDate ? `提交时间 ${candidate.initiatorDate}` : '',
  ])
}

export function candidateDetail(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return `备注：${candidate.remark.trim() || '暂无备注'}`
  if (candidate.kind === 'submitted') return candidate.currentAuditUserNames ? `当前处理人 ${candidate.currentAuditUserNames}` : '暂无当前处理人'
  return `实例编号 ${candidate.flowInstanceId}`
}

export function candidateDetailTitle(candidate: FlowCandidate): string {
  return candidateDetail(candidate)
}
