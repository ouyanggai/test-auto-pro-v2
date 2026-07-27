#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证计划筛选、状态动作与新建表单规则\n'
node --no-warnings --experimental-strip-types --test \
  test/unit/frontend/plan_filters_test.mjs \
  test/unit/frontend/plan_actions_test.mjs \
  test/unit/frontend/plan_form_test.mjs

printf '验证计划页面结构与静态边界\n'
./test/integration/plan_pages_structure.sh

printf '验证前端类型与生产构建\n'
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

printf 'F-001 自动验证全部通过\n'
