#!/usr/bin/env bash
# F-036 独立执行服务：只跑本切片相关检查。
# 用法：在项目根目录执行 bash test/run-executor-service.sh
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== F-036 独立执行服务 typecheck =="
pnpm --dir executor-service typecheck
pnpm --dir form-runtime typecheck

echo "== F-036 执行服务单元 =="
node --experimental-strip-types --test \
  test/unit/executor-service/skeleton.test.ts \
  test/unit/executor-service/flow-source-sync.test.ts \
  test/unit/executor-service/session-manager.test.ts \
  test/unit/executor-service/protocol.test.ts \
  test/unit/executor-service/target-route.test.ts \
  test/unit/executor-service/completion.test.ts

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
  ./test/unit/backend/runtimeexec ./test/unit/backend -count=1

echo "== F-036 全部通过 =="
