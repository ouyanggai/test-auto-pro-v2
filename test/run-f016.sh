#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例要连真实 MySQL，配置只放本机被 Git 忽略的 .env.local。
# 本脚本全部为只读验证：目标写只发生在 test/manual/f016_real_write_runbook.md 的手动流程里。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-016] 编译与静态检查'
go build ./...
go vet ./internal/engine/... ./internal/adapter/target/... ./internal/repository/... ./internal/service/... ./internal/api/... ./test/unit/backend/executor ./test/unit/backend/run ./test/unit/backend/target ./test/integration

printf '%s\n' '[F-016] 执行器与状态机单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/executor/... ./test/unit/backend/run/... ./test/unit/backend/target/...

printf '%s\n' '[F-016] 真实 MySQL 集成测试（迁移、状态机、租约、崩溃恢复、事实表、API）'
integration_log="$(mktemp -t f016-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF016' ./test/integration ./test/unit/backend/executor 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-016] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF016MigrationCreatesRunRecordTables TestF016RunStateAllocatesMonotonicRunNumbersAndGuardsStatus TestF016LeaseMutexAndFencing TestF016CrashRecoveryForcesAwaitingReconciliation TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive TestF016RunsAPIGuards; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-016] 缺少必需的集成用例通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-016] 写端点白名单为两个端点的子集'
./test/contracts/f016/target_write_whitelist.sh

printf '%s\n' '[F-016] 接口与前端结构契约'
./test/contracts/f016/api_and_frontend_structure.sh

printf '%s\n' '[F-016] 前端构建'
(cd web && npm run build >/dev/null)

printf '%s\n' '[F-016] 日志目录未进入版本库'
git check-ignore -q logs/app.log || { printf '%s\n' '[F-016] logs/ 必须被 .gitignore 忽略' >&2; exit 1; }

git diff --check
printf '%s\n' 'F-016 定向验证完成'
