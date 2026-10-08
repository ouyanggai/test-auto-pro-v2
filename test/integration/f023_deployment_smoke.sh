#!/usr/bin/env bash

# 已部署环境的只读验活：不创建计划、不调用目标写接口、不停止生产服务。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${project_root}"
env_file="${F023_ENV_FILE:-deploy/.env}"
compose=(docker compose --env-file "${env_file}" -f deploy/docker-compose.yml)

for service in mysql backend web form-runtime executor code-server; do
  container="$("${compose[@]}" ps -q "${service}")"
  [ -n "${container}" ] || { echo "服务未启动：${service}" >&2; exit 1; }
  state="$(docker inspect --format '{{.State.Health.Status}}' "${container}")"
  [ "${state}" = healthy ] || { echo "服务未健康：${service} ${state}" >&2; exit 1; }
done

curl -fsS "http://127.0.0.1:${WEB_PORT:-19000}/" >/dev/null
curl -fsS "http://127.0.0.1:${WEB_PORT:-19000}/api/health"
curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19080}/api/health"
curl -fsS "http://127.0.0.1:${FORM_RUNTIME_PORT:-19001}/form-runtime/" >/dev/null
curl -fsS "http://127.0.0.1:${WEB_PORT:-19000}/form-runtime/runtime-health.json"
curl -fsS "http://127.0.0.1:${CODE_SERVER_PORT:-19002}/" >/dev/null
"${compose[@]}" exec -T executor node --experimental-strip-types src/probe.ts
"${compose[@]}" exec -T backend node -e "fetch('http://executor:19011/health').then(async r=>{if(!r.ok)process.exit(1);console.log(await r.text())}).catch(()=>process.exit(1))"

# 实际双向探针证明挂载可见且可写；只清理本次唯一探针文件。
probe=".deployment-probe-$$"
trap 'rm -f "logs/${probe}"' EXIT
printf 'host-ok\n' >"logs/${probe}"
"${compose[@]}" exec -T backend sh -c "grep -q host-ok '/app/logs/${probe}'"
"${compose[@]}" exec -T code-server sh -c "grep -q host-ok '/home/coder/logs/${probe}' && printf 'container-ok\\n' >>'/home/coder/logs/${probe}'"
grep -q container-ok "logs/${probe}"
echo '六服务健康、HTTP 入口、Chromium 启动和日志双向挂载验证通过'
