---
name: goreleaser-release
description: Use when releasing goreleaser binaries to GitHub and Gitee. Triggers on "release", "publish", "cut a release".
---

# GoReleaser Release

Build and publish goreleaser binaries to both GitHub Releases and Gitee Releases using the fork's custom config.

## Tag Convention

- Format: `vX.Y.Z-gitee.N` (based on upstream version + `-gitee.N` suffix)
- Example: upstream `v1.26.0` → fork `v1.26.0-gitee.1`, then `v1.26.0-gitee.2`
- N increments per release against the same upstream version.

## Flow

Follow these steps in order. Stop and report to the user if any step fails.

### 1. Token Check

Check environment variables:

```bash
echo "${GITHUB_TOKEN:+set}" && echo "${GITEE_TOKEN:+set}"
```

If either is not set:
1. Check for `.env` file: `test -f "$(git rev-parse --show-toplevel)/.env" && echo "exists"`
2. If `.env` exists, source it: `set -a; source "$(git rev-parse --show-toplevel)/.env"; set +a`
3. Check again. If still missing, prompt user: "Please set the missing token(s): export GITHUB_TOKEN=... and/or export GITEE_TOKEN=..." and STOP. Do NOT read or display token values.

### 2. Guard

```bash
git status --porcelain
git symbolic-ref --short HEAD
```

- If working tree is dirty, warn and STOP.
- If not on `main`, warn and STOP.

### 3. Remote Init — Upstream

```bash
git remote -v | grep upstream
```

- If no `upstream` remote, add it: `git remote add upstream https://github.com/goreleaser/goreleaser.git`
- If URL mismatches, warn and STOP.

### 4. Remote Init — Gitee

```bash
git remote -v | grep gitee
```

- If no `gitee` remote, add it: `git remote add gitee https://gitee.com/next-bin/goreleaser.git`
- If URL mismatches, warn and STOP.

### 5. Determine Version

Run the helper script:

```bash
./scripts/determine-version.sh
```

This outputs the next version (e.g. `v1.26.0-gitee.1`).

Check if this is the first release for this upstream version:

```bash
UPSTREAM_VERSION=$(git tag -l 'v*' --sort=-v:refname | grep -vE '\-gitee\.|rc\.|beta\.|alpha\.' | head -1)
git tag -l "$UPSTREAM_VERSION"
```

If the plain upstream tag does NOT exist yet, this is the first release.

**IMPORTANT:** Always confirm the version with the user before proceeding. Ask: "Release version will be $VERSION. Proceed?"

### 6. First Tag — Tag Only (No Release)

If the plain upstream tag (e.g. `v1.26.0`) does NOT exist yet, this is the first release for this upstream version. Create the tag and push to both GitHub and Gitee, but do NOT build or release:

```bash
git tag -a $UPSTREAM_VERSION -m "sync: tag $UPSTREAM_VERSION from upstream"
git push origin $UPSTREAM_VERSION
git push gitee $UPSTREAM_VERSION
```

Then tell the user: "Created plain tag $UPSTREAM_VERSION and pushed to GitHub + Gitee (no binary release). This prevents Gitee failures on large initial content. Run the release skill again to create the first -gitee.N release." and STOP.

### 7. Create Tag (Normal Flow)

If the plain upstream tag already exists (not first release), create the `-gitee.N` tag:

```bash
git tag -a $VERSION -m "release $VERSION"
git push origin $VERSION
git push gitee $VERSION
```

### 8. Bootstrap Build

Compile goreleaser from current source (output outside `dist/` to avoid `--clean` deleting it):

```bash
go build -o ./build/goreleaser-bootstrap .
```

If build fails, report the error and STOP.

### 9. Self-Release

Use the bootstrapped binary with the fork-specific config:

```bash
GORELEASER_CURRENT_TAG=$VERSION ./build/goreleaser-bootstrap release --config .goreleaser-sync.yaml --clean
```

This publishes to both GitHub Releases and Gitee Releases.

If release fails, suggest checking `.goreleaser-sync.yaml` config and report the error.

Alternatively, use `make release` which combines steps 7-9.

### 10. Verify

```bash
gh release view $VERSION --json tagName,isDraft
```

```bash
curl -s "https://gitee.com/api/v5/repos/next-bin/goreleaser/releases/tags/$VERSION" | jq '.tag_name'
```

### 11. Output

Print release URLs for both platforms:
- GitHub: `https://github.com/next-bin/goreleaser/releases/tag/$VERSION`
- Gitee: `https://gitee.com/next-bin/goreleaser/releases/$VERSION`
