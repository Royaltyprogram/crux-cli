# Closed Beta Runbook

## Scope

This runbook is the shortest path from local verification to a release-candidate bundle for the current closed beta build.

It assumes:

- the repo root is your working directory
- Go, Node, `wire`, and the local toolchain are already installed
- the server runs on `http://127.0.0.1:8082` by default

## 1. Local Development Run

Start the local development server:

```bash
make generate
make install-codex-runner
make run
```

Open `http://127.0.0.1:8082/` and sign in with the local demo account:

- email: `demo@example.com`
- password: `demo1234`

From another shell, run the CLI against the local server:

```bash
go run ./cmd/agentopt login --server http://127.0.0.1:8082 --token <CLI_TOKEN_FROM_DASHBOARD>
go run ./cmd/agentopt connect --repo-path .
go run ./cmd/agentopt workspace
go run ./cmd/agentopt snapshot --file examples/config-snapshot.json
go run ./cmd/agentopt session
go run ./cmd/agentopt collect
go run ./cmd/agentopt autoupload enable --interval 30m
go run ./cmd/agentopt autoupload status
go run ./cmd/agentopt recommendations
go run ./cmd/agentopt impact
```

Notes:

- In the MVP, every connected repository rolls into one shared workspace per organization.
- `agentopt connect` updates that shared workspace.
- `agentopt collect` uploads recent session data immediately and skips unchanged snapshots by default.
- `agentopt autoupload enable` installs a background collector on macOS via `launchd`.
- `pending`, `history`, `sync`, and `impact` all read from the same rollout stream.

If you want a beta machine to keep uploading usage data without manual CLI runs, install background uploads once:

```bash
./agentopt autoupload enable --interval 30m
./agentopt autoupload status
./agentopt autoupload disable
```

Notes:

- The current background installer targets macOS `launchd`.
- Prefer the bundled `./agentopt` binary for `autoupload`; do not rely on `go run` for long-lived beta machine setup.
- The background job runs `agentopt collect` on each interval.

## 2. Local Verification

Run the fast confidence checks:

```bash
go test ./...
make mock-e2e
```

Run the closed beta smoke with a seeded beta user:

```bash
export BETA_SMOKE_EMAIL=beta1@example.com
export BETA_SMOKE_PASSWORD=replace-me
make closed-beta-smoke
```

Run the real `APP_MODE=prod` smoke locally:

```bash
make closed-beta-prod-smoke
```

Run the full beta CI-equivalent path locally:

```bash
make ci-beta
```

## 3. Runtime Secrets

For closed beta or production, do not use the local demo path.

Preferred runtime secret inputs:

- `JWT_SECRET_FILE`
- `AUTH_BOOTSTRAP_USERS_FILE`
- `DB_DSN_FILE` if using MySQL
- `HTTP_ALLOWED_CIDRS`
- `HTTP_TRUSTED_PROXY_CIDRS`

Supported file-based secret envs:

- `JWT_SECRET_FILE`
- `DB_DSN_FILE`
- `APP_API_TOKEN_FILE`
- `AUTH_BOOTSTRAP_USERS_FILE`

Bootstrap users are managed identities:

- removing a bootstrap user revokes existing tokens
- rotating a bootstrap password revokes prior sessions

## 4. Local Prod-Like Run

Run the server in `prod` mode with secret files:

```bash
APP_MODE=prod \
JWT_SECRET_FILE=/run/secrets/agentopt-jwt-secret \
AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/agentopt-beta-users.json \
go run .
```

Health endpoints:

- `GET /healthz`
- `GET /readyz`

If you want to lock access down in-app during closed beta:

```bash
APP_MODE=prod \
JWT_SECRET_FILE=/run/secrets/agentopt-jwt-secret \
AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/agentopt-beta-users.json \
HTTP_ALLOWED_CIDRS='203.0.113.10/32,198.51.100.0/24' \
HTTP_TRUSTED_PROXY_CIDRS='10.0.0.0/8' \
go run .
```

## 5. Container Run

Build the container:

```bash
docker build -t agentopt-beta .
```

Run with SQLite-backed beta state:

```bash
docker run --rm -p 8082:8082 \
  -v "$PWD/.runtime-data:/app/data" \
  -e JWT_SECRET_FILE=/run/secrets/agentopt-jwt-secret \
  -e AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/agentopt-beta-users.json \
  -v "$PWD/secrets/agentopt-jwt-secret:/run/secrets/agentopt-jwt-secret:ro" \
  -v "$PWD/secrets/agentopt-beta-users.json:/run/secrets/agentopt-beta-users.json:ro" \
  agentopt-beta
```

Run with MySQL-backed beta state:

```bash
docker run --rm -p 8082:8082 \
  -e DB_DIALECT=mysql \
  -e DB_DSN_FILE=/run/secrets/agentopt-db-dsn \
  -e JWT_SECRET_FILE=/run/secrets/agentopt-jwt-secret \
  -e AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/agentopt-beta-users.json \
  -v "$PWD/secrets/agentopt-db-dsn:/run/secrets/agentopt-db-dsn:ro" \
  -v "$PWD/secrets/agentopt-jwt-secret:/run/secrets/agentopt-jwt-secret:ro" \
  -v "$PWD/secrets/agentopt-beta-users.json:/run/secrets/agentopt-beta-users.json:ro" \
  agentopt-beta
```

## 6. Release Candidate Build

If your working tree is clean, build the release artifacts directly:

```bash
make beta-cli-bundle
make verify-beta-bundle
make build-release-index
```

Output:

- `output/release/agentopt-<version>-<os>-<arch>.tar.gz`
- `output/release/agentopt-<version>-<os>-<arch>.tar.gz.sha256`
- `output/release/agentopt-<version>-<os>-<arch>.json`
- `output/release/agentopt-<version>.release-index.json`
- `output/release/agentopt-<version>.release-index.json.sha256`

If your working tree is dirty but you want a clean release candidate from `HEAD`, use a temporary worktree:

```bash
tmpdir=$(mktemp -d /tmp/agentopt-release.XXXXXX)
git worktree add --detach "$tmpdir" HEAD
cd "$tmpdir"
make closed-beta-prod-smoke
make beta-cli-bundle
make verify-beta-bundle
make build-release-index
```

## 7. Runtime Store Backup

Export the live runtime store:

```bash
make store-export OUTPUT=output/runtime-store-backup.json
```

Restore the live runtime store:

```bash
make store-import INPUT=output/runtime-store-backup.json
```

`store-import` is destructive by design and is gated behind `--yes` internally.

## 8. Final Pre-Deploy Checklist

Run this before handing the build to beta users:

1. `git status --short` shows only expected files.
2. `go test ./...` passes.
3. `make mock-e2e` passes.
4. `make closed-beta-prod-smoke` passes.
5. `make beta-cli-bundle` and `make verify-beta-bundle` pass.
6. The release bundle contains `agentopt` and `tools/codex-runner`.
7. Runtime secrets are mounted from files, not hardcoded.
8. `GET /healthz` and `GET /readyz` respond successfully in the target environment.
9. A seeded beta user can log in on the dashboard and issue a CLI token.
10. A beta machine can complete `agentopt login`, `agentopt connect`, and `agentopt collect`.
11. If background uploads are part of the beta flow, `agentopt autoupload enable --interval 30m` and `agentopt autoupload status` both succeed on the target macOS machine.
