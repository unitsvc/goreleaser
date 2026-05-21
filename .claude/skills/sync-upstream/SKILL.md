---
name: sync-upstream
description: Use when syncing from upstream goreleaser/goreleaser to the GitHub development repo. Triggers on "sync upstream", "update from upstream", "pull upstream".
---

# Sync Upstream

Sync the upstream `goreleaser/goreleaser` repository to the GitHub development repo (`next-bin/goreleaser`) via a `sync/upstream` branch and PR.

## Prerequisites

- `gh` CLI installed and authenticated
- Git repository with `origin` pointing to `next-bin/goreleaser`

## Flow

Follow these steps in order. Stop and report to the user if any step fails.

### 1. Guard — Uncommitted Changes

Check for uncommitted changes:

```bash
git status --porcelain
```

If output is non-empty, warn the user: "Working tree has uncommitted changes. Please commit or stash before syncing." and STOP.

### 2. Remote Init — Upstream

Check if `upstream` remote exists and verify its URL:

```bash
git remote -v | grep upstream
```

- If no `upstream` remote, add it: `git remote add upstream https://github.com/goreleaser/goreleaser.git`
- If `upstream` exists but URL does not match `https://github.com/goreleaser/goreleaser.git`, warn the user and STOP.

### 3. Fetch Upstream

```bash
git fetch upstream --no-tags
```

Note: `--no-tags` avoids pulling upstream tags. Only `main` branch code is synced. Tags are managed separately via the release skill and `sync-gitee`.

If this fails (network error), report the error and STOP.

### 4. Switch to Main and Pull

```bash
git checkout main
git pull origin main
```

### 5. Create/Update sync/upstream Branch

```bash
git branch -f sync/upstream upstream/main
```

### 6. Check for New Commits

```bash
git log --oneline main..sync/upstream
```

- If output is empty, print "Already up to date. No new commits from upstream." and STOP.
- If output is non-empty, continue.

### 7. Push to GitHub

```bash
git push origin sync/upstream --force-with-lease
```

### 8. Check for Existing Open PR

```bash
gh pr list --head sync/upstream --state open --json number,url
```

- If an open PR already exists, print the PR URL and tell the user: "An open PR already exists: <url>. Please review and merge it." and STOP.

### 9. Create PR

Generate the PR title with commit range and date:

```bash
RANGE="$(git rev-parse --short main)..$(git rev-parse --short sync/upstream)"
TITLE="sync: upstream $RANGE ($(date +%Y-%m-%d))"
```

Generate the PR body:
1. Read the full commit log: `git log main..sync/upstream`
2. Write a concise **Summary** section, grouping commits by category (features, bug fixes, deps, docs, other).
3. Append the raw commit log (capped at 50 lines):

```bash
git log --oneline main..sync/upstream | head -50
echo "---"
echo "Total: $(git rev-list --count main..sync/upstream) commits"
```

Then create the PR:

```bash
gh pr create --base main --head sync/upstream --title "$TITLE" --body "$BODY"
```

### 10. Conflict Detection

Poll GitHub for merge status (up to 3 times, 10s intervals):

```bash
gh pr view <PR_NUMBER> --json mergeable
```

- If `MERGEABLE` → no conflicts. Print PR URL.
- If `CONFLICTING` → warn the user: "The PR has merge conflicts with main. Would you like me to resolve them locally?" If yes:
  ```bash
  git checkout sync/upstream
  git merge main
  # resolve conflicts, then:
  git push origin sync/upstream --force-with-lease
  git checkout main
  ```
- If still `UNKNOWN` after 3 polls → warn: "Could not determine merge status. Check the PR on GitHub."

### 11. Output

Print the PR URL and tell the user to review and merge.
