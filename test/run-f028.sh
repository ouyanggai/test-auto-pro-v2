#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

# 集成用例通过本机被 Git 忽略的 .env.local 读取真实 MySQL 配置；go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local，这里显式指向绝对路径避免真实数据库用例被静默跳过。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-${project_root}/.env.local}"
if [ -f "${project_root}/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  . "${project_root}/.env.local"
  set +a
fi

printf '%s\n' '[F-028] 编译与静态检查'
go build ./...
go vet ./internal/engine/control ./internal/engine/run ./internal/repository/... ./internal/service ./internal/api \
  ./test/unit/backend/run ./test/unit/backend/run_orchestration ./test/unit/backend/debugger ./test/integration
if [ -n "$(gofmt -l internal/model/run.go internal/repository/mysql/run_repository.go internal/repository/run.go \
  internal/engine/run/service.go internal/engine/control/control.go internal/engine/control/breakpoints.go \
  internal/service/run_retry.go internal/service/run_orchestration.go internal/api/runs.go \
  test/unit/backend/run/state_machine_test.go test/unit/backend/run_orchestration/run_retry_test.go \
  test/unit/backend/debugger/retry_breakpoints_test.go test/integration/f028_retry_mysql_test.go)" ]; then
  printf '%s\n' '[F-028] 存在未格式化文件' >&2
  exit 1
fi

printf '%s\n' '[F-028] 状态机与重试规则单元测试'
go test -count=1 ./test/unit/backend/run/ ./test/unit/backend/run_orchestration/ ./test/unit/backend/debugger/

printf '%s\n' '[F-028] 关键实现契约'
# 失败 -> 运行中是唯一用户受控的终态出口，必须在模型迁移表中有注释锚点。
grep -qF 'PathRunStatusFailed:    {PathRunStatusRunning}' internal/model/run.go
# 重试装填必须先做前缀一致性校验，且重试本身不发任何写请求。
grep -qF 'planFailedStepRetry' internal/service/run_orchestration.go
grep -qF 'ReopenForRetry' internal/engine/run/service.go
grep -qF 'ArmRetrySession' internal/engine/control/control.go
grep -qF 'ControlFactRetryRequested' internal/model/run.go
# 待对账终局裁决不得被重试绕过：详情的 retryable 必须排除 write_uncertain。
grep -qF 'model.FailureClassWriteUncertain' internal/service/run_orchestration.go
# 界面重试入口只信服务端 retryable 字段。
grep -qF 'detail.retryable' web/src/views/RunDetailView.vue
grep -qF 'props.detail.retryable' web/src/features/runs/RunNodePanel.vue

printf '%s\n' '[F-028] 真实 MySQL 集成测试'
integration_log="$(mktemp -t f028-integration)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF028' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-028] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in \
  TestF028ReopenFailedPathRunForRetry \
  TestF028ReopenRejectsIllegalStates; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-028] 缺少必需用例的通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-028] 前端类型检查'
if [ -x web/node_modules/.bin/vue-tsc ]; then
  (cd web && ./node_modules/.bin/vue-tsc --noEmit)
else
  printf '%s\n' '[F-028] 跳过前端类型检查：web/node_modules/.bin/vue-tsc 不存在' >&2
fi

git diff --check
printf '%s\n' 'F-028 定向验证完成'
