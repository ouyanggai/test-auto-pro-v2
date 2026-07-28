#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
new_plan="${project_root}/web/src/views/NewPlanView.vue"
plans_view="${project_root}/web/src/views/PlansView.vue"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
persistence="${project_root}/web/src/features/plans/persistence.ts"
router="${project_root}/web/src/router/index.ts"

grep -Fq "createPlan(" "${new_plan}"
grep -Fq "buildCreatePlanRequest(form, verifiedSummary.value, selectedCandidate.value)" "${new_plan}"
grep -Fq "creationKey.value || crypto.randomUUID()" "${new_plan}"
grep -Fq ':loading="creationLoading"' "${new_plan}"
grep -Fq "await router.push(\`/plans/\${plan.id}/paths\`)" "${new_plan}"
grep -Fq "Idempotency-Key" "${persistence}"
grep -Fq "candidate.flowName" "${persistence}"
grep -Fq "candidate.name" "${persistence}"
grep -Fq "candidate.flowInstanceName" "${persistence}"

grep -Fq "fetchPlans(filters" "${plans_view}"
grep -Fq 'loadError' "${plans_view}"
grep -Fq '>重试</n-button>' "${plans_view}"
if grep -Fq "features/plans/mock" "${plans_view}"; then
  printf '生产计划列表仍导入 mock 数据源\n' >&2
  exit 1
fi

grep -Fq "/plans/:id/paths" "${router}"
grep -Fq "fetchPlan(planID.value" "${paths_view}"
grep -Fq "还没有选择执行路径，计划已经保存，可以稍后继续" "${paths_view}"
grep -Fq "真实流程结构与路径选择将在后续功能开放" "${paths_view}"

if grep -RInE 'execution_paths|vue-flow|dagre' "${project_root}/internal" "${project_root}/web/src" >/dev/null; then
  printf 'F-003 生产代码越界引入路径表或流程图依赖\n' >&2
  exit 1
fi

printf 'F-003 前端创建、真实列表与继续配置边界验证通过\n'
