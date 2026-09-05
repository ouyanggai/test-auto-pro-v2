#!/usr/bin/env bash

# F-023 编排结构契约：五个服务、端口约定、健康检查、卷边界与配置注入口径。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-023] $1" >&2
  exit 1
}

compose='deploy/docker-compose.yml'
[ -f "${compose}" ] || fail "缺少 Compose 文件：${compose}"

# 五个服务齐备。
for service in mysql backend web form-runtime code-server; do
  grep -qE "^  ${service}:" "${compose}" || fail "缺少服务：${service}"
done

# 端口约定与本机一致：19000/19080/19001/19002。
for port in 19000 19080 19001 19002; do
  grep -qF "${port}" "${compose}" || fail "端口约定缺失：${port}"
done

# 后端等待 MySQL 健康；MySQL 不发布宿主端口（仅 Compose 网络可达）。
grep -qF 'service_healthy' "${compose}" || fail '缺少依赖健康条件'
if grep -E '^\s+- "?[0-9]+:3306' "${compose}" >/dev/null 2>&1; then
  fail 'MySQL 不得发布宿主端口'
fi

# 日志双向挂载：后端与 code-server 共用宿主 logs/。
grep -qF './logs:/app/logs' "${compose}" || fail '后端缺少 logs 挂载'
grep -qF './logs:/home/coder/logs' "${compose}" || fail 'code-server 缺少 logs 挂载'

# 凭证只经环境注入，不写死在编排文件里。
if grep -E 'TARGET_LOGIN_PASSWORD: [^$]' "${compose}" | grep -v '\${' >/dev/null 2>&1; then
  fail '凭证被写死在编排文件里'
fi

# code-server 固定版本（用户已认可的镜像）。
grep -qF 'codercom/code-server:4.96.4' "${compose}" || fail 'code-server 镜像版本漂移'

# 环境样例齐备且必需变量在列。
[ -f 'deploy/.env.example' ] || fail '缺少 deploy/.env.example'
for var in TARGET_API_GATEWAY TARGET_LOGIN_PASSWORD TARGET_LOGIN_AES_KEY TARGET_LOGIN_CODE PLAN_DB_PASSWORD; do
  grep -qF "${var}=" deploy/.env.example || fail ".env.example 缺少 ${var}"
done

# Compose 语法可解析（docker 不可用时跳过实测，仅静态契约保持有效）。
if command -v docker >/dev/null 2>&1; then
  if ! docker compose -f "${compose}" config -q >/dev/null 2>&1; then
    fail 'docker compose config 解析失败'
  fi
fi

printf '%s\n' 'F-023 编排结构契约检查通过'
