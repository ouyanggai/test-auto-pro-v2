#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-004 流程树来源核对与图分析\n'
go test -count=1 ./test/unit/backend -run '^TestFlowGraph'

printf '验证 F-004 流程图 API 契约与安全边界\n'
go test -count=1 ./test/contracts -run '^TestFlowGraph'

printf '验证 F-004 三类目标请求顺序与代理 ID 转换\n'
go test -count=1 ./test/integration -run '^TestFlowTreeRead'

printf '验证 F-004 前端布局、错误、取消和首次适配\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/flow_graph_test.mjs

printf '验证 F-004 前端只读结构与依赖边界\n'
./test/integration/f004_frontend_structure.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build -o .runtime/server ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

printf 'F-004 自动验证全部通过\n'
