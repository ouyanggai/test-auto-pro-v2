#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${project_root}"

cleanup() {
  make stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

make stop >/dev/null
make dev
make status | grep -Fq '后端：running'
make status | grep -Fq '前端：running'
./test/integration/frontend_smoke.sh

backend_before="$(tr -d '[:space:]' < .runtime/backend.pid)"
frontend_before="$(tr -d '[:space:]' < .runtime/frontend.pid)"

make restart

backend_after="$(tr -d '[:space:]' < .runtime/backend.pid)"
frontend_after="$(tr -d '[:space:]' < .runtime/frontend.pid)"
[[ "${backend_before}" != "${backend_after}" ]]
[[ "${frontend_before}" != "${frontend_after}" ]]
./test/integration/frontend_smoke.sh
logs_output="$(LOG_LINES=10 make logs)"
grep -Fq '后端服务监听 127.0.0.1:19080' <<< "${logs_output}"

make stop
make status | grep -Fq '后端：stopped'
make status | grep -Fq '前端：stopped'
trap - EXIT
