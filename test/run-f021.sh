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

printf '%s\n' '[F-021] 编译与静态检查'
go build ./...
go vet ./internal/service ./internal/api ./test/integration
test -z "$(gofmt -l internal cmd test)"

printf '%s\n' '[F-021] 真实 MySQL 集成测试（事件增量与状态筛选）'
integration_log="$(mktemp -t f021-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF021RunEvents' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-021] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestF021RunEventsIncrementalAndStatusFilter; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-021] 缺少必需用例的通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-021] 只读契约'
./test/contracts/f021/api_and_readonly.sh

printf '%s\n' '[F-021] 前端类型检查与构建'
(cd web && npx vue-tsc --noEmit >/dev/null && npm run build >/dev/null)

git diff --check
printf '%s\n' 'F-021 定向验证完成'
