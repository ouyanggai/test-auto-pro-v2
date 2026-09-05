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

printf '%s\n' '[F-019] 编译与静态检查'
go build ./...
go vet ./internal/engine/step/... ./internal/adapter/target/... ./test/unit/backend/executor

printf '%s\n' '[F-019] 回归：F-016/F-017/F-018 单元与集成（真实 MySQL，无跳过）'
integration_log="$(mktemp -t f019-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF01[678]' ./test/unit/backend/executor ./test/unit/backend/debugger ./test/unit/backend/reconcile ./test/unit/backend/run ./test/unit/backend/target ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-019] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF016SingleStepControlLoop TestF016StopControl TestF016StepLogBidirectionalReachability TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite TestF017PausedPathRunSurvivesRestart; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-019] 缺少必需的集成用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-019] 动作白名单契约（11 端点）'
./test/contracts/f019/action_whitelist.sh
printf '%s\n' '[F-018] 对账只读契约'
./test/contracts/f018/reconcile_readonly.sh
printf '%s\n' '[F-017] 写端点白名单契约'
./test/contracts/f017/target_write_whitelist.sh

printf '%s\n' '[F-019] 前端构建'
(cd web && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-019 定向验证完成'
