#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
package_file="${project_root}/web/package.json"
main_file="${project_root}/web/src/main.ts"
view_file="${project_root}/web/src/views/PlanPathsView.vue"
canvas_file="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
layout_file="${project_root}/web/src/features/flow-graph/layout.ts"

grep -Fq '"@vue-flow/core"' "${package_file}"
grep -Fq '"@vue-flow/controls"' "${package_file}"
grep -Fq '"@dagrejs/dagre"' "${package_file}"
grep -Fq "@vue-flow/core/dist/style.css" "${main_file}"
grep -Fq "@vue-flow/core/dist/theme-default.css" "${main_file}"
grep -Fq "@vue-flow/controls/dist/style.css" "${main_file}"

grep -Fq "fetchFlowGraph(planID.value" "${view_file}"
grep -Fq "loadController?.abort()" "${view_file}"
grep -Fq '<flow-graph-canvas :graph="graph" />' "${view_file}"
grep -Fq 'min-height: 560px' "${view_file}"

grep -Fq ":nodes-draggable=\"false\"" "${canvas_file}"
grep -Fq ":nodes-connectable=\"false\"" "${canvas_file}"
grep -Fq ":elements-selectable=\"false\"" "${canvas_file}"
grep -Fq ":delete-key-code=\"null\"" "${canvas_file}"
grep -Fq ":pan-on-drag=\"true\"" "${canvas_file}"
grep -Fq ":zoom-on-scroll=\"true\"" "${canvas_file}"
grep -Fq "fitView({ padding: 0.18" "${canvas_file}"
grep -Fq "useThemeVars()" "${canvas_file}"
grep -Fq "rankdir: 'TB'" "${layout_file}"

if grep -RInE 'execution_paths|save.*path|create.*path|update.*path|selectedPath' "${project_root}/web/src/features/flow-graph" "${view_file}" >/dev/null; then
  printf 'F-004 前端越界引入路径选择或保存\n' >&2
  exit 1
fi

printf 'F-004 只读流程图依赖、布局和交互边界验证通过\n'
