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

printf '%s\n' '[F-018] 编译与静态检查'
go build ./...
go vet ./internal/engine/reconcile/... ./internal/engine/control/... ./test/unit/backend/reconcile

printf '%s\n' '[F-018] 对账判定单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/reconcile/...

printf '%s\n' '[F-018] 恢复链路真实 MySQL 集成测试（待对账 → 对账 → 三个唯一动作 → 重放为新尝试）'
recovery_log="$(mktemp -t f018-recovery)"
if ! go test -count=1 -v -run 'TestF018NotEffectiveLeadsToReplay|TestF018ReplayLimitIsEnforced|TestF018EffectiveLeadsToAdvanceOnly|TestF018MissingDimensionDegradesToManualEnd' ./test/unit/backend/executor 2>&1 | tee "${recovery_log}"; then
  rm -f "${recovery_log}"
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${recovery_log}"; then
  printf '%s\n' '[F-018] 恢复链路用例被跳过，判定为失败' >&2
  rm -f "${recovery_log}"
  exit 1
fi
for required in TestF018NotEffectiveLeadsToReplay TestF018ReplayLimitIsEnforced TestF018EffectiveLeadsToAdvanceOnly TestF018MissingDimensionDegradesToManualEnd; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${recovery_log}"; then
    printf '[F-018] 缺少必需的恢复链路用例通过记录：%s\n' "${required}" >&2
    rm -f "${recovery_log}"
    exit 1
  fi
done
rm -f "${recovery_log}"

printf '%s\n' '[F-018] 真实 MySQL 集成测试（F-016/F-017 全量回归，含迁移 028 应用）'
integration_log="$(mktemp -t f018-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF01[678]' ./test/unit/backend/executor ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-018] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF016MigrationCreatesRunRecordTables TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite TestF017PausedPathRunSurvivesRestart; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-018] 缺少必需的集成用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-018] 真实目标只读维度验证（已办记录与动作痕迹）'
# 这两维决定「未生效 → 允许重放」是否成立，而重放会再写一次：必须用真实账号证明读得到，
# 不能只靠源码推断响应形状。全程只读，不发任何写请求。
readonly_log="$(mktemp -t f018-readonly)"
trap 'rm -f "${readonly_log}"' EXIT
if ! go test -count=1 -v -run 'TestF018DimensionReadsAgainstRealTarget|TestF018AuditTraceMatchesRealNode|TestF018DoneRecordMatchesRealDoneTask|TestF018ToolCreatedInstanceIsVisibleByExactLookup|TestF018CompanyRelevanceFilterHidesToolCreatedInstance' ./test/integration 2>&1 | tee "${readonly_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${readonly_log}"; then
  printf '%s\n' '[F-018] 真实目标只读用例被跳过，判定为失败' >&2
  exit 1
fi
for required in TestF018DimensionReadsAgainstRealTarget TestF018AuditTraceMatchesRealNode TestF018DoneRecordMatchesRealDoneTask \
  TestF018ToolCreatedInstanceIsVisibleByExactLookup TestF018CompanyRelevanceFilterHidesToolCreatedInstance; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${readonly_log}"; then
    printf '[F-018] 缺少必需的真实目标只读用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-018] 对账只读契约'
./test/contracts/f018/reconcile_readonly.sh
printf '%s\n' '[F-018] 写端点白名单未扩张'
./test/contracts/f017/target_write_whitelist.sh

printf '%s\n' '[F-018] 前端构建'
(cd web && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-018 定向验证完成'
