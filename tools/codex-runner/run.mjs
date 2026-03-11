#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import process from "node:process";

import { Codex } from "@openai/codex-sdk";

import { buildPrompt } from "./prompt.mjs";

async function main() {
  const requestPath = process.argv[2];
  if (!requestPath) {
    throw new Error("usage: run.mjs <request.json>");
  }

  const request = JSON.parse(await readFile(requestPath, "utf8"));
  const codex = new Codex();
  const threadOptions = {
    workingDirectory: request.working_directory,
    additionalDirectories: request.additional_directories ?? [],
    sandboxMode: request.sandbox_mode ?? "workspace-write",
    approvalPolicy: request.approval_policy ?? "never",
    modelReasoningEffort: request.model_reasoning_effort || undefined,
    skipGitRepoCheck: request.skip_git_repo_check !== false,
    networkAccessEnabled: request.network_access_enabled === true,
  };
  const thread = request.resume_thread_id
    ? codex.resumeThread(request.resume_thread_id, threadOptions)
    : codex.startThread(threadOptions);

  const outputSchema = {
    type: "object",
    properties: {
      status: {
        type: "string",
        enum: ["applied", "blocked"],
      },
      summary: {
        type: "string",
      },
    },
    required: ["status", "summary"],
    additionalProperties: false,
  };

  const changedFiles = new Set();
  const executedCommands = [];
  let finalResponse = "";
  let usage = null;

  const { events } = await thread.runStreamed(await buildPrompt(request), {
    outputSchema,
  });

  for await (const event of events) {
    if (event.type === "item.completed") {
      switch (event.item.type) {
        case "agent_message":
          finalResponse = event.item.text;
          break;
        case "file_change":
          for (const change of event.item.changes ?? []) {
            if (change?.path) {
              changedFiles.add(change.path);
            }
          }
          break;
        case "command_execution":
          executedCommands.push({
            command: event.item.command,
            status: event.item.status,
          });
          break;
      }
    } else if (event.type === "turn.completed") {
      usage = event.usage;
    } else if (event.type === "turn.failed") {
      throw new Error(event.error?.message || "Codex turn failed");
    } else if (event.type === "error") {
      throw new Error(event.message || "Codex stream failed");
    }
  }

  let structured;
  try {
    structured = JSON.parse(finalResponse);
  } catch {
    structured = {
      status: "blocked",
      summary: finalResponse || "Codex did not return structured output.",
    };
  }

  process.stdout.write(
    JSON.stringify(
        {
        thread_id: thread.id ?? request.resume_thread_id ?? null,
        status: structured.status,
        summary: structured.summary,
        final_response: finalResponse,
        changed_files: Array.from(changedFiles),
        executed_commands: executedCommands,
        usage,
      },
      null,
      2,
    ),
  );
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`${message}\n`);
  process.exit(1);
});
