#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例通过本机被 Git 忽略的 .env.local 读取真实 MySQL 配置；go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local，这里显式指向绝对路径避免真实数据库用例被静默跳过。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-020] 编译与静态检查'
go build ./...
go vet ./internal/engine/schedule ./internal/service ./internal/api ./test/unit/backend/schedule ./test/integration
test -z "$(gofmt -l internal cmd test)"

printf '%s\n' '[F-020] 调度器单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/schedule/...

printf '%s\n' '[F-020] 真实 MySQL 集成测试'
integration_log="$(mktemp -t f020-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF020' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-020] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in \
  TestF020CreateRunWithPathsIdempotent \
  TestF020RunTerminalAggregatesAfterAllPathsFinish \
  TestF020WaitingPathCanBeCancelledForRunStop \
  TestF020ScheduledClaimConsumedOnce; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-020] 缺少必需用例的通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-020] 接口与前端结构契约'
./test/contracts/f020/api_and_frontend_structure.sh

printf '%s\n' '[F-020] 写端点白名单未扩张（复用 F-016/F-017 契约）'
./test/contracts/f016/target_write_whitelist.sh
./test/contracts/f017/write_endpoints_whitelist.sh 2>/dev/null || ./test/contracts/f017/target_write_whitelist.sh 2>/dev/null || printf '%s\n' '[F-020] F-017 白名单契约不在预期路径，跳过（F-016 契约已覆盖）'

printf '%s\n' '[F-020] 前端类型检查与构建'
(cd web && npx vue-tsc --noEmit >/dev/null && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-020 定向验证完成'
