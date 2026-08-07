#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
config_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
config_api="${project_root}/web/src/features/path-configuration/api.ts"
config_logic="${project_root}/web/src/features/path-configuration/logic.ts"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
canvas_view="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"

if ! grep -Fq "/plans/:planId/paths/:pathId/configure" "${router_file}" || ! grep -Fq "PlanPathConfigurationView" "${router_file}"; then
  echo 'F-007 必须存在单条路径配置页路由' >&2
  exit 1
fi
if ! grep -Fq "配置路径" "${paths_view}"; then
  echo 'F-007 已保存路径详情必须提供配置路径入口' >&2
  exit 1
fi
if grep -Fq "/plans/:id/requirements" "${router_file}" || grep -Fq "RequirementsView" "${router_file}"; then
  echo 'F-007 不得恢复独立路径要求核对路由' >&2
  exit 1
fi
if grep -Fq "JSON.stringify" "${config_view}" || grep -Eq "<textarea|contenteditable" "${config_view}"; then
  echo 'F-007 配置页不得出现 JSON 编辑器或文本伪造输入' >&2
  exit 1
fi
if grep -Fq "/web/" "${config_api}"; then
  echo 'F-007 生产前端不得直接调用目标平台写接口' >&2
  exit 1
fi

# 配置页要求独立内容滚动与固定保存区。
grep -Fq "path-configuration-page__body" "${config_view}"
grep -Fq "overflow: auto" "${config_view}"
grep -Fq "path-configuration-page__footer" "${config_view}"

# 缺口、受影响、不同意提示和必填状态必须保留明确中文呈现。
grep -Fq "path-configuration-node__gaps" "${config_view}"
grep -Fq "path-configuration-field--affected" "${config_view}"
grep -Fq "disagreeWarning" "${config_view}"
grep -Fq "必填" "${config_view}"
grep -Fq "NDatePicker" "${config_view}"
grep -Fq 'type="date"' "${config_view}"
grep -Fq 'type="datetime"' "${config_view}"
grep -Fq "canSavePathConfiguration" "${config_view}"

# 首次无配置记录必须允许保存；日期与日期时间控件必须在普通计划详情和画布入口之外可见。
grep -Fq "status === 'pending'" "${config_logic}"
grep -Fq "path-preparation" "${paths_view}"
grep -Fq "下一步：新增执行路径" "${paths_view}"
grep -Fq "配置下一条" "${paths_view}"
grep -Fq "返回计划详情" "${config_view}"
grep -Fq "executionPaths" "${config_view}"
grep -Fq "已配置" "${paths_view}"
grep -Fq "未配置" "${paths_view}"
grep -Fq "canvas-actions-normal" "${paths_view}"
grep -Fq "编辑路径" "${paths_view}"
grep -Fq 'name="canvas-actions-normal"' "${canvas_view}"
grep -Fq 'name="canvas-actions"' "${canvas_view}"
if ! awk '/name="canvas-actions-normal"/{normal=NR} /页面全屏/{fullscreen=NR} END{exit !(normal && fullscreen && normal < fullscreen)}' "${canvas_view}"; then
  echo 'F-007 普通画布必须先渲染编辑路径插槽，再渲染文字页面全屏按钮' >&2
  exit 1
fi
grep -Fq '点击“页面全屏”只放大当前流程图' "${project_root}/test/manual/F-007.md"
grep -Fq '点击“编辑路径”才进入线路管理' "${project_root}/test/manual/F-007.md"

if [[ ! -f "${config_logic}" ]]; then
  echo 'F-007 配置纯逻辑模块缺失' >&2
  exit 1
fi

echo 'F-007 配置页路由、入口、只读边界、滚动与保存区结构检查通过'
