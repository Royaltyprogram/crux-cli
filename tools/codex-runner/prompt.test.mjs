import assert from "node:assert/strict";
import test from "node:test";

import { buildPrompt } from "./prompt.mjs";

test("buildPrompt includes workflow and generalization guidance", async () => {
  const prompt = await buildPrompt({
    allowed_files: ["/tmp/demo/AGENTS.md"],
    steps: [
      {
        target_file: "/tmp/demo/AGENTS.md",
        operation: "append_block",
        summary: "Append research findings for the local apply agent to interpret.",
        content_preview: "## AgentOpt Research Findings\n- Verification has to be requested explicitly.",
      },
    ],
  });

  assert.match(prompt, /server-side system already approved the local change plan/i);
  assert.match(prompt, /Do not overfit to a single prompt, bug, latency spike, or token-heavy session/i);
  assert.match(prompt, /If the target is a reusable instruction file like `AGENTS\.md`/i);
  assert.match(prompt, /## Approved Files/);
  assert.match(prompt, /- \/tmp\/demo\/AGENTS\.md/);
  assert.match(prompt, /required_text:/);
  assert.match(prompt, /## AgentOpt Research Findings/);
});
