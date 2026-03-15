#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STUB_HOST="${GOOGLE_STUB_HOST:-127.0.0.1}"
STUB_PORT="${GOOGLE_STUB_PORT:-19090}"
GOOGLE_CLIENT_ID="${GOOGLE_CLIENT_ID:-local-google-client-id}"
GOOGLE_CLIENT_SECRET="${GOOGLE_CLIENT_SECRET:-local-google-client-secret}"
GOOGLE_STUB_EMAIL="${GOOGLE_STUB_EMAIL:-local@example.com}"
GOOGLE_STUB_NAME="${GOOGLE_STUB_NAME:-Local Developer}"
STUB_LOG_PATH="${GOOGLE_STUB_LOG_PATH:-$ROOT_DIR/output/local-google-oauth-stub.log}"
STUB_PID=""

cleanup() {
  if [[ -n "$STUB_PID" ]]; then
    kill "$STUB_PID" >/dev/null 2>&1 || true
    wait "$STUB_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

mkdir -p "$(dirname "$STUB_LOG_PATH")"

python3 "$ROOT_DIR/scripts/google_oauth_stub.py" \
  --host "$STUB_HOST" \
  --port "$STUB_PORT" \
  --client-id "$GOOGLE_CLIENT_ID" \
  --client-secret "$GOOGLE_CLIENT_SECRET" \
  --email "$GOOGLE_STUB_EMAIL" \
  --name "$GOOGLE_STUB_NAME" >"$STUB_LOG_PATH" 2>&1 &
STUB_PID=$!

echo "local Google OAuth stub: http://${STUB_HOST}:${STUB_PORT}"
echo "stub account: ${GOOGLE_STUB_EMAIL}"

cd "$ROOT_DIR"
APP_MODE=local \
AUTH_GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID" \
AUTH_GOOGLE_CLIENT_SECRET="$GOOGLE_CLIENT_SECRET" \
AUTH_GOOGLE_AUTH_URL="http://${STUB_HOST}:${STUB_PORT}/auth" \
AUTH_GOOGLE_TOKEN_URL="http://${STUB_HOST}:${STUB_PORT}/token" \
AUTH_GOOGLE_USERINFO_URL="http://${STUB_HOST}:${STUB_PORT}/userinfo" \
go run main.go wire_gen.go
