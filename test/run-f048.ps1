# F-048 只运行日志归属、动作详情与事件总览的本地验证，不启动浏览器或连接真实目标平台。
# 文件使用 UTF-8 BOM，让 Windows PowerShell 5 正确读取中文检查名称。
$F048Root = Split-Path -Parent $PSScriptRoot

# Invoke-F048Check 在固定工作目录实际执行检查，原生命令失败时立即停止。
function Invoke-F048Check([string]$CheckName, [string]$CheckDirectory, [scriptblock]$CheckCommand) {
    Push-Location -LiteralPath $CheckDirectory
    try {
        Write-Host $CheckName
        & $CheckCommand
        if ($LASTEXITCODE -ne 0) { throw "$CheckName 失败，退出码 $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
}

Invoke-F048Check 'Go：运行转发、真实日志归属、事件与详情契约' $F048Root {
    go test ./test/unit/backend/runtimeexec ./test/unit/backend/run_request_timings ./test/unit/backend/run_events ./test/contracts/runtimeexec
}
Invoke-F048Check 'Go：传输正文计时、页面请求转发与目标原话' $F048Root {
    go test ./test/unit/backend/target -run 'TestForwardRuntimeRequest|TestForwardTimingIncludesResponseBody|TestInspectEnvelope|TestTargetMessage'
}
Invoke-F048Check 'Go：现有执行结果、转发与运行详情回归' $F048Root {
    go test ./test/unit/backend/executor ./test/unit/backend/run_orchestration -run 'TestF016PhaseTimings|TestF016RunDetail|Test.*Runtime|Test.*Forward|Test.*No.*Write|Test.*Step.*Skip|Test.*Target.*Skip'
}
Invoke-F048Check 'Node：动作接口合并、空状态、页面结构、生命周期与执行器防重发' $F048Root {
    node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON --experimental-strip-types --test --test-reporter=spec test/unit/web/action_requests.test.ts test/unit/web/runtime_execution_detail.test.ts test/unit/frontend/action_detail_structure_test.mjs test/unit/frontend/run_detail_lifecycle/run_detail_lifecycle_test.mjs test/unit/executor-service/completion.test.ts test/unit/executor-service/target_route_log_identity.test.ts
}
Invoke-F048Check '执行服务：TypeScript 类型检查' (Join-Path $F048Root 'executor-service') { npm run typecheck }
Invoke-F048Check '前端：类型检查与正式构建' (Join-Path $F048Root 'web') { npm run build }
Invoke-F048Check 'Go：服务构建' $F048Root { go build ./cmd/server }
Invoke-F048Check 'Git：差异格式检查' $F048Root { git diff --check }
