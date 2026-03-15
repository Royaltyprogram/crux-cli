# Crux CLI

Public distribution repo for the Crux CLI build artifacts and mirrored CLI source snapshots.

Install the latest GitHub release:

```bash
curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/crux-cli/main/scripts/install.sh | sh
```

Install a specific release:

```bash
CRUX_VERSION=0.1.0-beta.2-627f308 curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/crux-cli/main/scripts/install.sh | sh
```

Re-upload all local Codex sessions after clearing the saved upload cursor:

```bash
crux collect --reset-sessions
```

If one local session is rejected with `Invalid Params`, the collector now skips that session, records it in the JSON result, and continues with the rest of the backlog.

Current mirrored release artifacts:

- source repository: `Royaltyprogram/aiops`
- source commit: `627f308`
- mirrored version: `0.1.0-beta.2-627f308`
- artifact path: `artifacts/crux-0.1.0-beta.2-627f308-linux-amd64.tar.gz`

Current mirrored CLI source snapshot:

- source repository: `Royaltyprogram/aiops`
- source commit: `cc0f6db`
- mirrored paths: `source/cmd/crux/`, `source/scripts/install_local_dev.sh`
- note: the mirrored CLI source files are unchanged relative to the `627f308` release snapshot; this source snapshot is refreshed against the current `aiops` `HEAD`
