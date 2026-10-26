# F-050 只验证快速同意准备、配置状态和运行入口，不启动浏览器，不向目标平台提交审批。
$F050Root = Split-Path -Parent $PSScriptRoot
$ErrorActionPreference = 'Stop'

# Invoke-F050Check 在项目根目录执行当前范围检查，失败立即中止，不能把后续成功掩盖前面的失败。
function Invoke-F050Check([string]$CheckName, [scriptblock]$CheckCommand) {
    Push-Location -LiteralPath $F050Root
    try {
        Write-Host $CheckName
        & $CheckCommand
        if ($LASTEXITCODE -ne 0) { throw "$CheckName 失败，退出码 $LASTEXITCODE" }
    } finally { Pop-Location }
}

Invoke-F050Check 'Go：默认场景、人员、保存、回放、运行前检查与执行回归' {
    go test ./test/unit/backend/action_orchestration ./test/unit/backend/run_readiness ./test/unit/backend/run_orchestration ./test/unit/backend/history_replay ./test/unit/backend/run -count=1
}
Invoke-F050Check 'Go：提交、同意、人员门禁与默认场景执行回归' {
    go test ./test/unit/backend/executor -run '(Approve|Submit|Consent|Gate|Default|Scenario)' -count=1
}
Invoke-F050Check 'MySQL：临时库准备、列表、重连和运行前检查一致性' {
    if (-not $env:TEST_AUTO_PRO_PLAN_DB_ENV_FILE) { $env:TEST_AUTO_PRO_PLAN_DB_ENV_FILE = Join-Path $F050Root '.env.local' }
    go test ./test/integration -run TestF050 -count=1 -v
}
Invoke-F050Check 'Node：实际页面事件、准备结果、选择范围与现有配置交互' {
    node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON --experimental-strip-types --test --test-reporter=spec test/unit/frontend/history_replay/quick_approve_test.mjs test/unit/frontend/execution_path_test.mjs test/unit/frontend/history_replay/history_replay_task_test.mjs test/unit/frontend/history_replay/base_form_data_picker_test.mjs test/unit/frontend/action_orchestration/action_configuration_test.mjs test/unit/frontend/action_orchestration/action_flow_test.mjs test/unit/frontend/path_configuration/save_success_feedback_test.mjs test/unit/frontend/appearance/appearance_test.mjs
}
Invoke-F050Check 'Go：正式构建' { go build ./... }
Invoke-F050Check 'Go：受影响代码静态检查' { go vet ./internal/service ./internal/engine/scenario ./test/unit/backend/action_orchestration }
Invoke-F050Check '前端：类型检查' { node web/node_modules/vue-tsc/bin/vue-tsc.js --noEmit -p web/tsconfig.json }
Invoke-F050Check '前端：正式构建' { node web/node_modules/vite/bin/vite.js build web }
Invoke-F050Check 'Git：差异格式' { git diff --check }
