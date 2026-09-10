#!/usr/bin/env bash

set -euo pipefail

cd "${0%/*}/.."

printf '%s\n' '[F-026] 编译与静态检查'
go build ./...
go vet ./internal/... ./test/unit/backend/action_orchestration ./test/unit/backend/executor ./test/unit/backend/run_orchestration
if [ -n "$(gofmt -l internal/engine/scenario/scenario.go internal/model/action_orchestration.go internal/analyzer/path_config_plan.go internal/engine/step/types.go internal/engine/step/executor.go internal/engine/control/control.go internal/engine/control/runner.go internal/service/run_orchestration.go internal/service/path_action_configuration.go test/unit/backend/action_orchestration/scenario_compiler_test.go test/unit/backend/action_orchestration/action_catalog_projection_test.go test/unit/backend/action_orchestration/action_configuration_service_test.go)" ]; then
  printf '%s\n' '[F-026] 存在未格式化文件' >&2
  gofmt -l internal/engine/scenario/scenario.go internal/model/action_orchestration.go internal/analyzer/path_config_plan.go internal/engine/step/types.go internal/engine/step/executor.go internal/engine/control/control.go internal/engine/control/runner.go internal/service/run_orchestration.go internal/service/path_action_configuration.go test/unit/backend/action_orchestration/scenario_compiler_test.go test/unit/backend/action_orchestration/action_catalog_projection_test.go test/unit/backend/action_orchestration/action_configuration_service_test.go >&2
  exit 1
fi

printf '%s\n' '[F-026] 编译器、恢复链与动作组用例'
go test -count=1 -v -run 'TestF026' ./test/unit/backend/action_orchestration

printf '%s\n' '[F-026] 固定尾动作与动作组契约'
grep -qF 'ActionStepSourceSystemDefault' internal/model/action_orchestration.go
grep -qF 'ReleaseGroup' internal/model/action_orchestration.go
grep -qF 'ReleaseRequired' internal/model/action_orchestration.go
grep -qF 'item.Action == model.ActionSubmit || item.Action == model.ActionApprove' internal/analyzer/path_config_plan.go

printf '%s\n' '[F-026] 新运行按 user_actions 重新编译'
grep -qF 'compileRunSteps' internal/service/run_orchestration.go
grep -qF 'len(config.UserActions) > 0' internal/service/run_orchestration.go

printf '%s\n' '[F-026] 人工控制一次放行只执行一个动作组'
grep -qF 'continueManualGroup' internal/engine/control/control.go
grep -qF '人工控制一次放行只执行一个动作组' internal/engine/control/runner.go

printf '%s\n' '[F-026] 执行器与运行状态机回归'
go test -count=1 ./test/unit/backend/executor ./test/unit/backend/run ./test/unit/backend/run_orchestration

printf '%s\n' '[F-026] 固定尾动作与动作组界面契约'
grep -qF 'data-testid="fixed-tail-action"' web/src/features/path-configuration/ActionOrchestrationEditor.vue
grep -qF 'system_default' web/src/features/path-configuration/types.ts
grep -qF '放行边界' web/src/features/path-configuration/ActionFlowDialog.vue

printf '%s\n' '[F-026] 前端类型检查'
if [ -x web/node_modules/.bin/vue-tsc ]; then
  (cd web && ./node_modules/.bin/vue-tsc --noEmit)
else
  printf '%s\n' '[F-026] 跳过前端类型检查：web/node_modules/.bin/vue-tsc 不存在' >&2
fi

printf '%s\n' 'F-026 定向验证完成'
