#!/bin/bash
# Build for macOS (Intel - AMD64)

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${root}/scripts/generate-api.sh"

echo "Building for macOS (amd64 - Intel)..."
wails build -platform darwin/amd64 -clean
echo "Build complete! Check build/bin/"
