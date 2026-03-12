# Local Manual E2E

This document is the shortest manual path for testing the full local flow:

- local Codex usage data is uploaded
- the server generates recommendations
- a change plan is approved
- the local CLI pulls and applies it
- follow-up usage is uploaded
- the evaluation agent reviews the real raw-query context

## Scope

This is for a fresh local test on one machine.

Assumptions:

- repo root is the current working directory
- Python env exists at `myenv/`
- Node dependencies for the local Codex runner can be installed
- you will use a real OpenAI API key so both research and evaluation agents run
- you will create at least one real Codex session before the first upload, and at least one more after local apply

## 1. Start Clean

These commands remove local AgentOpt state, local databases, generated backups, repo-local Codex config, and ignored secret material:

```bash
rm -rf .agentopt-dev .agentopt-live-daemon .agentopt-live-test .codex
rm -f data/agentopt.db data/agentopt-local.db data/agentopt-local.db.backup-* data/agentopt-store.json.backup-*
find secrets -maxdepth 1 -type f ! -name '.gitignore' ! -name '*.example' -delete
```

If you also want a clean user-level CLI state outside the repo, remove it explicitly:

```bash
rm -rf ~/.agentopt
```

Do not remove `~/.codex/sessions` if you still need older real Codex sessions for upload.

## 2. Recreate Local Secrets

Copy the example files and fill in real values:

```bash
cp secrets/agentopt-jwt-secret.example secrets/agentopt-jwt-secret
cp secrets/agentopt-beta-users.json.example secrets/agentopt-beta-users.json
cp secrets/agentopt-openai-api-key.example secrets/agentopt-openai-api-key
```

Required contents:

- `secrets/agentopt-jwt-secret`: one strong random string
- `secrets/agentopt-beta-users.json`: at least one real beta user entry
- `secrets/agentopt-openai-api-key`: one real OpenAI API key

Minimal bootstrap user example:

```json
[
  {
    "id": "beta-user-1",
    "org_id": "beta-org",
    "org_name": "Beta Org",
    "email": "beta1@example.com",
    "name": "Beta Operator",
    "password": "replace-me"
  }
]
```

## 3. Install Local Prereqs

```bash
source myenv/bin/activate
make generate
make install-codex-runner
```

Optional runner sanity check:

```bash
make check-codex-runner
```

## 4. Start The Server

Run the app in `prod` mode so the real auth path, research agent, and evaluation agent are all active:

```bash
APP_MODE=prod \
JWT_SECRET_FILE=secrets/agentopt-jwt-secret \
AUTH_BOOTSTRAP_USERS_FILE=secrets/agentopt-beta-users.json \
OPENAI_API_KEY_FILE=secrets/agentopt-openai-api-key \
DB_DSN='data/agentopt.db?_fk=1' \
DB_DIALECT=sqlite3 \
go run .
```

Keep this shell open.

## 5. Log In On The Web

Open `http://127.0.0.1:8082/`.

Sign in with the user you put into `secrets/agentopt-beta-users.json`.

From the dashboard, issue a CLI token.

## 6. Register The Local CLI

In another shell:

```bash
source myenv/bin/activate
go run ./cmd/agentopt login --server http://127.0.0.1:8082 --token <CLI_TOKEN_FROM_DASHBOARD>
go run ./cmd/agentopt connect --repo-path .
go run ./cmd/agentopt workspace
```

## 7. Upload A Snapshot

Use a baseline snapshot first:

```bash
source myenv/bin/activate
go run ./cmd/agentopt snapshot --file examples/config-snapshot.json
```

## 8. Generate Real Local Usage Data

Before uploading anything, create a real Codex session in this repo. The session should contain real raw queries, not a hand-written summary JSON.

Good seed prompts for the first session:

- `Inspect the approval and sync flow and summarize the current behavior.`
- `Find the smallest patch that would improve the local apply flow.`
- `List the exact verification steps after the patch.`

After that session exists under `~/.codex/sessions`, upload it:

```bash
source myenv/bin/activate
go run ./cmd/agentopt collect --codex-home ~/.codex --recent 1 --snapshot-mode changed
go run ./cmd/agentopt sessions --limit 5
go run ./cmd/agentopt recommendations
```

You should now see active recommendations in the CLI and dashboard.

## 9. Approve A Change Plan

Recommended path: approve from the web dashboard.

Manual CLI path:

```bash
source myenv/bin/activate
go run ./cmd/agentopt apply --recommendation-id <RECOMMENDATION_ID>
go run ./cmd/agentopt review --apply-id <APPLY_ID> --decision approve
go run ./cmd/agentopt pending
```

If the plan was auto-approved by policy, the explicit review step is not needed.

## 10. Pull And Apply Locally

```bash
source myenv/bin/activate
go run ./cmd/agentopt preflight --apply-id <APPLY_ID>
go run ./cmd/agentopt sync
go run ./cmd/agentopt history
```

At this point the approved patch should be applied locally by the Codex runner.

## 11. Generate Post-Apply Usage

Now create one or more new real Codex sessions after the apply, in the same workspace.

Use prompts that make the changed workflow visible. For example:

- `Work with the new agent instructions and summarize the current flow.`
- `Explain whether the new configuration made repo discovery easier or harder.`
- `List the exact verification steps with the current setup.`

Then upload the recent post-apply sessions:

```bash
source myenv/bin/activate
go run ./cmd/agentopt collect --codex-home ~/.codex --recent 2 --snapshot-mode skip
go run ./cmd/agentopt experiments
go run ./cmd/agentopt impact
```

## 12. Verify Evaluation Agent Output

Expected places to inspect:

- Dashboard experiment lifecycle
- `go run ./cmd/agentopt experiments`
- `go run ./cmd/agentopt impact`

The evaluation result now includes qualitative fields such as:

- `evaluation_mode`
- `evaluation_model`
- `evaluation_decision`
- `evaluation_confidence`
- `evaluation_summary`

The final experiment decision should now come from the qualitative review of the raw queries, with numeric metrics only used as supporting context for the prompt.

## 13. Optional Rollback

If the experiment requests rollback, or you want to verify cleanup manually:

```bash
source myenv/bin/activate
go run ./cmd/agentopt pending
go run ./cmd/agentopt sync
```

Or force a direct rollback:

```bash
source myenv/bin/activate
go run ./cmd/agentopt rollback --apply-id <APPLY_ID>
```

## 14. Clean Up After The Run

```bash
rm -rf .agentopt-dev .agentopt-live-daemon .agentopt-live-test .codex
rm -f data/agentopt.db data/agentopt-local.db data/agentopt-local.db.backup-* data/agentopt-store.json.backup-*
find secrets -maxdepth 1 -type f ! -name '.gitignore' ! -name '*.example' -delete
```
