# Closed Beta Runbook

## Scope

This runbook is the shortest path from local verification to a release-candidate bundle for the current closed beta build.

It assumes:

- the repo root is your working directory
- Go, `wire`, and the local toolchain are already installed
- the server runs on `http://127.0.0.1:8082` by default

For beta user machines that install the released CLI, Go is not required. Only the development-from-source flow below uses `go run`.

## 1. Local Development Run

Start the local development server:

```bash
make generate
OPENAI_API_KEY_FILE=secrets/autoskills-openai-api-key make run-local-google-stub
```

Open `http://127.0.0.1:8082/` and sign in with Google. In this local-dev path the Google round-trip is served by the local OAuth stub, so you do not need real GCP credentials.

If you want to test against a real Google app instead, use:

```bash
OPENAI_API_KEY_FILE=secrets/autoskills-openai-api-key make run
```

Required runtime auth envs for the web flow:

- `AUTH_GOOGLE_CLIENT_ID`
- `AUTH_GOOGLE_CLIENT_SECRET`

Optional:

- `AUTH_GOOGLE_ALLOWED_DOMAINS` if you want to restrict sign-in to specific email domains
- `AUTH_BOOTSTRAP_USERS_FILE` if you want matching Google emails to join a pre-seeded org or inherit fixed roles

From another shell, run the CLI against the local server. These commands are for source development:

```bash
go run ./cmd/crux setup --server http://127.0.0.1:8082 --token <CLI_TOKEN_FROM_DASHBOARD>
go run ./cmd/crux workspace
go run ./cmd/crux snapshot --file examples/config-snapshot.json
go run ./cmd/crux session
go run ./cmd/crux collect --watch --recent 1 --interval 30m
go run ./cmd/crux reports
go run ./cmd/crux audit
```

Notes:

- In the MVP, every connected repository rolls into one shared workspace per organization.
- `autoskills setup` is the shortest onboarding path and includes the initial workspace connection and first local session upload automatically.
- installed macOS beta machines also get background collection automatically when setup can register a launchd agent
- `autoskills connect` remains available when you need to reconnect a different repo manually.
- `autoskills collect --watch` keeps session and snapshot uploads flowing while the shared workspace is being observed by reacting to session file changes and using the interval as a fallback scan.
- Reports are now read-only feedback reports for the user; nothing is auto-applied.

If you want a beta machine to keep uploading usage data without repeated manual CLI runs, keep a long-lived collector running:

```bash
autoskills collect --watch --recent 1 --interval 30m
```

Notes:

- Prefer the installed `autoskills` command for long-lived beta machine setup. On supported installed macOS environments, `autoskills setup --server ...` now enrolls that background collector automatically.
- The collector stores a session cursor locally and uploads every new logical session after that cursor, so `--recent 1` only limits the initial backfill when no cursor exists yet.
- Keep `autoskills collect` without `--watch` for one-off manual uploads.

For beta users who should install from GitHub Releases instead of an unpacked bundle:

```bash
curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/autoskills-cli/main/scripts/install.sh | sh
AUTOSKILLS_VERSION=0.1.1-beta curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/autoskills-cli/main/scripts/install.sh | sh
```

The installer downloads the matching release bundle, installs it under `~/.local/share/autoskills/<version>`, and writes a wrapper to `~/.local/bin/autoskills`.
That release install uses a prebuilt binary, so Go is not required on the beta machine.

## 2. Local Verification

Run the fast confidence checks:

```bash
go test ./...
```

Run the closed beta smoke with a seeded beta user:

`make closed-beta-smoke` expects `BETA_SMOKE_CLI_TOKEN`, which you issue from the dashboard after signing in with Google.

`make closed-beta-prod-smoke` uses a local OAuth stub plus an ephemeral MySQL container and stays fully automated. It seeds a beta user, starts the server in `APP_MODE=prod`, completes a synthetic Google sign-in, issues a CLI token, and runs the CLI smoke flow without a browser.

Run the real `APP_MODE=prod` smoke locally:

```bash
make closed-beta-prod-smoke
```

Run the same smoke against the ignored local secret files under `secrets/` and require the research agent to use the live OpenAI path:

