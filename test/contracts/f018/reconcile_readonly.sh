#!/usr/bin/env bash
# F-018 对账只读契约：internal/engine/reconcile 不得引用任何写端点或写方法。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-018] 对账包不引用写端点与写出口'
if grep -rnE 'flowInstanceApi|urgeHandleRecord|CallWrite|WriteEndpoint' internal/engine/reconcile/; then
  printf '%s\n' '[F-018] 对账包禁止出现写端点与写出口' >&2
  exit 1
fi

printf '%s\n' '[F-018] 三值结论与四个合法动作固定'
grep -qF 'VerdictEffective     Verdict = "effective"' internal/engine/reconcile/reconcile.go
grep -qF 'VerdictNotEffective  Verdict = "not_effective"' internal/engine/reconcile/reconcile.go
grep -qF 'VerdictIndeterminate Verdict = "indeterminate"' internal/engine/reconcile/reconcile.go
grep -qF 'ActionAdvance' internal/engine/reconcile/reconcile.go
grep -qF 'ActionReplay' internal/engine/reconcile/reconcile.go
grep -qF 'ActionManualEnd' internal/engine/reconcile/reconcile.go
grep -qF 'ActionReconcileAgain' internal/engine/reconcile/reconcile.go

printf '%s\n' '[F-018] 部分生效固定降级与强证据规则在判定器内'
grep -qF 'PartialEffect' internal/engine/reconcile/reconcile.go
grep -qF 'missing' internal/engine/reconcile/reconcile.go

printf '%s\n' '[F-018] 对账只读契约通过'

printf '%s\n' '[F-018] 人工结论登记按纲领第 12 节基线'
# 唯一的人工输入点：必须走组件库表单、三项必填、提交前二次确认，不允许裸 input 拼。
if grep -nE '<input |<select ' web/src/views/RunDetailView.vue; then
  printf '%s\n' '[F-018] 人工结论登记不得使用裸 input/select' >&2
  exit 1
fi
for token in 'NForm' 'manualRules' 'NPopconfirm' '登记人' '实例状态' '当前节点'; do
  grep -qF "${token}" web/src/views/RunDetailView.vue || {
    printf '[F-018] 人工结论表单缺少「%s」\n' "${token}" >&2
    exit 1
  }
done
grep -qF 'manualFormRef.value?.validate()' web/src/views/RunDetailView.vue || {
  printf '%s\n' '[F-018] 人工结论必须先过表单校验再提交' >&2
  exit 1
}

printf '%s\n' '[F-018] 五维证据必须真的读到：已办与审核记录有真实只读实现'
grep -qF 'FindDoneTaskOnNode' internal/adapter/target/client_fact_reads.go
grep -qF 'FindAuditTraceOnNode' internal/adapter/target/client_fact_reads.go
grep -qF 'DoneRecordsRead' internal/engine/control/reconcile.go
grep -qF 'ActionTraceRead' internal/engine/control/reconcile.go

printf '%s\n' '[F-018] 按实例精确复查事实不得附加公司业务关联过滤'
# 根因见语义清单第 19 条：带上 flowInstanceBizRelevanceList 会让本工具自己发起的实例查不到，
# 从而把已生效的发起判成不确定、把对账五维全部变成读不到。这一条用反向断言锁死。
# 只看代码行：函数上方的中文说明里会引用这个字段名解释根因，注释不算违规。
if awk '/^func \(c \*Client\) FindSubmittedFlow/,/^}/' internal/adapter/target/client_fact_reads.go |
  grep -v '^[[:space:]]*//' | grep -qF 'flowInstanceBizRelevanceList'; then
  printf '%s\n' '[F-018] FindSubmittedFlow 不得再带业务关联过滤（会重现实例可见性问题）' >&2
  exit 1
fi

printf '%s\n' '[F-018] 恢复层可达性：不确定写后保留现场、待对账可回运行中、恢复动作不按终态一律拒绝'
grep -qF 'CanRecoverPathRunStatus' internal/model/run.go
grep -qF 'CanRecoverPathRunStatus' internal/engine/control/reconcile.go
grep -qF 'BackFromReconciliation' internal/engine/run/service.go
grep -qF 'PathRunStatusAwaitingReconciliation: {PathRunStatusRunning}' internal/model/run.go
grep -qF 'OutcomeUncertain' internal/engine/control/control.go

printf '%s\n' '[F-018] 重放记账：is_replay 落库、重放上限用真实计数判断'
grep -qF 'IsReplay' internal/engine/step/executor.go
grep -qF 'is_replay' internal/repository/mysql/run_repository_controls.go
grep -qF 'ReplayExhausted' internal/engine/control/reconcile.go
printf '%s\n' '[F-018] 待对账文案按真实结论分支，不出现陈旧文案'
# 重放提示必须按"服务端此刻给出的唯一动作"渲染：按 verdict 渲染会在重放用完后
# 一边显示"唯一动作是重放"一边显示人工登记表单（纲领第 12 节禁止陈旧文案）。
grep -qF "reconcileView.action === 'replay'" web/src/views/RunDetailView.vue || {
  printf '%s\n' '[F-018] 重放提示必须按 action 渲染，不能按 verdict 渲染' >&2
  exit 1
}
grep -qF 'replayExhausted' web/src/views/RunDetailView.vue || {
  printf '%s\n' '[F-018] 重放用完时必须有对应中文说明' >&2
  exit 1
}
# 「表单数据可能已经写进去了」是语义清单第 2.4 节那个特定形状的结论，不得无条件显示。
grep -qF 'partialEffectWarned' web/src/views/RunDetailView.vue || {
  printf '%s\n' '[F-018] 部分生效提示必须按依据判断，不得无条件显示' >&2
  exit 1
}

printf '%s\n' '[F-018] 待对账现场可按运行事实重建，且不提供放行入口'
# 根治「对账现场只在内存里」：写前基准随尝试行落库，现场可按运行事实重建；
# 保留现场不等于还能放行——待对账的可用命令必须是空集。
grep -qF 'before_facts' internal/repository/mysql/migrations/029_f018_attempt_before_facts.sql
grep -qF 'BeforeFacts' internal/engine/step/executor.go
grep -qF 'func (s *Service) Rehydrate' internal/engine/control/rehydrate.go
grep -qF 'ensureReconcileSession' internal/service/run_orchestration.go
grep -qF 'awaitingReconciliation' internal/engine/control/control.go
grep -qF 'PauseStateUncertain' internal/engine/control/control.go
# 基准缺失必须如实降级，不得把零值读成「写之前实例不存在」。
grep -qF 'BeforeUnknown' internal/engine/reconcile/reconcile.go
grep -qF '写之前的目标事实基准没有落库' internal/engine/reconcile/reconcile.go

printf '%s\n' '[F-018] 对账契约全部通过'
