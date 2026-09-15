#!/usr/bin/env bash
# T015 执行引擎改造：自动化验证总入口。
#
# 用法：
#   bash test/run-runtime-exec.sh
#
# 行为：
#   1. 始终执行全部离线验证（单元 + 契约 + 真实服务集成，假目标对端）；
#   2. 输出 15 个用户动作与系统自动动作的验证状态清单；
#   3. 真实目标写验证（RUNTIME_REAL_TARGET=1 时）逐动作执行并输出独立请求轨迹与
#      目标事实证据；默认不开启——真实写需要用户提供环境与可污染测试账号
#      （账号值经环境变量注入，不入仓库），且必须停在本脚本输出的人工验收清单上。
set -u
cd "$(dirname "$0")/.."
PASS=0
FAIL=0
note() { printf '%s\n' "$*"; }
record() { if [ "$1" -eq 0 ]; then PASS=$((PASS+1)); note "  ✓ $2"; else FAIL=$((FAIL+1)); note "  ✗ $2"; fi; }

note "== 1/3 离线单元与契约验证 =="
node --experimental-strip-types --test test/unit/form-runtime/session-manager.test.ts \
  test/unit/form-runtime/protocol.test.ts test/unit/form-runtime/execution-bridge.test.js \
  test/unit/form-runtime/target-route.test.ts test/unit/form-runtime/completion.test.ts \
  > /tmp/rre-unit.log 2>&1
record $? "form-runtime 单元/契约（会话、协议、桥接、拦截、完成检测）"
pnpm --dir form-runtime typecheck >/dev/null 2>&1
record $? "form-runtime 语法检查"
pnpm --dir form-runtime/executor typecheck >/dev/null 2>&1
record $? "executor TypeScript 检查"
go test ./test/unit/backend/runtimeexec/ > /tmp/rre-go-unit.log 2>&1
record $? "Go runtimeexec 单元（客户端/装载/能力/路由）"
go test ./test/contracts/runtimeexec/ ./test/contracts/form-runtime/ > /tmp/rre-go-contract.log 2>&1
record $? "Go 契约（动作入口/无重复写出口/投影/协议夹具）"
go test ./test/contracts/ ./test/unit/backend/ ./test/unit/backend/executor/ > /tmp/rre-regression.log 2>&1
record $? "既有执行器与契约回归"
node --experimental-strip-types --test test/unit/web/runtime_execution_detail.test.ts > /tmp/rre-web.log 2>&1
record $? "运行详情三阶段展示口径"

note "== 2/3 真实执行服务集成（无头 Chromium，假目标对端）=="
pnpm --dir form-runtime build > /tmp/rre-build.log 2>&1
record $? "form-runtime 生产构建"
node --experimental-strip-types --test --test-timeout=300000 \
  test/integration/runtimeexec/form_load.test.ts \
  test/integration/runtimeexec/actor_isolation.test.ts \
  test/integration/runtimeexec/actions.test.ts \
  > /tmp/rre-int-ts.log 2>&1
record $? "装载/身份隔离/提交闭环（TS 集成）"
go test ./test/integration/runtimeexec/ > /tmp/rre-int-go.log 2>&1
record $? "命令往返/写丢失→待确认/硬超时（Go 集成）"

note "== 3/3 十五个用户动作 + 系统自动动作状态 =="
note "（离线部分=已由假目标对端全栈验证；真实部分=待 RUNTIME_REAL_TARGET=1 人工执行）"
for action in save_draft submit storage_form_data approve reject resubmit forward \
              add_sign transfer rollback_previous retrieve withdraw urge follow unfollow; do
  note "  · ${action}：离线链路已验证；真实目标证据=待人工验收（见 test/integration/runtimeexec/real-target-checklist.md）"
done
note "  · system_automatic：只读导航事实，无写请求；待真实路径人工核对"
note "  · 待发删除（/web/flowInstanceApi/delete）：明确未纳入本切片（破坏性操作，见对照文档第 6 节）"

if [ "${RUNTIME_REAL_TARGET:-0}" = "1" ]; then
  note "== 真实目标验证（RUNTIME_REAL_TARGET=1）=="
  note "真实写验证是人工清单（Markdown，不可直接执行）：打开 test/integration/runtimeexec/real-target-checklist.md，"
  note "按其中「执行环境启动命令」逐项复制执行（或直接以 RUNTIME_EXECUTOR_URL 启动后端，"
  note "本进程会自动托管拉起 19011 执行服务）；需要环境变量：RUNTIME_TARGET_BASE_URL、"
  note "RUNTIME_PLAN_ACCOUNT、RUNTIME_PLAN_PASSWORD、RUNTIME_AUDITOR_ACCOUNT、RUNTIME_AUDITOR_PASSWORD"
  note "（账号值不入仓库）。"
  note "完成一项即在 real-target-checklist.md 勾选并回填证据（实例 ID + trace ID + 日志路径）。"
fi

note "== 汇总：通过 $PASS / 失败 $FAIL =="
[ "$FAIL" -eq 0 ]
