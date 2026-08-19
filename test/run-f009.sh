#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-009 条件求解与批量任务仓储\n'
go test -count=1 ./test/unit/backend -run '^TestF009'
go test -count=1 ./test/contracts -run '^TestF009'

printf '验证 F-009 前端与跨层结构\n'
./test/integration/f009_frontend_structure.sh

if [[ -f "${project_root}/.env.local" ]]; then
  TEST_AUTO_PRO_PLAN_DB_ENV_FILE="${project_root}/.env.local" go test -count=1 ./test/integration -run '^(TestF009PathPreparationMySQLCheckpoints|TestF009TemplateCoverageReadsAllVisibleTemplates)$'
elif [[ -n "${PLAN_DB_HOST:-}" ]] && [[ -n "${PLAN_DB_USER:-}" ]] && [[ -n "${PLAN_DB_PASSWORD:-}" ]]; then
  go test -count=1 ./test/integration -run '^(TestF009PathPreparationMySQLCheckpoints|TestF009TemplateCoverageReadsAllVisibleTemplates)$'
else
  echo '本机未配置 PLAN_DB_* 或 .env.local，无法执行 F-009 MySQL 集成测试' >&2
  exit 1
fi

git diff --check
printf 'F-009 定向结构与后端验证通过\n'
