#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE="${REMOTE:-origin}"
REF="${REF:-main}"
DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/agentopt}"
RELEASES_DIR="${RELEASES_DIR:-$DEPLOY_ROOT/releases}"
CURRENT_LINK="${CURRENT_LINK:-$DEPLOY_ROOT/current}"
PREVIOUS_LINK="${PREVIOUS_LINK:-$DEPLOY_ROOT/previous}"
SERVICE_NAME="${SERVICE_NAME:-agentopt}"
ENV_FILE="${ENV_FILE:-/etc/agentopt/agentopt.env}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-}"
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-30}"
GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.gopath/pkg/mod}"
GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
GO_BIN="${GO_BIN:-}"
DATA_LINK_TARGET="${DATA_LINK_TARGET:-}"
LOG_LINK_TARGET="${LOG_LINK_TARGET:-}"
SKIP_PUBLIC_CHECK="${SKIP_PUBLIC_CHECK:-false}"

TMP_ROOT=""
WORKTREE_PATH=""
PREV_RELEASE=""
NEW_RELEASE=""
TARGET_COMMIT=""
TARGET_COMMIT_SHORT=""
TARGET_VERSION=""

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  SUDO=()
else
  SUDO=(sudo)
fi

usage() {
  cat <<'EOF'
Fetch a remote git ref, build it in a temporary worktree, and deploy it in place.

Usage:
  ./scripts/deploy_remote_main.sh [options]

Options:
  --remote <name>          git remote to fetch from (default: origin)
  --ref <name>             remote branch or local ref to deploy (default: main)
  --service <name>         systemd service name (default: agentopt)
  --deploy-root <path>     deployment root containing current/releases (default: /opt/agentopt)
  --env-file <path>        env file used to infer APP_ADDR (default: /etc/agentopt/agentopt.env)
  --public-url <url>       optional public base URL for post-deploy verification
  --skip-public-check      skip public URL verification even if configured
  --health-timeout <sec>   seconds to wait for each health/readiness check (default: 30)
  --data-link-target <p>   override release data symlink target
  --log-link-target <p>    override release log symlink target
  --help                   show this help

Environment:
  GO_BIN                   override Go binary path
  GOMODCACHE               override Go module cache path
  GOCACHE                  override Go build cache path
  REMOTE / REF / PUBLIC_BASE_URL and the other defaults above can also be set via env.
EOF
}

say() {
  printf 'deploy-remote-main: %s\n' "$*" >&2
}

