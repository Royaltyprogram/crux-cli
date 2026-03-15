#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SMOKE_HOST="${SMOKE_HOST:-127.0.0.1}"
SMOKE_PORT="${SMOKE_PORT:-18082}"
BASE_URL="${BASE_URL:-http://${SMOKE_HOST}:${SMOKE_PORT}}"
SERVER_ADDR="${SERVER_ADDR:-:${SMOKE_PORT}}"
SERVER_BIN="${SERVER_BIN:-$ROOT_DIR/output/server}"
CLI_BIN="${CLI_BIN:-$ROOT_DIR/output/crux}"
EMAIL="${BETA_SMOKE_EMAIL:-beta1@example.com}"
JWT_SECRET_VALUE="${JWT_SECRET_VALUE:-closed-beta-prod-smoke-secret}"
JWT_SECRET_FILE_OVERRIDE="${JWT_SECRET_FILE_OVERRIDE:-}"
AUTH_BOOTSTRAP_USERS_FILE_OVERRIDE="${AUTH_BOOTSTRAP_USERS_FILE_OVERRIDE:-}"
OPENAI_API_KEY_FILE_OVERRIDE="${OPENAI_API_KEY_FILE_OVERRIDE:-}"
EXPECT_RESEARCH_MODE="${EXPECT_RESEARCH_MODE:-}"
GOOGLE_STUB_HOST="${GOOGLE_STUB_HOST:-127.0.0.1}"
GOOGLE_STUB_PORT="${GOOGLE_STUB_PORT:-19090}"
GOOGLE_CLIENT_ID="${GOOGLE_CLIENT_ID:-smoke-google-client-id}"
GOOGLE_CLIENT_SECRET="${GOOGLE_CLIENT_SECRET:-smoke-google-client-secret}"
SKIP_BUILD="${SKIP_BUILD:-false}"
KEEP_RUNTIME="${KEEP_RUNTIME:-false}"
RUNTIME_DIR="$(mktemp -d)"
DATA_DIR="$RUNTIME_DIR/data"
SECRET_DIR="$RUNTIME_DIR/secrets"
SERVER_LOG_PATH="${SERVER_LOG_PATH:-$RUNTIME_DIR/server.log}"
GOOGLE_STUB_LOG_PATH="${GOOGLE_STUB_LOG_PATH:-$RUNTIME_DIR/google-oauth-stub.log}"
SERVER_PID=""
GOOGLE_STUB_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$GOOGLE_STUB_PID" ]]; then
    kill "$GOOGLE_STUB_PID" >/dev/null 2>&1 || true
    wait "$GOOGLE_STUB_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_RUNTIME" != "true" ]]; then
    rm -rf "$RUNTIME_DIR"
  fi
}
trap cleanup EXIT

mkdir -p "$DATA_DIR" "$SECRET_DIR"

printf "%s\n" "$JWT_SECRET_VALUE" > "$SECRET_DIR/jwt-secret"
printf "%s\n" "$DATA_DIR/crux-prod.db?_fk=1" > "$SECRET_DIR/db-dsn"
cat > "$SECRET_DIR/bootstrap-users.json" <<EOF
[
  {
    "id": "beta-user-1",
    "org_id": "beta-org",
    "org_name": "Beta Org",
    "email": "$EMAIL",
    "name": "Beta Operator",
    "role": "admin"
  }
]
EOF

JWT_SECRET_FILE_PATH="$SECRET_DIR/jwt-secret"
AUTH_BOOTSTRAP_USERS_FILE_PATH="$SECRET_DIR/bootstrap-users.json"
if [[ -n "$JWT_SECRET_FILE_OVERRIDE" ]]; then
  JWT_SECRET_FILE_PATH="$JWT_SECRET_FILE_OVERRIDE"
fi
if [[ -n "$AUTH_BOOTSTRAP_USERS_FILE_OVERRIDE" ]]; then
  AUTH_BOOTSTRAP_USERS_FILE_PATH="$AUTH_BOOTSTRAP_USERS_FILE_OVERRIDE"
