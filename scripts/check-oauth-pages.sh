#!/usr/bin/env bash
# Ensure committed oauth-pages assets match the source build.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

echo "==> Build oauth-pages assets"
pnpm -C oauth-pages install --frozen-lockfile
pnpm -C oauth-pages build

echo "==> Verify embedded assets are up to date"
if ! git diff --quiet -- internal/oauthpages/assets; then
  echo "oauth-pages assets are out of date; run: pnpm -C oauth-pages build" >&2
  git diff -- internal/oauthpages/assets >&2 || true
  exit 1
fi

echo "check-oauth-pages: ok"
