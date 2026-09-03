#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-012] 后端内部编译与测试'
go test -count=1 ./internal/...
go test -count=1 ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012
go test -count=1 ./test/integration/f012
go test -race -count=1 ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012
go vet ./internal/... ./test/unit/backend/history_replay ./test/unit/backend/action_orchestration ./test/contracts/f012

printf '%s\n' '[F-012] 前端运行时协议测试'
node --no-warnings --experimental-strip-types --test \
  test/unit/frontend/history_replay/*.mjs \
  test/unit/frontend/action_orchestration/*.mjs \
  test/unit/frontend/execution_path_test.mjs \
  test/unit/frontend/form_runtime_test.mjs

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
