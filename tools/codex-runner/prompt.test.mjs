import assert from "node:assert/strict";
import test from "node:test";

import { buildPrompt } from "./prompt.mjs";

test("buildPrompt includes workflow and generalization guidance", async () => {
  const prompt = await buildPrompt({
    allowed_files: ["/tmp/home/.codex/AGENTS.md"],
    steps: [
      {
        target_file: "/tmp/home/.codex/AGENTS.md",
        operation: "append_block",
        summary: "Append research findings for the local apply agent to interpret.",
        content_preview: "## AgentOpt Research Findings\n- Verification has to be requested explicitly.",
      },
    ],
  });

  assert.match(prompt, /server-side system already approved the local change plan/i);
  assert.match(prompt, /Do not overfit to a single prompt, bug, latency spike, or token-heavy session/i);
  assert.match(prompt, /If the target is a reusable instruction file like `~\/\.codex\/AGENTS\.md`/i);
  assert.match(prompt, /## Approved Files/);
  assert.match(prompt, /- \/tmp\/home\/\.codex\/AGENTS\.md/);
  assert.match(prompt, /required_text:/);
  assert.match(prompt, /## AgentOpt Research Findings/);
});

test("buildPrompt marks harness targets as materialization seeds", async () => {
  const prompt = await buildPrompt({
    allowed_files: [
      "/tmp/repo/internal/calculator/add_test.go",
      "/tmp/repo/.codex/skills/agentopt-test-harness/SKILL.md",
    ],
    steps: [
      {
        target_file: "/tmp/repo/internal/calculator/add_test.go",
        operation: "text_replace",
        summary: "Materialize calculator contract examples into concrete tests.",
        content_preview: "Example: 2 + 3 should equal 5.",
      },
      {
        target_file: "/tmp/repo/.codex/skills/agentopt-test-harness/SKILL.md",
        operation: "text_replace",
        summary: "Teach future sessions when to load the calculator harness.",
        content_preview: "Load the calculator regression tests when arithmetic behavior changes.",
      },
    ],
  });

  assert.match(prompt, /Harness Materialization Rules/i);
  assert.match(prompt, /materialization=allowed/);
  assert.match(prompt, /contract_seed:/);
  assert.match(prompt, /Do not run the new harness automatically during this apply/i);
});
