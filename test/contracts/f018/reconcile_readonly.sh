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
