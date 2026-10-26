import type { ExecutionPath } from '../execution-paths/types.ts'
import { isExecutionPathRunnable } from '../execution-paths/logic.ts'
import type { HistoryReplayItem } from './types.ts'

export interface PreparationOutcome {
  tone: 'success' | 'warning'
  message: string
  details: Array<{ pathId: string; name: string; reason: string }>
}

// 按本次任务的路径明细核对双状态，不能把处理结束或单纯数据成功当成整条路径准备成功。
// 本地双状态就绪仍须经过服务端对真实结构与人员的运行前检查。
export function preparationOutcome(paths: ExecutionPath[], items: HistoryReplayItem[]): PreparationOutcome {
  const byId = new Map(paths.map(path => [path.id, path]))
  const details: PreparationOutcome['details'] = []
  for (const item of items) {
    const pathId = String(item.pathId), path = byId.get(pathId)
    const issue = item.issues.find(issue => issue.code === 'AUTO_ACTION_CONFIGURE_FAILED' || issue.blocking)
    if (path && isExecutionPathRunnable(path) && item.status === 'ready' && !issue) continue
    const reason = issue?.message || (!path ? '路径已删除或状态未能读取'
      : path.configurationStatus !== 'configured' ? path.configurationDetail || '节点尚未准备完成，请检查处理人规则'
        : path.dataDetail || '表单数据仍需补充')
    details.push({ pathId, name: path?.name?.trim() || (path ? `路径 ${path.sequenceNo}` : '本次路径'), reason })
  }
  const prepared = items.length - details.length
  return {
    tone: details.length ? 'warning' : 'success',
    message: details.length ? `准备结束：${prepared} 条已准备，${details.length} 条仍需处理。请查看下方原因。`
      : `${prepared} 条路径已准备，下一步点击“检查并运行”。`,
    details,
  }
}
