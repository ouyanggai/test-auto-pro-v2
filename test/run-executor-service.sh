#!/usr/bin/env bash
# F-036 独立执行服务：只跑本切片相关检查。
# 用法：在项目根目录执行 bash test/run-executor-service.sh
set -euo pipefail
cd "$(dirname "$0")/.."

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

echo "== F-036 真实闭环验证台（L1/L2）=="
echo "说明：未配置 TEST_AUTO_PRO_TARGET_ACCOUNT / RUNTIME_EXECUTOR_URL 时用例会打印跳过原因后跳过，不允许静默通过。"
go test ./test/integration/runtimeexec -run 'TestF036' -count=1 -v

echo "== F-036 全部通过 =="
