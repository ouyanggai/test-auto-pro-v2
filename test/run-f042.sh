#!/usr/bin/env bash
# F-042 报告中心真实度量（总分总）汇总测试入口。
# 覆盖本轮返工的四个方向：性能（一次查询 + 只扫 network.log + 并发 + 终态缓存）、
# 层级收敛与名词解释、数据口径（分子/分母/样本数/数据截至时间）、界面三态与下钻。
# 用法：在项目根目录执行 bash test/run-f042.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# 真实 MySQL 集成用例通过本机被 Git 忽略的 .env.local 读取数据库配置（与 F-020 同纪律）。
export TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${TEST_AUTO_PRO_PLAN_DB_ENV_FILE:-$(pwd)/.env.local}"
export TEST_AUTO_PRO_TARGET_ENV_FILE="${TEST_AUTO_PRO_TARGET_ENV_FILE:-$(pwd)/.env.local}"

printf '%s\n' '[F-042] 编译与静态检查'
go build ./...
go vet ./internal/service ./internal/repository/... ./internal/api ./cmd/server ./test/integration

printf '%s\n' '[F-042] 快速聚合与性能门禁（单元）'
go test -count=1 ./test/unit/backend/report_center_scan/
go test -count=1 ./test/unit/backend/run_request_timings/

printf '%s\n' '[F-042] 真实 MySQL 集成（聚合口径 / executor_ms / 真实 network.log）'
# 不用带前缀的模板：Windows 上的 mktemp 只接受末尾带 X 的模板，直接取临时文件最稳妥。
integration_log="$(mktemp)"
trap 'rm -f "${integration_log}"' EXIT
if ! go test -count=1 -v -run 'TestF042' ./test/integration 2>&1 | tee "${integration_log}"; then
  exit 1
fi
if grep -Eq -- '^[[:space:]]*--- SKIP' "${integration_log}"; then
  printf '%s\n' '[F-042] 集成测试存在跳过用例，判定为失败' >&2
  exit 1
fi
for required in \
  TestF042ReportAggregation \
  TestF042ReportInterfaceStatsFromNetworkLog \
  TestF042ExecutorMsRoundTrip; do
  if ! grep -Eq -- "^[[:space:]]*--- PASS: ${required}" "${integration_log}"; then
    printf '[F-042] 缺少必需用例的通过记录：%s\n' "${required}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-042] 前端术语表与数据口径（node）'
node --experimental-strip-types --test test/unit/frontend/report_glossary_test.mjs
node --experimental-strip-types --test test/unit/frontend/report_metrics_format_test.mjs
node --experimental-strip-types --test test/unit/frontend/report_views_structure_test.mjs

printf '%s\n' '[F-042] 前端类型检查与构建'
(cd web && npx vue-tsc --noEmit)
(cd web && npx vite build --logLevel error)

printf '%s\n' '[F-042] 差异检查'
git diff --check

printf '%s\n' 'F-042 定向验证完成'
