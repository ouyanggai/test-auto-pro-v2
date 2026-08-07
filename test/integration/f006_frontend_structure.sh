#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
requirements_view="${project_root}/web/src/views/RequirementsView.vue"
requirements_logic="${project_root}/web/src/features/plans/requirements.ts"
requirements_test="${project_root}/test/unit/frontend/path_requirements_test.mjs"

if grep -Fq "/plans/:id/requirements" "${router_file}" || grep -Fq "RequirementsView" "${router_file}"; then
  echo 'F-006 不得保留面向用户的路径要求页面路由' >&2
  exit 1
fi
if grep -Fq '核对路径要求' "${paths_view}" || grep -Fq '/requirements' "${paths_view}"; then
  echo 'F-006 不得在路径配置页显示独立核对入口' >&2
  exit 1
fi
for removed in "${requirements_view}" "${requirements_logic}" "${requirements_test}"; do
  if [[ -e "${removed}" ]]; then
    echo "F-006 页面专用前端文件必须删除：${removed}" >&2
    exit 1
  fi
done

# F-005 画布入口和路径能力仍然存在，删除 F-006 页面不得改变路径配置边界。
grep -Fq 'FlowGraphCanvas' "${paths_view}"
grep -Fq '新增路径' "${paths_view}"
grep -Fq '页面全屏' "${paths_view}"
grep -Fq '保存路径' "${paths_view}"

echo 'F-006 独立核对入口、路由、页面及页面专用前端模块已移除；F-005 路径边界保留'
