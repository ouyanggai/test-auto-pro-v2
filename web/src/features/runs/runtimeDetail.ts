// T013 runtime 执行详情的展示口径：把后端三阶段投影映射为运行详情的中文视图。
// 纪律：业务失败不显示为流程失败；结果待确认不显示重试按钮；不展示 SID 与完整正文。

export type StageState = 'pending' | 'running' | 'done' | 'failed' | 'uncertain'

export interface RuntimeStageInput {
  name: string
  state: StageState
  note?: string
  requests: number
}

export interface RuntimeStageView {
  name: string
  stateLabel: string
  requests: number
  tone: 'ok' | 'bad' | 'warn' | 'idle'
}

// RUNTIME_STATUS_LABELS 是七值判定的中文口径；业务失败与流程失败分开表达。
export const RUNTIME_STATUS_LABELS: Record<string, string> = {
  success: '执行成功',
  failed: '执行失败',
  target_rejected: '目标平台拒绝',
  uncertain: '结果待确认',
  business_failed: '流程已推进，后置业务未完成',
  unsupported: '当前能力未开放',
  timeout: '执行超时'
}

// runtimeStatusLabel 返回状态中文；未知状态显示安全占位，不渲染原始键。
export function runtimeStatusLabel (status: string): string {
  return RUNTIME_STATUS_LABELS[status] ?? '结果待确认'
}

// runtimeRetryable 判定是否允许重试：只有确定无目标副作用的失败可重试；
// 结果待确认（uncertain）与超时（timeout）一律不允许重试入口。
export function runtimeRetryable (status: string): boolean {
  return status === 'failed' || status === 'target_rejected'
}

// runtimeStageViews 把三阶段投影映射为带中文状态与色调的视图列表。
// 阶段顺序固定：表单业务处理 → 流程动作 → 提交后业务处理。
export function runtimeStageViews (stages: RuntimeStageInput[]): RuntimeStageView[] {
  const stateLabels: Record<StageState, string> = {
    pending: '未开始',
    running: '进行中',
    done: '完成',
    failed: '未完成',
    uncertain: '结果待确认'
  }
  const tones: Record<StageState, RuntimeStageView['tone']> = {
    pending: 'idle',
    running: 'warn',
    done: 'ok',
    failed: 'bad',
    uncertain: 'warn'
  }
  return stages.map(stage => ({
    name: stage.name,
    stateLabel: stateLabels[stage.state] ?? stage.state,
    requests: stage.requests,
    tone: tones[stage.state] ?? 'idle'
  }))
}
