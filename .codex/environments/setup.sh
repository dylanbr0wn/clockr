#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '\n==> %s\n' "$*"
}

warn() {
  printf '\nWARN: %s\n' "$*" >&2
}

repo_root() {
  if [[ -n "${CODEX_WORKTREE_PATH:-}" ]]; then
    printf '%s\n' "$CODEX_WORKTREE_PATH"
    return
  fi

  git rev-parse --show-toplevel 2>/dev/null
}

cd "$(repo_root)"

if command -v go >/dev/null 2>&1; then
  GOPATH_BIN="$(go env GOPATH)/bin"
  export PATH="$GOPATH_BIN:$PATH"
else
  export PATH="$HOME/go/bin:$PATH"
fi

install_ubuntu_wails_deps() {
  if [[ "$(uname -s)" != "Linux" ]] || ! command -v apt-get >/dev/null 2>&1; then
    return
  fi

  local deps=(
    build-essential
    libayatana-appindicator3-dev
    libgtk-3-dev
    libwebkit2gtk-4.1-dev
    pkg-config
  )
  local missing=()

  for dep in "${deps[@]}"; do
    if ! dpkg-query -W -f='${Status}' "$dep" 2>/dev/null | grep -q "install ok installed"; then
      missing+=("$dep")
    fi
  done

  if [[ "${#missing[@]}" -eq 0 ]]; then
    log "Ubuntu Wails system dependencies are already installed"
    return
  fi

  log "Installing Ubuntu Wails system dependencies: ${missing[*]}"
  if [[ "$(id -u)" -eq 0 ]]; then
    apt-get update
    apt-get install -y "${missing[@]}"
  elif command -v sudo >/dev/null 2>&1; then
    sudo apt-get update
    sudo apt-get install -y "${missing[@]}"
  else
    warn "Cannot install apt packages without root or sudo: ${missing[*]}"
  fi
}

ensure_pnpm() {
  if command -v pnpm >/dev/null 2>&1; then
    log "Using pnpm $(pnpm --version)"
    return
  fi

  if ! command -v corepack >/dev/null 2>&1; then
    warn "pnpm is missing and corepack is unavailable; skipping frontend setup"
    return
  fi

  log "Enabling pnpm 11 with corepack"
  corepack enable
  corepack prepare pnpm@11 --activate
}

install_frontend() {
  if ! command -v pnpm >/dev/null 2>&1; then
    warn "pnpm is unavailable; skipping frontend dependency install and build"
    return
  fi

  log "Installing frontend dependencies"
  pnpm -C frontend install --frozen-lockfile

  log "Building frontend assets for Go embed"
  pnpm -C frontend build
}

install_go_tools() {
  if ! command -v go >/dev/null 2>&1; then
    warn "go is unavailable; skipping Go module download and Wails CLI install"
    return
  fi

  export GOCACHE="${GOCACHE:-/tmp/clockr-gocache}"
  mkdir -p "$GOCACHE"

  log "Downloading Go modules"
  go mod download

  log "Installing Wails CLI v2.12.0"
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
}

install_ubuntu_wails_deps
ensure_pnpm
install_frontend
install_go_tools

log "Clockr environment setup complete"
printf 'Use DISPLAY=:1 wails dev -tags webkit2_41 on Ubuntu 24.04 environments.\n'
