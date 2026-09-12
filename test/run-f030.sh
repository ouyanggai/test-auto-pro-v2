#!/usr/bin/env bash
# F-030 汇总测试入口：后端定向测试 + 前端类型检查与构建。
# 用法：在项目根目录执行 bash test/run-f030.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== F-030 后端定向测试 =="
go test ./test/unit/backend/run_request_timings/ ./test/unit/backend/schedule/ ./test/unit/backend/executor/ ./test/unit/backend/run_orchestration/ ./test/unit/backend/logging/ ./test/unit/backend/target/

echo "== F-030 前端类型检查 =="
(cd web && npx vue-tsc --noEmit)

echo "== F-030 前端构建 =="
(cd web && npx vite build --logLevel error)

echo "== F-030 全部通过 =="
