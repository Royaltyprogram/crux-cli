import path from "node:path";
import { readFile } from "node:fs/promises";

const applyPlanPromptTemplateURL = new URL("./prompts/apply_plan_prompt.md", import.meta.url);
const rollbackPlanPromptTemplateURL = new URL("./prompts/rollback_plan_prompt.md", import.meta.url);

export async function buildPrompt(request) {
  const templateURL =
    request.mode === "rollback" ? rollbackPlanPromptTemplateURL : applyPlanPromptTemplateURL;
  const template = await readFile(templateURL, "utf8");
  return renderPromptTemplate(template, {
    ROLLBACK_CONTEXT: buildRollbackContextBlock(request),
    APPROVED_FILES: buildApprovedFilesBlock(request.allowed_files ?? []),
    APPROVED_STEPS: buildApprovedStepsBlock(request.steps ?? []),
  });
}

export function renderPromptTemplate(template, replacements) {
  let rendered = String(template);
  for (const [key, value] of Object.entries(replacements)) {
    rendered = rendered.replaceAll(`{{${key}}}`, String(value));
  }
  return rendered;
}

export function buildApprovedFilesBlock(files) {
  if (!files.length) {
    return "- none";
  }
  return files.map((file) => `- ${file}`).join("\n");
}

export function buildApprovedStepsBlock(steps) {
  if (!steps.length) {
    return "- none";
  }

  const lines = [];
  for (const [index, step] of steps.entries()) {
    lines.push(`${index + 1}. target_file=${step.target_file}`);
    lines.push(`   operation=${step.operation || "merge_patch"}`);
    if (step.summary) {
      lines.push(`   summary=${step.summary}`);
    }
    if (isMaterializationTarget(step.target_file)) {
      lines.push("   materialization=allowed");
    }
    if (step.content_preview) {
      lines.push(isMaterializationTarget(step.target_file) ? "   contract_seed:" : "   required_text:");
      lines.push(indentBlock(step.content_preview, "     "));
    }
    if (step.settings_updates && Object.keys(step.settings_updates).length > 0) {
      lines.push("   required_json_updates:");
      lines.push(indentBlock(JSON.stringify(step.settings_updates, null, 2), "     "));
    }
  }
  return lines.join("\n");
}

function buildRollbackContextBlock(request) {
  if (request.mode !== "rollback") {
    return "";
  }

  const lines = [];
  if (request.apply_id) {
    lines.push(`- apply_id=${request.apply_id}`);
  }
  if (request.resume_thread_id) {
    lines.push(`- resume_thread_id=${request.resume_thread_id}`);
  }
  return lines.length ? lines.join("\n") : "- none";
}

function indentBlock(text, prefix) {
  return String(text)
    .replace(/\r\n/g, "\n")
    .split("\n")
    .map((line) => `${prefix}${line}`)
    .join("\n");
}

function isMaterializationTarget(target) {
  return isRepoTestTarget(target) || isAgentoptTestHarnessSkillTarget(target);
}

function isRepoTestTarget(target) {
  const normalized = normalizeTargetPath(target);
  if (!normalized) {
    return false;
  }
  const base = path.posix.basename(normalized);
  switch (true) {
    case base.endsWith("_test.go"):
    case base.endsWith(".test.ts"):
    case base.endsWith(".test.tsx"):
    case base.endsWith(".test.js"):
    case base.endsWith(".test.jsx"):
    case base.endsWith(".spec.ts"):
    case base.endsWith(".spec.tsx"):
    case base.endsWith(".spec.js"):
    case base.endsWith(".spec.jsx"):
      return true;
    case base.startsWith("test_") && base.endsWith(".py"):
      return true;
    case normalized.startsWith("tests/"):
    case normalized.startsWith("test/"):
    case normalized.startsWith("__tests__/"):
    case normalized.includes("/tests/"):
    case normalized.includes("/test/"):
    case normalized.includes("/__tests__/"):
      return true;
    default:
      return false;
  }
}

function isAgentoptTestHarnessSkillTarget(target) {
  const normalized = normalizeTargetPath(target);
  if (!normalized) {
    return false;
  }
  switch (true) {
    case normalized === ".codex/skills/agentopt-test-harness/SKILL.md":
    case normalized === "~/.codex/skills/agentopt-test-harness/SKILL.md":
    case normalized.includes("/.codex/skills/agentopt-test-harness/SKILL.md"):
      return true;
    default:
      return false;
  }
}

function normalizeTargetPath(target) {
  const trimmed = String(target ?? "").trim();
  if (!trimmed) {
    return "";
  }
  return path.posix.normalize(trimmed.replaceAll("\\", "/"));
}