fi

if [[ "$SKIP_BUILD" != "true" ]]; then
  (cd "$ROOT_DIR" && make build)
fi

python3 "$ROOT_DIR/scripts/google_oauth_stub.py" \
  --host "$GOOGLE_STUB_HOST" \
  --port "$GOOGLE_STUB_PORT" \
  --client-id "$GOOGLE_CLIENT_ID" \
  --client-secret "$GOOGLE_CLIENT_SECRET" \
  --email "$EMAIL" \
  --name "Beta Operator" >"$GOOGLE_STUB_LOG_PATH" 2>&1 &
GOOGLE_STUB_PID=$!

APP_MODE=prod \
APP_ADDR="$SERVER_ADDR" \
JWT_SECRET_FILE="$JWT_SECRET_FILE_PATH" \
DB_DSN_FILE="$SECRET_DIR/db-dsn" \
AUTH_BOOTSTRAP_USERS_FILE="$AUTH_BOOTSTRAP_USERS_FILE_PATH" \
AUTH_GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID" \
AUTH_GOOGLE_CLIENT_SECRET="$GOOGLE_CLIENT_SECRET" \
AUTH_GOOGLE_AUTH_URL="http://${GOOGLE_STUB_HOST}:${GOOGLE_STUB_PORT}/auth" \
AUTH_GOOGLE_TOKEN_URL="http://${GOOGLE_STUB_HOST}:${GOOGLE_STUB_PORT}/token" \
AUTH_GOOGLE_USERINFO_URL="http://${GOOGLE_STUB_HOST}:${GOOGLE_STUB_PORT}/userinfo" \
OPENAI_API_KEY_FILE="$OPENAI_API_KEY_FILE_OVERRIDE" \
HTTP_LOG_TO_STDOUT=true \
"$SERVER_BIN" >"$SERVER_LOG_PATH" 2>&1 &
SERVER_PID=$!

for attempt in $(seq 1 30); do
  if curl -fsS "$BASE_URL/healthz" >/dev/null && curl -fsS "$BASE_URL/readyz" >/dev/null; then
    COOKIE_JAR="$(mktemp)"
    login_status="$(curl -fsS -L -o /dev/null -w '%{http_code}' \
      -c "$COOKIE_JAR" \
      -b "$COOKIE_JAR" \
      "$BASE_URL/api/v1/auth/google/start")"
    if [[ "$login_status" != "200" ]]; then
      echo "automated Google smoke login failed with status $login_status" >&2
      cat "$SERVER_LOG_PATH" >&2
      cat "$GOOGLE_STUB_LOG_PATH" >&2
      rm -f "$COOKIE_JAR"
      exit 1
    fi
    token_response="$(curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
      -H 'Content-Type: application/json' \
      -d '{"label":"Closed beta smoke"}' \
      "$BASE_URL/api/v1/auth/cli-tokens")"
    CLI_TOKEN="$(python3 - <<'PY' "$token_response"
import json
import sys
env = json.loads(sys.argv[1])
if env.get("code") != 0:
    raise SystemExit(f"cli token issue failed: {env}")
data = env.get("data") or {}
token = data.get("token", "")
if not token:
    raise SystemExit(f"cli token missing: {env}")
print(token)
PY
)"
    rm -f "$COOKIE_JAR"
    BASE_URL="$BASE_URL" \
    CLI_BIN="$CLI_BIN" \
    EXPECT_RESEARCH_MODE="$EXPECT_RESEARCH_MODE" \
    BETA_SMOKE_CLI_TOKEN="$CLI_TOKEN" \
    "$ROOT_DIR/scripts/closed_beta_smoke.sh"
    echo "closed beta prod smoke passed"
    exit 0
  fi
  sleep 1
done

cat "$SERVER_LOG_PATH" >&2
exit 1
