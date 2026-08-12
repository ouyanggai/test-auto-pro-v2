#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
config_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
config_api="${project_root}/web/src/features/path-configuration/api.ts"
config_logic="${project_root}/web/src/features/path-configuration/logic.ts"
config_panel="${project_root}/web/src/features/path-configuration/NodeConfigurationPanel.vue"
config_analyzer="${project_root}/internal/analyzer/path_config.go"
config_plan_analyzer="${project_root}/internal/analyzer/path_config_plan.go"
form_frame="${project_root}/web/src/features/path-configuration/FormRuntimeFrame.vue"
runtime_app="${project_root}/form-runtime/src/App.vue"
runtime_template="${project_root}/form-runtime/src/runtime/formTemplate.js"
runtime_protocol="${project_root}/form-runtime/src/runtime/protocol.js"
runtime_policy="${project_root}/form-runtime/src/runtime/requestPolicy.js"
canvas_view="${project_root}/web/src/features/flow-graph/FlowGraphCanvas.vue"
flow_node="${project_root}/web/src/features/flow-graph/FlowGraphNode.vue"
routing_hub="${project_root}/web/src/features/flow-graph/FlowRoutingHub.vue"
feature_doc="${project_root}/docs/features/F-007-path-data-action-configuration.md"

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
grep -Fq '@node-click="handleConfigurationNodeClick"' "${canvas_view}"
grep -Fq '@nodes-change="handleConfigurationNodeChanges"' "${canvas_view}"
grep -Fq ':nodes-focusable="configurationMode"' "${canvas_view}"
grep -Fq 'selectable: configurationInteractive' "${canvas_view}"
grep -Fq 'focusable: configurationInteractive' "${canvas_view}"
grep -Fq '.vue-flow__node.selectable' "${canvas_view}"
grep -Fq '!props.configurationNodeStates[nodeID]?.interactive' "${canvas_view}"
grep -Fq "change.type !== 'select' || !change.selected" "${canvas_view}"
if grep -Eq '@select="handleSelectConfigurationNode|defineEmits<\{ select:' "${canvas_view}" "${flow_node}" "${routing_hub}"; then
  echo 'F-007 节点切换必须统一走 Vue Flow 官方包装层事件，不能保留内部平行点击链' >&2
  exit 1
fi
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

# 节点保存正常响应和 GET 对账必须复用同一同路径推进规则，不得再出现跨路径“配置下一条”。
grep -Fq "resolveConfirmedNodeSaveDestination" "${config_logic}"
grep -Fq "copyPathConfigArrivals" "${config_logic}"
grep -Fq "pathConfigActionRowsToArrivals" "${config_panel}"
grep -Fq "copyPathConfigArrivals" "${config_view}"
if grep -Eq 'structuredClone\((draft\.arrivals|props\.draft\.arrivals|value\))' "${config_logic}" "${config_panel}" "${config_view}"; then
  echo 'F-007 节点动作保存链不得直接 structuredClone Vue 响应式草稿' >&2
  exit 1
fi
grep -Fq "savePathConfigurationNode(planID.value, pathID.value, node.key" "${config_view}"
grep -Fq "async function finishConfirmedNodeSave" "${config_view}"
if [[ "$(grep -Fc 'await finishConfirmedNodeSave()' "${config_view}")" -ne 2 ]]; then
  echo 'F-007 正常保存与响应不确定对账必须共同推进当前路径下一节点' >&2
  exit 1
fi
grep -Fq 'selectedNodeID.value = destination.nodeID' "${config_view}"
grep -Fq 'await focusSelectedNode()' "${config_view}"
grep -Fq ':form-complete="configuration.form.status === '\''valid'\''"' "${config_view}"
grep -Fq '>配置表单数据</n-button>' "${config_panel}"
grep -Fq '当前路径的节点与表单配置均已完成' "${config_panel}"
if grep -Eq 'nextUnconfiguredPath|configureNextPath|hasNextPath|configureNext|配置下一条' "${config_view}" "${config_panel}"; then
  echo 'F-007 节点配置页不得把下一节点误写成另一条路径' >&2
  exit 1
fi

