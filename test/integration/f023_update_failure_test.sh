#!/usr/bin/env bash
# 在独立沙箱注入 Docker 失败，不访问运行中的容器、数据库或目标平台。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
sandbox="$(mktemp -d /tmp/f023-update-test.XXXXXX)"
trap 'rm -rf -- "$sandbox"' EXIT
mkdir -p "$sandbox/bin"
export UPDATE_COMMAND_LOG="$sandbox/commands.log"
cat >"$sandbox/bin/docker" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$UPDATE_COMMAND_LOG"
case " $* " in
  *' config --quiet '*) exit 0 ;;
  *' build '*) [[ "$UPDATE_TEST_FAILURE" != build && "$UPDATE_TEST_FAILURE" != proxy ]] || exit 17 ;;
  *' image inspect '*) [[ "$UPDATE_TEST_FAILURE" != proxy ]] || exit 1 ;;
  *' ps -q '*) echo fake-container ;;
  *'inspect --format'*) echo unhealthy ;;
esac
EOF
chmod +x "$sandbox/bin/docker"
cat >"$sandbox/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
echo "systemctl $*" >>"$UPDATE_COMMAND_LOG"
EOF
chmod +x "$sandbox/bin/systemctl"
export PATH="$sandbox/bin:$PATH"
# 仅沙箱副本改写绝对路径和等待时长；正式脚本保持固定生产目录与完整等待边界。
sed -e "s|/opt/test-auto-pro-v2|$sandbox/app|g" \
    -e "s|/opt/.test-auto-pro-v2-update-|$sandbox/upload-|g" \
    -e "s|/etc/docker/daemon.json|$sandbox/daemon.json|g" \
    -e "s|/run/systemd/system/docker.service.d|$sandbox/systemd|g" \
    -e 's/seq 1 90/seq 1 1/g' -e 's/sleep 5/sleep 0/g' \
    "$project_root/deploy/update-server.sh" >"$sandbox/server.sh"

# 每个故障场景独立准备源码包，保证只通过当前场景实际退出码和文件事实断言。
prepare() {
  mkdir -p "$sandbox/app/deploy" "$sandbox/app/internal" "$sandbox/upload-case" "$sandbox/source/deploy"
  printf '配置必须保持\n' >"$sandbox/app/deploy/.env"
  printf 'old-code\n' >"$sandbox/app/internal/version.txt"
  for directory in cmd internal web form-runtime executor-service; do mkdir -p "$sandbox/source/$directory"; done
  printf 'new-code\n' >"$sandbox/source/internal/version.txt"
  printf 'services: {}\n' >"$sandbox/source/deploy/docker-compose.yml"
  printf 'FROM node:22-bookworm-slim\n' >"$sandbox/source/deploy/Dockerfile.backend"
  printf '{}\n' >"$sandbox/source/deploy/release-source.json"
  tar -czf "$sandbox/upload-case/source.tar.gz" -C "$sandbox/source" .
  digest="$(sha256sum "$sandbox/upload-case/source.tar.gz" | cut -d' ' -f1)"
  : >"$UPDATE_COMMAND_LOG"
}
prepare
export UPDATE_TEST_FAILURE=build
if bash "$sandbox/server.sh" "$sandbox/app" "$sandbox/upload-case" "$digest" 0 >"$sandbox/build.log" 2>&1; then
  echo '构建失败被错误报告为成功'; exit 1
fi
grep -q old-code "$sandbox/app/internal/version.txt"
grep -q ' build ' "$UPDATE_COMMAND_LOG"
! grep -q ' up -d --no-build' "$UPDATE_COMMAND_LOG"
[[ ! -f "$sandbox/app/.runtime/last-release.json" ]]
grep -q '配置必须保持' "$sandbox/app/deploy/.env"
echo '构建失败：未替换旧源码、未切换容器、配置保持'

prepare
export UPDATE_TEST_FAILURE=health
if bash "$sandbox/server.sh" "$sandbox/app" "$sandbox/upload-case" "$digest" 0 >"$sandbox/health.log" 2>&1; then
  echo '健康失败被错误报告为成功'; exit 1
fi
grep -q ' up -d --no-build' "$UPDATE_COMMAND_LOG"
grep -q '六服务未全部健康' "$sandbox/health.log"
[[ ! -f "$sandbox/app/.runtime/last-release.json" ]]
grep -q '配置必须保持' "$sandbox/app/deploy/.env"
echo '健康失败：非零退出、无成功标记、配置保持'

# 真实 flock 锁冲突必须在构建前拒绝，不靠模拟锁结果。
exec 8>"$sandbox/app/.runtime/update.lock"
flock -n 8
: >"$UPDATE_COMMAND_LOG"
if bash "$sandbox/server.sh" "$sandbox/app" "$sandbox/upload-case" "$digest" 0 >"$sandbox/lock.log" 2>&1; then
  echo '并发发布未被拒绝'; exit 1
fi
grep -q '已有更新发布正在执行' "$sandbox/lock.log"
[[ ! -s "$UPDATE_COMMAND_LOG" ]]
flock -u 8
echo '并发发布：锁冲突在构建前拒绝'

# 缺镜像时的临时代理失败也必须恢复配置；systemctl 被替换为沙箱记录器。
prepare
printf '{"registry-mirrors":["https://original.invalid"],"live-restore":true}\n' >"$sandbox/daemon.json"
cp "$sandbox/daemon.json" "$sandbox/daemon.expected"
export UPDATE_TEST_FAILURE=proxy
if bash "$sandbox/server.sh" "$sandbox/app" "$sandbox/upload-case" "$digest" 7897 >"$sandbox/proxy.log" 2>&1; then
  echo '代理构建失败被错误报告为成功'; exit 1
fi
cmp "$sandbox/daemon.expected" "$sandbox/daemon.json"
[[ ! -f "$sandbox/systemd/test-auto-pro-v2-update-proxy.conf" ]]
[[ "$(grep -c '^systemctl restart docker$' "$UPDATE_COMMAND_LOG")" = 2 ]]
! grep -q ' up -d --no-build' "$UPDATE_COMMAND_LOG"
echo '临时代理失败：恢复 Docker 原配置、撤除临时配置、未切换容器'
