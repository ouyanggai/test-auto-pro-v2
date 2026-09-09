#!/usr/bin/env bash

# F-021 只读契约：事件流与列表筛选接口只读、前端无控制入口回退、日志引用可复制。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-021] $1" >&2
  exit 1
}

api='internal/api/runs.go'
scheduling='internal/service/run_scheduling.go'

# 事件流端点必须存在且只读（GET），带游标参数。
grep -qF 'GET /api/runs/{runId}/events' "${api}" || fail '缺少事件流只读端点'
grep -qF 'ListRunEvents' "${scheduling}" || fail '缺少事件流服务方法'
grep -qF 'ListRunsFiltered' internal/repository/run.go || fail '缺少状态筛选读取'

# 详情页：事件流与日志位置复制必须存在；不得出现重试/继续/跳过类只读违规入口。
panel='web/src/views/RunDetailView.vue'
list='web/src/views/RunsView.vue'
empty_state='web/src/features/runs/RunListEmptyState.vue'
grep -qF 'run-detail__events' "${panel}" || fail '运行详情缺少事件流时间线'
grep -qF 'copyLogRef' web/src/features/runs/RunNodePanel.vue || fail '尝试行缺少日志位置复制'
grep -qF '<RunListEmptyState' "${list}" || fail '运行列表缺少独立的正常空态'
grep -qF 'role="status"' "${empty_state}" || fail '正常空态缺少可访问状态语义'
grep -qF "filtered ? '没有符合条件的运行记录' : '还没有运行记录'" "${empty_state}" || fail '正常空态没有区分筛选为空与首次为空'
grep -qF '前往测试计划' "${empty_state}" || fail '首次空态缺少下一步入口'
grep -qF '查看全部记录' "${empty_state}" || fail '筛选空态缺少清除筛选入口'
if grep -qF '<NEmpty' "${list}"; then
  fail '运行列表仍在使用容易与错误混淆的默认空态'
fi
# 事件流必须按游标增量：轮询只允许 push 追加；
# 唯一允许的整体赋值是切换路径时的清空重置（runEvents.value = []，换游标后重拉当前路径）。
if ! grep -qF 'runEvents.value.push' "${panel}"; then
  fail '事件流未按游标增量追加'
fi
if grep -E 'runEvents\.value[[:space:]]*=' "${panel}" | grep -vE 'runEvents\.value[[:space:]]*= \[\]' >/dev/null 2>&1; then
  fail '事件流被整体覆盖成别的列表，违背增量追加语义'
fi

printf '%s\n' 'F-021 只读契约检查通过'
