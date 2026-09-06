#!/usr/bin/env bash

# F-015 接口与前端结构契约：锁定只读边界、业务语言与阻塞提醒分区，防止后续改动悄悄破坏这几条。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-015] $1" >&2
  exit 1
}

api='internal/api/run_readiness.go'
[ -f "${api}" ] || fail "缺少运行准备接口文件：${api}"

# 只读端点必须注册；本文件不得出现启动运行一类的写端点（启动入口由 F-016 起的前端经运行模块 API 承担）。
for route in 'GET /api/plans/{id}/run-readiness'; do
  grep -qF "${route}" "${api}" || fail "接口未注册：${route}"
done
if grep -qEi 'POST /api/plans/\{id\}/runs|/start|/pause|/resume' "${api}"; then
  fail '运行准备接口出现了启动或运行控制端点，它们属于 F-016 之后的切片'
fi

# 成功断言已按用户决定移除，后续用断点表达"跑到哪里算成功"，这里防止它悄悄回归。
if [ -e 'internal/engine/assert' ] || [ -e 'internal/model/path_success_assertion.go' ]; then
  fail '成功断言已移除，不应再出现相关代码'
fi
if grep -RIn 'success-assertion' internal web/src >/dev/null 2>&1; then
  fail '成功断言端点或界面入口又出现了'
fi

# 前端：两个组件必须存在，且阻塞与提醒分区、没有启动入口。
panel='web/src/features/run-readiness/RunPreflightDialog.vue'
[ -f "${panel}" ] || fail "缺少前端组件：${panel}"
grep -qF 'data-testid="run-preflight-dialog"' "${panel}" || fail '预检结果必须用组件库弹窗承载'
grep -qF 'n-modal' "${panel}" || fail '预检弹窗必须使用组件库的 NModal，不自造弹层'
grep -qF 'data-testid="run-readiness-blocks"' "${panel}" || fail '预检弹窗缺少阻塞分区'
# 2026-09-06 用户裁决：提醒区从弹窗移除——只有明确的阻塞才需要用户处理。
if grep -qF 'run-readiness-reminders' "${panel}"; then
  fail '预检弹窗仍渲染提醒区，用户已裁决移除'
fi
# 文案必须是大白话：检查模板（用户可见部分），禁止再出现“断点”“写请求”“安全阀”这类词。
template_start=$(grep -n '^<template>' "${panel}" | head -1 | cut -d: -f1)
if tail -n "+${template_start}" "${panel}" | grep -qE '断点|写请求|安全阀'; then
  fail '弹窗文案出现内部概念词，用户裁决要求大白话'
fi
# 运行按钮在计划列表：模板形态或 h() 渲染形态任一即可。
if ! grep -qF "'data-testid': 'plan-run-button'" web/src/views/PlansView.vue; then
  if ! grep -qF 'data-testid="plan-run-button"' web/src/views/PlansView.vue; then
    fail '计划列表缺少运行按钮'
  fi
fi
# 2026-09-06 用户裁决：运行入口改放到计划列表；路径页不得再出现第二套运行发起入口。
if grep -qE 'data-testid="plan-run-button"|openPreflight' web/src/views/PlanPathsView.vue; then
  fail '路径页仍残留运行发起入口，应只保留计划列表一处'
fi
grep -qF 'pathIds' web/src/features/run-readiness/api.ts || fail '预检必须只检查勾选路径'
# 2026-09-05 更新：F-016 已交付启动运行、F-017 已交付运行模式三选一，预检弹窗包含运行模式与开始运行
# 属现行产品行为；本脚本不再把「本切片当时未交付启动」当成永久边界反向锁定。
# 纲领 12.1 硬性禁止的反向断言：本弹窗内不得出现裸 input/select，必须用既有组件库控件。
if grep -qE '<input|<select' "${panel}"; then
  fail '预检弹窗出现了裸 input/select，必须改用组件库控件（纲领 12.1）'
fi
# 运行方式必须是可选中的真实控件（radio 语义卡片，2026-09-06 用户裁决改版）。
grep -qF 'role="radio"' "${panel}" || fail '运行方式必须用 radio 语义的卡片控件'
grep -qF 'run-preflight__card--active' "${panel}" || fail '运行方式选中态必须可见（卡片描边 + 标题变色，不只靠颜色）'
grep -qF 'NCheckbox' "${panel}" || fail '写入前确认开关必须使用 NCheckbox'
# 界面不得出现目标内部标识：节点键既不能让用户输入，也不能回显。
if grep -qE '节点键|nodeBreakpointInput|bp\.nodeKey' "${panel}"; then
  fail '界面出现了内部标识（节点键），纲领 12.1 禁止'
fi
# 启动失败必须有界面反馈：startError 必须在模板里被渲染。
grep -qE 'v-if="startError"' "${panel}" || fail '启动失败信息没有在界面显示'
# 界面只出现业务语言：不允许把内部稳定键当文案，也不允许出现内部术语。
if grep -qE '历史来源|历史回放|success_claim|confirmed_failure' "${panel}"; then
  fail '界面出现了内部术语或内部稳定键'
fi
for forbidden in 'var(--n-color'; do
  if grep -qF "${forbidden}" "${panel}"; then
    fail "组件直接引用了 naive 内部变量 ${forbidden}，应改用 useThemeVars"
  fi
done

# 已删除的旧布尔判断不得回归。
if grep -RIn 'IsExecutionPathRunnable' internal web/src >/dev/null 2>&1; then
  fail 'IsExecutionPathRunnable 又出现了，本切片要求删除且不保留兼容层'
fi

printf '%s\n' 'F-015 接口与前端结构契约检查通过'
