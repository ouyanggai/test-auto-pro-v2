#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
package_file="${project_root}/web/package.json"
main_file="${project_root}/web/src/main.ts"
view_file="${project_root}/web/src/views/PlanPathsView.vue"
canvas_file="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
hub_file="${project_root}/web/src/features/flow-graph/FlowRoutingHub.vue"
edge_file="${project_root}/web/src/features/flow-graph/FlowTreeEdge.vue"
layout_file="${project_root}/web/src/features/flow-graph/layout.ts"

grep -Fq '"@vue-flow/core"' "${package_file}"
grep -Fq '"@vue-flow/controls"' "${package_file}"
if grep -Rq '@dagrejs/dagre' "${package_file}" "${project_root}/pnpm-lock.yaml" "${layout_file}"; then
  printf 'F-004 不得继续依赖或导入 dagre\n' >&2
  exit 1
fi
grep -Fq "@vue-flow/core/dist/style.css" "${main_file}"
grep -Fq "@vue-flow/core/dist/theme-default.css" "${main_file}"
grep -Fq "@vue-flow/controls/dist/style.css" "${main_file}"

grep -Fq "fetchFlowGraph(planID.value" "${view_file}"
grep -Fq "loadController?.abort()" "${view_file}"
grep -Fq '<flow-graph-canvas' "${view_file}"
grep -Fq '@retry="retryGraph"' "${view_file}"
grep -Fq 'min-height: 560px' "${view_file}"

grep -Fq ":nodes-draggable=\"false\"" "${canvas_file}"
grep -Fq ":nodes-connectable=\"false\"" "${canvas_file}"
grep -Fq ":elements-selectable=\"false\"" "${canvas_file}"
grep -Fq ":delete-key-code=\"null\"" "${canvas_file}"
grep -Fq ":pan-on-drag=\"true\"" "${canvas_file}"
grep -Fq ":zoom-on-scroll=\"true\"" "${canvas_file}"
grep -Fq "setViewport(viewport" "${canvas_file}"
grep -Fq "getViewport" "${canvas_file}"
grep -Fq "requestedPageFullscreen" "${canvas_file}"
grep -Fq "pageFullscreenTask" "${canvas_file}"
grep -Fq "runPageFullscreenTransitions" "${canvas_file}"
grep -Fq "while (!pageFullscreenDisposed && isPageFullscreen.value !== requestedPageFullscreen)" "${canvas_file}"
grep -Fq "requestPageFullscreen" "${canvas_file}"
grep -Fq "compensateViewportForContainerWidth" "${canvas_file}"
grep -Fq "void requestPageFullscreen(false)" "${canvas_file}"
grep -Fq "@click=\"requestPageFullscreen(!isPageFullscreen)\"" "${canvas_file}"
grep -Fq "compensateViewportForContainerWidth" "${layout_file}"
if grep -Fq "pageFullscreenVersion" "${canvas_file}"; then
  printf 'F-004 页面全屏不得仅以版本号丢弃切换补偿\n' >&2
  exit 1
fi
if grep -Fq "fitView(" "${canvas_file}"; then
  printf 'F-004 不得在初次加载时自动适配整图\n' >&2
  exit 1
fi
grep -Fq "flow-graph-canvas--page-fullscreen" "${canvas_file}"
grep -Fq "position: fixed" "${canvas_file}"
grep -Fq "inset: 0" "${canvas_file}"
grep -Fq "event.key !== 'Escape'" "${canvas_file}"
grep -Fq "document.addEventListener('keydown'" "${canvas_file}"
grep -Fq "document.removeEventListener('keydown'" "${canvas_file}"
grep -Fq "页面全屏" "${canvas_file}"
if grep -Eq "requestFullscreen|fullscreenElement|fullscreenchange" "${canvas_file}"; then
  printf 'F-004 页面全屏不得调用浏览器 Fullscreen API\n' >&2
  exit 1
fi
grep -Fq "useThemeVars()" "${canvas_file}"
grep -Fq "safeLayoutFlowGraph(props.graph)" "${canvas_file}"
grep -Fq 'v-if="laidOut"' "${canvas_file}"
grep -Fq 'layoutResult.error' "${canvas_file}"
grep -Fq "emit('retry')" "${canvas_file}"
grep -Fq "return { layout: null, error: flowStructureErrorMessage }" "${layout_file}"
grep -Fq "class FlowTreeLayout" "${layout_file}"
grep -Fq "type: 'treeEdge'" "${layout_file}"
if grep -Eq "type: ['\"]?(step|smoothstep)|dagre" "${layout_file}"; then
  printf 'F-004 不得保留通用自动边或 dagre 临时布局\n' >&2
  exit 1
fi
grep -Fq "type: routingHub ? 'routingHub' : 'flowNode'" "${layout_file}"
grep -Fq '<flow-routing-hub />' "${canvas_file}"
grep -Fq '<flow-tree-edge v-bind="edgeProps"' "${canvas_file}"
grep -Fq "BaseEdge" "${edge_file}"
grep -Fq 'flow-tree-edge__direction' "${edge_file}"
grep -Fq 'stroke-dasharray:' "${edge_file}"
grep -Fq '@keyframes flow-tree-direction' "${edge_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${edge_file}"
grep -Fq 'animation: none' "${edge_file}"
grep -Fq 'pointer-events: none' "${hub_file}"
grep -Fq 'opacity: 0' "${hub_file}"

if grep -RInE 'createExecutionPath|updateExecutionPath|deleteExecutionPath|features/execution-paths/api' "${project_root}/web/src/features/flow-graph" >/dev/null; then
  printf 'F-004 流程图展示组件不得直接持久化执行路径\n' >&2
  exit 1
fi

printf 'F-004 只读流程图依赖、布局和交互边界验证通过\n'
