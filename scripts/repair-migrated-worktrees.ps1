# repair-migrated-worktrees.ps1 —— 修复 macOS→Windows 迁移后失效的 worktree 绝对路径
#
# 背景：从 Mac 整体复制过来的仓库里，worktree 的 .git 是指向
# `/Volumes/oygsky/bigsys/<主仓库>/.git/worktrees/<名字>` 的 gitdir 指针文件，
# 而 worktree 的管理目录里 gitdir/commondir 又写回了 `/Volumes/oygsky/...`。
# Windows 上没有 /Volumes，git 对这类仓库直接报 `not a git repository: (NULL)`。
#
# 本脚本只做两件事：
#   1) 把每个 worktree 的 `.git` 指针和 `.git/worktrees/<名字>/{gitdir,commondir}` 改写成真实 Windows 路径；
#   2) 主仓库 config 里 `core.worktree` / `core.bare` 残留一并清掉。
# 不删除仓库、不 reset、不动工作树文件；改写前把原文件备份成 `<文件>.mac-backup`。
# 幂等：路径已经是正确 Windows 路径时直接跳过。

[CmdletBinding()]
param(
    # 要修复的主仓库路径（其 .git/worktrees 下登记的 worktree 会被逐个检查）
    [Parameter(Mandatory = $true)]
    [string[]] $RepoPath,
    # 非兄弟目录布局的 worktree：形如 "主仓库路径=worktree 实际路径"。
    # 默认按「同盘同名的兄弟目录」推断；当 worktree 被复制到别处（例如旧项目内嵌的副本）时用它显式指定。
    [string[]] $ExtraWorktree = @(),
    # 只报告不改写
    [switch] $DryRun
)

$ErrorActionPreference = 'Stop'

# toWindowsGitPath 把路径规范成 git 在 Windows 上使用的正斜杠形式。
function toWindowsGitPath ([string] $path) {
    return ([System.IO.Path]::GetFullPath($path) -replace '\\', '/')
}

# repairOne 修复单个 worktree（或主仓库）的 gitdir 指向。
function repairOne ([string] $worktreePath, [string] $expectedAdminDir, [string] $expectedCommonDir) {
    $dotGit = Join-Path $worktreePath '.git'
    $pointerTarget = $null

    if (Test-Path -LiteralPath $dotGit -PathType Container) {
        # 主仓库或已经是标准仓库：只检查 core.worktree/core.bare 残留。
        return
    }
    if (-not (Test-Path -LiteralPath $dotGit -PathType Leaf)) {
        Write-Host "  跳过（没有 .git）：$worktreePath"
        return
    }

    $content = (Get-Content -LiteralPath $dotGit -Raw).Trim()
    if ($content -notmatch '^gitdir:\s*(.+)$') {
        throw "无法识别的 .git 指针文件：$dotGit"
    }
    $current = $matches[1].Trim()
    $expected = "$(toWindowsGitPath $expectedAdminDir)"
    $pointerTarget = $expected

    if ($current -eq $expected) {
        Write-Host "  已是正确路径：$worktreePath"
    } else {
        Write-Host "  改写 .git 指针：$current -> $expected"
        if (-not $DryRun) {
            if (-not (Test-Path -LiteralPath "$dotGit.mac-backup")) {
                Copy-Item -LiteralPath $dotGit -Destination "$dotGit.mac-backup"
            }
            Set-Content -LiteralPath $dotGit -Value "gitdir: $expected" -NoNewline -Encoding ascii
        }
    }

    # 管理目录内的 gitdir / commondir 也要指向真实路径，否则 git 仍会去找 /Volumes。
    $adminDir = $expectedAdminDir
    foreach ($pair in @(
            @{ Name = 'gitdir'; Value = "$(toWindowsGitPath $worktreePath)/.git" },
            @{ Name = 'commondir'; Value = '../..' }
        )) {
        $file = Join-Path $adminDir $pair.Name
        if (-not (Test-Path -LiteralPath $file)) { continue }
        $value = (Get-Content -LiteralPath $file -Raw).Trim()
        if ($value -eq $pair.Value) { continue }
        Write-Host "  改写 $($pair.Name)：$value -> $($pair.Value)"
        if (-not $DryRun) {
            if (-not (Test-Path -LiteralPath "$file.mac-backup")) {
                Copy-Item -LiteralPath $file -Destination "$file.mac-backup"
            }
            Set-Content -LiteralPath $file -Value $pair.Value -NoNewline -Encoding ascii
        }
    }
}

foreach ($repo in $RepoPath) {
    $repo = (Resolve-Path -LiteralPath $repo).Path
    Write-Host "主仓库：$repo"

    $commonDir = Join-Path $repo '.git'
    $adminRoot = Join-Path $commonDir 'worktrees'
    if (-not (Test-Path -LiteralPath $adminRoot)) {
        Write-Host '  没有 worktrees 管理目录，跳过'
        continue
    }

    # 非兄弟布局的 worktree 由 -ExtraWorktree 按「主仓库=<名称>=<实际路径>」显式指定。
    # 必须按名称精确绑定：只用主仓库前缀匹配会让同一主仓库的其它 worktree 也命中同一个覆盖值。
    $pinned = @{}
    foreach ($item in $ExtraWorktree) {
        $parts = $item -split '=', 3
        if ($parts.Count -ne 3) { throw "ExtraWorktree 需要 <主仓库>=<名称>=<实际路径>：$item" }
        $pinned["$($parts[0])|$($parts[1])"] = $parts[2]
    }

    foreach ($admin in Get-ChildItem -LiteralPath $adminRoot -Directory) {
        # worktree 的 gitdir 文件记录工作树根目录；迁移后它指向失效的 Mac 路径，无法用它反查，
        # 因此以「同盘同名的兄弟目录」为约定：主仓库 <root>/<name> 对应 worktree <root>/<name>。
        # 布局不成立时由 -ExtraWorktree 显式给出实际路径。
        $siblingRoot = Split-Path -Parent $repo
        $worktreePath = Join-Path $siblingRoot $admin.Name
        $override = $pinned["$repo|$($admin.Name)"]
        if ($override) { $worktreePath = $override }
        if (-not (Test-Path -LiteralPath $worktreePath)) {
            Write-Host "  跳过（找不到工作树目录）：$worktreePath"
            continue
        }
        Write-Host "  worktree：$worktreePath"
        repairOne -worktreePath $worktreePath -expectedAdminDir $admin.FullName -expectedCommonDir $commonDir
    }

    # 主仓库自身的 core.worktree/core.bare 残留清理。
    $configPath = Join-Path $commonDir 'config'
    $config = Get-Content -LiteralPath $configPath -Raw
    $cleaned = $config -replace '(?m)^\s*(worktree|bare)\s*=.*\r?\n', ''
    if ($cleaned -ne $config) {
        Write-Host '  清理主仓库 config 里的 core.worktree/core.bare 残留'
        if (-not $DryRun) {
            if (-not (Test-Path -LiteralPath "$configPath.mac-backup")) {
                Copy-Item -LiteralPath $configPath -Destination "$configPath.mac-backup"
            }
            Set-Content -LiteralPath $configPath -Value $cleaned -NoNewline -Encoding utf8
        }
    }
}
