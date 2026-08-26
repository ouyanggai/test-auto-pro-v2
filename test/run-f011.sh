#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '验证 F-011 字段、约束与冲突黄金基线\n'
go test -count=1 ./test/unit/backend -run '^TestF011'

printf '验证 F-011 iframe 消息、生命周期与请求影子基线\n'
node --no-warnings --experimental-strip-types --test \
  test/contracts/f011_iframe_protocol_test.mjs \
  test/unit/frontend/form_runtime_test.mjs \
  test/unit/frontend/path_configuration_lifecycle_test.mjs

printf '验证 F-009 与 F-010 相关回归\n'
./test/run-f009.sh
./test/run-f010.sh

git diff --check
printf 'F-011 契约基线自动验证通过\n'
