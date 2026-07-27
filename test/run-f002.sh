#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-002 后端配置、会话与 DTO 单元测试\n'
go test -count=1 ./test/unit/backend -run '^(TestTargetConfig|TestSessionManager|TestPasswordEncryption)'

printf '验证 F-002 三个公开 API 与安全错误契约\n'
go test -count=1 ./test/contracts -run '^(TestTargetAPI|TestMissingTargetConfig)'

printf '验证 F-002 假目标登录、三类读取、失效重登、异常、超时与取消\n'
go test -count=1 ./test/integration -run '^(TestRealRead|TestSessionExpiry|TestLoginRejection|TestEmptyBad|TestTargetTimeout)'

printf '验证 F-002 前端防抖、版本保护、取消、分页去重与错误恢复\n'
node --no-warnings --experimental-strip-types --test test/unit/frontend/remote_candidates_test.mjs

printf '验证 F-002 前端真实数据源与稳定候选区结构\n'
./test/integration/f002_frontend_structure.sh

printf '验证 Go 构建、前端类型与生产构建\n'
go build ./cmd/server
pnpm --filter test-auto-pro-v2-web typecheck
pnpm --filter test-auto-pro-v2-web build

printf 'F-002 自动验证全部通过\n'
