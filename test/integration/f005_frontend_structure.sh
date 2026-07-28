#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
view_file="${project_root}/web/src/views/PlanPathsView.vue"
canvas_file="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
edge_file="${project_root}/web/src/features/flow-graph/FlowTreeEdge.vue"
logic_file="${project_root}/web/src/features/execution-paths/logic.ts"
api_file="${project_root}/web/src/features/execution-paths/api.ts"

if grep -Fq '<template #toolbar>' "${view_file}" || grep -Fq 'class="path-toolbar"' "${view_file}"; then
  echo 'F-005 不得保留横跨画布的旧路径工具条' >&2
  exit 1
fi
grep -Fq '新增路径' "${view_file}"
grep -Fq '复制此路径' "${view_file}"
grep -Fq '保存路径' "${view_file}"
grep -Fq '<n-popconfirm' "${view_file}"
grep -Fq '<template #selection-panel>' "${view_file}"
grep -Fq 'class="path-selection-panel__summary"' "${view_file}"
grep -Fq 'aria-label="实时线路摘要"' "${view_file}"
grep -Fq 'projectExecutionPathSummary' "${view_file}"
grep -Fq ':selection-mode="selectionMode"' "${view_file}"
grep -Fq '@enter-selection="enterSelectionMode"' "${view_file}"
grep -Fq '@exit-selection="exitSelectionMode"' "${view_file}"
grep -Fq '@select-branch="selectBranch"' "${view_file}"
grep -Fq 'pathsLoaded: pathsLoaded.value' "${view_file}"
grep -Fq "apiError.code === 'EXECUTION_PATH_INVALID'" "${view_file}"
grep -Fq 'refreshExecutionPathDraft' "${view_file}"
grep -Fq '流程已变化，需要重新选择' "${view_file}"
grep -Fq 'flow-graph-canvas--page-fullscreen' "${canvas_file}"
grep -Fq "await requestPageFullscreen(true)" "${canvas_file}"
grep -Fq "emit('enterSelection')" "${canvas_file}"
grep -Fq "emit('exitSelection')" "${canvas_file}"
grep -Fq "if (!props.selectionMode) return" "${canvas_file}"
grep -Fq '线路选择' "${canvas_file}"
grep -Fq '继续选择' "${canvas_file}"
grep -Fq 'flow-graph-canvas__selection-panel' "${canvas_file}"
grep -Fq 'width: 320px' "${canvas_file}"
grep -Fq ':aria-expanded="!isSelectionPanelCollapsed"' "${canvas_file}"
grep -Fq 'nextExecutionPathRouteID' "${canvas_file}"
grep -Fq 'reservedRight' "${canvas_file}"
grep -Fq 'duration: reducedMotion() ? 0 : 250' "${canvas_file}"
grep -Fq 'aria-live="polite"' "${canvas_file}"
grep -Fq '下一步：请选择一条分支' "${canvas_file}"
grep -Fq '线路已完整，请保存' "${canvas_file}"
grep -Fq 'nodes-draggable="false"' "${canvas_file}"
grep -Fq 'nodes-connectable="false"' "${canvas_file}"
grep -Fq 'elements-selectable="false"' "${canvas_file}"
grep -Fq 'min-height: 32px' "${edge_file}"
grep -Fq '<button' "${edge_file}"
grep -Fq ':aria-pressed="data.selected"' "${edge_file}"
grep -Fq '并行必经' "${edge_file}"
grep -Fq 'data.dimmed' "${edge_file}"
grep -Fq 'flow-tree-edge__choice--dimmed' "${edge_file}"
grep -Fq "'flow-tree-edge__direction--animated': !data.selectionEnabled || data.selected" "${edge_file}"
grep -Fq 'flow-tree-edge__base--candidate' "${edge_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${edge_file}"
grep -Fq 'analyzeExecutionPath' "${logic_file}"
grep -Fq 'graph.entryNodeIds.length === 0' "${logic_file}"
grep -Fq 'canEnterExecutionPathSelection' "${logic_file}"
grep -Fq 'projectExecutionPathSummary' "${logic_file}"
grep -Fq 'Idempotency-Key' "${api_file}"

if grep -Eq 'requestFullscreen|fullscreenElement|fullscreenchange' "${canvas_file}"; then
  echo 'F-005 线路选择不得调用浏览器 Fullscreen API' >&2
  exit 1
fi

if grep -RInE 'execution[_-]?run|approve(user|r)|form(value|data)|target.*write|start.*execution' \
  "${project_root}/web/src/features/execution-paths" "${view_file}" >/dev/null; then
  echo 'F-005 前端出现了路径选择之外的运行、人员、表单或目标写操作' >&2
  exit 1
fi

echo 'F-005 前端结构边界通过'
