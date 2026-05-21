---
name: sync-upstream
description: Use when syncing from upstream goreleaser/goreleaser to the GitHub development repo. Triggers on "sync upstream", "update from upstream", "pull upstream". Automatically resolves conflicts and merges the PR.
---

# Sync Upstream

Sync the upstream `goreleaser/goreleaser` repository to the GitHub development repo (`next-bin/goreleaser`) via a `sync/upstream` branch and PR. Automatically resolves merge conflicts and merges the PR.

## Prerequisites

- `gh` CLI installed and authenticated
- Git repository with `origin` pointing to `next-bin/goreleaser`

## Known Fork Files

The following paths are fork-specific and must be preserved from `main` (not overwritten by upstream) during conflict resolution:

```
.claude/
.goreleaser-sync.yaml
scripts/determine-version.sh
.env
build/
docs/superpowers/
```

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

- If an open PR already exists, note its number and URL. Skip to step 10 (conflict detection).
- If no open PR, continue to step 9.

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

Note the PR number from the output.

### 10. Conflict Detection & Auto-Resolution

Poll GitHub for merge status (up to 3 times, 10s intervals):

```bash
gh pr view <PR_NUMBER> --json mergeable
```

- If `MERGEABLE` → no conflicts. Proceed to step 11 (auto-merge).
- If `CONFLICTING` → auto-resolve conflicts locally (see below).
- If still `UNKNOWN` after 3 polls → warn: "Could not determine merge status. Attempting local merge check."

#### Auto-Resolve Conflicts

When conflicts are detected, resolve them automatically:

```bash
git checkout sync/upstream
git merge main --no-edit
```

If merge fails with conflicts:

1. List all conflicted files:

```bash
CONFLICTS=$(git diff --name-only --diff-filter=U)
```

2. For each conflicted file, apply resolution strategy:
   - **If the file is a known fork file** (path starts with `.claude/`, `.goreleaser-sync.yaml`, `scripts/determine-version.sh`, `docs/superpowers/`): keep `main`'s version:
     ```bash
     git checkout --theirs <file>
     git add <file>
     ```
   - **For all other files**: keep upstream's version (`sync/upstream`):
     ```bash
     git checkout --ours <file>
     git add <file>
     ```

3. After all conflicts are resolved, commit and push:

```bash
git commit --no-edit
git push origin sync/upstream --force-with-lease
```

4. Switch back to main:

```bash
git checkout main
```

5. Re-poll GitHub merge status (up to 5 times, 10s intervals). Once `MERGEABLE`, proceed to step 11. If still `UNKNOWN` after 5 polls, proceed anyway — `gh pr merge` will fail safely if not ready.

### 11. Auto-Merge PR

Merge the PR using GitHub's merge (not squash, to preserve upstream commit history):

```bash
gh pr merge <PR_NUMBER> --merge --delete-branch=false
```

Do NOT delete the `sync/upstream` branch — it is reused for future syncs.

### 12. Update Local Main

```bash
git checkout main
git pull origin main
```

### 13. Output

Print:
- The merged PR URL
- Latest commit on main: `git log --oneline -1 main`
- "Sync complete. Local main is now up to date with upstream."
