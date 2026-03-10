# AgentOpt Closed Beta CLI Bundle

Build metadata:

- Version: `__VERSION__`
- Commit: `__COMMIT__`
- Build date: `__BUILD_DATE__`

This bundle includes:

- `agentopt`
- `tools/codex-runner/run.mjs`
- the pinned Node dependencies required for local apply

One-command install is available for GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/aiops/main/scripts/install.sh | sh
AGENTOPT_VERSION=0.1.0-beta.1 curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/aiops/main/scripts/install.sh | sh
```

The installer downloads the matching release bundle for the current OS and architecture, installs it under `~/.local/share/agentopt/<version>`, and writes a wrapper to `~/.local/bin/agentopt`.

Run the bundled CLI from this directory so the local apply runner stays adjacent to the binary:

```bash
./agentopt version
./agentopt login --server http://127.0.0.1:8082
./agentopt connect --repo-path .
./agentopt snapshot
./agentopt session --recent 1
./agentopt pending
./agentopt sync
```

If you move the binary out of this bundle, also move the `tools/codex-runner` directory with it or set `AGENTOPT_CODEX_RUNNER` manually.
