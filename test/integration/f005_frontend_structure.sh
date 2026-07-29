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
grep -Fq '一键生成全部路径' "${view_file}"
grep -Fq 'generateAllExecutionPaths' "${view_file}"
grep -Fq 'previewAllExecutionPaths' "${view_file}"
grep -Fq '稳定序号 #' "${view_file}"
grep -Fq ':title="pathDisplayName(item)"' "${view_file}"
grep -Fq 'class="saved-paths-popover__name"' "${view_file}"
grep -Fq 'text-overflow: ellipsis' "${view_file}"
grep -Fq 'flex: 0 0 auto' "${view_file}"
grep -Fq 'overflow-y: auto' "${view_file}"
grep -Fq 'v-model:value="draftName"' "${view_file}"
grep -Fq 'maxlength="50"' "${view_file}"
grep -Fq '生成全部路径失败，请重试' "${view_file}"
grep -Fq '当前未保存草稿、名称和单条创建键都不能被新路径覆盖' "${view_file}"
if grep -Fq 'selectSavedPath(latestCreated)' "${view_file}"; then
  echo 'F-005 批量成功不得覆盖当前未保存草稿' >&2
  exit 1
fi
grep -Fq '<n-popconfirm' "${view_file}"
grep -Fq '<template #workspace-panel>' "${view_file}"
grep -Fq '<template #saved-paths>' "${view_file}"
grep -Fq ':saved-paths-open="savedPathsOpen"' "${view_file}"
grep -Fq '@close-saved-paths="closeSavedPaths"' "${view_file}"
grep -Fq 'NVirtualList' "${view_file}"
grep -Fq '<n-virtual-list' "${view_file}"
grep -Fq ':item-size="SAVED_PATH_ITEM_SIZE"' "${view_file}"
grep -Fq 'const SAVED_PATH_ITEM_SIZE = 44' "${view_file}"
grep -Fq '<h3>已保存的路径</h3>' "${view_file}"
grep -Fq '>已保存的路径<' "${view_file}"
if grep -Eq '已保存路径[[:space:]]*\{\{|已保存的路径[[:space:]]*[0-9]' "${view_file}"; then
  echo 'F-005 已保存路径入口和标题不得直接拼接数量' >&2
  exit 1
fi
grep -Fq 'requestSavedPathSwitch' "${view_file}"
grep -Fq 'hasExecutionPathDraftChanges' "${view_file}"
grep -Fq '当前路径存在未保存的名称或线路变化' "${view_file}"
grep -Fq 'class="path-selection-panel__summary"' "${view_file}"
grep -Fq 'class="path-selection-panel__operations"' "${view_file}"
grep -Fq 'class="path-selection-panel__hint"' "${view_file}"
grep -Fq 'aria-label="路径操作"' "${view_file}"
grep -Fq 'aria-label="操作提示"' "${view_file}"
grep -Fq '<p>{{ workspacePresentation.hint }}</p>' "${view_file}"
grep -Fq 'aria-label="实时线路摘要"' "${view_file}"
grep -Fq 'projectExecutionPathSummary' "${view_file}"
grep -Fq ':workspace-open="pathWorkspaceOpen"' "${view_file}"
grep -Fq ':branch-editing="workspacePresentation.branchEditing"' "${view_file}"
grep -Fq ':save-guide-visible="workspacePresentation.showSave"' "${view_file}"
grep -Fq '@enter-workspace="enterSelectionMode"' "${view_file}"
grep -Fq '@exit-workspace="exitSelectionMode"' "${view_file}"
grep -Fq '@select-branch="selectBranch"' "${view_file}"
grep -Fq "transitionExecutionPathWorkspace(workspaceMode.value, 'select-saved')" "${view_file}"
grep -Fq 'selectSavedPath(saved)' "${view_file}"
grep -Fq "workspaceMode === 'view' && activePath" "${view_file}"
grep -Fq 'v-if="workspacePresentation.showNameInput"' "${view_file}"
grep -Fq 'v-if="workspacePresentation.showSave"' "${view_file}"
grep -Fq 'pathsLoaded: pathsLoaded.value' "${view_file}"
grep -Fq "apiError.code === 'EXECUTION_PATH_INVALID'" "${view_file}"
grep -Fq 'refreshExecutionPathDraft' "${view_file}"
grep -Fq '流程已变化，需要重新选择' "${view_file}"
grep -Fq 'flow-graph-canvas--page-fullscreen' "${canvas_file}"
grep -Fq "await requestPageFullscreen(true)" "${canvas_file}"
grep -Fq "emit('enterWorkspace')" "${canvas_file}"
grep -Fq "emit('exitWorkspace')" "${canvas_file}"
grep -Fq 'workspaceOpen?: boolean' "${canvas_file}"
grep -Fq 'branchEditing?: boolean' "${canvas_file}"
grep -Fq 'saveGuideVisible?: boolean' "${canvas_file}"
if grep -Fq 'selectionMode' "${canvas_file}" || grep -Fq 'selectionEnabled' "${edge_file}"; then
  echo 'F-005 路径工作区与分支编辑不得继续复用旧选择状态' >&2
  exit 1
