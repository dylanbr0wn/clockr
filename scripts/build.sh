#!/bin/bash
# Simple production build for current platform

set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Running OAuth pre-build checks..."
"${root}/scripts/prebuild-oauth-check.sh"

echo "Building for production..."
wails build -clean
echo "Build complete! Check build/bin/"

echo "Running OAuth post-build checks..."
"${root}/scripts/postbuild-oauth-check.sh"
