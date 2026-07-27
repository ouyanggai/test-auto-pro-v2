#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 Go 单元与健康契约\n'
pnpm check:backend

printf '验证前端结构、类型与构建\n'
./test/unit/frontend/frontend_structure.sh
pnpm check:frontend

printf '验证项目文件契约\n'
./test/contracts/project_structure.sh

printf '验证参考仓库状态与脏目录保护\n'
make refs-status
./test/integration/refs_dirty_guard.sh

printf '验证前台开发命令、健康接口与热重载\n'
./test/integration/foreground_dev_commands.sh

printf 'F-000 自动验证全部通过\n'
