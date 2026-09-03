#!/usr/bin/env bash

# F-012 动作编排界面结构检查：node 的类型剥离运行器无法解析 .vue，改用结构断言覆盖
# 拖拽排序、门禁原因展示、实例动作工作区和只读编译预览。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"

editor="${project_root}/web/src/features/path-configuration/ActionOrchestrationEditor.vue"
preview="${project_root}/web/src/features/path-configuration/CompiledScenarioPreview.vue"
view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
panel="${project_root}/web/src/features/path-configuration/NodeConfigurationPanel.vue"

for required_file in "${editor}" "${preview}" "${view}" "${panel}"; do
  if [[ ! -f "${required_file}" ]]; then
    printf '%s\n' "[F-012] 缺少动作编排界面文件：${required_file}" >&2
    exit 1
  fi
done

# require 断言指定文件包含目标片段，缺失时给出功能语义而不是裸 grep 失败。
require() {
  local file="$1" pattern="$2" reason="$3"
  if ! grep -Fq -- "${pattern}" "${file}"; then
    printf '%s\n' "[F-012] ${reason}（${file} 缺少 ${pattern}）" >&2
    exit 1
  fi
}

# forbid 断言指定文件不包含目标片段。
forbid() {
  local file="$1" pattern="$2" reason="$3"
  if grep -Fq -- "${pattern}" "${file}"; then
    printf '%s\n' "[F-012] ${reason}（${file} 仍包含 ${pattern}）" >&2
    exit 1
  fi
}

# 动作编辑器必须支持真实拖拽排序，并保留键盘可达的上移/下移。
for handler in 'function reorderAction' 'function handleDragStart' 'function handleDragOver' 'function handleDrop' 'function handleDragEnd'; do
  require "${editor}" "${handler}" '动作编排缺少拖拽排序实现'
done
require "${editor}" ':draggable="!readOnly"' '动作行未开启拖拽'
require "${editor}" 'function expandActions' '动作次数没有展开成独立记录'
require "${editor}" 'function collapseActions' '已保存动作没有折叠回次数显示'
require "${editor}" '实例级动作（作用于整个流程实例）' '动作下拉缺少实例级分组'
require "${editor}" 'function renderActionOption' '动作下拉没有给出简短说明'
require "${editor}" 'requiredParameters' '动作参数没有按目录必填项渲染'
forbid "${editor}" 'parameterPlaceholder' '动作配置不应再出现自由填写的参数 JSON 框'
require "${editor}" '@drop.prevent="handleDrop(index)"' '动作行未绑定放置事件'
require "${editor}" 'moveAction(index, -1)' '动作排序缺少键盘可达的上移入口'
require "${editor}" 'moveAction(index, 1)' '动作排序缺少键盘可达的下移入口'

# 目录必须展示精确禁用原因、前置事实和重读要求，不能只隐藏动作。
require "${editor}" 'item.disabledReason' '动作目录未展示精确禁用原因'
require "${editor}" 'item.preconditions.length' '动作目录未展示前置事实'
require "${editor}" 'item.reloadRequirements.length' '动作目录未展示运行时重读要求'
require "${editor}" 'item.systemOnly' '动作目录未区分系统只读语义'

# 只读编译预览必须区分用户动作、系统恢复和系统导航三类来源。
require "${preview}" "user: '用户动作'" '编译预览缺少用户动作来源'
require "${preview}" "system_recovery: '系统恢复'" '编译预览缺少系统恢复来源'
require "${preview}" "system_navigation: '系统导航'" '编译预览缺少系统导航来源'
require "${preview}" '编译场景预览（只读）' '编译预览未声明只读'
require "${preview}" 'step.reloadRequired' '编译预览缺少重读事实屏障'
require "${preview}" 'step.stopOnFailure' '编译预览缺少失败停止条件'
require "${preview}" 'step.recoveryPolicy' '编译预览缺少恢复策略'

# 配置页必须挂载动作编辑器与只读预览，并提供独立的实例动作工作区。
require "${view}" 'ActionOrchestrationEditor' '配置页未挂载动作编排编辑器'
require "${panel}" 'CompiledScenarioPreview' '节点配置面板未挂载只读编译预览'
require "${view}" "workspace = ref<'nodes' | 'form'>" '配置页工作区应只保留节点配置与表单数据'
require "${panel}" 'node-configuration-panel__scenario' '节点配置面板缺少可折叠的编译步骤预览'
require "${panel}" 'function handleScenarioToggle' '编译步骤预览没有按需读取服务端结果'
forbid "${view}" '动作场景' '独立的动作场景工作区应已删除'
require "${view}" 'function loadCompiledScenario' '配置页未读取服务端编译场景'
require "${view}" 'function saveInstanceActions' '配置页无法保存实例作用域动作'
require "${view}" 'function updateInstanceActionConfiguration' '配置页缺少实例动作草稿写入'
require "${view}" 'instanceActionsComplete' '配置页未在保存前校验实例动作门禁'
require "${view}" 'draftHasUnsavedChanges' '离开配置页未拦截未保存的实例动作草稿'

# 浏览器只能读取编译步骤：编译结果来自 GET 或保存响应，不得由页面自行拼装并提交。
require "${project_root}/web/src/features/path-configuration/api.ts" "configuration/compiled-scenario" '缺少只读编译场景接口'
forbid "${view}" 'sourceActionKey:' '配置页不得自行拼装编译步骤'
forbid "${view}" "source: 'system_recovery'" '配置页不得伪造系统恢复步骤'
forbid "${view}" 'compiledScenario:' '配置页不得向服务端提交编译步骤'

printf '%s\n' 'F-012 动作编排界面结构检查通过'
