#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例通过 config.LoadPlanDBConfig 读取本机忽略配置，而 go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local。这里显式指向绝对路径，避免真实 MySQL 用例被静默跳过。
plan_db_env_file="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
if [[ ! -f "${plan_db_env_file}" ]]; then
  printf '%s\n' "[F-012] 缺少计划数据库本机配置：${plan_db_env_file}" >&2
  exit 1
fi
for required in PLAN_DB_HOST PLAN_DB_USER PLAN_DB_PASSWORD; do
  if [[ -n "${!required:-}" ]]; then
    continue
  fi
  if ! grep -Eq "^[[:space:]]*(export[[:space:]]+)?${required}=..*" "${plan_db_env_file}"; then
    printf '%s\n' "[F-012] 计划数据库配置缺少 ${required}，真实数据库用例无法执行" >&2
    exit 1
  fi
done
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${plan_db_env_file}"

printf '%s\n' '[F-012] 后端内部编译与测试'
go test -count=1 ./internal/...
go test -count=1 ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012

printf '%s\n' '[F-012] 真实数据库集成测试'
# 目标业务库只读连接是候选列表的可选加速路径；配置了 TARGET_DB_* 时该用例必须真实执行。
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"
integration_log="$(mktemp -t f012-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v ./test/integration/f012 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-012] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
if ! grep -Eq -- '^[[:space:]]*--- PASS' "${integration_log}"; then
  printf '%s\n' '[F-012] 集成测试没有任何通过用例，判定为失败' >&2
  exit 1
fi
go test -race -count=1 ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012
go vet ./internal/... ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012

printf '%s\n' '[F-012] 前端运行时协议测试'
node --no-warnings --experimental-strip-types --test \
  test/unit/frontend/history_replay/*.mjs \
  test/unit/frontend/action_orchestration/*.mjs \
  test/unit/frontend/execution_path_test.mjs \
  test/unit/frontend/form_runtime_test.mjs

printf '%s\n' '[F-012] 动作编排界面结构检查'
./test/unit/frontend/action_orchestration/action_orchestration_structure.sh

printf '%s\n' '[F-012] 旧入口静态清理'
./test/contracts/f012/legacy_system_removal.sh

printf '%s\n' '[F-012] 类型检查与构建'
pnpm --dir web typecheck
pnpm --dir form-runtime typecheck
pnpm --dir web build
pnpm --dir form-runtime build
go build -o /tmp/test-auto-pro-v2-f012-server ./cmd/server

git diff --check
printf '%s\n' 'F-012 定向验证完成'
