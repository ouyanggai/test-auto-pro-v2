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

# 三个只读端点必须注册，且不得出现启动运行一类的写端点。
for route in \
  'GET /api/plans/{id}/execution-paths/{pathId}/success-assertion' \
  'PUT /api/plans/{id}/execution-paths/{pathId}/success-assertion' \
  'GET /api/plans/{id}/run-readiness'; do
  grep -qF "${route}" "${api}" || fail "接口未注册：${route}"
done
if grep -qEi 'POST /api/plans/\{id\}/runs|/start|/pause|/resume' "${api}"; then
  fail '运行准备接口出现了启动或运行控制端点，它们属于 F-016 之后的切片'
fi

# 断言判定包必须保持纯判定：不引入 IO、数据库与目标调用。
assert_dir='internal/engine/assert'
[ -d "${assert_dir}" ] || fail "缺少断言判定包：${assert_dir}"
if grep -RInE '"net/http"|"database/sql"|internal/adapter/target|internal/repository' "${assert_dir}" >/dev/null 2>&1; then
  fail '断言判定包引入了 IO、数据库或目标调用，违反纯判定边界'
fi
grep -qF 'OutcomeUndecidable' "${assert_dir}/assert.go" || fail '断言判定包缺少无法判定这一档'

# 前端：两个组件必须存在，且阻塞与提醒分区、没有启动入口。
panel='web/src/features/run-readiness/RunPreflightDialog.vue'
card='web/src/features/run-readiness/SuccessAssertionCard.vue'
for file in "${panel}" "${card}"; do
  [ -f "${file}" ] || fail "缺少前端组件：${file}"
done
grep -qF 'data-testid="run-preflight-dialog"' "${panel}" || fail '预检结果必须用组件库弹窗承载'
grep -qF 'n-modal' "${panel}" || fail '预检弹窗必须使用组件库的 NModal，不自造弹层'
grep -qF 'data-testid="run-readiness-blocks"' "${panel}" || fail '预检弹窗缺少阻塞分区'
grep -qF 'data-testid="run-readiness-reminders"' "${panel}" || fail '预检弹窗缺少提醒分区'
grep -qF 'data-testid="plan-run-button"' web/src/views/PlanPathsView.vue || fail '计划页缺少运行按钮'
# 断言入口必须在路径配置页顶部可见，不能只挂在固定高度的画布侧栏里被裁掉。
grep -qF 'data-testid="open-success-assertion"' web/src/views/PlanPathConfigurationView.vue || fail '路径配置页缺少成功断言入口'
grep -qF 'pathIds' web/src/features/run-readiness/api.ts || fail '预检必须只检查勾选路径'
grep -qF 'data-testid="success-assertion-card"' "${card}" || fail '成功断言卡片缺少测试标记'
# 本切片不交付启动运行：弹窗里只允许出现明确禁用的占位按钮，不得有可点击的启动入口。
if grep -qE '运行模式|单步|运行记录' "${panel}" "${card}"; then
  fail '本切片界面不得出现运行模式、单步或运行记录入口'
fi
if grep -q '开始运行' "${panel}" && ! grep -q 'disabled' "${panel}"; then
  fail '开始运行按钮必须是禁用占位，本切片不交付启动运行'
fi
# 界面只出现业务语言：不允许把内部稳定键当文案，也不允许出现内部术语。
if grep -qE '历史来源|历史回放|success_claim|confirmed_failure' "${panel}" "${card}"; then
  fail '界面出现了内部术语或内部稳定键'
fi
for forbidden in 'var(--n-color'; do
  if grep -qF "${forbidden}" "${panel}" "${card}"; then
    fail "组件直接引用了 naive 内部变量 ${forbidden}，应改用 useThemeVars"
  fi
done

# 已删除的旧布尔判断不得回归。
if grep -RIn 'IsExecutionPathRunnable' internal web/src >/dev/null 2>&1; then
  fail 'IsExecutionPathRunnable 又出现了，本切片要求删除且不保留兼容层'
fi

printf '%s\n' 'F-015 接口与前端结构契约检查通过'
