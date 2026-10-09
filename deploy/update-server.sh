#!/usr/bin/env bash
# 服务器发布仅处理本项目源码与容器，数据库、运行时维护卷和日志均保留。
set -Eeuo pipefail
root="${1:?缺少部署目录}"
upload="${2:?缺少上传目录}"
digest="${3:?缺少包摘要}"
proxy_port="${4:-0}"
[[ "$root" = /opt/test-auto-pro-v2 ]] || { echo '部署目录必须为 /opt/test-auto-pro-v2'; exit 1; }
[[ "$upload" = /opt/.test-auto-pro-v2-update-* && "$upload" != *'..'* ]] || exit 1
[[ "$proxy_port" =~ ^[0-9]+$ ]] || exit 1
cd "$root"
for tool in docker flock tar sha256sum curl; do command -v "$tool" >/dev/null; done
[[ -f deploy/.env ]] || { echo '服务器缺少 deploy/.env'; exit 1; }
mkdir -p .runtime logs
exec 9>.runtime/update.lock
flock -n 9 || { echo '已有更新发布正在执行，请等待完成'; exit 1; }
log="$root/logs/update-$(basename "$upload").log"
exec > >(tee -a "$log") 2>&1
stage="$upload/source"
proxy_dropin=/run/systemd/system/docker.service.d/test-auto-pro-v2-update-proxy.conf
proxy_enabled=0

# 恢复原下载配置后再验活，成功不能依赖尚未撤除的临时代理。
restore_proxy() {
  rm -f "$proxy_dropin" &&
    cp "$upload/daemon.json.original" /etc/docker/daemon.json &&
    systemctl daemon-reload && systemctl restart docker || return 1
  proxy_enabled=0
}

# 任何失败都保留原始日志；临时 Docker 代理必须在隧道关闭前撤除。
cleanup() {
  status=$?
  trap - EXIT
  if [[ "$proxy_enabled" = 1 ]]; then
    if ! restore_proxy; then
      echo '撤除临时 Docker 代理后重启失败，请检查 Docker 服务'
      status=1
    fi
  fi
  if [[ "$status" != 0 ]]; then
    echo "更新发布失败，保留现场：$upload；日志：$log"
  else
    # 路径在入口已固定验证，成功后只删除本次上传及构建上下文。
    rm -rf -- "$upload"
    echo "更新发布与验活完成；日志：$log"
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'echo "发布失败：行 $LINENO，退出码 $?"' ERR
printf '%s  %s\n' "$digest" "$upload/source.tar.gz" | sha256sum -c -
mkdir -p "$stage"
tar -xzf "$upload/source.tar.gz" -C "$stage"
chmod 600 deploy/.env
export DOCKER_BUILDKIT=1
build=(docker compose --env-file "$root/deploy/.env" -f "$stage/deploy/docker-compose.yml")
compose=(docker compose --env-file deploy/.env -f deploy/docker-compose.yml)
"${build[@]}" config --quiet

build_args=()
if [[ "$proxy_port" != 0 ]]; then
  proxy="http://127.0.0.1:$proxy_port"
  for name in HTTP_PROXY HTTPS_PROXY http_proxy https_proxy; do build_args+=(--build-arg "$name=$proxy"); done
  build_args+=(--build-arg NO_PROXY=localhost,127.0.0.1,192.168.0.0/16)
  cat >"$upload/build-network.yml" <<'EOF'
services:
  backend:
    build:
      network: host
  web:
    build:
      network: host
  form-runtime:
    build:
      network: host
  executor:
    build:
      network: host
EOF
  build+=(-f "$upload/build-network.yml")
  # 已缓存基础镜像不访问仓库；只有缺失镜像时才临时调整守护进程下载代理。
  missing=0
  while read -r image; do docker image inspect "$image" >/dev/null 2>&1 || missing=1; done < <(
    awk 'toupper($1)=="FROM" {print $2}' "$stage"/deploy/Dockerfile.*
    printf '%s\n' mysql:8.4 codercom/code-server:4.96.4
  )
  if [[ "$missing" = 1 ]]; then
    [[ ! -e "$proxy_dropin" ]] || { echo '已有临时 Docker 代理配置，请先处理'; exit 1; }
    command -v python >/dev/null
    cp /etc/docker/daemon.json "$upload/daemon.json.original"
    mkdir -p "$(dirname "$proxy_dropin")"
    proxy_enabled=1
    # 原镜像加速站曾返回摘要不匹配的清单；临时代理直接访问官方仓库，结束恢复原文件。
    python -c 'import json; p="/etc/docker/daemon.json"; d=json.load(open(p)); d["registry-mirrors"]=[]; json.dump(d,open(p,"w"))'
    printf '[Service]\nEnvironment="HTTP_PROXY=%s" "HTTPS_PROXY=%s" "NO_PROXY=localhost,127.0.0.1,192.168.0.0/16"\n' "$proxy" "$proxy" >"$proxy_dropin"
    systemctl daemon-reload
    systemctl restart docker
  fi
fi
build_args+=(--build-arg DEBIAN_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian)
build_args+=(--build-arg DEBIAN_SECURITY_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian-security)
echo '开始构建四个应用镜像；构建失败不切换运行中的容器'
"${build[@]}" build "${build_args[@]}" backend web form-runtime executor

# 源码目录整体替换，删除旧版本已移除文件；这些目录不包含数据库、日志或参考仓库。
for directory in cmd internal web form-runtime executor-service; do
  [[ -d "$stage/$directory" ]] || { echo "发布包缺少目录：$directory"; exit 1; }
done
for directory in cmd internal web form-runtime executor-service; do
  if [[ -e "$root/$directory" ]]; then mv "$root/$directory" "$upload/previous-$directory"; fi
  mv "$stage/$directory" "$root/$directory"
done
cp -a "$stage/." "$root/"
echo '镜像构建完成，更新六服务'
"${compose[@]}" up -d --no-build
if [[ "$proxy_enabled" = 1 ]]; then
  echo '恢复 Docker 原下载配置，再检查六服务运行状态'
  restore_proxy
fi
healthy=0
for attempt in $(seq 1 90); do
  healthy=0
  for service in mysql backend web form-runtime executor code-server; do
    container="$("${compose[@]}" ps -q "$service")"
    if [[ -n "$container" && "$(docker inspect --format '{{.State.Health.Status}}' "$container")" = healthy ]]; then
      healthy=$((healthy+1))
    fi
  done
  [[ "$healthy" = 6 ]] && break
  sleep 5
done
"${compose[@]}" ps
[[ "$healthy" = 6 ]] || { echo '六服务未全部健康，停止发布验活'; exit 1; }
bash test/integration/f023_deployment_smoke.sh
# 只在验活通过后记录当前发布；失败不写入成功标记，也不回退数据库迁移。
cp deploy/release-source.json .runtime/last-release.json
