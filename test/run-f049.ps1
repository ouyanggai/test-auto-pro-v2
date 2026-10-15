# F-049 只检查工具前端外观与相关回归，不启动浏览器，不连接真实目标平台。
# 使用 UTF-8 BOM，确保 Windows PowerShell 5 正确显示中文检查名称。
$F049Root = Split-Path -Parent $PSScriptRoot
$ErrorActionPreference = 'Stop'

# Invoke-F049Check 在固定目录执行检查，任何一项失败都立即停止。
function Invoke-F049Check([string]$CheckName, [string]$CheckDirectory, [scriptblock]$CheckCommand) {
    Push-Location -LiteralPath $CheckDirectory
    try {
        Write-Host $CheckName
        & $CheckCommand
        if ($LASTEXITCODE -ne 0) { throw "$CheckName 失败，退出码 $LASTEXITCODE" }
    } finally {
        Pop-Location
    }
}

Invoke-F049Check 'Node：八种主题、偏好、对比度、弹窗、颜色与动效预算' $F049Root {
    node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON --experimental-strip-types --test --test-reporter=spec test/unit/frontend/appearance/appearance_test.mjs test/unit/frontend/appearance/studio_motion_test.mjs test/unit/frontend/appearance/skin_state_test.mjs test/unit/frontend/reports/report_chart_layout_test.mjs
}
Invoke-F049Check 'Node：画布、动作详情、运行生命周期、数据选择与报告回归' $F049Root {
    node --disable-warning=MODULE_TYPELESS_PACKAGE_JSON --experimental-strip-types --test --test-reporter=spec test/unit/frontend/flow_graph_test.mjs test/unit/frontend/action_detail_structure_test.mjs test/unit/frontend/run_detail_lifecycle/run_detail_lifecycle_test.mjs test/unit/frontend/history_replay/base_form_data_picker_test.mjs test/unit/frontend/action_orchestration/action_flow_test.mjs test/unit/frontend/report_views_structure_test.mjs test/unit/frontend/report_metrics_format_test.mjs test/unit/frontend/report_glossary_test.mjs test/unit/web/action_requests.test.ts
}
Invoke-F049Check '前端：类型检查' $F049Root { pnpm --dir web typecheck }
Invoke-F049Check '前端：正式构建' $F049Root { pnpm --dir web build }
Invoke-F049Check 'Git：差异格式' $F049Root { git diff --check }
