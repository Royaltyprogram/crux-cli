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

The installer downloads the matching release bundle for the current OS and architecture, installs it under `~/.local/share/agentopt/<version>`, writes a wrapper to `~/.local/bin/agentopt`, and provisions a local Node.js runtime automatically when the machine does not already have a compatible one.
The release install uses a prebuilt binary, so Go is not required.

After install, run the CLI directly:

```bash
agentopt version
agentopt login --server http://127.0.0.1:8082
agentopt connect --repo-path .
agentopt snapshot
agentopt session --recent 1
agentopt pending
agentopt sync
```

If your shell cannot find `agentopt`, add `~/.local/bin` to `PATH`. If you bypass the installer and run the unpacked bundle directly, local sync/apply still require Node.js 18+ because the bundled runner is executed with `node`.
