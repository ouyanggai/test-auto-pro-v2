#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例通过 config.LoadPlanDBConfig 读取本机忽略配置，而 go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local。这里显式指向绝对路径，避免真实依赖用例被静默跳过。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-${project_root}/.env.local}"

printf '%s\n' '[F-013] 编译与静态检查'
go build ./...
go vet ./internal/... ./cmd/... ./test/unit/backend/logging ./test/integration

printf '%s\n' '[F-013] 日志底座单元测试（含竞态检测）'
go test -race -count=1 ./test/unit/backend/logging/...

printf '%s\n' '[F-013] 日志集成测试'
integration_log="$(mktemp -t f013-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestTargetRequestLogging|TestAPIFailureLog|TestAPIPanicReturns' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-013] 日志集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in TestTargetRequestLoggingRecordsFailureAndReplayableCurl TestAPIFailureLogMatchesResponseMessage TestAPIPanicReturnsStableChineseErrorWithoutStack; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '%s\n' "[F-013] 缺少必需的集成用例通过记录：${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-013] 目标写端点白名单为空'
./test/contracts/f013/target_write_whitelist.sh

printf '%s\n' '[F-013] 日志目录未进入版本库'
if ! git check-ignore -q logs/app.log; then
  printf '%s\n' '[F-013] logs/ 必须被 .gitignore 忽略' >&2
  exit 1
fi
git status --porcelain logs 2>/dev/null | grep -q . && {
  printf '%s\n' '[F-013] logs/ 出现在待提交列表' >&2
  exit 1
}

git diff --check
printf '%s\n' 'F-013 定向验证完成'
