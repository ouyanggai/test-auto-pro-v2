#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
requirements_view="${project_root}/web/src/views/RequirementsView.vue"
requirements_logic="${project_root}/web/src/features/plans/requirements.ts"

grep -Fq "{ path: '/plans/:id/requirements', component: RequirementsView }" "${router_file}"
grep -Fq '核对路径要求' "${paths_view}"
grep -Fq 'pathsLoaded && paths.length > 0' "${paths_view}"
grep -Fq 'router.push(`/plans/${planID}/requirements`)' "${paths_view}"

if awk '
  /<template #canvas-actions>/ { in_canvas = 1 }
  in_canvas && /核对路径要求/ { exit 1 }
  in_canvas && /<\/template>/ { exit 0 }
' "${paths_view}"; then
  :
else
  echo 'F-006 核对入口不得进入 F-005 画布工具栏' >&2
  exit 1
fi

grep -Fq '返回路径选择' "${requirements_view}"
grep -Fq '尚未保存执行路径，请先返回选择并保存路径' "${requirements_view}"
grep -Fq 'class="requirements-layout"' "${requirements_view}"
grep -Fq 'grid-template-columns: 250px minmax(0, 1fr)' "${requirements_view}"
grep -Fq 'aria-label="已保存路径"' "${requirements_view}"
grep -Fq 'aria-live="polite"' "${requirements_view}"
grep -Fq 'requirementController?.abort()' "${requirements_view}"
grep -Fq 'shouldApplyRequirementResponse' "${requirements_view}"
grep -Fq "requirementError.code === 'EXECUTION_PATH_INVALID'" "${requirements_view}"
grep -Fq '当前路径已不符合最新流程，请返回重新选择' "${requirements_logic}"
grep -Fq '目标平台自动确定' "${requirements_logic}"
grep -Fq '运行时确定' "${requirements_logic}"
grep -Fq '需要人工核对' "${requirements_logic}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${requirements_view}"

for forbidden in '运行计划' 'v-model:value' 'FlowGraphCanvas' '@vue-flow/core' '@click="save' '保存后生效'; do
  if grep -Fq "${forbidden}" "${requirements_view}"; then
    echo "F-006 只读核对页出现禁止内容：${forbidden}" >&2
    exit 1
  fi
done

echo 'F-006 前端路由、只读页面、迟到保护与 F-005 工具栏边界检查通过'
