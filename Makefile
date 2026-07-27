.PHONY: refs-sync refs-status

refs-sync:
	@./scripts/reference-repos.sh sync

refs-status:
	@./scripts/reference-repos.sh status
