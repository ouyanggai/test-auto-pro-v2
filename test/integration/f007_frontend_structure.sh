#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
config_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
config_api="${project_root}/web/src/features/path-configuration/api.ts"
config_logic="${project_root}/web/src/features/path-configuration/logic.ts"
config_types="${project_root}/web/src/features/path-configuration/types.ts"
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
grep -Fq "新增路径" "${paths_view}"
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
grep -Fq "const formGenerating = ref(false)" "${config_view}"
grep -Fq "const formSaving = ref(false)" "${config_view}"
grep -Fq ":loading=\"formGenerating && formGenerationKind === 'smart'\"" "${config_view}"
grep -Fq ":loading=\"formGenerating && formGenerationKind === 'next'\"" "${config_view}"
grep -Fq ":loading=\"formSaving\"" "${config_view}"
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

# F-008 已独立节点动作协议；F-007 只保留同路径节点推进和表单工作区隔离，不能重新断言已删除的旧动作模型。
grep -Fq "resolveConfirmedNodeSaveDestination" "${config_logic}"
grep -Fq "copyPathConfigActions" "${config_logic}"
grep -Fq "actionConfigurations" "${config_panel}"
grep -Fq "copyPathConfigActions" "${config_view}"
if grep -Eq 'structuredClone\((draft\.actionConfigurations|props\.draft\.actionConfigurations|value\))' "${config_logic}" "${config_panel}" "${config_view}"; then
  echo 'F-007 节点保存链不得直接 structuredClone Vue 响应式草稿' >&2
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
grep -Fq 'form' "${config_view}"
if grep -Eq 'nextUnconfiguredPath|configureNextPath|hasNextPath|configureNext|配置下一条' "${config_view}" "${config_panel}"; then
  echo 'F-007 节点配置页不得把下一节点误写成另一条路径' >&2
  exit 1
fi

# 节点侧栏不承载表单字段；真实 FormMaking 工作区仍独立渲染。
if grep -Eq 'node\.fields|node\.gaps|暂不支持|NDatePicker|NCheckbox|NSwitch' "${config_panel}"; then
  echo 'F-007 节点侧栏不得呈现表单字段、组件缺口或模拟目标表单控件' >&2
  exit 1
fi

# iframe 契约必须隔离 SID、拒绝迟到会话并阻断目标写接口；完整 values 只经 getValues 返回。
grep -Fq "sessionId" "${form_frame}"
grep -Fq "event.source !== iframe.value?.contentWindow" "${form_frame}"
grep -Fq "event.origin !== runtimeOrigin.value" "${form_frame}"
grep -Fq "getValues" "${runtime_app}"
grep -Fq "captureFormValues" "${runtime_app}"
grep -Fq "fieldRules: props.form.fieldRules" "${form_frame}"
grep -Fq "setGeneratedData" "${form_frame}"
grep -Fq "current.form.conditionBindings = generated.conditionBindings" "${config_view}"
grep -Fq "current.form.conditionReviews = generated.conditionReviews" "${config_view}"
grep -Fq "current.form.fieldRules = generated.fieldRules" "${config_view}"
grep -Fq "await frame.setGeneratedData" "${config_view}"
if grep -Fq "reloadRuntime" "${form_frame}" || grep -Fq "reloadRuntime" "${config_view}"; then
  echo 'F-007 智能生成不得销毁并重建真实表单运行时' >&2
  exit 1
fi
grep -Fq "当前路径分支条件" "${config_view}"
grep -Fq "v-if=\"binding.selected\"" "${config_view}"
grep -Fq "字段已锁定" "${config_view}"
grep -Fq "需要人工核对" "${config_view}"
grep -Fq "binding.expression" "${config_view}"
if grep -Fq "无法精确映射 · 可编辑" "${config_view}"; then
  echo 'F-007 条件提示不得把无法安全映射误导为可编辑' >&2
  exit 1
fi
grep -Fq "pathConfigurationStatusName" "${config_view}"
grep -Fq "pathConfigurationStatusName" "${config_panel}"
grep -Fq "conditionHintKey" "${config_plan_analyzer}" || grep -Fq "conditionHintKey" "${project_root}/internal/service/path_config_workspace.go"
if grep -Eq '需要重新确认|配置失效|受影响需确认' "${config_view}" "${config_panel}"; then
  echo 'F-007 面向用户的配置页不得渲染旧确认或失效状态文案' >&2
  exit 1
fi
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

# 父页面只失效会话，iframe 子组件独占幂等销毁；离开后不得接受迟到的运行时或 HTTP 回写。
grep -Fq "function invalidateRuntimeSession()" "${config_view}"
grep -Fq "runtimeSessionController?.abort()" "${config_view}"
grep -Fq "formOperationController?.abort()" "${config_view}"
grep -Fq "function isActiveFormOperation" "${config_view}"
grep -Fq "function handleRuntimeError" "${config_view}"
if grep -Fq "formFrame.value?.destroyRuntime()" "${config_view}"; then
  echo 'F-007 父页面不得和 iframe 子组件重复销毁表单运行时' >&2
  exit 1
fi
grep -Fq "let runtimeGeneration = 0" "${form_frame}"
grep -Fq "function resetRuntime(notifyFrame: boolean)" "${form_frame}"
grep -Fq "if (!runtimeActive && pending.size === 0) return" "${form_frame}"
grep -Fq "generation !== runtimeGeneration" "${form_frame}"

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
assert.match(view, /useNotification/)
assert.doesNotMatch(view, /path-configuration-page__form-feedback/)
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
grep -Fq "normalizedPathConfigSeed" "${config_logic}"
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
