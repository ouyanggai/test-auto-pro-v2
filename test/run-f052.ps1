$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

# 只执行本次执行器回填、身份和写入门禁相关验证，不连接目标平台、不启动验收浏览器。
pnpm --dir executor-service typecheck
if ($LASTEXITCODE -ne 0) { throw '执行器类型检查失败' }
node --experimental-strip-types --test test/unit/executor-service/native_form_values.test.ts test/unit/executor-service/completion.test.ts test/unit/executor-service/target-route.test.ts test/unit/executor-service/target_route_log_identity.test.ts test/integration/executor-service/data-patch-readback.test.ts test/integration/executor-service/actions.test.ts test/integration/executor-service/choice-continue.test.ts test/integration/executor-service/actor-isolation.test.ts test/integration/executor-service/detail-load.test.ts test/integration/executor-service/initiate-template-select.test.ts
if ($LASTEXITCODE -ne 0) { throw '执行器定向回归失败' }
go test ./test/contracts/executor-service ./test/contracts/runtimeexec ./test/contracts/form-runtime -count=1
if ($LASTEXITCODE -ne 0) { throw '执行器与 Go 协议契约失败' }
pnpm --dir executor-service build-form-making
if ($LASTEXITCODE -ne 0) { throw '独立原生表单产物检查失败' }
pnpm --dir executor-service build-native-page
if ($LASTEXITCODE -ne 0) { throw '独立原生页面构建失败' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw '差异检查失败' }
