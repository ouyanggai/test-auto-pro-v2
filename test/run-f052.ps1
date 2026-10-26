$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

# 只执行本次执行器回填、身份和写入门禁相关验证，不连接目标平台、不启动验收浏览器。
pnpm --dir executor-service typecheck
if ($LASTEXITCODE -ne 0) { throw '执行器类型检查失败' }
node --experimental-strip-types --test test/unit/executor-service/native_form_values.test.ts test/unit/executor-service/completion.test.ts test/unit/executor-service/target-route.test.ts test/unit/executor-service/target_route_log_identity.test.ts test/integration/executor-service/data-patch-readback.test.ts test/integration/executor-service/actions.test.ts test/integration/executor-service/choice-continue.test.ts test/integration/executor-service/actor-isolation.test.ts test/integration/executor-service/detail-load.test.ts test/integration/executor-service/initiate-template-select.test.ts
if ($LASTEXITCODE -ne 0) { throw '执行器定向回归失败' }
go test ./test/contracts/executor-service ./test/contracts/runtimeexec ./test/contracts/form-runtime -count=1
if ($LASTEXITCODE -ne 0) { throw '执行器与 Go 协议契约失败' }
go test ./test/unit/backend/run_orchestration -count=1
if ($LASTEXITCODE -ne 0) { throw '步骤来源与运行展示回归失败' }
# 仅连接临时测试数据库；不运行含真实目标操作的其他集成用例。
$env:TEST_AUTO_PRO_PLAN_DB_ENV_FILE = Join-Path (Get-Location) '.env.local'
go test ./test/integration -run '^TestF052LongErrorFinishesRun$' -count=1
if ($LASTEXITCODE -ne 0) { throw '长错误事务集成验证失败' }
node --experimental-strip-types --test test/unit/frontend/action_outcome_test.mjs test/unit/frontend/action_detail_structure_test.mjs test/unit/web/action_requests.test.ts
if ($LASTEXITCODE -ne 0) { throw '动作详情展示回归失败' }
pnpm --dir web typecheck
if ($LASTEXITCODE -ne 0) { throw '前端类型检查失败' }
pnpm --dir web build
if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
pnpm --dir executor-service build-form-making
if ($LASTEXITCODE -ne 0) { throw '独立原生表单产物检查失败' }
pnpm --dir executor-service build-native-page
if ($LASTEXITCODE -ne 0) { throw '独立原生页面构建失败' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw '差异检查失败' }
