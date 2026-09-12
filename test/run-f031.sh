#!/usr/bin/env bash
# F-031 汇总测试入口：后端定向测试 + 结构契约 + 前端类型检查与构建。
# 用法：在项目根目录执行 bash test/run-f031.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== F-031 后端定向测试 =="
go test ./test/unit/backend/target/ ./test/unit/backend/executor/ ./test/unit/backend/logging/ ./test/unit/backend/run_orchestration/ ./test/unit/backend/run_request_timings/

echo "== F-031 结构契约检查 =="
bash test/contracts/f031/task_endpoint_contract.sh

echo "== F-031 前端类型检查 =="
(cd web && npx vue-tsc --noEmit)

echo "== F-031 前端构建 =="
(cd web && npx vite build --logLevel error)

echo "== F-031 全部通过 =="
