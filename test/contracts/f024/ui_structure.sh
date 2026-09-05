#!/usr/bin/env bash

# F-024 界面结构契约（纲领 12.3）：反向断言第 12.1 节六条禁止项中与本切片相关的部分。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-024] $1" >&2
  exit 1
}

panel='web/src/features/path-configuration/FormDataHintsPanel.vue'
[ -f "${panel}" ] || fail "缺少路径关键信息面板：${panel}"
view='web/src/views/PlanPathConfigurationView.vue'

# 禁止裸 input/select：本切片界面（节点视图切换器与提示面板）必须使用组件库控件。
for file in "${panel}" "${view}"; do
  if grep -qE '<input|<select ' "${file}"; then
    fail "界面出现裸 input/select（${file}），必须改用组件库控件"
  fi
done

# 视图切换器必须使用 NSelect 并带中文标签，不得只靠 placeholder 表达含义。
grep -qF 'n-select' "${panel}" || fail '节点视图切换器必须使用 NSelect'
grep -qF 'aria-label' "${panel}" || fail '节点视图切换器缺少中文 aria-label'

# 界面不得出现内部标识与字段英文名：标签缺失时必须回落通用中文说明。
if grep -qE 'nodeKey|\{\{ *field\.path|field\.path\b.*兜底' "${panel}"; then
  fail '面板出现了内部标识兜底，标签缺失必须回落中文说明'
fi
grep -qF '条件字段' "${panel}" || fail '字段标签缺失时必须回落「条件字段」通用说明'

# 视图身份按键匹配：同名节点的视图必须能各自选中与保存（防回退到按名称匹配）。
if grep -qF 'view.nodeName ===' "${view}"; then
  fail '视图选择退回了按节点名称匹配，同名节点会失效'
fi
grep -qF 'viewKey' "${panel}" || fail '视图选项必须以 viewKey 为值'
if grep -qF 'viewNodeName' "${view}" web/src/features/path-configuration/types.ts; then
  fail '保存请求仍携带旧的按名称视图身份 viewNodeName'
fi

# 更换历史数据回显必须按当前视图过滤：不得把整份样本值推回运行时。
if grep -qF 'setValues(data.effectiveFormData' "${view}"; then
  fail '更换历史数据未按视图过滤样本值，会回显本应留空的字段'
fi

printf '%s\n' 'F-024 界面结构契约检查通过'
