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

printf '%s\n' '[F-017] 界面按纲领第 12 节基线：无裸控件、无陈旧文案、断点五类可区分'
# 12.1 禁止裸 input／select 拼界面：运行详情页的输入必须走组件库并带中文标签。
if grep -nE '<input |<select ' web/src/views/RunDetailView.vue web/src/features/runs/*.vue; then
  printf '%s\n' '[F-017] 运行详情页不得使用裸 input/select，必须用组件库并带中文标签' >&2
  exit 1
fi
# 12.1 禁止陈旧文案：模式已三选一，界面不得再宣称固定。
if grep -nF '（固定）' web/src/views/RunDetailView.vue; then
  printf '%s\n' '[F-017] 模式已支持三选一，界面不得再显示"（固定）"' >&2
  exit 1
fi
# 五类断点都要能在界面上区分，强制断点与可删断点分区。
for token in 节点断点 步骤断点 动作断点 首次写断点 路径偏离断点 强制生效 已挂载; do
  grep -qF "${token}" web/src/views/RunDetailView.vue || {
    printf '[F-017] 断点区缺少「%s」的中文表达\n' "${token}" >&2
    exit 1
  }
done
# 不可逆操作必须有前置确认：停止与人工结论登记都走二次确认。
grep -qF 'NPopconfirm' web/src/views/RunDetailView.vue

printf '%s\n' '[F-017] 接口与前端结构契约通过'
