#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-006 条件、人员、动作、约束、分组和当前路径重验\n'
go test -count=1 ./test/unit/backend -run '^(TestPathRequirement|TestExecutionPathAnalyzer)'

printf '验证 F-006 只读 API、归属隔离、路径失效和公开字段边界\n'
go test -count=1 ./test/contracts -run '^TestPathRequirementAPI'

printf '验证 F-006 三类来源的流程详情与模板/代理表单字段读取\n'
go test -count=1 ./test/integration -run '^(TestFlowRequirementSnapshotReadsSourceSpecificFormMetadata|TestFlowTreeReadUsesExactSourceLookupBeforeDetails|TestFlowTreeReadSessionExpiryReplaysWholeChainOnce)'

printf '验证 F-006 前端路径切换、F-005 路径遍历和 F-004 图布局必要回归\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/path_requirements_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/execution_path_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/flow_graph_test.mjs
./test/integration/f004_frontend_structure.sh
./test/integration/f005_frontend_structure.sh
./test/integration/f006_frontend_structure.sh
./test/integration/f006_go_comments.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build -o .runtime/server ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

git diff --check
printf 'F-006 自动验证全部通过\n'
