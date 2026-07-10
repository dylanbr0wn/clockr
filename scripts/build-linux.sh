#!/bin/bash
# Build for Linux (AMD64)

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${root}/scripts/generate-api.sh"

echo "Building for Linux (amd64)..."
wails build -platform linux/amd64 -clean
echo "Build complete! Check build/bin/"
