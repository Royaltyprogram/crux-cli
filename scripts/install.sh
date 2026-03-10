#!/bin/sh

set -eu

usage() {
  cat <<'EOF'
agentopt install script

Usage:
  sh install.sh [--version <tag>] [--install-root <dir>] [--bin-dir <dir>]

Environment overrides:
  AGENTOPT_VERSION           release tag to install (default: newest published release)
  AGENTOPT_REPO              GitHub repo in owner/name form (default: Royaltyprogram/aiops)
  AGENTOPT_RELEASE_BASE_URL  release download base URL (default: https://github.com/<repo>/releases/download)
  AGENTOPT_RELEASES_API_URL  releases API URL used when AGENTOPT_VERSION is unset
  AGENTOPT_GITHUB_TOKEN      optional token used for GitHub API requests
  AGENTOPT_INSTALL_ROOT      install root (default: $HOME/.local/share/agentopt)
  AGENTOPT_BIN_DIR           wrapper script directory (default: $HOME/.local/bin)

Examples:
  curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/aiops/main/scripts/install.sh | sh
  AGENTOPT_VERSION=0.1.0-beta.1 curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/aiops/main/scripts/install.sh | sh
EOF
}

say() {
  printf 'agentopt-install: %s\n' "$*" >&2
}

die() {
  say "$*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

json_string_field() {
  key="$1"
  file="$2"
  awk -v key="$key" '
    index($0, "\"" key "\"") {
      line = $0
      sub(/^.*"[^\"]+"[[:space:]]*:[[:space:]]*"/, "", line)
      sub(/".*$/, "", line)
      print line
      exit
    }
  ' "$file"
}

fetch_to_file() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ -n "${AGENTOPT_GITHUB_TOKEN:-}" ]; then
      curl -fsSL -H "Authorization: Bearer ${AGENTOPT_GITHUB_TOKEN}" "$url" -o "$dest"
    else
      curl -fsSL "$url" -o "$dest"
    fi
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    if [ -n "${AGENTOPT_GITHUB_TOKEN:-}" ]; then
      wget -q --header="Authorization: Bearer ${AGENTOPT_GITHUB_TOKEN}" -O "$dest" "$url"
    else
      wget -q -O "$dest" "$url"
    fi
    return
  fi
  die "curl or wget is required"
}

fetch_to_stdout() {
  url="$1"
  if command -v curl >/dev/null 2>&1; then
    if [ -n "${AGENTOPT_GITHUB_TOKEN:-}" ]; then
      curl -fsSL -H "Authorization: Bearer ${AGENTOPT_GITHUB_TOKEN}" "$url"
    else
      curl -fsSL "$url"
    fi
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    if [ -n "${AGENTOPT_GITHUB_TOKEN:-}" ]; then
      wget -q --header="Authorization: Bearer ${AGENTOPT_GITHUB_TOKEN}" -O - "$url"
    else
      wget -q -O - "$url"
    fi
    return
  fi
  die "curl or wget is required"
}

latest_version() {
  api_url="$1"
  tmpfile="$2/releases.json"
  fetch_to_file "$api_url" "$tmpfile"
  version="$(tr ',' '\n' <"$tmpfile" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [ -n "$version" ] || die "unable to determine latest release tag from $api_url"
  printf '%s\n' "$version"
}

sha256_file() {
  file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
    return
  fi
  die "sha256sum, shasum, or openssl is required"
}

extract_bundle_dir() {
  archive="$1"
  dest="$2"
  tar -xzf "$archive" -C "$dest"
  first_entry="$(tar -tzf "$archive" | head -n 1)"
  [ -n "$first_entry" ] || die "bundle archive is empty: $archive"
  first_dir="${first_entry%%/*}"
  [ -n "$first_dir" ] || die "failed to determine extracted bundle directory"
  printf '%s\n' "$dest/$first_dir"
}

create_wrapper() {
  bin_path="$1"
  target_path="$2"
  cat >"$bin_path" <<EOF
#!/bin/sh
exec "$target_path" "\$@"
EOF
  chmod 755 "$bin_path"
}

VERSION="${AGENTOPT_VERSION:-}"
INSTALL_ROOT="${AGENTOPT_INSTALL_ROOT:-$HOME/.local/share/agentopt}"
BIN_DIR="${AGENTOPT_BIN_DIR:-$HOME/.local/bin}"
REPO="${AGENTOPT_REPO:-Royaltyprogram/aiops}"
RELEASE_BASE_URL="${AGENTOPT_RELEASE_BASE_URL:-https://github.com/$REPO/releases/download}"
RELEASES_API_URL="${AGENTOPT_RELEASES_API_URL:-https://api.github.com/repos/$REPO/releases?per_page=20}"

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      [ $# -ge 2 ] || die "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --install-root)
      [ $# -ge 2 ] || die "--install-root requires a value"
      INSTALL_ROOT="$2"
      shift 2
      ;;
    --bin-dir)
      [ $# -ge 2 ] || die "--bin-dir requires a value"
      BIN_DIR="$2"
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

need_cmd uname
need_cmd tar
need_cmd mktemp
need_cmd chmod
need_cmd mkdir

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  darwin) GOOS="darwin" ;;
  linux) GOOS="linux" ;;
  *) die "unsupported operating system: $OS" ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

TMPDIR_ROOT="${TMPDIR:-/tmp}"
TMPDIR_WORK="$(mktemp -d "$TMPDIR_ROOT/agentopt-install.XXXXXX")"
cleanup() {
  rm -rf "$TMPDIR_WORK"
}
trap cleanup EXIT INT TERM

if [ -z "$VERSION" ]; then
  say "resolving latest release tag from $RELEASES_API_URL"
  VERSION="$(latest_version "$RELEASES_API_URL" "$TMPDIR_WORK")"
fi

BUNDLE_NAME="agentopt-$VERSION-$GOOS-$GOARCH"
ARCHIVE_NAME="$BUNDLE_NAME.tar.gz"
CHECKSUM_NAME="$ARCHIVE_NAME.sha256"
ARCHIVE_URL="$RELEASE_BASE_URL/$VERSION/$ARCHIVE_NAME"
CHECKSUM_URL="$RELEASE_BASE_URL/$VERSION/$CHECKSUM_NAME"
ARCHIVE_PATH="$TMPDIR_WORK/$ARCHIVE_NAME"
CHECKSUM_PATH="$TMPDIR_WORK/$CHECKSUM_NAME"

say "downloading $ARCHIVE_URL"
fetch_to_file "$ARCHIVE_URL" "$ARCHIVE_PATH"
fetch_to_file "$CHECKSUM_URL" "$CHECKSUM_PATH"

EXPECTED_SHA="$(awk '{print $1}' "$CHECKSUM_PATH")"
ACTUAL_SHA="$(sha256_file "$ARCHIVE_PATH")"
[ "$EXPECTED_SHA" = "$ACTUAL_SHA" ] || die "checksum mismatch for $ARCHIVE_NAME"

EXTRACT_DIR="$TMPDIR_WORK/extract"
mkdir -p "$EXTRACT_DIR"
BUNDLE_DIR="$(extract_bundle_dir "$ARCHIVE_PATH" "$EXTRACT_DIR")"
[ -x "$BUNDLE_DIR/agentopt" ] || die "bundle is missing agentopt executable"
[ -f "$BUNDLE_DIR/tools/codex-runner/run.mjs" ] || die "bundle is missing tools/codex-runner/run.mjs"

VERSION_DIR="$INSTALL_ROOT/$VERSION"
CURRENT_LINK="$INSTALL_ROOT/current"
BIN_PATH="$BIN_DIR/agentopt"
STAGE_DIR="$INSTALL_ROOT/.install-$VERSION.$$"

mkdir -p "$INSTALL_ROOT" "$BIN_DIR"
rm -rf "$STAGE_DIR"
cp -R "$BUNDLE_DIR" "$STAGE_DIR"
rm -rf "$VERSION_DIR"
mv "$STAGE_DIR" "$VERSION_DIR"
ln -sfn "$VERSION_DIR" "$CURRENT_LINK"
create_wrapper "$BIN_PATH" "$CURRENT_LINK/agentopt"

say "installed $VERSION to $VERSION_DIR"
say "wrapper created at $BIN_PATH"
if ! command -v agentopt >/dev/null 2>&1; then
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) say "add $BIN_DIR to PATH to run agentopt directly" ;;
  esac
fi

printf 'agentopt %s installed\n' "$VERSION"
