#!/bin/bash
# Build for Windows (AMD64)

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
"${root}/scripts/generate-api.sh"

echo "Building for Windows (amd64)..."
wails build -platform windows/amd64 -clean
echo "Build complete! Check build/bin/"
