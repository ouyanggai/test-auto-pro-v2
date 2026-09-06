#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

# 2026-09-06 产品裁决：用户侧对账功能整体移除。本脚本锁定移除后的行为：
# 不确定写是终局、运行聚合随之收尾、重启恢复不留僵尸运行；安全必需的内部判定与日志保留。

printf '%s\n' '[F-018] 编译与静态检查'
go build ./...
go vet ./internal/engine/control/... ./internal/service/ ./test/integration

printf '%s\n' '[F-018] 真实 MySQL 集成测试（移除用户侧对账后的终局行为 + F-016/F-017/F-020 回归）'
integration_log="$(mktemp -t f018-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF018UncertainWriteClosesRunAggregation|TestF018RecoveredRunsCloseAggregatesAfterRestart|TestF016|TestF017|TestF020|TestF021' ./test/unit/backend/executor ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-018] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF018UncertainWriteClosesRunAggregation TestF018RecoveredRunsCloseAggregatesAfterRestart \
  TestF016MigrationCreatesRunRecordTables TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive \
  TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite \
  TestF017PausedPathRunSurvivesRestart; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-018] 缺少必需的集成用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-018] 真实目标只读验证（核验重读与实例可见性仍然依赖的读取口径）'
# 全程只读，不发任何写请求。按实例精确复查不带业务关联过滤是 F-016 遗留问题的修复根基，
# 移除用户侧对账后核验重读（verify）依旧依赖这条口径，必须持续锁定。
readonly_log="$(mktemp -t f018-readonly)"
if ! go test -count=1 -v -run 'TestF018DimensionReadsAgainstRealTarget|TestF018AuditTraceMatchesRealNode|TestF018DoneRecordMatchesRealDoneTask|TestF018ToolCreatedInstanceIsVisibleByExactLookup|TestF018CompanyRelevanceFilterHidesToolCreatedInstance' ./test/integration 2>&1 | tee "${readonly_log}"; then
  rm -f "${readonly_log}"
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${readonly_log}"; then
  printf '%s\n' '[F-018] 真实目标只读用例被跳过，判定为失败' >&2
  rm -f "${readonly_log}"
  exit 1
fi
for required in TestF018DimensionReadsAgainstRealTarget TestF018AuditTraceMatchesRealNode TestF018DoneRecordMatchesRealDoneTask \
  TestF018ToolCreatedInstanceIsVisibleByExactLookup TestF018CompanyRelevanceFilterHidesToolCreatedInstance; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${readonly_log}"; then
    printf '[F-018] 缺少必需的真实目标只读用例通过记录：%s\n' "${required}" >&2
    rm -f "${readonly_log}"
    exit 1
  fi
done
rm -f "${readonly_log}"

printf '%s\n' '[F-018] 移除用户侧对账契约'
./test/contracts/f018/reconcile_readonly.sh
printf '%s\n' '[F-018] 写端点白名单未扩张'
./test/contracts/f017/target_write_whitelist.sh

printf '%s\n' '[F-018] 前端构建'
(cd web && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-018 定向验证完成'
