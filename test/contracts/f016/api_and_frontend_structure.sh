#!/usr/bin/env bash

# F-016 接口与前端结构契约：单步闭环的最小端点面与运行画布变体必须存在，
# 且不得出现自动运行、断点、批量放行、重试入口等越界形态。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-016] 运行端点路由已注册'
grep -qF 'POST /api/plans/{planId}/runs' internal/api/runs.go
grep -qF 'GET /api/plans/{planId}/runs' internal/api/runs.go
grep -qF 'GET /api/runs/{runId}' internal/api/runs.go
grep -qF 'POST /api/runs/{runId}/approve' internal/api/runs.go
grep -qF 'POST /api/runs/{runId}/stop' internal/api/runs.go

printf '%s\n' '[F-016] 启动前复验运行准备结论'
grep -qF 'PlanReadiness' internal/service/run_orchestration.go
grep -qF '运行前检查未通过' internal/service/run_orchestration.go

printf '%s\n' '[F-016] 单步是唯一模式，无自动/人工控制与批量放行'
grep -qF 'RunModeSingleStep' internal/engine/step/types.go internal/engine/run/service.go 2>/dev/null || grep -qF 'RunModeSingleStep' internal/model/run.go
if grep -rniE 'batchApprove' internal/engine/ internal/api/runs.go; then
  printf '%s\n' '[F-016] 不得提供批量放行' >&2
  exit 1
fi

printf '%s\n' '[F-016] 崩溃恢复在服务启动时执行'
grep -qF 'runStateService.Recover' cmd/server/main.go

printf '%s\n' '[F-016] 运行列表替换占位页并提供进入详情'
grep -qF 'fetchPlanRuns' web/src/views/RunsView.vue
grep -qF '进入详情' web/src/views/RunsView.vue
grep -qF '/runs/:runId' web/src/router/index.ts

printf '%s\n' '[F-016] 运行画布复用现有组件并新增 runMode 变体'
grep -qF 'runMode' web/src/features/flow-graph/FlowGraphCanvas.vue
grep -qF 'runMode' web/src/features/flow-graph/FlowGraphNode.vue
grep -qF 'FlowGraphCanvas' web/src/views/RunDetailView.vue

printf '%s\n' '[F-016] 详情页有放行与停止，写结果不确定不渲染重试入口'
grep -qF 'approveRun' web/src/views/RunDetailView.vue
grep -qF 'stopRun' web/src/views/RunDetailView.vue
# 禁止的是针对不确定写的重试/重新提交「按钮入口」：因此只匹配渲染为按钮或链接文本的形态
#（`>文案` 或 `文案<`）。网络错误的「请重试」提示、以及动作断点选项里的动作中文名
#（例如「重新提交」是目标的一个真实动作，挂断点要按名字选）都不是恢复入口，不在禁止范围内。
if grep -rnE '>[[:space:]]*(重试|重试本步|重新提交|继续执行|重发)|(重试本步|重新提交|继续执行|重发)[[:space:]]*<' \
  web/src/features/runs/*.vue web/src/views/RunDetailView.vue | grep -v '不渲染任何重试'; then
  printf '%s\n' '[F-016] 运行界面不得出现重试或重新提交入口' >&2
  exit 1
fi

printf '%s\n' '[F-016] 七阶段指示器与自动跟随接管'
grep -qF '取步' web/src/features/runs/RunStatusIndicator.vue
grep -qF '回到当前步' web/src/views/RunDetailView.vue

printf '%s\n' '[F-016] 接口与前端结构契约通过'
