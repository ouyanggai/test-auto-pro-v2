#!/usr/bin/env bash
# F-032 汇总测试入口：计划状态与最近运行结果修复的定向验证。
# 用法：在项目根目录执行 bash test/run-f032.sh（需要 .env.local 提供真实 MySQL 配置）。
set -euo pipefail
cd "$(dirname "$0")/.."

# 集成用例通过本机被 Git 忽略的 .env.local 读取真实 MySQL 配置；go test 的工作目录是包目录，
# 相对路径读不到项目根的 .env.local，这里显式指向绝对路径避免真实数据库用例被静默跳过。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-$(pwd)/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-$(pwd)/.env.local}"
if [ -f .env.local ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env.local
  set +a
fi

echo "== F-032 编译与静态检查 =="
go build ./...
go vet ./internal/repository/... ./internal/api/... ./test/integration/ ./test/unit/backend/

echo "== F-032 后端定向测试（单元 + 真实 MySQL 集成） =="
go test ./test/unit/backend/ -run 'TestPlanLastRunText|TestPlanServiceUpdatesPlanAfterRun|TestFlowGraphServiceInvalidatesCacheAfterPlanBindingChange'
go test ./test/contracts/ -run 'TestPlanAPIContractsAndIdempotency|TestPlanAPIParameterAndStableErrorContracts'
go test ./test/integration/ -run 'TestF032|TestPlanMySQLMigrationCRUDIdempotencyAndRestartRead'

echo "== F-032 前端类型检查 =="
(cd web && npx vue-tsc --noEmit)

echo "== F-032 前端构建 =="
(cd web && npx vite build --logLevel error)

echo "== F-032 全部通过 =="
