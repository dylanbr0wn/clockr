#!/usr/bin/env bash
# Pre-build Google OAuth validation (DYL-86).
# Runs focused broker connect/refresh Go tests before packaging.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

echo "==> Broker connect/refresh smoke (Go): desktop BrokerFlow + broker HTTP API"
go test ./internal/config/ ./internal/integration/google/ ./internal/broker/... -count=1
echo "prebuild-oauth-check: ok"
