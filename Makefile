.PHONY: refs-sync refs-status plan-db-config-sync

refs-sync:
	@./scripts/reference-repos.sh sync

refs-status:
	@./scripts/reference-repos.sh status

plan-db-config-sync:
	@test -n "$(V1_CONFIG)" || (echo "请通过 V1_CONFIG 指定 V1 config.yaml" >&2; exit 1)
	@go run ./cmd/sync-v1-plan-db-config -source "$(V1_CONFIG)"
