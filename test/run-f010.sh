#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-010 全模板规则目录、Vue 页面分类、自定义组件与生成边界\n'
go test -count=1 ./test/unit/backend -run '^TestF010'
go test -count=1 ./test/contracts -run '^TestF010'
go test -count=1 ./test/integration -run '^TestF009TemplateCoverageReadsAllVisibleTemplates$'
if [[ -f "${project_root}/.env.local" ]]; then
  TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${project_root}/.env.local" go test -count=1 ./test/integration -run '^TestF010TemplateCatalogMySQLPersistence$'
elif [[ -n "${PLAN_DB_HOST:-}" ]] && [[ -n "${PLAN_DB_USER:-}" ]] && [[ -n "${PLAN_DB_PASSWORD:-}" ]]; then
  go test -count=1 ./test/integration -run '^TestF010TemplateCatalogMySQLPersistence$'
else
  echo '本机未配置 PLAN_DB_* 或 .env.local，无法执行 F-010 MySQL 集成测试' >&2
  exit 1
fi

printf '验证 F-010 规则目录与 Vue 表单工作区结构\n'
./test/integration/f010_frontend_structure.sh
node --no-warnings --experimental-strip-types --test test/unit/frontend/form_runtime_test.mjs

printf '验证 F-010 Go 编译、前端类型检查与双构建\n'
go build -o .runtime/server ./cmd/server
pnpm --dir web run typecheck
pnpm --dir form-runtime run typecheck
pnpm --dir web run build
pnpm --dir form-runtime run build

git diff --check
printf 'F-010 自动验证通过\n'
