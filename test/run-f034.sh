#!/usr/bin/env bash
# F-034 汇总测试入口：一键配置动作多样化与运行详情可读性修复的定向验证。
# 用法：在项目根目录执行 bash test/run-f034.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== F-034 编译与静态检查 =="
go build ./...
go vet ./internal/service/... ./internal/engine/... ./test/unit/backend/action_orchestration/ ./test/unit/backend/run_orchestration/

echo "== F-034 后端定向测试（动作矩阵 / 分支阻塞 / 语义判定 / 请求摘要） =="
go test ./test/unit/backend/action_orchestration/ ./test/unit/backend/run_orchestration/ ./test/unit/backend/target_semantics/ ./test/unit/backend/run_request_timings/ ./test/unit/backend/executor/ ./test/unit/backend/run/

echo "== F-034 前端定向测试（动作名映射 / 展示结构） =="
node --experimental-strip-types --test test/unit/frontend/action_label_test.mjs test/unit/frontend/run_detail_scene_lost_test.mjs

echo "== F-034 前端类型检查 =="
(cd web && npx vue-tsc --noEmit)

echo "== F-034 前端构建 =="
(cd web && npx vite build --logLevel error)

echo "== F-034 全部通过 =="
