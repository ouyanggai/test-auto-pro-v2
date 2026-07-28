#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-005 真实入口、路径分析、写前重验和稳定错误\n'
go test -count=1 ./test/unit/backend -run '^(TestExecutionPath|TestFlowGraphService)'

printf '验证 F-005 流程入口与四个路径 API 契约\n'
go test -count=1 ./test/contracts -run '^(TestExecutionPathAPI|TestPlanAPIExposesRealPathCount|TestFlowGraphAPI)'

printf '验证 F-005 假目标三类入口、结束状态和会话只重放一次\n'
go test -count=1 ./test/integration -run '^(TestFlowTreeReadUsesExactSourceLookupBeforeDetails|TestFlowTreeSnapshotUsesSourceSpecificEntryNodes|TestDueFlowSnapshotReadsAllWaitingSendPages|TestSubmittedFinishedInstanceIsNotConfigurable|TestSubmittedAwaitSentInstanceRemainsConfigurable|TestFlowTreeReadSessionExpiryReplaysWholeChainOnce)'

printf '验证 F-005 随机临时 MySQL 的迁移、事务、计数器、幂等、归属和重连读取\n'
TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${project_root}/.env.local" \
  go test -count=1 ./test/integration -run '^TestExecutionPathMySQL'

printf '验证 F-005 前端路径状态、API、树形图回归和结构边界\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/execution_path_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/flow_graph_test.mjs
./test/integration/f004_frontend_structure.sh
./test/integration/f005_frontend_structure.sh
./test/integration/f005_go_comments.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build -o .runtime/server ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

git diff --check
printf 'F-005 自动验证全部通过\n'