die() {
  say "$*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

run_root() {
  if [[ "${#SUDO[@]}" -gt 0 ]]; then
    "${SUDO[@]}" "$@"
    return
  fi
  "$@"
}

cleanup() {
  if [[ -n "$WORKTREE_PATH" && -d "$WORKTREE_PATH" ]]; then
    git -C "$ROOT_DIR" worktree remove --force "$WORKTREE_PATH" >/dev/null 2>&1 || true
  fi
  if [[ -n "$TMP_ROOT" && -d "$TMP_ROOT" ]]; then
    rm -rf "$TMP_ROOT"
  fi
}

trap cleanup EXIT

detect_go_bin() {
  if [[ -n "$GO_BIN" ]]; then
    printf '%s\n' "$GO_BIN"
    return
  fi
  if [[ -x "$ROOT_DIR/.toolchain/go/bin/go" ]]; then
    printf '%s\n' "$ROOT_DIR/.toolchain/go/bin/go"
    return
  fi
  command -v go >/dev/null 2>&1 || die "go not found and $ROOT_DIR/.toolchain/go/bin/go is missing"
  command -v go
}

env_value() {
  local file="$1"
  local key="$2"
  python3 - "$file" "$key" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
key = sys.argv[2]

if not path.exists():
    raise SystemExit(1)

for raw_line in path.read_text(encoding="utf-8").splitlines():
    line = raw_line.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    name, value = line.split("=", 1)
    if name.strip() != key:
        continue
    print(value.strip())
    raise SystemExit(0)

raise SystemExit(1)
PY
}

local_base_url() {
  local app_addr
  app_addr="$(env_value "$ENV_FILE" APP_ADDR 2>/dev/null || true)"
  app_addr="${app_addr:-127.0.0.1:8082}"

  if [[ "$app_addr" == http://* || "$app_addr" == https://* ]]; then
    printf '%s\n' "$app_addr"
    return
  fi
  if [[ "$app_addr" == :* ]]; then
    printf 'http://127.0.0.1%s\n' "$app_addr"
    return
  fi
  printf 'http://%s\n' "$app_addr"
}

wait_for_endpoint() {
  local url="$1"
  local expected_status="$2"
  local expected_version="$3"
  local expected_commit="$4"
  local deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
  local body=""

  while (( SECONDS < deadline )); do
    if body="$(curl -fsS --max-time 5 "$url" 2>/dev/null)"; then
      if python3 - "$expected_status" "$expected_version" "$expected_commit" "$body" <<'PY'
import json
import sys

expected_status = sys.argv[1]
expected_version = sys.argv[2]
expected_commit = sys.argv[3]
body = json.loads(sys.argv[4])
data = body.get("data") or {}

status = str(data.get("status") or "")
version = str(data.get("version") or "")
commit = str(data.get("commit") or "")

if status != expected_status:
    raise SystemExit(1)
if expected_version and version != expected_version:
    raise SystemExit(1)
if expected_commit and not commit.startswith(expected_commit):
    raise SystemExit(1)
PY
      then
        printf '%s\n' "$body"
        return 0
      fi
    fi
    sleep 1
  done

  return 1
}

rollback() {
  if [[ -z "$PREV_RELEASE" ]]; then
    return
  fi

  say "rolling back to $PREV_RELEASE"
  run_root ln -sfn "$PREV_RELEASE" "${CURRENT_LINK}.next"
  run_root mv -Tf "${CURRENT_LINK}.next" "$CURRENT_LINK"
  run_root ln -sfn "$PREV_RELEASE" "${PREVIOUS_LINK}.next"
  run_root mv -Tf "${PREVIOUS_LINK}.next" "$PREVIOUS_LINK"
  run_root systemctl restart "$SERVICE_NAME" || true
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote)
      [[ $# -ge 2 ]] || die "--remote requires a value"
      REMOTE="$2"
      shift 2
      ;;
    --ref)
      [[ $# -ge 2 ]] || die "--ref requires a value"
      REF="$2"
      shift 2
      ;;
    --service)
      [[ $# -ge 2 ]] || die "--service requires a value"
      SERVICE_NAME="$2"
      shift 2
      ;;
    --deploy-root)
      [[ $# -ge 2 ]] || die "--deploy-root requires a value"
      DEPLOY_ROOT="$2"
      RELEASES_DIR="$DEPLOY_ROOT/releases"
      CURRENT_LINK="$DEPLOY_ROOT/current"
      PREVIOUS_LINK="$DEPLOY_ROOT/previous"
      shift 2
      ;;
    --env-file)
      [[ $# -ge 2 ]] || die "--env-file requires a value"
      ENV_FILE="$2"
      shift 2
      ;;
    --public-url)
      [[ $# -ge 2 ]] || die "--public-url requires a value"
      PUBLIC_BASE_URL="$2"
      shift 2
      ;;
    --skip-public-check)
      SKIP_PUBLIC_CHECK="true"
      shift
      ;;
    --health-timeout)
      [[ $# -ge 2 ]] || die "--health-timeout requires a value"
      HEALTH_TIMEOUT_SECONDS="$2"
      shift 2
      ;;
    --data-link-target)
      [[ $# -ge 2 ]] || die "--data-link-target requires a value"
      DATA_LINK_TARGET="$2"
      shift 2
      ;;
    --log-link-target)
      [[ $# -ge 2 ]] || die "--log-link-target requires a value"
      LOG_LINK_TARGET="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

need_cmd git
need_cmd curl
need_cmd python3
need_cmd systemctl

GO_BIN="$(detect_go_bin)"
LOCAL_BASE_URL="$(local_base_url)"

say "fetching $REMOTE"
git -C "$ROOT_DIR" fetch "$REMOTE" --tags --force >/dev/null

TARGET_COMMIT="$(git -C "$ROOT_DIR" rev-parse -q --verify "refs/remotes/$REMOTE/$REF^{commit}" 2>/dev/null || true)"
if [[ -z "$TARGET_COMMIT" ]]; then
  TARGET_COMMIT="$(git -C "$ROOT_DIR" rev-parse -q --verify "$REF^{commit}" 2>/dev/null || true)"
fi
[[ -n "$TARGET_COMMIT" ]] || die "unable to resolve deploy target from remote=$REMOTE ref=$REF"

TARGET_COMMIT_SHORT="$(git -C "$ROOT_DIR" rev-parse --short "$TARGET_COMMIT")"
TARGET_VERSION="$(git -C "$ROOT_DIR" describe --tags --always "$TARGET_COMMIT" 2>/dev/null || printf '%s\n' "$TARGET_COMMIT_SHORT")"
say "selected commit $TARGET_COMMIT_SHORT ($TARGET_VERSION)"

current_health="$(curl -fsS --max-time 5 "$LOCAL_BASE_URL/healthz" 2>/dev/null || true)"
current_ready="$(curl -fsS --max-time 5 "$LOCAL_BASE_URL/readyz" 2>/dev/null || true)"
if [[ -n "$current_health" && -n "$current_ready" ]]; then
  if python3 - "$TARGET_COMMIT_SHORT" "$current_health" "$current_ready" <<'PY'
import json
import sys

target_commit = sys.argv[1]
health = json.loads(sys.argv[2]).get("data") or {}
ready = json.loads(sys.argv[3]).get("data") or {}

if health.get("status") != "ok":
    raise SystemExit(1)
if ready.get("status") != "ready":
    raise SystemExit(1)
if not str(health.get("commit") or "").startswith(target_commit):
    raise SystemExit(1)
if not str(ready.get("commit") or "").startswith(target_commit):
    raise SystemExit(1)
PY
  then
    say "already running commit $TARGET_COMMIT_SHORT; nothing to do"
    exit 0
  fi
fi

[[ -L "$CURRENT_LINK" ]] || die "current deploy link not found: $CURRENT_LINK"
PREV_RELEASE="$(readlink -f "$CURRENT_LINK")"
[[ -d "$PREV_RELEASE" ]] || die "current release path does not exist: $PREV_RELEASE"

if [[ -z "$DATA_LINK_TARGET" ]]; then
  if [[ -L "$PREV_RELEASE/data" ]]; then
    DATA_LINK_TARGET="$(readlink "$PREV_RELEASE/data")"
  else
    DATA_LINK_TARGET="/var/lib/agentopt"
  fi
fi
if [[ -z "$LOG_LINK_TARGET" ]]; then
  if [[ -L "$PREV_RELEASE/log" ]]; then
    LOG_LINK_TARGET="$(readlink "$PREV_RELEASE/log")"
  else
    LOG_LINK_TARGET="/var/log/agentopt"
  fi
fi

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/aiops-deploy.XXXXXX")"
WORKTREE_PATH="$TMP_ROOT/worktree"

say "preparing worktree at $WORKTREE_PATH"
git -C "$ROOT_DIR" worktree add --detach "$WORKTREE_PATH" "$TARGET_COMMIT" >/dev/null

mkdir -p "$GOMODCACHE" "$GOCACHE" "$WORKTREE_PATH/output"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

say "building server binary"
(
  cd "$WORKTREE_PATH"
  PATH="$(dirname "$GO_BIN"):$PATH" \
    GOMODCACHE="$GOMODCACHE" \
    GOCACHE="$GOCACHE" \
    "$GO_BIN" generate ./data
  PATH="$(dirname "$GO_BIN"):$PATH" \
    GOMODCACHE="$GOMODCACHE" \
    GOCACHE="$GOCACHE" \
    "$GO_BIN" tool wire gen wire.go
  PATH="$(dirname "$GO_BIN"):$PATH" \
    GOMODCACHE="$GOMODCACHE" \
    GOCACHE="$GOCACHE" \
    "$GO_BIN" build \
      -ldflags "-X github.com/Royaltyprogram/aiops/pkg/buildinfo.Version=$TARGET_VERSION -X github.com/Royaltyprogram/aiops/pkg/buildinfo.Commit=$TARGET_COMMIT_SHORT -X github.com/Royaltyprogram/aiops/pkg/buildinfo.Date=$BUILD_DATE" \
      -o output/server \
      main.go wire_gen.go
)

timestamp="$(date +%Y%m%d%H%M%S)"
NEW_RELEASE="$RELEASES_DIR/$timestamp"

say "staging release into $NEW_RELEASE"
run_root install -d -m 755 "$RELEASES_DIR" "$NEW_RELEASE"
run_root cp -a "$WORKTREE_PATH/configs" "$NEW_RELEASE/"
run_root install -m 755 "$WORKTREE_PATH/output/server" "$NEW_RELEASE/server"
run_root ln -s "$DATA_LINK_TARGET" "$NEW_RELEASE/data"
run_root ln -s "$LOG_LINK_TARGET" "$NEW_RELEASE/log"
run_root ln -sfn "$PREV_RELEASE" "${PREVIOUS_LINK}.next"
run_root mv -Tf "${PREVIOUS_LINK}.next" "$PREVIOUS_LINK"
run_root ln -sfn "$NEW_RELEASE" "${CURRENT_LINK}.next"
run_root mv -Tf "${CURRENT_LINK}.next" "$CURRENT_LINK"

say "restarting $SERVICE_NAME"
if ! run_root systemctl restart "$SERVICE_NAME"; then
  rollback
  die "failed to restart $SERVICE_NAME after switching to $NEW_RELEASE"
fi

say "verifying local health endpoints at $LOCAL_BASE_URL"
if ! wait_for_endpoint "$LOCAL_BASE_URL/healthz" "ok" "$TARGET_VERSION" "$TARGET_COMMIT_SHORT" >/dev/null; then
  rollback
  die "local /healthz verification failed after deploying $TARGET_VERSION"
fi
if ! wait_for_endpoint "$LOCAL_BASE_URL/readyz" "ready" "$TARGET_VERSION" "$TARGET_COMMIT_SHORT" >/dev/null; then
  rollback
  die "local /readyz verification failed after deploying $TARGET_VERSION"
fi

if [[ "$SKIP_PUBLIC_CHECK" != "true" && -n "$PUBLIC_BASE_URL" ]]; then
  say "verifying public health endpoints at $PUBLIC_BASE_URL"
  if ! wait_for_endpoint "$PUBLIC_BASE_URL/healthz" "ok" "$TARGET_VERSION" "$TARGET_COMMIT_SHORT" >/dev/null; then
    rollback
    die "public /healthz verification failed after deploying $TARGET_VERSION"
  fi
  if ! wait_for_endpoint "$PUBLIC_BASE_URL/readyz" "ready" "$TARGET_VERSION" "$TARGET_COMMIT_SHORT" >/dev/null; then
    rollback
    die "public /readyz verification failed after deploying $TARGET_VERSION"
  fi
fi

say "deployed $TARGET_VERSION ($TARGET_COMMIT_SHORT) to $NEW_RELEASE"
