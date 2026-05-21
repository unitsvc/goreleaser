.PHONY: sync-upstream sync-gitee release bootstrap

check-clean:
	@if [ -n "$$(git status --porcelain)" ]; then echo "Error: dirty working tree" >&2; exit 1; fi

check-main:
	@if [ "$$(git symbolic-ref --short HEAD)" != "main" ]; then echo "Error: not on main branch" >&2; exit 1; fi

# Sync upstream goreleaser/goreleaser → origin (GitHub), main branch only, no tags
sync-upstream: check-clean
	@git remote add upstream https://github.com/goreleaser/goreleaser.git 2>/dev/null || true
	git fetch upstream --no-tags
	git checkout main
	git pull origin main
	git branch -f sync/upstream upstream/main
	@if [ -z "$$(git log --oneline main..sync/upstream)" ]; then echo "Already up to date"; exit 0; fi
	git push origin sync/upstream --force-with-lease
	@echo "Branch sync/upstream pushed. Create PR manually or via skill."

# Sync origin (GitHub) → gitee: main branch + all tags
sync-gitee: check-clean
	@git remote add gitee https://gitee.com/next-bin/goreleaser.git 2>/dev/null || true
	git checkout main
	git pull origin main
	git push gitee main
	git push gitee --tags
	@echo "Synced to Gitee."

# Bootstrap build: compile goreleaser from source
bootstrap:
	@mkdir -p build
	go build -o ./build/goreleaser-bootstrap .

# Full release: tag → publish to both GitHub and Gitee
release: check-clean check-main bootstrap
	@VERSION=$${VERSION:-$$(./scripts/determine-version.sh)}; \
	echo "Releasing $$VERSION"; \
	git tag -a "$$VERSION" -m "release $$VERSION"; \
	git push origin "$$VERSION"; \
	git push gitee "$$VERSION"; \
	GORELEASER_CURRENT_TAG="$$VERSION" ./build/goreleaser-bootstrap release --config .goreleaser-sync.yaml --clean
