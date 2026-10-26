import type { RunStep } from './api'
import { formatElapsed } from './api'
import { actionLabel } from './presentation'

// 自动跳过没有执行动作；导航只有只读确认。必须先区分事实，再描述处理人与耗时。
export function stepOutcome(step: RunStep, blocked = false): string {
  if (blocked) return '阻塞'
  if (step.status === 'skipped') return '已跳过'
  if (step.status === 'failed') return '失败'
  if (step.status === 'uncertain') return '结果待确认'
  if (step.status === 'succeeded') return step.source === 'system_navigation' ? '节点已确认' : '成功'
  return step.statusName
}

export function stepNoActionSummary(step: RunStep, nodeName: string): string {
  if (step.status === 'skipped') return `目标平台自动跳过「${nodeName}」，未执行「${actionLabel(step.actionName, step.action)}」动作。`
  if (step.source === 'system_navigation' && step.status === 'succeeded') return `已只读确认「${nodeName}」的流程状态，未执行审批动作。`
  return ''
}

// 总时长只取完整步骤测量或有效起止时间；尝试耗时只说明已记录部分，绝不借接口传输时间补总时长。
export function stepElapsedText(step: RunStep): string {
  if (step.status === 'skipped') return '未执行动作，无动作耗时'
  const navigation = step.source === 'system_navigation'
  const timestampMs = Date.parse(step.finishedAt) - Date.parse(step.startedAt)
  const total = step.durationMs > 0 ? step.durationMs : timestampMs > 0 ? timestampMs : 0
  if (Number.isFinite(total) && total > 0) return `${navigation ? '确认用时' : '总耗时'} ${formatElapsed(total)}`
  const attempts = step.attempts.map(attempt => attempt.durationMs).filter(ms => Number.isFinite(ms) && ms > 0)
  if (attempts.length) return `总耗时未记录；已记录尝试耗时 ${formatElapsed(attempts.reduce((sum, ms) => sum + ms, 0))}`
  return navigation ? '只读确认，耗时未记录' : '总耗时未记录'
}
