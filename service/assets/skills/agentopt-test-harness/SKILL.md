---
name: agentopt-test-harness
description: Use when AgentOpt has suggested a reusable harness and the local coding agent should turn that abstract contract into concrete repo-local tests and reusable skill instructions.
---

# AgentOpt Test Harness

Use this skill when AgentOpt has proposed a reusable harness for the repository and the local coding agent needs to materialize it into concrete test assets.

## Workflow

1. Read the relevant AgentOpt harness suggestion under `.agentopt/harness/` and identify the intended behavior contract, representative examples, and anti-goals.
2. Map that abstract contract onto the repository's native test conventions by creating or updating the smallest concrete test assets that fit the stack.
3. Update this repo-local skill so future tasks know when those tests should be loaded and run.
4. Keep the harness description abstract and reusable; put concrete implementation details in the test files, not in the suggestion itself.
5. During an AgentOpt apply handoff, materialize the files but do not auto-run the tests.
6. Only run the concrete tests when the current task actually touches the covered behavior or when the user asks for verification.

## Guardrails

- Do not invent product requirements that are not implied by the AgentOpt suggestion or the user's repeated requests.
- Do not broaden the patch beyond the minimal concrete tests and skill instructions needed to capture the reusable contract.
- Treat the AgentOpt harness JSON as an abstract contract seed; the executable truth should live in the repo's native test files.
