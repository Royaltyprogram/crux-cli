# AgentOpt Closed Beta CLI Bundle

Build metadata:

- Version: `__VERSION__`
- Commit: `__COMMIT__`
- Build date: `__BUILD_DATE__`

This bundle includes:

- `agentopt`
- `tools/codex-runner/run.mjs`
- the pinned Node dependencies required for local apply

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
