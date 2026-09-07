#!/usr/bin/env bash
# F-018 契约（2026-09-06 产品裁决后）：用户侧对账功能整体移除，
# 写结果无法确认是终局；工具只保留安全必需的内部判定与日志，不把处理责任推给用户。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-018] 用户侧对账入口已彻底移除'
if grep -rnE 'reconcile|recovery' internal/api/runs.go; then
  printf '%s\n' '[F-018] 运行 API 不得再暴露对账与恢复动作端点' >&2
  exit 1
fi
if grep -rnE 'ReconcileNow|RecoveryAction|ReconcileView' internal/service/ internal/engine/control/; then
  printf '%s\n' '[F-018] 服务与控制层不得再提供用户侧对账与恢复动作' >&2
  exit 1
fi
if [ -d internal/engine/reconcile ]; then
  printf '%s\n' '[F-018] 对账判定引擎包应已删除（用户侧对账已移除，内部不再重建）' >&2
  exit 1
fi
if grep -rnE 'reconcileNow|recoveryAction|ReconcileView|run-detail__reconcile|manualForm' web/src/; then
  printf '%s\n' '[F-018] 前端不得再保留对账工作区、人工登记表单或相关状态' >&2
  exit 1
fi

printf '%s\n' '[F-018] 写结果无法确认是终局：无出边、无恢复动作、聚合收尾'
grep -qF 'PathRunStatusAwaitingReconciliation: {}' internal/model/run.go
if grep -rqF 'CanRecoverPathRunStatus' internal/model/run.go internal/engine/ internal/service/; then
  printf '%s\n' '[F-018] 「可恢复停摆态」判据应已随恢复动作一并删除' >&2
  exit 1
fi
grep -qF 'FinishRunIfAllPathsClosed' internal/engine/control/control.go
grep -qF 'PathRunStatusAwaitingReconciliation, model.RunStatusStopped' internal/repository/mysql/run_repository.go

printf '%s\n' '[F-018] 不确定写绝不自动继续、重放或重复发送真实写请求'
grep -qF '一次尝试最多一次写请求' internal/engine/step/executor.go || \
  grep -qF 'preview.writeSent = true' internal/engine/step/executor.go
if grep -rnE 'IsReplay\s*=\s*true' internal/engine/; then
  printf '%s\n' '[F-018] 移除恢复动作后不得再产生重放尝试' >&2
  exit 1
fi

printf '%s\n' '[F-018] 现场丢失的大白话说明与一次返回（禁止跳转循环）'
grep -qF 'sceneLost' internal/service/run_orchestration.go
grep -qF 'sceneLost' web/src/features/runs/api.ts
# 终局运行必须能从运行记录重新进入详情查看；现场丢失只提供只读说明，
# 不得在详情加载时自动跳回列表，否则用户无法查看失败证据。
grep -qF 'sceneLostNote' web/src/views/RunDetailView.vue
if grep -qF "router.push('/runs')" web/src/views/RunDetailView.vue; then
  printf '%s\n' '[F-018] 运行详情不得因 sceneLost 自动跳回运行列表' >&2
  exit 1
fi
if grep -qF 'sceneLostBouncing' web/src/views/RunDetailView.vue; then
  printf '%s\n' '[F-018] 运行详情不得保留现场丢失跳转状态' >&2
  exit 1
fi

printf '%s\n' '[F-018] 显示名口径：结果待确认（不再出现「待对账」内部术语）'
grep -qF '"结果待确认"' internal/model/run.go
if grep -rnF '待对账' web/src/ internal/service/ internal/api/; then
  printf '%s\n' '[F-018] 界面与服务文案不得再出现「待对账」' >&2
  exit 1
fi

printf '%s\n' '[F-018] 安全必需的内部判定与日志仍然保留'
# 写前基准随尝试行落库（迁移 029）与实例可见性修复是核验重读正确性的根基，不随用户界面对账删除。
grep -qF 'before_facts' internal/repository/mysql/migrations/029_f018_attempt_before_facts.sql
grep -qF 'BeforeFacts' internal/engine/step/executor.go
grep -qF 'FindSubmittedFlow' internal/adapter/target/client_fact_reads.go
# recovery.log 继续记录写结果无法确认的内部判定。
grep -qF 'SetRecoveryLog' cmd/server/main.go
grep -qF 'write_uncertain=1' internal/engine/control/control.go

printf '%s\n' '[F-018] 按实例精确复查事实不得附加公司业务关联过滤'
# 根因见语义清单第 19 条：带上 flowInstanceBizRelevanceList 会让本工具自己发起的实例查不到，
# 从而把已生效的发起误判为不确定。这一条用反向断言锁死。
# 只看代码行：函数上方的中文说明里会引用这个字段名解释根因，注释不算违规。
if awk '/^func \(c \*Client\) FindSubmittedFlow/,/^}/' internal/adapter/target/client_fact_reads.go |
  grep -v '^[[:space:]]*//' | grep -qF 'flowInstanceBizRelevanceList'; then
  printf '%s\n' '[F-018] FindSubmittedFlow 不得再带业务关联过滤（会重现实例可见性问题）' >&2
  exit 1
fi

printf '%s\n' '[F-018] 契约全部通过'
