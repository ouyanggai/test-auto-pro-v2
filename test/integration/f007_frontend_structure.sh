#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
config_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
config_api="${project_root}/web/src/features/path-configuration/api.ts"
config_logic="${project_root}/web/src/features/path-configuration/logic.ts"
config_panel="${project_root}/web/src/features/path-configuration/NodeConfigurationPanel.vue"
form_frame="${project_root}/web/src/features/path-configuration/FormRuntimeFrame.vue"
runtime_app="${project_root}/form-runtime/src/App.vue"
runtime_protocol="${project_root}/form-runtime/src/runtime/protocol.js"
runtime_policy="${project_root}/form-runtime/src/runtime/requestPolicy.js"
canvas_view="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
flow_node="${project_root}/web/src/features/flow-graph/FlowGraphNode.vue"
routing_hub="${project_root}/web/src/features/flow-graph/FlowRoutingHub.vue"

if ! grep -Fq "/plans/:planId/paths/:pathId/configure" "${router_file}" || ! grep -Fq "PlanPathConfigurationView" "${router_file}"; then
  echo 'F-007 必须保留单条已保存路径的节点配置路由' >&2
  exit 1
fi
if grep -Fq "/plans/:id/requirements" "${router_file}" || grep -Fq "RequirementsView" "${router_file}"; then
  echo 'F-007 不得恢复独立路径要求核对路由' >&2
  exit 1
fi
if grep -Fq "/web/" "${config_api}"; then
  echo 'F-007 生产前端不得直接调用目标平台写接口' >&2
  exit 1
fi

# 无路径先进入既有 F-005 配置路径；有路径才提供逐条节点配置，两个职责不能混用。
grep -Fq "请先配置并保存执行路径" "${paths_view}"
grep -Fq ">配置路径</n-button>" "${paths_view}"
grep -Fq ">配置节点</n-button>" "${paths_view}"
grep -Fq "编辑路径" "${paths_view}"
grep -Fq "configurationStatus === 'configured'" "${paths_view}"
if grep -Fq "下一步：新增执行路径" "${paths_view}"; then
  echo 'F-007 无路径状态不得显示无效节点配置或含糊的新增步骤' >&2
  exit 1
fi

# 配置页在同一路由内切换节点画布和隔离表单工作区，不再把表单字段塞进节点侧栏。
grep -Fq "FlowGraphCanvas" "${config_view}"
grep -Fq "FormRuntimeFrame" "${config_view}"
grep -Fq "workspace === 'nodes'" "${config_view}"
grep -Fq "workspace === 'form'" "${config_view}"
grep -Fq "智能生成" "${config_view}"
grep -Fq "换一组" "${config_view}"
grep -Fq "恢复已保存" "${config_view}"
grep -Fq "保存表单数据" "${config_view}"
grep -Fq "configuration-mode" "${config_view}"
grep -Fq "configuration-node-states" "${config_view}"
grep -Fq 'name="configuration-panel"' "${canvas_view}"
grep -Fq "flow-graph-canvas__configuration-panel" "${canvas_view}"
grep -Fq "position: absolute" "${canvas_view}"
grep -Fq "NodeConfigurationPanel" "${config_view}"
grep -Fq "bindPathConfigurationNodes" "${config_view}"
grep -Fq "focusNode" "${canvas_view}"
grep -Fq "viewportForPointCentered" "${canvas_view}"
if grep -Fq 'v-for="(group' "${config_view}" || grep -Fq "path-configuration-page__group" "${config_view}"; then
  echo 'F-007 配置页不得恢复整页字段分组表单' >&2
  exit 1
fi

# 当前路径节点可操作，路径外节点只作弱化上下文；路由节点仍复用同一真实拓扑。
grep -Fq "configurationInteractive" "${flow_node}"
grep -Fq "configurationStatusName" "${flow_node}"
grep -Fq "flow-node--configuration-selected" "${flow_node}"
grep -Fq "flow-routing-hub__configuration" "${routing_hub}"
grep -Fq "flow-node--path-muted" "${canvas_view}"
grep -Fq "branchEditing: props.configurationMode ? false" "${canvas_view}"

# 节点侧栏只呈现人员、动作和规则；表单字段必须由独立 FormMaking runtime 渲染。
grep -Fq "node.requirements" "${config_panel}"
grep -Fq "node.persons" "${config_panel}"
grep -Fq "person.editable" "${config_panel}"
grep -Fq "运行时确定" "${config_panel}"
grep -Fq "固定提交" "${config_panel}"
grep -Fq "disagreeWarning" "${config_panel}"
grep -Fq "暂不支持" "${config_panel}"
grep -Fq "overflow-y: auto" "${config_panel}"
grep -Fq "保存当前节点" "${config_panel}"
if grep -Eq 'node\.fields|NDatePicker|NInput|NCheckbox|NSwitch' "${config_panel}"; then
  echo 'F-007 节点侧栏不得重新模拟目标表单字段控件' >&2
  exit 1
fi

# iframe 契约必须隔离 SID、拒绝迟到会话并阻断目标写接口；完整 values 只经 getValues 返回。
grep -Fq "sessionId" "${form_frame}"
grep -Fq "event.source !== iframe.value?.contentWindow" "${form_frame}"
grep -Fq "event.origin !== runtimeOrigin.value" "${form_frame}"
grep -Fq "getValues" "${runtime_app}"
grep -Fq "getData(true)" "${runtime_app}"
grep -Fq "destroySession" "${runtime_app}"
grep -Fq "FORM_RUNTIME_VERSION" "${runtime_protocol}"
grep -Fq "targetRequestAllowed" "${runtime_policy}"
grep -Fq "WRITE_SEGMENT_PREFIXES" "${runtime_policy}"
grep -Fq "READ_SEGMENT_PREFIXES" "${runtime_policy}"
grep -Fq "window.fetch = async" "${runtime_policy}"
grep -Fq "SID" "${runtime_policy}"

# 浏览器只处理不透明键，结构变化保留可对应草稿；不得出现假运行控制。
grep -Fq "pathConfigNodeKey" "${config_logic}"
grep -Fq "reconcilePathConfigDraft" "${config_logic}"
grep -Fq "status === 'pending'" "${config_logic}"
if grep -Eq '> *(暂停|继续|单步|运行|设置断点) *<' "${config_view}" "${config_panel}" "${canvas_view}"; then
  echo 'F-007 不得显示尚未生效的运行、单步或断点按钮' >&2
  exit 1
fi
if grep -Fq "JSON.stringify" "${config_view}" || grep -Eq "<textarea|contenteditable" "${config_view}" "${config_panel}"; then
  echo 'F-007 配置页不得出现目标原始 JSON 编辑器或文本伪造输入' >&2
  exit 1
fi

# F-005 普通画布与页面全屏入口保持原样，节点配置模式只增加独立面板。
grep -Fq 'name="canvas-actions-normal"' "${canvas_view}"
grep -Fq 'name="canvas-actions"' "${canvas_view}"
grep -Fq "页面全屏" "${canvas_view}"
grep -Fq "workspaceOpen" "${canvas_view}"
grep -Fq "branchEditing" "${canvas_view}"

echo 'F-007 节点可视化配置入口、同图投影、模板侧栏与只读运行边界结构检查通过'