fi
grep -Fq '线路选择' "${canvas_file}"
grep -Fq '继续选择' "${canvas_file}"
grep -Fq 'flow-graph-canvas__selection-panel' "${canvas_file}"
grep -Fq 'width: 320px' "${canvas_file}"
grep -Fq ':aria-expanded="!isSelectionPanelCollapsed"' "${canvas_file}"
grep -Fq 'savedPathsOpen?: boolean' "${canvas_file}"
grep -Fq 'closeSavedPaths: []' "${canvas_file}"
grep -Fq '<slot name="saved-paths" />' "${canvas_file}"
grep -Fq 'flow-graph-canvas__saved-paths' "${canvas_file}"
grep -Fq 'width: 280px' "${canvas_file}"
grep -Fq 'max-height: 320px' "${canvas_file}"
grep -Fq 'right: 340px' "${canvas_file}"
grep -Fq '90%, transparent' "${canvas_file}"
grep -Fq 'backdrop-filter: blur(8px)' "${canvas_file}"
grep -Fq 'if (props.savedPathsOpen)' "${canvas_file}"
grep -Fq "emit('closeSavedPaths')" "${canvas_file}"
grep -Fq 'nextExecutionPathRouteID' "${canvas_file}"
grep -Fq 'reservedRight' "${canvas_file}"
grep -Fq 'viewportForCandidateGroupCentered' "${canvas_file}"
grep -Fq 'projectExecutionPathGuide' "${canvas_file}"
grep -Fq 'duration: reducedMotion() ? 0 : 250' "${canvas_file}"
grep -Fq 'aria-live="polite"' "${canvas_file}"
grep -Fq 'flow-graph-canvas__guide-lines' "${canvas_file}"
grep -Fq 'v-for="candidate in guideProjection.visibleCandidates"' "${canvas_file}"
grep -Fq 'marker-end="url(#flow-guide-arrow)"' "${canvas_file}"
grep -Fq '@viewport-change="handleViewportChange"' "${canvas_file}"
grep -Fq '请在以下 ${candidates.length} 个分支中选择 1 个' "${canvas_file}"
grep -Fq '还有 {{ guideProjection.hiddenLeftCount }} 个候选' "${canvas_file}"
grep -Fq '还有 {{ guideProjection.hiddenRightCount }} 个候选' "${canvas_file}"
grep -Fq '线路已完整，请保存' "${canvas_file}"
if grep -Fq 'guideBubbleTimer' "${canvas_file}"; then
  echo 'F-005 等待分支选择时引导不得自动消失' >&2
  exit 1
fi
grep -Fq 'nodes-draggable="false"' "${canvas_file}"
grep -Fq 'nodes-connectable="false"' "${canvas_file}"
grep -Fq 'elements-selectable="false"' "${canvas_file}"
grep -Fq 'min-height: 32px' "${edge_file}"
grep -Fq '<button' "${edge_file}"
grep -Fq ':aria-pressed="data.selected"' "${edge_file}"
grep -Fq ':disabled="data.selected"' "${edge_file}"
grep -Fq '并行必经' "${edge_file}"
grep -Fq 'data.dimmed' "${edge_file}"
grep -Fq 'flow-tree-edge__choice--dimmed' "${edge_file}"
grep -Fq '尚未到达' "${edge_file}"
grep -Fq "data.branchEditing && (data.kind === 'condition' || data.kind === 'manual') && data.active !== false" "${edge_file}"
grep -Fq '!data.branchEditing' "${edge_file}"
if ! awk '
  BEGIN { found_button = 0 }
  /<edge-label-renderer/ { in_renderer = 1; next }
  /<button/ { if (in_renderer) { found_button = 1; exit 0 } }
  in_renderer && /data\.active !== false/ { exit 1 }
  END { if (!found_button) exit 1 }
' "${edge_file}"; then
  echo 'F-005 未到达分支标签不得在渲染器外层整体隐藏' >&2
  exit 1
fi
grep -Fq "'flow-tree-edge__direction--animated': !data.workspaceOpen || data.selected" "${edge_file}"
grep -Fq 'flow-tree-edge__base--candidate' "${edge_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${edge_file}"
grep -Fq 'vue-flow__controls-button:hover' "${canvas_file}"
grep -Fq 'vue-flow__controls-button:disabled' "${canvas_file}"
grep -Fq 'fill: currentcolor' "${canvas_file}"
grep -Fq 'analyzeExecutionPath' "${logic_file}"
grep -Fq 'graph.entryNodeIds.length === 0' "${logic_file}"
grep -Fq 'canEnterExecutionPathSelection' "${logic_file}"
grep -Fq 'projectExecutionPathSummary' "${logic_file}"
grep -Fq 'projectExecutionPathGuide' "${logic_file}"
grep -Fq 'viewportForCandidateGroupCentered' "${logic_file}"
grep -Fq 'hasExecutionPathDraftChanges' "${logic_file}"
grep -Fq 'deriveExecutionPathWorkspacePresentation' "${logic_file}"
grep -Fq 'transitionExecutionPathWorkspace' "${logic_file}"
grep -Fq "kind: 'pending'" "${logic_file}"
grep -Fq "'路径详情'" "${logic_file}"
grep -Fq "'编辑路径'" "${logic_file}"
grep -Fq "'新建路径'" "${logic_file}"
grep -Fq "'复制路径'" "${logic_file}"
grep -Fq "'修改未保存'" "${logic_file}"
grep -Fq "'已保存'" "${logic_file}"
grep -Fq 'Idempotency-Key' "${api_file}"
grep -Fq 'execution-paths/generate-all' "${api_file}"
grep -Fq 'path-summary-flow' "${view_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${view_file}"

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
