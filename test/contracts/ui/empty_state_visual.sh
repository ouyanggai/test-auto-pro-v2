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
run_empty='web/src/features/runs/RunListEmptyState.vue'

grep -qF ':component-options="componentOptions"' "${app}" || fail '应用没有接入全局空态配置'
grep -qF 'Empty:' "${app}" || fail '组件库空态没有统一覆盖'
grep -qF 'renderIcon: () => h(AppEmptyIcon)' "${app}" || fail '组件库仍可能显示默认叉号图标'
grep -qF 'aria-hidden="true"' "${icon}" || fail '装饰性空态插画会被辅助技术重复朗读'
grep -qF '<AppEmptyIcon />' "${run_empty}" || fail '运行记录空态没有复用全局插画'

printf '%s\n' '界面空态视觉契约检查通过'
