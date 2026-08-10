#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-007 字段/动作投影、保存校验、幂等、修订号与 F-006 必要回归\n'
go test -count=1 ./test/unit/backend -run '^(TestPathConfig|TestPathRequirement|TestExecutionPath|TestFormRuntimeMaintenance|TestPnpmOperator|TestMaintenance|TestManifest)'

printf '验证 F-007 配置 API 契约与 F-006/F-005 稳定错误回归\n'
go test -count=1 ./test/contracts -run '^(TestPathConfigurationAPI|TestPathRequirementAPI|TestExecutionPathAPI|TestFormRuntimeMaintenanceAPI)'

printf '验证 F-007 三类来源字段详情与实例现值读取及 F-006 回归\n'
go test -count=1 ./test/integration -run '^(TestPathConfigurationSnapshot|TestFlowRequirementSnapshotReadsSourceSpecificFormMetadata|TestFlowRequirementReadPreservesTimeoutCancellationAndResponseLimit|TestFlowRequirementReadSessionExpiryReplaysWholeChainOnce|TestFlowTreeReadUsesExactSourceLookupBeforeDetails|TestFormRuntimeFixedSourceSnapshot)'

if [[ -f "${project_root}/.env.local" ]]; then
  printf '验证 F-007 配置表迁移、事务、幂等、级联与维护任务持久化（本机忽略配置）\n'
  TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${project_root}/.env.local" go test -count=1 ./test/integration -run '^(TestPathConfigurationMySQL|TestFormRuntimeMaintenanceMySQL)'
elif [[ -n "${PLAN_DB_HOST:-}" ]] && [[ -n "${PLAN_DB_USER:-}" ]] && [[ -n "${PLAN_DB_PASSWORD:-}" ]]; then
  printf '验证 F-007 配置表迁移、事务、幂等与级联\n'
  go test -count=1 ./test/integration -run '^(TestPathConfigurationMySQL|TestFormRuntimeMaintenanceMySQL)'
else
  echo '本机未配置 PLAN_DB_* 或 .env.local，无法执行 F-007 核心 MySQL 集成测试' >&2
  exit 1
fi

printf '验证 F-007 前端配置逻辑、F-005 路径逻辑与 F-004 图布局必要回归\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/path_configuration_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/form_runtime_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/execution_path_test.mjs
node --no-warnings --experimental-strip-types --test test/unit/frontend/flow_graph_test.mjs
./test/integration/f004_frontend_structure.sh
./test/integration/f005_frontend_structure.sh
./test/integration/f006_frontend_structure.sh
./test/integration/f007_frontend_structure.sh
./test/integration/f007_form_runtime_maintenance_structure.sh
./test/integration/f006_go_comments.sh
./test/integration/f007_go_comments.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build -o .runtime/server ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-form-runtime typecheck
pnpm --filter test-auto-pro-v2-web build
pnpm --filter test-auto-pro-v2-form-runtime build

git diff --check
printf 'F-007 自动验证全部通过\n'
