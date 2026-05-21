---
name: sync-gitee
description: Use when syncing from GitHub to Gitee mirror. Triggers on "sync gitee", "push to gitee", "mirror gitee".
---

# Sync Gitee

Sync the GitHub development repo (`next-bin/goreleaser`) to the Gitee mirror (`gitee.com/next-bin/goreleaser`).

## Prerequisites

- The `sync-upstream` PR has been merged into `main`.
- Git authentication configured for Gitee.

## Flow

Follow these steps in order. Stop and report to the user if any step fails.

### 1. Guard — Uncommitted Changes

```bash
git status --porcelain
```

If output is non-empty, warn the user and STOP.

### 2. Remote Init — Gitee

```bash
git remote -v | grep gitee
```

- If no `gitee` remote, add it: `git remote add gitee https://gitee.com/next-bin/goreleaser.git`
- If `gitee` exists but URL does not match `https://gitee.com/next-bin/goreleaser.git`, warn and STOP.

### 3. Remote Init — Upstream

```bash
git remote -v | grep upstream
```

- If no `upstream` remote, add it: `git remote add upstream https://github.com/goreleaser/goreleaser.git`
- If exists but URL mismatches, warn and STOP.

### 4. Update Local Main

```bash
git checkout main
git pull origin main
```

### 5. Verify Main Is Up to Date with Upstream

```bash
git fetch upstream --quiet
git log --oneline main..upstream/main
```

If output is non-empty, warn: "main is behind upstream by N commits. Have you merged the sync PR?" and STOP.

### 6. Push to Gitee

```bash
git push gitee main
git push gitee --tags
```

If push fails (auth, network), report the error and STOP.

### 7. Output

Confirm sync success:

```bash
git log --oneline -1 main
git tag --sort=-creatordate | head -3
```

Print: "Successfully synced to Gitee. Latest commit: <hash> <message>"
