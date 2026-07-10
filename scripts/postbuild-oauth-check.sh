#!/usr/bin/env bash
# Post-build Google OAuth validation (DYL-86).
# Probes the deployed broker for liveness after packaging.
#
#   BROKER_BASE_URL  — default https://auth.shiet.app
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

broker_base="${BROKER_BASE_URL:-https://auth.shiet.app}"
broker_base="${broker_base%/}"

fail() {
  echo "postbuild-oauth-check: $*" >&2
  exit 1
}

echo "==> Deployed broker liveness: ${broker_base}/healthz"
if ! curl -fsS --max-time 15 "${broker_base}/healthz" >/dev/null; then
  fail "broker healthz failed at ${broker_base}/healthz"
fi
if ! curl -fsS --max-time 15 "${broker_base}/readyz" >/dev/null; then
  fail "broker readyz failed at ${broker_base}/readyz"
fi
echo "deployed broker liveness passed"

echo "Manual release checklist (when credentials/network allow):"
echo "  - Connect Google via Settings in a public build (broker mode)"
echo "  - Confirm token refresh after expiry / forced 401 path"
echo "postbuild-oauth-check: ok"
