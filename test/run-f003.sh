#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-003 计划配置、业务校验与幂等服务\n'
go test -count=1 ./test/unit/backend -run '^(TestPlanDB|TestValidDatabaseName|TestPlanService)'

printf '验证 F-003 本机配置安全同步\n'
./test/integration/f003_local_config_sync.sh

printf '验证 F-003 创建、列表与详情 API 契约\n'
go test -count=1 ./test/contracts -run '^TestPlanAPI'

printf '验证 F-003 随机临时 MySQL 的迁移、CRUD、幂等与重连读取\n'
TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${project_root}/.env.local" \
  go test -count=1 ./test/integration -run '^TestPlanMySQL'

printf '验证 F-003 前端创建请求、真实列表和错误恢复\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/plan_persistence_test.mjs

printf '验证 F-003 前端生产数据源与继续配置边界\n'
./test/integration/f003_frontend_structure.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build -o .runtime/server ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

printf 'F-003 自动验证全部通过\n'