```bash
JWT_SECRET_FILE_OVERRIDE=secrets/autoskills-jwt-secret \
AUTH_BOOTSTRAP_USERS_FILE_OVERRIDE=secrets/autoskills-beta-users.json \
OPENAI_API_KEY_FILE_OVERRIDE=secrets/autoskills-openai-api-key \
EXPECT_RESEARCH_MODE=openai_responses_api \
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
- `AUTH_GOOGLE_CLIENT_ID_FILE`
- `AUTH_GOOGLE_CLIENT_SECRET_FILE`
- `AUTH_BOOTSTRAP_USERS_FILE`
- `DB_DSN_FILE` if using MySQL
- `OPENAI_API_KEY` or `OPENAI_API_KEY_FILE` for the cloud research agent
- `HTTP_ALLOWED_CIDRS`
- `HTTP_ADMIN_ALLOWED_CIDRS`
- `HTTP_TRUSTED_PROXY_CIDRS`

Supported file-based secret envs:

- `JWT_SECRET_FILE`
- `OPENAI_API_KEY_FILE`
- `DB_DSN_FILE`
- `APP_API_TOKEN_FILE`
- `AUTH_GOOGLE_CLIENT_ID_FILE`
- `AUTH_GOOGLE_CLIENT_SECRET_FILE`
- `AUTH_BOOTSTRAP_USERS_FILE`

Bootstrap users are pre-seeded Google-linked identities:

- removing a bootstrap user revokes existing tokens
- Google sign-in links to the seeded record when the email matches
- use them when you need multiple users to join the same org or want fixed `admin` or `member` roles before first sign-in

## 4. Local Prod-Like Run

Run the server in `prod` mode with secret files:

```bash
APP_MODE=prod \
JWT_SECRET_FILE=/run/secrets/autoskills-jwt-secret \
AUTH_GOOGLE_CLIENT_ID_FILE=/run/secrets/autoskills-google-client-id \
AUTH_GOOGLE_CLIENT_SECRET_FILE=/run/secrets/autoskills-google-client-secret \
AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/autoskills-beta-users.json \
OPENAI_API_KEY_FILE=/run/secrets/autoskills-openai-api-key \
go run .
```

Health endpoints:

- `GET /healthz`
- `GET /readyz`

If you want to lock access down in-app during closed beta:

```bash
APP_MODE=prod \
JWT_SECRET_FILE=/run/secrets/autoskills-jwt-secret \
AUTH_GOOGLE_CLIENT_ID_FILE=/run/secrets/autoskills-google-client-id \
AUTH_GOOGLE_CLIENT_SECRET_FILE=/run/secrets/autoskills-google-client-secret \
AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/autoskills-beta-users.json \
HTTP_ALLOWED_CIDRS='203.0.113.10/32,198.51.100.0/24' \
HTTP_ADMIN_ALLOWED_CIDRS='203.0.113.10/32' \
HTTP_TRUSTED_PROXY_CIDRS='10.0.0.0/8' \
go run .
```

## 5. Container Run

Build the container:

```bash
docker build -t autoskills-beta .
```

Run with MySQL-backed beta state override:

```bash
docker run --rm -p 8082:8082 \
  -e JWT_SECRET_FILE=/run/secrets/autoskills-jwt-secret \
  -e AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/autoskills-beta-users.json \
  -e DB_DIALECT=mysql \
  -e DB_DSN_FILE=/run/secrets/autoskills-db-dsn \
  -v "$PWD/secrets/autoskills-jwt-secret:/run/secrets/autoskills-jwt-secret:ro" \
  -v "$PWD/secrets/autoskills-beta-users.json:/run/secrets/autoskills-beta-users.json:ro" \
  -v "$PWD/secrets/autoskills-db-dsn:/run/secrets/autoskills-db-dsn:ro" \
  autoskills-beta
```

Run with MySQL-backed beta state override:

```bash
docker run --rm -p 8082:8082 \
  -e DB_DIALECT=mysql \
  -e DB_DSN_FILE=/run/secrets/autoskills-db-dsn \
  -e JWT_SECRET_FILE=/run/secrets/autoskills-jwt-secret \
  -e AUTH_BOOTSTRAP_USERS_FILE=/run/secrets/autoskills-beta-users.json \
  -v "$PWD/secrets/autoskills-db-dsn:/run/secrets/autoskills-db-dsn:ro" \
  -v "$PWD/secrets/autoskills-jwt-secret:/run/secrets/autoskills-jwt-secret:ro" \
  -v "$PWD/secrets/autoskills-beta-users.json:/run/secrets/autoskills-beta-users.json:ro" \
  autoskills-beta
```

## 6. Release Candidate Build

If your working tree is clean, build the release artifacts directly:

```bash
make beta-cli-bundle
make verify-beta-bundle
make verify-install-script
make build-release-index
VERSION_LABEL=0.1.1-beta make publish-github-release
```

Output:

- `output/release/autoskills-<version>-<os>-<arch>.tar.gz`
- `output/release/autoskills-<version>-<os>-<arch>.tar.gz.sha256`
- `output/release/autoskills-<version>-<os>-<arch>.json`
- `output/release/autoskills-<version>.release-index.json`
- `output/release/autoskills-<version>.release-index.json.sha256`

`publish-github-release` expects the `gh` CLI to be authenticated and uploads those assets to the matching GitHub Release tag.

The GitHub Actions beta workflow now updates a rolling `beta` prerelease automatically on `main` pushes and still publishes versioned releases on tag pushes.

For manual GitHub Actions runs, `workflow_dispatch` now accepts `version`, `draft`, `prerelease`, and `latest` inputs so you can build and publish the same release flow without pushing the tag first.

If your working tree is dirty but you want a clean release candidate from `HEAD`, use a temporary worktree:

```bash
tmpdir=$(mktemp -d /tmp/autoskills-release.XXXXXX)
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
3. The dashboard shows workflow feedback reports instead of approval actions.
4. `make closed-beta-prod-smoke` passes.
5. `make beta-cli-bundle`, `make verify-beta-bundle`, and `make verify-install-script` pass.
6. The release bundle contains `autoskills` and the generated `README.md`.
7. Runtime secrets are mounted from files, not hardcoded.
8. `GET /healthz` and `GET /readyz` respond successfully in the target environment.
9. A seeded beta user can log in on the dashboard and issue a CLI token.
10. A beta machine can complete `autoskills setup` and `autoskills collect`.
11. If background collection is part of the beta flow, `autoskills collect --watch --recent 1 --interval 30m` succeeds on the target machine.
