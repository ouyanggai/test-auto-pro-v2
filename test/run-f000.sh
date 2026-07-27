#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

mkdir -p .runtime/test-build

printf '验证 Go 单元与健康契约\n'
go test ./test/unit/backend ./test/contracts
go build -o .runtime/test-build/server ./cmd/server

printf '验证前端结构、类型与构建\n'
./test/unit/frontend/frontend_structure.sh
pnpm --dir web typecheck
pnpm --dir web build

printf '验证项目文件契约\n'
./test/contracts/project_structure.sh

printf '验证参考仓库状态与脏目录保护\n'
make refs-status
./test/integration/refs_dirty_guard.sh

printf '验证 PID 归属保护与运行生命周期\n'
./test/integration/runtime_pid_guard.sh
./test/integration/runtime_lifecycle.sh

printf 'F-000 自动验证全部通过\n'
