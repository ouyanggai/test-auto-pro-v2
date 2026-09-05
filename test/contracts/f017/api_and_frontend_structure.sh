#!/usr/bin/env bash
# F-017 接口与前端结构契约：可用命令集合来自后端；越界入口一律禁止。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-017] 控制端点已注册'
grep -qF 'POST /api/runs/{runId}/breakpoints' internal/api/runs.go
grep -qF 'DELETE /api/runs/{runId}/breakpoints' internal/api/runs.go
grep -qF 'POST /api/runs/{runId}/pause' internal/api/runs.go

printf '%s\n' '[F-017] 可用命令集合由后端按模式与状态计算'
grep -qF 'func AvailableCommands' internal/engine/control/commands.go
grep -qF 'commands' internal/service/run_orchestration.go

printf '%s\n' '[F-017] 断点集合由事实回放得出，无断点可变表'
grep -qF 'func ReplayBreakpoints' internal/engine/control/breakpoints.go
if grep -rln 'UPDATE run_controls\|DELETE FROM run_controls' internal/ | grep -v migrations; then
  printf '%s\n' '[F-017] run_controls 不得存在 UPDATE/DELETE 路径' >&2
  exit 1
fi
if ls internal/repository/mysql/migrations/ | grep -vE '026|027' | grep -q breakpoint; then
  printf '%s\n' '[F-017] 不得新建可变断点表' >&2
  exit 1
fi

printf '%s\n' '[F-017] 暂停只在阶段 3 生效，恢复不自动继续暂停态'
grep -qF 'ControlFactPauseRequested' internal/engine/control/control.go
grep -qF "status IN ('running', 'verifying')" internal/repository/mysql/run_repository.go

printf '%s\n' '[F-017] 前端：模式三选一与断点预置在启动对话框'
grep -qF "value: 'single_step'" web/src/features/run-readiness/RunPreflightDialog.vue
grep -qF "value: 'auto'" web/src/features/run-readiness/RunPreflightDialog.vue
grep -qF "value: 'manual_control'" web/src/features/run-readiness/RunPreflightDialog.vue

printf '%s\n' '[F-017] 前端：控制命令区按后端集合渲染，无跳过/跳节点/重置入口'
grep -qF 'detail.commands' web/src/views/RunDetailView.vue
grep -qF '回到当前步' web/src/views/RunDetailView.vue
if grep -rnE '跳过|跳节点|重置实例|重置目标' web/src/views/RunDetailView.vue web/src/features/runs/; then
  printf '%s\n' '[F-017] 界面不得出现跳过/跳节点/重置入口' >&2
  exit 1
fi

printf '%s\n' '[F-017] 接口与前端结构契约通过'
