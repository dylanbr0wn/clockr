#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v buf >/dev/null 2>&1; then
  printf 'Buf is required to generate API sources. Install Buf v1.71.0, then rerun.\n' >&2
  exit 1
fi

cd "$root"
buf lint
buf generate
