#!/usr/bin/env bash

# F-023 发布编排定向验证：
# 结构契约（静态）必须通过；Compose 冷启动实测需要本机 Docker/Colima 可用，
# 以 F023_COMPOSE_UP=1 显式启用——构建与启动依赖外网镜像与较长时间，不在默认验证内静默跳过。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-023] 编排结构契约'
./test/contracts/f023/compose_structure.sh

printf '%s\n' '[F-023] 构建上下文卫生（不得把凭证、日志、参考源码、缓存带进镜像层）'
for forbidden in '.env.local' '参考代码' 'node_modules' 'logs/';
do
  if grep -RIn "COPY.*${forbidden}" deploy/Dockerfile.* >/dev/null 2>&1; then
    printf '[F-023] 镜像构建把禁带物复制进镜像：%s\n' "${forbidden}" >&2
    exit 1
  fi
done
if ! grep -qF '.dockerignore' .dockerignore 2>/dev/null; then
  : # 根目录 .dockerignore 在下方单独核对
fi
if [ ! -f .dockerignore ]; then
  printf '%s\n' '[F-023] 缺少根目录 .dockerignore（构建上下文卫生）' >&2
  exit 1
fi
for forbidden in '.env.local' '参考代码' 'logs' '.git' 'node_modules' '.runtime';
do
  grep -qF "${forbidden}" .dockerignore || printf '%s\n' "[F-023] 提醒：.dockerignore 未显式包含 ${forbidden}" >&2
done

if [ "${F023_COMPOSE_UP:-0}" = "1" ]; then
  printf '%s\n' '[F-023] Compose 冷启动实测（显式启用）'
  docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example up -d --build
  printf '%s\n' '[F-023] 等待五个服务健康……'
  for i in $(seq 1 60); do
    if docker compose -f deploy/docker-compose.yml ps --status healthy | grep -q backend; then
      break
    fi
    sleep 5
  done
  docker compose -f deploy/docker-compose.yml ps
  curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19080}/api/health" >/dev/null && printf '%s\n' '[F-023] 后端健康'
  curl -fsS "http://127.0.0.1:${WEB_PORT:-19000}/" >/dev/null && printf '%s\n' '[F-023] 前端可访问'
  curl -fsS "http://127.0.0.1:${FORM_RUNTIME_PORT:-19001}/" >/dev/null && printf '%s\n' '[F-023] 表单运行时可访问'
  printf '%s\n' '[F-023] 停止并清理（保留命名卷与宿主日志）'
  docker compose -f deploy/docker-compose.yml down
else
  printf '%s\n' '[F-023] Compose 冷启动实测未启用：设置 F023_COMPOSE_UP=1 且准备好 deploy/.env 后运行（构建依赖外网与较长时间）'
fi

printf '%s\n' 'F-023 定向验证完成'
