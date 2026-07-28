#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
view_file="${project_root}/web/src/views/PlanPathsView.vue"
canvas_file="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
edge_file="${project_root}/web/src/features/flow-graph/FlowTreeEdge.vue"
logic_file="${project_root}/web/src/features/execution-paths/logic.ts"
api_file="${project_root}/web/src/features/execution-paths/api.ts"

grep -Fq '<template #toolbar>' "${view_file}"
grep -Fq '新增路径' "${view_file}"
grep -Fq '复制此路径' "${view_file}"
grep -Fq '保存路径' "${view_file}"
grep -Fq '<n-popconfirm' "${view_file}"
grep -Fq ':selection-enabled="draftMode !== null"' "${view_file}"
grep -Fq '@select-branch="selectBranch"' "${view_file}"
grep -Fq ':disabled="saving || deleting"' "${view_file}"
grep -Fq 'flow-graph-canvas--page-fullscreen' "${canvas_file}"
grep -Fq 'nodes-draggable="false"' "${canvas_file}"
grep -Fq 'nodes-connectable="false"' "${canvas_file}"
grep -Fq 'elements-selectable="false"' "${canvas_file}"
grep -Fq 'min-height: 32px' "${edge_file}"
grep -Fq '<button' "${edge_file}"
grep -Fq ':aria-pressed="data.selected"' "${edge_file}"
grep -Fq '并行必经' "${edge_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${edge_file}"
grep -Fq 'analyzeExecutionPath' "${logic_file}"
grep -Fq 'Idempotency-Key' "${api_file}"

if grep -RInE 'execution[_-]?run|approve(user|r)|form(value|data)|target.*write|start.*execution' \
  "${project_root}/web/src/features/execution-paths" "${view_file}" >/dev/null; then
  echo 'F-005 前端出现了路径选择之外的运行、人员、表单或目标写操作' >&2
  exit 1
fi

echo 'F-005 前端结构边界通过'
