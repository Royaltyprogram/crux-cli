#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${BASE_URL:-http://127.0.0.1:8082}"
CLI_TOKEN="${BETA_SMOKE_CLI_TOKEN:-}"
CLI_BIN="${CLI_BIN:-$ROOT_DIR/output/crux}"
EXPECT_RESEARCH_MODE="${EXPECT_RESEARCH_MODE:-}"
CRUX_HOME_DIR="$(mktemp -d)"
WORKSPACE_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$CRUX_HOME_DIR" "$WORKSPACE_DIR"
}
trap cleanup EXIT

if [[ -z "$CLI_TOKEN" ]]; then
  echo "BETA_SMOKE_CLI_TOKEN must be set to a dashboard-issued CLI token." >&2
  exit 1
fi

if [[ ! -x "$CLI_BIN" ]]; then
  (cd "$ROOT_DIR" && go build -o output/crux ./cmd/crux)
fi

curl -fsS "$BASE_URL/healthz" >/dev/null
curl -fsS "$BASE_URL/readyz" >/dev/null

export CRUX_HOME="$CRUX_HOME_DIR"

"$CLI_BIN" login \
  --server "$BASE_URL" \
  --token "$CLI_TOKEN" \
  --device "closed-beta-smoke" \
  --hostname "closed-beta-smoke.local" \
  --platform "smoke/test" >/dev/null

"$CLI_BIN" connect \
  --repo-path "$WORKSPACE_DIR" >/dev/null

"$CLI_BIN" snapshot >/dev/null
"$CLI_BIN" session --file "$ROOT_DIR/examples/session-summary.json" >/dev/null
reports_output="$("$CLI_BIN" reports)"

if [[ -n "$EXPECT_RESEARCH_MODE" ]]; then
  python3 - <<'PY' "$reports_output" "$EXPECT_RESEARCH_MODE"
import json
import sys

payload = json.loads(sys.argv[1])
expected = sys.argv[2]
if isinstance(payload, dict) and "code" in payload:
    if payload.get("code") != 0:
        raise SystemExit(f"reports failed: {payload}")
    data = payload.get("data") or {}
else:
    data = payload
items = (data or {}).get("items") or []
if not items:
    raise SystemExit(f"reports missing items: {payload}")
evidence = items[0].get("evidence") or []
needle = f"generation_mode={expected}"
if needle not in evidence:
    raise SystemExit(f"expected {needle} in evidence, got {evidence}")
PY
fi

echo "closed beta smoke passed"
