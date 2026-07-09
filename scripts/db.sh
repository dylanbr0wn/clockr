#!/usr/bin/env bash
# Thin wrapper around the Go db tool (cmd/db). Run from anywhere.
#   ./scripts/db.sh up
#   ./scripts/db.sh seed --dev
#   ./scripts/db.sh status
#   ./scripts/db.sh --help
# Target db: --db flag, else $SHIET_DB, else ./shiet.dev.db
set -euo pipefail
cd "$(dirname "$0")/.."
exec go run ./cmd/db "$@"
