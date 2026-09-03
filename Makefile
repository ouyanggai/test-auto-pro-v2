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

# logs-viewer 用固定版本的 code-server 挂载本机 logs/，内网直接访问，无登录、可读写。
# 容器内以当前用户身份运行，保证挂载目录的写权限与本机一致。
logs-viewer:
	@mkdir -p logs
	@docker rm -f test-auto-pro-logs-viewer >/dev/null 2>&1 || true
	@docker run -d --name test-auto-pro-logs-viewer \
		-p 19002:8080 \
		-u "$$(id -u):$$(id -g)" \
		-e DOCKER_USER="$$(id -un)" \
		-v "$$(pwd)/logs:/home/coder/logs" \
		codercom/code-server:4.96.4 \
		--auth none --bind-addr 0.0.0.0:8080 /home/coder/logs
	@echo "日志查看器已启动：http://127.0.0.1:19002"

logs-viewer-stop:
	@docker rm -f test-auto-pro-logs-viewer >/dev/null 2>&1 || true
	@echo "日志查看器已停止"
