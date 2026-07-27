#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
runtime_dir="${project_root}/.runtime"
backend_pid_file="${runtime_dir}/backend.pid"
frontend_pid_file="${runtime_dir}/frontend.pid"
backend_log="${runtime_dir}/backend.log"
frontend_log="${runtime_dir}/frontend.log"
backend_binary="${runtime_dir}/test-auto-pro-server"
health_body='{"status":"ok","service":"test-auto-pro","version":"dev"}'

mkdir -p "${runtime_dir}"

read_pid() {
  local pid_file="$1"
  local pid

  [[ -f "${pid_file}" ]] || return 1
  pid="$(tr -d '[:space:]' < "${pid_file}")"
  [[ "${pid}" =~ ^[0-9]+$ ]] || return 1
  printf '%s\n' "${pid}"
}

process_command() {
  ps -p "$1" -o command= 2>/dev/null || true
}

is_expected_process() {
  local service="$1"
  local pid="$2"
  local command

  kill -0 "${pid}" 2>/dev/null || return 1
  command="$(process_command "${pid}")"
  case "${service}" in
    backend)
      [[ "${command}" == *"${backend_binary}"* ]]
      ;;
    frontend)
      [[ "${command}" == *"${project_root}/web/node_modules/"* && "${command}" == *"vite"* ]]
      ;;
    *)
      return 1
      ;;
  esac
}

service_state() {
  local service="$1"
  local pid_file="$2"
  local pid

  if ! pid="$(read_pid "${pid_file}")"; then
    printf 'stopped\n'
  elif ! kill -0 "${pid}" 2>/dev/null; then
    printf 'stale\n'
  elif is_expected_process "${service}" "${pid}"; then
    printf 'running\n'
  else
    printf 'mismatch\n'
  fi
}

stop_service() {
  local service="$1"
  local pid_file="$2"
  local pid
  local attempt

  if ! pid="$(read_pid "${pid_file}")"; then
    rm -f "${pid_file}"
    return 0
  fi

  if ! kill -0 "${pid}" 2>/dev/null; then
    rm -f "${pid_file}"
    return 0
  fi

  if ! is_expected_process "${service}" "${pid}"; then
    printf '拒绝终止未验证属于本项目的 PID：%s (%s)\n' "${pid}" "${service}" >&2
    return 1
  fi

  kill "${pid}"
  for attempt in {1..50}; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      rm -f "${pid_file}"
      return 0
    fi
    sleep 0.1
  done

  printf '进程未在 5 秒内退出：%s (%s)\n' "${pid}" "${service}" >&2
  return 1
}

stop_all() {
  local result=0

  stop_service frontend "${frontend_pid_file}" || result=1
  stop_service backend "${backend_pid_file}" || result=1
  [[ "${result}" -eq 0 ]] || return 1
  printf '前后端已停止\n'
}

start_backend() {
  go build -o "${backend_binary}" ./cmd/server
  nohup "${backend_binary}" >> "${backend_log}" 2>&1 &
  printf '%s\n' "$!" > "${backend_pid_file}"
}

start_frontend() {
  (
    cd "${project_root}/web"
    nohup "${project_root}/web/node_modules/.bin/vite" --host 127.0.0.1 --port 19000 --strictPort >> "${frontend_log}" 2>&1 &
    printf '%s\n' "$!" > "${frontend_pid_file}"
  )
}

wait_until_ready() {
  local attempt
  local backend_body
  local frontend_page
  local proxy_body

  for attempt in {1..80}; do
    backend_body="$(curl --silent --max-time 1 http://127.0.0.1:19080/api/health || true)"
    frontend_page="$(curl --silent --max-time 1 http://127.0.0.1:19000/ || true)"
    proxy_body="$(curl --silent --max-time 1 http://127.0.0.1:19000/api/health || true)"

    if [[ "${backend_body}" == "${health_body}" && "${proxy_body}" == "${health_body}" && "${frontend_page}" == *'<title>流程自动化测试平台</title>'* ]]; then
      return 0
    fi

    if [[ "$(service_state backend "${backend_pid_file}")" != 'running' || "$(service_state frontend "${frontend_pid_file}")" != 'running' ]]; then
      break
    fi
    sleep 0.25
  done

  printf '服务未能就绪，请执行 make logs 查看原因\n' >&2
  return 1
}

start_all() {
  local backend_state
  local frontend_state

  backend_state="$(service_state backend "${backend_pid_file}")"
  frontend_state="$(service_state frontend "${frontend_pid_file}")"

  if [[ "${backend_state}" == 'mismatch' || "${frontend_state}" == 'mismatch' ]]; then
    printf 'PID 文件指向非本项目进程，拒绝启动；请人工核对 .runtime/*.pid\n' >&2
    return 1
  fi

  if [[ "${backend_state}" == 'running' && "${frontend_state}" == 'running' ]]; then
    wait_until_ready
    printf '服务已在运行：前端 http://127.0.0.1:19000，后端 http://127.0.0.1:19080\n'
    return 0
  fi

  if [[ "${backend_state}" == 'running' || "${frontend_state}" == 'running' ]]; then
    stop_all
  else
    rm -f "${backend_pid_file}" "${frontend_pid_file}"
  fi

  : > "${backend_log}"
  : > "${frontend_log}"
  start_backend
  start_frontend

  if ! wait_until_ready; then
    stop_all || true
    return 1
  fi

  printf '服务已就绪：前端 http://127.0.0.1:19000，后端 http://127.0.0.1:19080\n'
}

show_status() {
  local backend_state
  local frontend_state

  backend_state="$(service_state backend "${backend_pid_file}")"
  frontend_state="$(service_state frontend "${frontend_pid_file}")"
  printf '后端：%s\n前端：%s\n' "${backend_state}" "${frontend_state}"

  [[ "${backend_state}" != 'mismatch' && "${frontend_state}" != 'mismatch' ]]
}

show_logs() {
  local lines="${LOG_LINES:-80}"

  [[ "${lines}" =~ ^[1-9][0-9]*$ ]] || {
    printf 'LOG_LINES 必须是正整数\n' >&2
    return 1
  }

  printf '== 后端日志 ==\n'
  if [[ -f "${backend_log}" ]]; then
    tail -n "${lines}" "${backend_log}"
  else
    printf '暂无后端日志\n'
  fi

  printf '\n== 前端日志 ==\n'
  if [[ -f "${frontend_log}" ]]; then
    tail -n "${lines}" "${frontend_log}"
  else
    printf '暂无前端日志\n'
  fi
}

main() {
  cd "${project_root}"
  case "${1:-}" in
    dev) start_all ;;
    restart)
      stop_all
      start_all
      ;;
    stop) stop_all ;;
    status) show_status ;;
    logs) show_logs ;;
    *)
      printf '用法：runtime.sh dev|restart|stop|status|logs\n' >&2
      exit 1
      ;;
  esac
}

main "$@"
