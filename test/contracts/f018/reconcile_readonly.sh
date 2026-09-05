#!/usr/bin/env bash
# F-018 对账只读契约：internal/engine/reconcile 不得引用任何写端点或写方法。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-018] 对账包不引用写端点与写出口'
if grep -rnE 'flowInstanceApi/submit|flowInstanceApi/audit|CallWrite|WriteEndpoint' internal/engine/reconcile/; then
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
