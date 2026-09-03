.PHONY: refs-sync refs-status form-runtime-sync form-runtime-status plan-db-config-sync logs-viewer

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

# logs-viewer 直接在本机起零依赖的日志浏览服务，等同于在 logs-viewer/ 目录执行 pnpm dev:l。
# 开发阶段不用 Docker；统一容器化留到发布编排切片。
logs-viewer:
	@cd logs-viewer && pnpm dev:l
