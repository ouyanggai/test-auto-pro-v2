import type { FlowCandidate, FlowSource, PlanFormValue } from './types.ts'

export const FLOW_CANDIDATE_BATCH_SIZE = 6

export const flowSourceLabels: Record<FlowSource, string> = {
  new: '新发起',
  started: '已发',
  pending: '待发',
}

export const flowSelectionLabels: Record<FlowSource, string> = {
  new: '流程模板',
  started: '已发流程',
  pending: '待发流程',
}

export function isFlowSourceAvailable(source: FlowSource, accountVerified: boolean): boolean {
  return source === 'new' || accountVerified
}

export function getCandidateSearchText(candidate: FlowCandidate): string {
	if (candidate.kind === 'template') {
		return [
			candidate.flowName,
			candidate.code,
			candidate.typeName,
			candidate.groupName,
			candidate.remark,
			candidate.formExist,
			String(candidate.formTemplateCount),
		].join(' ')
	}
  if (candidate.kind === 'submitted') {
    return [candidate.name, candidate.status, candidate.createDate, candidate.currentNodeName, candidate.currentAuditUserNames].join(' ')
  }
  return [candidate.flowInstanceName, candidate.statusName, candidate.initiator, candidate.initiatorDate].join(' ')
}

export function filterFlowCandidates(candidates: readonly FlowCandidate[], query: string): FlowCandidate[] {
  const keyword = query.trim().toLocaleLowerCase('zh-CN')
  if (!keyword) return [...candidates]
  return candidates.filter((candidate) => getCandidateSearchText(candidate).toLocaleLowerCase('zh-CN').includes(keyword))
}

export function takeCandidateBatches(
  candidates: readonly FlowCandidate[],
  batchCount: number,
  batchSize = FLOW_CANDIDATE_BATCH_SIZE,
): FlowCandidate[] {
  return candidates.slice(0, Math.max(1, batchCount) * batchSize)
}

export type PostSelectionGuidanceTarget = 'scheduledAt' | 'maxConcurrency' | 'submit'

export function resolvePostSelectionGuidance(form: Pick<PlanFormValue, 'scheduleEnabled' | 'scheduledAt' | 'runMode' | 'maxConcurrency'>): PostSelectionGuidanceTarget {
  if (form.scheduleEnabled && !form.scheduledAt) return 'scheduledAt'
  if (form.runMode === 'parallel' && (typeof form.maxConcurrency !== 'number' || form.maxConcurrency < 2 || form.maxConcurrency > 20)) {
    return 'maxConcurrency'
  }
  return 'submit'
}

export function calculateNearestScrollDelta(
  targetTop: number,
  targetBottom: number,
  viewportTop: number,
  viewportBottom: number,
  inset = 16,
): number {
  if (targetTop < viewportTop + inset) return targetTop - viewportTop - inset
  if (targetBottom > viewportBottom - inset) return targetBottom - viewportBottom + inset
  return 0
}
