#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 只读验证入口：目标写只发生在 test/manual/f016_real_write_runbook.md 的手动流程里。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-017] 编译与静态检查'
go build ./...
go vet ./internal/engine/control/... ./internal/api/... ./test/unit/backend/debugger ./test/unit/backend/executor

printf '%s\n' '[F-017] 断点与命令集单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/debugger/...

printf '%s\n' '[F-017] 真实 MySQL 集成测试（控制事实 append-only、断点回放、幂等放行、暂停态重启）'
integration_log="$(mktemp -t f017-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF017' ./test/unit/backend/executor 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-017] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite TestF017PausedPathRunSurvivesRestart; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-017] 缺少必需的集成用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-017] 写端点白名单未扩张'
./test/contracts/f017/target_write_whitelist.sh

printf '%s\n' '[F-017] 接口与前端结构契约'
./test/contracts/f017/api_and_frontend_structure.sh

printf '%s\n' '[F-017] 前端构建'
(cd web && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-017 定向验证完成'
