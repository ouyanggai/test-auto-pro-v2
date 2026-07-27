#!/usr/bin/env bash

set -euo pipefail
set -m

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
runtime_dir="${project_root}/.runtime/foreground-dev-test"
backend_log="${runtime_dir}/backend.log"
frontend_log="${runtime_dir}/frontend.log"
health_source="${project_root}/internal/api/health.go"
health_source_backup="${runtime_dir}/health.go.original"
expected_health='{"status":"ok","service":"test-auto-pro","version":"dev"}'
backend_pid=''
frontend_pid=''

mkdir -p "${runtime_dir}"

stop_job() {
  local command_pid="$1"

  [[ -n "${command_pid}" ]] || return 0
  kill -0 "${command_pid}" 2>/dev/null || return 0
  kill -INT -- "-${command_pid}" 2>/dev/null || kill -INT "${command_pid}" 2>/dev/null || true
  wait "${command_pid}" 2>/dev/null || true
}

cleanup() {
  if [[ -f "${health_source_backup}" ]]; then
    cp "${health_source_backup}" "${health_source}"
  fi
  stop_job "${frontend_pid}"
  stop_job "${backend_pid}"
  for attempt in {1..40}; do
    if ! lsof -nP -iTCP:19080 -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  printf '后端热更新进程未停止\n' >&2
  return 1
}
trap cleanup EXIT

wait_for_health() {
  local attempt

  for attempt in {1..80}; do
    if [[ "$(curl --silent --max-time 1 http://127.0.0.1:19080/api/health || true)" == "${expected_health}" ]]; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

wait_for_frontend() {
  local attempt
  local frontend_page
  local proxy_body

  for attempt in {1..80}; do
    frontend_page="$(curl --silent --max-time 1 http://127.0.0.1:19000/ || true)"
    proxy_body="$(curl --silent --max-time 1 http://127.0.0.1:19000/api/health || true)"
    if [[ "${frontend_page}" == *'<title>流程自动化测试平台</title>'* && "${proxy_body}" == "${expected_health}" ]]; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

cd "${project_root}"
cp "${health_source}" "${health_source_backup}"
pnpm dev:backend > "${backend_log}" 2>&1 &
backend_pid="$!"
wait_for_health || {
  cat "${backend_log}" >&2
  exit 1
}

perl -0pi -e 's/热更新探针=初始/热更新探针=已触发/' "${health_source}"
grep -Fq '热更新探针=已触发' "${health_source}"
for attempt in {1..80}; do
  if grep -Fq 'internal/api/health.go has changed' "${backend_log}" && [[ "$(grep -Fc 'running...' "${backend_log}")" -ge 2 ]]; then
    break
  fi
  sleep 0.25
done
grep -Fq 'internal/api/health.go has changed' "${backend_log}"
[[ "$(grep -Fc 'running...' "${backend_log}")" -ge 2 ]]
wait_for_health

pnpm dev:frontend > "${frontend_log}" 2>&1 &
frontend_pid="$!"
wait_for_frontend || {
  cat "${frontend_log}" >&2
  exit 1
}

[[ "$(curl --silent --fail http://127.0.0.1:19000/api/health)" == "${expected_health}" ]]
for route_name in plans runs settings; do
  curl --silent --fail "http://127.0.0.1:19000/${route_name}" | grep -Fq '<title>流程自动化测试平台</title>'
done
