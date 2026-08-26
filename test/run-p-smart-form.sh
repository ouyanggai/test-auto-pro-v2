#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 P1-P4 后端约束、字段契约、请求清单与运行输入预检\n'
go test -count=1 ./test/unit/backend -run '^TestP00[1-4]'
go test -count=1 ./test/contracts -run '^TestP004'

printf '验证 P2-P3 前端字段往返、iframe 回传与只读清单\n'
node --no-warnings --experimental-strip-types --test \
  test/unit/frontend/p002_vue_field_bridge_test.mjs \
  test/unit/frontend/p003_iframe_manifest_test.mjs

printf '验证 P0 黄金基线及智能表单关联主链\n'
./test/run-f011.sh

go vet ./internal/formdata ./internal/adapter/target ./internal/service ./internal/api
git diff --check
printf 'P 类智能表单方案自动验证通过\n'
