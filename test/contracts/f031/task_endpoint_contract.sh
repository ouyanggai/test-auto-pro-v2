#!/usr/bin/env bash

# F-031 结构契约检查：锁住「协议单一出口」与「当前处理人只来自目标事实」两条结构约束，
# 防止后续改动把已经修好的问题按语义猜字段位置重新引入。
# 只扫描源码，不发任何目标请求。
#
# 判定依据（docs/features/F-031-task-facts-and-instance-log-naming.md）：
#   1. `/web/flowJobTaskLink/list` 的实例筛选只能在协议顶层 flowInstanceIdList；
#      精确实例读取的三处调用必须全部走 task_query.go 的 buildTaskSnapshotBody。
#   2. 配置里的 NextNodeAuditors 只表示下一次流转要发送的选人参数，绝不参与当前任务发现。
#   3. 运行日志目录只由 logging 包的带实例作用域函数产生，业务代码不得自己拼路径。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-031] $1" >&2
  exit 1
}

query_file='internal/adapter/target/task_query.go'

# 1. 载荷出口存在，且只写协议顶层 flowInstanceIdList。
if [ ! -f "${query_file}" ]; then
  fail "缺少任务查询载荷出口：${query_file}"
fi
if ! grep -qF 'buildTaskSnapshotBody' "${query_file}"; then
  fail "载荷出口没有构造函数：${query_file}"
fi
if ! grep -qF '"flowInstanceIdList"' "${query_file}"; then
  fail "载荷出口没有写协议顶层 flowInstanceIdList"
fi
# data.flowInstanceId 会被目标忽略：出口里出现 data 用法的实例字段字面量即视为回归。
if grep -nE '"flowInstanceId"[[:space:]]*:' "${query_file}" >/dev/null 2>&1; then
  fail "载荷出口出现了 data.flowInstanceId（目标会忽略实例筛选）"
fi

# 2. 三处精确实例读取都必须复用同一出口，且自身不得拼装实例字段。
for func_name in 'ListTaskSnapshotsForUser' 'FindDueFlow' 'FindDoneTaskOnNode'; do
  file='internal/adapter/target/client.go'
  if [ "${func_name}" = 'FindDueFlow' ] || [ "${func_name}" = 'FindDoneTaskOnNode' ]; then
    file='internal/adapter/target/client_fact_reads.go'
  fi
  body=$(awk -v name="func (c *Client) ${func_name}(" '
    index($0, name) > 0 { inside = 1 }
    inside { print }
    inside && /^}/ { exit }
  ' "${file}")
  if [ -z "${body}" ]; then
    fail "找不到精确实例读取函数：${func_name}"
  fi
  # 允许两种合规形态：自己调载荷出口，或复用统一快照读取（其载荷同样出自该出口）。
  if ! printf '%s' "${body}" | grep -qE 'buildTaskSnapshotBody|ListTaskSnapshotsForUser'; then
    fail "${func_name} 没有走统一载荷出口"
  fi
  if printf '%s' "${body}" | grep -nE '"flowInstanceId"[[:space:]]*:' >/dev/null 2>&1; then
    fail "${func_name} 仍在 data 里传实例筛选"
  fi
done

# 3. 当前处理人发现不得回到配置候选人路径：函数与调用都不允许残留。
if grep -RInF 'findCandidateTaskSnapshot' internal >/dev/null 2>&1; then
  fail "候选任务发现路径仍然存在（F-031/T03 已移除）"
fi
# NextNodeAuditors 只允许出现在下一节点写载荷构造与运行上下文装配处；
# 注释里说明这条纪律不算引用（只扫代码行，不扫注释行）。
leaks=$(grep -RIlF 'NextNodeAuditors' internal \
  | grep -v 'internal/engine/step/gate.go' \
  | grep -v 'internal/engine/step/types.go' \
  | grep -v 'internal/service/run_orchestration.go' || true)
for file in ${leaks}; do
  if grep -nE '^\s*[^/].*NextNodeAuditors' "${file}" | grep -vE '^\s*[0-9]+:\s*//' | grep -q .; then
    fail "NextNodeAuditors 出现在下一节点载荷之外的模块：${file}"
  fi
done
# 当前任务解析必须先用事实里的当前处理人。
if ! grep -qF 'switchToCurrentHandler' internal/engine/step/executor.go; then
  fail "执行器不再按目标事实发现当前处理人"
fi

# 4. 运行日志目录只由 logging 包产生：业务代码不得自己拼 logs/ 路径（注释除外）。
leaks=$(grep -RInF 'logs/' internal/engine internal/service internal/adapter internal/api cmd 2>/dev/null | grep -vE ':[0-9]+:\s*//' || true)
if [ -n "${leaks}" ]; then
  fail "业务代码自己拼了日志路径：${leaks}"
fi
if ! grep -qF 'NewRouterStepLogFactory' internal/engine/step/logfactory.go; then
  fail "step.log 不再经运行日志路由打开"
fi
# 实例身份必须只补进作用域与元数据，不参与目录寻址。
if ! grep -qF 'SetInstance(' internal/engine/step/steplog.go; then
  fail "step.log 缺少实例身份补写入口"
fi

printf '%s\n' '[F-031] 结构契约检查通过'
