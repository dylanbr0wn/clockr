#!/bin/bash
# Build for macOS (Apple Silicon - ARM64)

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${root}/scripts/generate-api.sh"

echo "Building for macOS (arm64 - Apple Silicon)..."
wails build -platform darwin/arm64 -clean
echo "Build complete! Check build/bin/"
