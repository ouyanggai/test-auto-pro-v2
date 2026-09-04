.PHONY: refs-sync refs-status form-runtime-sync form-runtime-status plan-db-config-sync logs-viewer logs-viewer-stop

refs-sync:
	@./scripts/reference-repos.sh sync

refs-status:
	@./scripts/reference-repos.sh status

form-runtime-sync:
	@./scripts/form-runtime-maintenance.sh sync

form-runtime-status:
	@./scripts/form-runtime-maintenance.sh status

plan-db-config-sync:
	@test -n "$(V1_CONFIG)" || (echo "请通过 V1_CONFIG 指定 V1 config.yaml" >&2; exit 1)
	@go run ./cmd/sync-v1-plan-db-config -source "$(V1_CONFIG)"

# logs-viewer 起一个固定版本的 code-server 容器，只挂载本项目的 logs/ 目录，内网直接访问。
# 容器以当前本机用户的 UID/GID 运行，保证挂载目录的读写权限与本机一致，日志不会出现权限异常。
# 本切片只提供这一个单容器启动方式；整套 Docker Compose 编排属于 F-023，不在这里提前实施。
# Docker 未启动时直接报错退出，不提供任何替代方案。
logs-viewer:
	@docker info >/dev/null 2>&1 || (echo "Docker 未启动：请先启动 Docker Desktop 或 Docker 守护进程后重试" >&2; exit 1)
	@mkdir -p logs
	@docker rm -f test-auto-pro-logs-viewer >/dev/null 2>&1 || true
	@docker run -d --name test-auto-pro-logs-viewer \
		-p 19002:8080 \
		-u "$$(id -u):$$(id -g)" \
		-e DOCKER_USER="$$(id -un)" \
		-e HOME=/home/coder \
		-v "$$(pwd)/logs:/home/coder/logs" \
		codercom/code-server:4.96.4 \
		--auth none --bind-addr 0.0.0.0:8080 /home/coder/logs >/dev/null
	@echo "日志查看器已启动：http://127.0.0.1:19002 （目录 /home/coder/logs）"

# logs-viewer-stop 只停止并删除日志查看容器，不动其他容器与日志文件。
logs-viewer-stop:
	@docker info >/dev/null 2>&1 || (echo "Docker 未启动：无需停止日志查看器" >&2; exit 1)
	@docker rm -f test-auto-pro-logs-viewer >/dev/null 2>&1 || true
	@echo "日志查看器已停止"
