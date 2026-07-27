#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${project_root}"

make stop >/dev/null
mkdir -p .runtime

sleep 30 &
unrelated_pid="$!"

cleanup() {
  rm -f .runtime/backend.pid
  kill "${unrelated_pid}" >/dev/null 2>&1 || true
  wait "${unrelated_pid}" 2>/dev/null || true
}
trap cleanup EXIT

printf '%s\n' "${unrelated_pid}" > .runtime/backend.pid
if make restart >/dev/null 2>&1; then
  printf 'restart 不应接受非项目 PID\n' >&2
  exit 1
fi

kill -0 "${unrelated_pid}"
[[ ! -f .runtime/frontend.pid ]]