# 节点侧栏只呈现人员和动作；模板要求收进标题弹层，表单字段仍由独立 FormMaking runtime 渲染。
grep -Fq "node.requirements" "${config_panel}"
grep -Fq "NPopover" "${config_panel}"
grep -Fq "查看模板要求" "${config_panel}"
grep -Fq "node.persons" "${config_panel}"
grep -Fq "person.editable" "${config_panel}"
grep -Fq "人员策略" "${config_panel}"
grep -Fq "person.strategies" "${config_panel}"
grep -Fq "最终使用" "${config_panel}"
grep -Fq "person.items" "${config_panel}"
grep -Fq "NModal" "${config_panel}"
grep -Fq "NScrollbar" "${config_panel}"
grep -Fq "查看全部" "${config_panel}"
grep -Fq "summarizePathConfigPersonItems" "${config_logic}"
grep -Fq "运行时确定" "${config_panel}"
grep -Fq "动作计划" "${config_panel}"
grep -Fq "actionRows" "${config_panel}"
grep -Fq "pathConfigActionRowsFromArrivals" "${config_logic}"
grep -Fq "pathConfigActionRowsToArrivals" "${config_logic}"
grep -Fq "normalizedPathConfigActionCount" "${config_logic}"
grep -Fq "canUsePathConfigAction" "${config_logic}"
grep -Fq 'aria-label="动作执行次数"' "${config_panel}"
grep -Fq '固定 1 次' "${config_panel}"
grep -Fq 'placeholder="选择动作"' "${config_panel}"
grep -Fq "disabledReason" "${config_panel}"
grep -Fq 'class="node-configuration-panel__action-info"' "${config_panel}"
grep -Fq '次数表示计划真实执行次数，不是网络自动重试' "${config_panel}"
grep -Fq 'node-configuration-panel__action-rules' "${config_panel}"
grep -Fq ':consistent-menu-width="false"' "${config_panel}"
grep -Fq 'minmax(132px, 1fr)' "${config_panel}"
grep -Fq 'min-width: 132px' "${config_panel}"
grep -Fq 'margin-top: 14px' "${config_panel}"
grep -Fq 'padding-top: 12px' "${config_panel}"
grep -Fq 'color-mix(in srgb, var(--flow-direction-color)' "${config_panel}"
if grep -Fq 'node-configuration-panel__disabled-actions' "${config_panel}"; then
  echo 'F-007 禁用原因必须集中在动作信息弹层，不能继续平铺列表' >&2
  exit 1
fi
if sed -n '/node-configuration-panel__action-row/,/node-configuration-panel__add-action/p' "${config_panel}" | grep -Fq 'actionDefinition(row.kind)?.description'; then
  echo 'F-007 动作说明不得继续平铺在动作行内' >&2
  exit 1
fi
if sed -n '/<template>/,/<\/template>/p' "${config_panel}" | grep -Eq '复制前一次|删除末次|到达次数|次到达|第 \{\{|步骤|前置动作|处理意见'; then
  echo 'F-007 动作区不得再暴露到达、步骤、复制或处理意见概念' >&2
  exit 1
fi
grep -Fq '| 节点类别 | 动作 | 配置阶段静态条件 | 是否可多次 | 运行时复验 |' "${feature_doc}"
grep -Fq '动作次数不是网络重试次数' "${feature_doc}"
grep -Fq '| 发起 | 提交 |' "${feature_doc}"
grep -Fq '否，固定 1 次' "${feature_doc}"
grep -Fq '| 审批/协同 | 回退上一级 |' "${feature_doc}"
grep -Fq "rollbackTargets" "${config_panel}"
grep -Fq "add_sign" "${config_panel}"
grep -Fq "transfer_approver" "${config_panel}"
grep -Fq '不同意' "${config_plan_analyzer}"
grep -Fq '回退上一级' "${config_plan_analyzer}"
grep -Fq ':multiple="requiredActionPerson(row.kind).multiple"' "${config_panel}"
grep -Fq "resolvedPersonStrategySelection" "${config_logic}"
grep -Fq "validPathConfigArrivals" "${config_logic}"
grep -Fq "normalizedPathConfigSeed" "${config_logic}"
grep -Fq "Number.MAX_SAFE_INTEGER" "${config_logic}"
grep -Fq ':max="MAX_SAFE_PERSON_SEED"' "${config_panel}"
grep -Fq '"transfer_approver"' "${config_plan_analyzer}"
grep -Fq "'transfer_approver'" "${config_logic}"
grep -Fq "overflow-y: auto" "${config_panel}"
grep -Fq "保存当前节点" "${config_panel}"
if grep -Eq 'node\.fields|node\.gaps|暂不支持|NDatePicker|NCheckbox|NSwitch' "${config_panel}"; then
  echo 'F-007 节点侧栏不得呈现表单字段、组件缺口或模拟目标表单控件' >&2
  exit 1
fi
if grep -Eq '名称需运行时解析|已配置 [^" ]+ 项范围' "${config_analyzer}"; then
  echo 'F-007 人员公开列表不得保留模糊名称或范围数量兜底' >&2
  exit 1
fi

# iframe 契约必须隔离 SID、拒绝迟到会话并阻断目标写接口；完整 values 只经 getValues 返回。
grep -Fq "sessionId" "${form_frame}"
grep -Fq "event.source !== iframe.value?.contentWindow" "${form_frame}"
grep -Fq "event.origin !== runtimeOrigin.value" "${form_frame}"
grep -Fq "getValues" "${runtime_app}"
grep -Fq "captureFormValues" "${runtime_app}"
grep -Fq "form.getData(true)" "${runtime_template}"
grep -Fq "delete config[hook]" "${runtime_template}"
grep -Fq "component.el" "${runtime_template}"
grep -Fq "TARGET_COMPONENT_NAMES.has(targetComponentName)" "${runtime_template}"
grep -Fq "refreshPreparedForm(this.form())" "${runtime_app}"
if grep -Eq 'applyFieldPermissions|form\.disabled\(|form\.hide\(' "${runtime_app}"; then
  echo 'F-007 表单刷新不得统一调用不兼容自定义组件权限 API' >&2
  exit 1
