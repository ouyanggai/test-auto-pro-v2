#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[界面空态] $1" >&2
  exit 1
}

app='web/src/App.vue'
icon='web/src/components/AppEmptyIcon.vue'
empty_state='web/src/components/AppEmptyState.vue'
plans='web/src/views/PlansView.vue'
runs='web/src/views/RunsView.vue'

grep -qF ':component-options="componentOptions"' "${app}" || fail '应用没有接入全局空态配置'
grep -qF 'Empty:' "${app}" || fail '组件库空态没有统一覆盖'
grep -qF 'renderIcon: () => h(AppEmptyIcon)' "${app}" || fail '组件库仍可能显示默认叉号图标'
grep -qF '<n-el tag="span" class="app-empty-icon"' "${icon}" || fail '空态插画没有接入主题变量'
grep -qF 'aria-hidden="true"' "${icon}" || fail '装饰性空态插画会被辅助技术重复朗读'
grep -qF '<AppEmptyIcon />' "${empty_state}" || fail '业务空态没有复用全局插画'
grep -qF 'class="app-empty-state__message" role="status"' "${empty_state}" || fail '空态消息缺少独立的可访问状态语义'
grep -qF '<app-empty-state' "${plans}" || fail '计划列表没有使用统一业务空态'
grep -qF 'v-if="!loading"' "${plans}" || fail '计划列表加载时仍可能暴露空态操作'
grep -qF '<AppEmptyState' "${runs}" || fail '运行记录没有使用统一业务空态'
grep -qF '新建计划' "${plans}" || fail '计划列表空态缺少下一步入口'

# 读取失败与空数据必须使用不同组件，避免全局空态插画掩盖真实错误。
for error_surface in \
  web/src/features/plans/FlowCandidateList.vue \
  web/src/features/flow-graph/FlowGraphCanvas.vue \
  web/src/views/RunPathsView.vue \
  web/src/views/RunDetailView.vue \
  web/src/views/PlanPathsView.vue \
  web/src/views/PlanPathConfigurationView.vue; do
  grep -qiE '<n-result|<NResult' "${error_surface}" || fail "错误状态未与空数据分离：${error_surface}"
  grep -qiF 'role="alert"' "${error_surface}" || fail "错误状态不会被辅助技术播报：${error_surface}"
done

if grep -RInE '<[nN]-?[Ee]mpty[^>]*(:description="(error|layoutResult)|暂时无法|读取失败|不可用)' web/src --include='*.vue' >/dev/null; then
  fail '仍有读取失败或不可用状态误用空数据组件'
fi

printf '%s\n' '界面空态视觉契约检查通过'
