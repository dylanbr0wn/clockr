#!/usr/bin/env bash
set -euo pipefail

image_name="${1:-shiet-oauth-broker:railway-smoke}"
container_name="shiet-oauth-broker-smoke-$$"
host_port="${SHIET_BROKER_SMOKE_PORT:-18080}"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build \
  -f deploy/railway/oauth-broker.Dockerfile \
  -t "$image_name" \
  .

docker run -d \
  --name "$container_name" \
  -p "127.0.0.1:${host_port}:8080" \
  -e PORT=8080 \
  -e SHIET_BROKER_PUBLIC_ORIGIN=https://auth.shiet.app \
  -e SHIET_BROKER_GOOGLE_CLIENT_ID=smoke-client-id \
  -e SHIET_BROKER_GOOGLE_CLIENT_SECRET=smoke-client-secret \
  -e SHIET_BROKER_DATASTORE_DSN=file:/tmp/oauth-broker-smoke.sqlite \
  "$image_name" >/dev/null

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:${host_port}/readyz" >/dev/null; then
    curl -fsS "http://127.0.0.1:${host_port}/healthz" >/dev/null
    echo "Railway broker smoke check passed on port ${host_port}."
    exit 0
  fi
  sleep 1
done

docker logs "$container_name" >&2 || true
echo "Railway broker smoke check failed." >&2
exit 1
