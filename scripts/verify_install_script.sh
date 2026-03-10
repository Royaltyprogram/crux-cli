#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${RELEASE_DIR:-$ROOT_DIR/output/release}"
BUNDLE_PATH="${1:-${BUNDLE:-}}"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

latest_bundle() {
  python3 - <<'PY' "$RELEASE_DIR"
import pathlib
import sys

release_dir = pathlib.Path(sys.argv[1])
archives = [p for p in release_dir.glob("agentopt-*.tar.gz") if p.is_file()]
if not archives:
    raise SystemExit("")
archives.sort(key=lambda p: p.stat().st_mtime, reverse=True)
print(archives[0])
PY
}

bundle_version() {
  local bundle_name
  bundle_name="$(basename "$1" .tar.gz)"
  python3 - <<'PY' "$bundle_name"
import sys

name = sys.argv[1]
prefix = "agentopt-"
if not name.startswith(prefix):
    raise SystemExit("")
raw = name[len(prefix):]
parts = raw.rsplit("-", 2)
if len(parts) != 3:
    raise SystemExit("")
print(parts[0])
PY
}

need_cmd python3
need_cmd tar
need_cmd sh

if [[ -z "$BUNDLE_PATH" ]]; then
  BUNDLE_PATH="$(latest_bundle)"
fi
if [[ -z "$BUNDLE_PATH" ]]; then
  echo "no beta bundle archive found" >&2
  exit 1
fi

BUNDLE_PATH="$(cd "$(dirname "$BUNDLE_PATH")" && pwd)/$(basename "$BUNDLE_PATH")"
[[ -f "$BUNDLE_PATH" ]] || {
  echo "bundle archive not found: $BUNDLE_PATH" >&2
  exit 1
}

VERSION_LABEL="${VERSION_LABEL:-$(bundle_version "$BUNDLE_PATH")}"
[[ -n "$VERSION_LABEL" ]] || {
  echo "failed to infer bundle version from $BUNDLE_PATH" >&2
  exit 1
}

CHECKSUM_PATH="$BUNDLE_PATH.sha256"
[[ -f "$CHECKSUM_PATH" ]] || {
  echo "bundle checksum not found: $CHECKSUM_PATH" >&2
  exit 1
}

TMPDIR_WORK="$(mktemp -d "${TMPDIR:-/tmp}/agentopt-install-verify.XXXXXX")"
cleanup() {
  rm -rf "$TMPDIR_WORK"
}
trap cleanup EXIT

STAGED_RELEASE_DIR="$TMPDIR_WORK/releases/$VERSION_LABEL"
INSTALL_ROOT="$TMPDIR_WORK/install-root"
BIN_DIR="$TMPDIR_WORK/bin"
mkdir -p "$STAGED_RELEASE_DIR" "$INSTALL_ROOT" "$BIN_DIR"

cp "$BUNDLE_PATH" "$STAGED_RELEASE_DIR/"
cp "$CHECKSUM_PATH" "$STAGED_RELEASE_DIR/"

AGENTOPT_VERSION="$VERSION_LABEL" \
AGENTOPT_RELEASE_BASE_URL="file://$TMPDIR_WORK/releases" \
AGENTOPT_INSTALL_ROOT="$INSTALL_ROOT" \
AGENTOPT_BIN_DIR="$BIN_DIR" \
sh "$ROOT_DIR/scripts/install.sh" >/dev/null

[[ -x "$BIN_DIR/agentopt" ]] || {
  echo "install script did not create wrapper: $BIN_DIR/agentopt" >&2
  exit 1
}

VERSION_OUTPUT="$("$BIN_DIR/agentopt" version)"
[[ "$VERSION_OUTPUT" == agentopt\ "$VERSION_LABEL"* ]] || {
  echo "unexpected version output: $VERSION_OUTPUT" >&2
  exit 1
}

[[ -L "$INSTALL_ROOT/current" ]] || {
  echo "install script did not create current symlink" >&2
  exit 1
}
[[ -f "$INSTALL_ROOT/current/tools/codex-runner/run.mjs" ]] || {
  echo "installed bundle missing codex runner" >&2
  exit 1
}

# Re-run install to verify idempotent upgrade behavior for the same version.
AGENTOPT_VERSION="$VERSION_LABEL" \
AGENTOPT_RELEASE_BASE_URL="file://$TMPDIR_WORK/releases" \
AGENTOPT_INSTALL_ROOT="$INSTALL_ROOT" \
AGENTOPT_BIN_DIR="$BIN_DIR" \
sh "$ROOT_DIR/scripts/install.sh" >/dev/null

VERSION_OUTPUT="$("$BIN_DIR/agentopt" version)"
[[ "$VERSION_OUTPUT" == agentopt\ "$VERSION_LABEL"* ]] || {
  echo "unexpected version output after reinstall: $VERSION_OUTPUT" >&2
  exit 1
}

echo "verified install script: $BUNDLE_PATH"
