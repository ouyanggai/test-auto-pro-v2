.PHONY: setup dev restart stop status logs refs-sync refs-status

setup:
	@command -v go >/dev/null || { echo '缺少 go'; exit 1; }
	@command -v node >/dev/null || { echo '缺少 node'; exit 1; }
	@command -v pnpm >/dev/null || { echo '缺少 pnpm'; exit 1; }
	@go mod download
	@pnpm --dir web install --frozen-lockfile

dev: setup
	@./scripts/runtime.sh dev

restart: setup
	@./scripts/runtime.sh restart

stop:
	@./scripts/runtime.sh stop

status:
	@./scripts/runtime.sh status

logs:
	@./scripts/runtime.sh logs

refs-sync:
	@./scripts/reference-repos.sh sync

refs-status:
	@./scripts/reference-repos.sh status
