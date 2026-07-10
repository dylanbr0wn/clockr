#!/bin/bash
# Build universal macOS binary (Intel + Apple Silicon)

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${root}/scripts/generate-api.sh"

echo "Building universal macOS binary..."
wails build -platform darwin/universal -clean
echo "Build complete! Check build/bin/"
