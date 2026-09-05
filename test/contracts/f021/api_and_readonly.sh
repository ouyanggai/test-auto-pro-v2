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
grep -qF 'run-detail__events' "${panel}" || fail '运行详情缺少事件流时间线'
grep -qF 'copyLogRef' web/src/features/runs/RunNodePanel.vue || fail '尝试行缺少日志位置复制'
# 事件流必须按游标增量：只能 push 追加，不能整体覆盖历史。
if grep -E 'runEvents\.value =' "${panel}" | grep -v 'runEvents.value.push' >/dev/null 2>&1; then
  fail '事件流被整体覆盖，违背增量追加语义'
fi

printf '%s\n' 'F-021 只读契约检查通过'
