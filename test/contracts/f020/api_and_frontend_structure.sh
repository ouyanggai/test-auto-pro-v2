#!/usr/bin/env bash

# F-020 接口与前端结构契约：多路径启动、按路径寻址的控制端点与运行级摘要。
# 同时反向锁定：写端点白名单未扩张（调度器不直接调用目标平台）。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-020] $1" >&2
  exit 1
}

api='internal/api/runs.go'
scheduling='internal/service/run_scheduling.go'
[ -f "${api}" ] || fail "缺少运行接口文件：${api}"
[ -f "${scheduling}" ] || fail "缺少多路径编排文件：${scheduling}"

# 启动必须按勾选路径集合：请求体携带 pathIds 数组，服务层逐条复验运行前检查。
grep -qE 'PathIDs[[:space:]]+\[\]uint64' "${api}" || fail '启动请求体必须携带勾选路径集合 pathIds'
grep -qF 'StartRunWithPaths' "${scheduling}" || fail '缺少多路径启动入口'
grep -qF '运行前检查未通过，不能启动' "${scheduling}" || fail '多路径启动必须复验运行前检查'

# 控制端点必须支持按路径运行寻址：pathRunId 查询参数贯穿 API 与服务层。
grep -qF 'parsePathRunIDQuery' "${api}" || fail '控制端点缺少路径运行寻址解析'
grep -qF 'RunDetailByRunAndPathRun' internal/service/run_orchestration.go || fail '详情必须支持按路径运行读取'

# 幂等：启动请求携带幂等键，仓储按（计划, 键）唯一约束防重。
grep -qF 'IdempotencyKey' internal/repository/run.go || fail '启动缺少幂等键约束'
grep -q 'uk_runs_plan_idempotency' internal/repository/mysql/migrations/030_f020_multi_path_scheduling.sql || fail '迁移缺少幂等唯一索引'

# 定时一次性消费：消费标记与原子领取必须存在。
grep -qF 'ClaimScheduledPlan' internal/repository/mysql/run_repository_scheduling.go || fail '定时领取缺少原子消费'
grep -q 'scheduled_consumed_at' internal/repository/mysql/migrations/030_f020_multi_path_scheduling.sql || fail '迁移缺少定时消费标记'

# 前端：预检弹窗启动全部勾选可执行路径；运行详情有多路径切换区。
grep -qF 'pathIds: pathIds.map(Number)' web/src/features/runs/api.ts || fail '前端启动必须提交勾选路径集合'
grep -qF 'run-detail__paths' web/src/views/RunDetailView.vue || fail '运行详情缺少路径切换区'
grep -qF 'pathsSummary' web/src/views/RunsView.vue || fail '运行列表缺少路径状态中文汇总'

# 调度器不得直接调用目标平台：internal/engine/schedule 不出现 target 引用。
if grep -RIn 'adapter/target' internal/engine/schedule >/dev/null 2>&1; then
  fail '调度器出现了目标平台引用，它只做分配不发请求'
fi

printf '%s\n' 'F-020 接口与前端结构契约检查通过'
