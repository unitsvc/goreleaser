#!/usr/bin/env bash
set -euo pipefail

git fetch upstream --tags --quiet 2>/dev/null || true
UPSTREAM_TAG=$(git tag -l 'v*' --sort=-v:refname | grep -vE '\-gitee\.|rc\.|beta\.|alpha\.|nightly' | head -1)
if [ -z "$UPSTREAM_TAG" ]; then
  echo "Error: no upstream tag found" >&2; exit 1
fi

BASE=${UPSTREAM_TAG#v}
EXISTING=$(git tag -l "v${BASE}-gitee.*" --sort=-v:refname | head -1)
if [ -n "$EXISTING" ]; then
  N=$(echo "$EXISTING" | sed -E 's/.*\-gitee\.([0-9]+)$/\1/')
else
  N=0
fi
NEXT=$((N + 1))
echo "v${BASE}-gitee.${NEXT}"