fi
if grep -Fq '<el-alert' "${runtime_app}"; then
  echo 'F-007 iframe 不得重复渲染宿主已经展示的阻塞告警' >&2
  exit 1
fi
grep -Fq "destroySession" "${runtime_app}"
grep -Fq "FORM_RUNTIME_VERSION" "${runtime_protocol}"
grep -Fq "targetRequestAllowed" "${runtime_policy}"
grep -Fq "KNOWN_TARGET_ORIGINS" "${runtime_policy}"
grep -Fq "rewriteTargetPath" "${runtime_policy}"
grep -Fq "WRITE_SEGMENT_PREFIXES" "${runtime_policy}"
grep -Fq "READ_SEGMENT_PREFIXES" "${runtime_policy}"
grep -Fq "window.fetch = async" "${runtime_policy}"
grep -Fq "SID" "${runtime_policy}"

# 表单模式必须用真实视口公式抵消 app-main 留白；不能只声明 class 或零散 flex 字段造成假通过。
node --input-type=module - "${config_view}" "${form_frame}" <<'NODE'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const view = readFileSync(process.argv[2], 'utf8')
const frame = readFileSync(process.argv[3], 'utf8')
const escapeRegExp = value => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const rule = (selector) => {
  const match = view.match(new RegExp(`${escapeRegExp(selector)}\\s*\\{([^}]*)\\}`, 's'))
  assert.ok(match, `缺少样式规则：${selector}`)
  return match[1]
}
const pxVariable = (name) => {
  const match = view.match(new RegExp(`${escapeRegExp(name)}:\\s*(\\d+)px`))
  assert.ok(match, `缺少像素变量：${name}`)
  return Number(match[1])
}

assert.match(view, /'path-configuration-page--form': workspace === 'form'/)
const formPage = rule('.path-configuration-page--form')
assert.match(formPage, /grid-template-rows:\s*minmax\(0, 1fr\)/)
assert.match(formPage, /width:\s*calc\(100% \+ var\(--path-config-main-inline-padding\) \+ var\(--path-config-main-inline-padding\)\)/)
assert.match(formPage, /height:\s*calc\(100dvh - var\(--path-config-app-header-height\)\)/)
assert.match(formPage, /margin:\s*calc\(0px - var\(--path-config-main-block-padding\)\) calc\(0px - var\(--path-config-main-inline-padding\)\)/)
assert.match(formPage, /overflow:\s*hidden/)

const hiddenChrome = view.match(/\.path-configuration-page--form > \.path-configuration-page__header,\s*\.path-configuration-page--form > \.path-configuration-page__switch\s*\{([^}]*)\}/s)
assert.ok(hiddenChrome)
assert.match(hiddenChrome[1], /display:\s*none/)
const flatStage = rule('.path-configuration-page--form > .path-configuration-page__stage')
assert.match(flatStage, /height:\s*100%/)
assert.match(flatStage, /border:\s*0/)
assert.match(flatStage, /border-radius:\s*0/)

const formWorkspace = rule('.path-configuration-page__form-workspace')
assert.match(formWorkspace, /display:\s*flex/)
assert.match(formWorkspace, /overflow:\s*hidden/)
const toolbar = rule('.path-configuration-page__form-toolbar')
assert.match(toolbar, /height:\s*var\(--path-config-form-toolbar-height\)/)
assert.match(toolbar, /flex:\s*0 0 auto/)
const feedback = rule('.path-configuration-page__form-feedback')
assert.match(feedback, /position:\s*static/)
assert.match(feedback, /flex:\s*0 0 auto/)
assert.doesNotMatch(feedback, /z-index|top:|right:|left:/)
const iframeHost = rule('.path-configuration-page__form-frame')
assert.match(iframeHost, /flex:\s*1 1 0/)
assert.match(iframeHost, /width:\s*100%/)
assert.match(iframeHost, /height:\s*100%/)
assert.match(iframeHost, /overflow:\s*hidden/)
assert.match(frame, /\.form-runtime-frame\s*\{[^}]*width:\s*100%[^}]*height:\s*100%[^}]*min-height:\s*0/s)

const appHeader = pxVariable('--path-config-app-header-height')
const toolbarHeight = pxVariable('--path-config-form-toolbar-height')
assert.ok(768 - appHeader - toolbarHeight >= 600, '1366×768 下 iframe 计算高度必须至少约 600px')
assert.doesNotMatch(`${view}\n${frame}`, /transform:\s*scale|zoom:/)
NODE

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
