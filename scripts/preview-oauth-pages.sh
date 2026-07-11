#!/usr/bin/env bash
# Render branded OAuth pages and open them in the default browser.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

out="${TMPDIR:-/tmp}/shiet-oauth-pages"
mkdir -p "$out"

echo "==> Rendering OAuth pages to $out"
go run ./scripts/preview-oauth-pages.go "$out"

echo "==> Opening previews"
if [[ "$(uname -s)" == "Darwin" ]]; then
  open "$out/success.html"
  open "$out/error.html"
  open "$out/close.html"
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open "$out/success.html"
  xdg-open "$out/error.html"
  xdg-open "$out/close.html"
else
  echo "Open these files in your browser:"
  printf '  %s\n' "$out/success.html" "$out/error.html" "$out/close.html"
fi
