#!/usr/bin/env bash
# F-036 独立执行服务：只跑本切片相关检查。
# 用法：在项目根目录执行 bash test/run-executor-service.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# 与 Go 真实用例一致，从被 Git 忽略的本机配置补充目标账号和执行服务地址；
# 已存在的环境变量优先，配置内容不会打印到日志。
if [ -f .env.local ]; then
  set -a
  . ./.env.local
  set +a
fi

echo "== F-036 独立执行服务 typecheck =="
pnpm --dir executor-service typecheck
pnpm --dir form-runtime typecheck

# flow-source 已改为按清单 git 同步；页面产物也不再依赖 macOS 上手工产出的 dist.zip。
# 这两步都是幂等的：已同步/已构建时只做核对，不重复下载或重复编译。
echo "== F-036 flow-source 同步与页面产物前置 =="
pnpm --dir executor-service sync-source
pnpm --dir executor-service build-native-page
pnpm --dir executor-service build-form-making

echo "== F-036 执行服务单元 =="
node --experimental-strip-types --test \
  test/unit/executor-service/skeleton.test.ts \
  test/unit/executor-service/flow-source-sync.test.ts \
  test/unit/executor-service/session-manager.test.ts \
  test/unit/executor-service/protocol.test.ts \
  test/unit/executor-service/target-route.test.ts \
  test/unit/executor-service/completion.test.ts

echo "== F-036 无渲染类型特殊路由 =="
if rg -n "FormRenderTypeFormMaking" executor-service/src internal/engine/step; then
  echo "检测到按 FormMaking 渲染类型分流 runtime 的残留" >&2
  exit 1
fi
if rg -n "businessDone|countRelevantPageErrors" executor-service/src; then
  echo "检测到页面信号参与完成判定的残留" >&2
  exit 1
fi

echo "== F-036 执行服务集成 =="
node --experimental-strip-types --test \
  test/integration/executor-service/forward-before-load.test.ts \
  test/integration/executor-service/initiate-template-select.test.ts \
  test/integration/executor-service/detail-load.test.ts \
  test/integration/executor-service/actor-isolation.test.ts \
  test/integration/executor-service/data-patch-readback.test.ts \
  test/integration/executor-service/actions.test.ts \
  test/integration/executor-service/choice-continue.test.ts \
  test/integration/executor-service/isolation.test.ts \
  test/integration/executor-service/session-lifecycle.test.ts

echo "== F-036 Go 契约与路由 =="
go test ./test/contracts/executor-service ./test/contracts/runtimeexec ./test/contracts/form-runtime \
  ./test/unit/backend/runtimeexec -count=1

# T4.1 门禁真实化：门禁必须真实跑通，不允许「未配置就静默跳过」冒充通过。
# REQUIRE_REAL=1 会把 L1/L2 里所有「环境未配置」的跳过路径升级为硬失败；
# 因此本脚本必须先把环境准备好，否则会（正确地）失败并打印缺什么。
echo "== F-036 真实闭环验证台（L1/L2，强制真实）=="
export TEST_AUTO_PRO_F036_REQUIRE_REAL=1
if [ -z "${TEST_AUTO_PRO_TARGET_ACCOUNT:-}" ]; then
  echo "门禁失败：未配置 TEST_AUTO_PRO_TARGET_ACCOUNT，L1/L2 无法真实执行。" >&2
  echo "请在 .env.local 配置目标账号后重跑（.env.local 会被用例自动加载）。" >&2
  exit 1
fi
if [ -z "${RUNTIME_EXECUTOR_URL:-}" ]; then
  echo "门禁失败：未配置 RUNTIME_EXECUTOR_URL，执行服务地址未知。" >&2
  exit 1
fi
go test ./test/integration/runtimeexec -run 'TestF036' -count=1 -v

# T4.2 真实动作台：由 TEST_AUTO_PRO_F036_ACTIONS 驱动逐个真实执行七个动作。
# 每个动作都实际调用 Go 真实闭环用例；动作台只登记已完成的动作，不能用键名覆盖
# 代替真实执行。任一动作失败立即收口，避免后续动作继续写目标数据。
echo "== F-036 真实动作台（G1）=="
F036_REQUIRED_ACTIONS="save_draft submit storage_form_data approve reject resubmit forward"
if [ -z "${TEST_AUTO_PRO_F036_ACTIONS:-}" ]; then
  echo "门禁失败：未配置 TEST_AUTO_PRO_F036_ACTIONS，本轮未验证任何真实写。" >&2
  echo "未覆盖动作清单：${F036_REQUIRED_ACTIONS// /, }" >&2
  exit 1
fi
IFS=', ' read -r -a F036_ACTION_LIST <<< "${TEST_AUTO_PRO_F036_ACTIONS}"
F036_COMPLETED_ACTIONS=""
for action in "${F036_ACTION_LIST[@]}"; do
  [ -z "${action}" ] && continue
  echo "-- 真实执行动作：${action} --"
  TEST_AUTO_PRO_F036_ACTION="${action}" go test ./test/integration/runtimeexec -run '^TestF036RealActionLoop$' -count=1 -v
  if [ -n "${F036_COMPLETED_ACTIONS}" ]; then F036_COMPLETED_ACTIONS="${F036_COMPLETED_ACTIONS},${action}"; else F036_COMPLETED_ACTIONS="${action}"; fi
done
node test/integration/executor-service/f036-real-action-bench.mjs \
  --actions "${F036_COMPLETED_ACTIONS}" \
  --required "${F036_REQUIRED_ACTIONS}"

echo "== F-036 全部通过 =="
