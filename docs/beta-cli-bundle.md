# AutoSkills Closed Beta CLI Bundle

Build metadata:

- Version: `__VERSION__`
- Commit: `__COMMIT__`
- Build date: `__BUILD_DATE__`

This bundle includes:

- `autoskills`

One-command install is available for GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/autoskills-cli/main/scripts/install.sh | sh
AUTOSKILLS_VERSION=0.1.1-beta curl -fsSL https://raw.githubusercontent.com/Royaltyprogram/autoskills-cli/main/scripts/install.sh | sh
```

The installer downloads the matching release bundle for the current OS and architecture, installs it under `~/.local/share/autoskills/<version>`, and writes a wrapper to `~/.local/bin/autoskills`.
The release install uses a prebuilt binary, so Go is not required.

After install, run the CLI directly:

```bash
autoskills version
autoskills setup
autoskills reports
autoskills audit
autoskills collect --reset-sessions
```

`autoskills setup` prompts for the issued CLI token if you omit `--token`, connects the current repo to the shared workspace, uploads an initial snapshot plus local Codex session history on first setup, and enrolls background collection automatically on supported installed macOS environments.
If background enrollment is not supported on the machine, setup returns the manual fallback command to run instead.
After setup, plain `autoskills` prints the current shared-workspace status.

If your shell cannot find `autoskills`, add `~/.local/bin` to `PATH`.
